package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
)

// claims é o conteúdo do JWT. Guardamos só o necessário para autorizar: um
// JWT é apenas codificado em base64 (não criptografado) — qualquer pessoa com
// o token lê o conteúdo. Nada sensível deve ir aqui.
type claims struct {
	jwt.RegisteredClaims
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Properties []string `json:"props,omitempty"`
}

var errInvalidToken = kernel.Unauthorized("auth.invalid_token", "token de acesso inválido ou expirado")

// Signer emite tokens. Usado exclusivamente pelo serviço identity.
type Signer struct {
	privateKey ed25519.PrivateKey
	keyID      string
	issuer     string
}

// NewSigner cria o emissor a partir da chave privada Ed25519.
//
// Por que assinatura assimétrica (EdDSA) e não HMAC com segredo compartilhado
// (HS256)? Com HMAC, todo serviço que VALIDA tokens também consegue EMITIR
// tokens — um serviço comprometido falsifica um administrador. Com chave
// assimétrica, só o identity (dono da chave privada) emite; os demais apenas
// conferem com a chave pública.
func NewSigner(privateKey ed25519.PrivateKey, issuer string) *Signer {
	public := privateKey.Public().(ed25519.PublicKey)
	return &Signer{privateKey: privateKey, keyID: KeyID(public), issuer: issuer}
}

// Sign emite um token de acesso para o usuário.
func (s *Signer) Sign(principal access.Principal, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	now := time.Now().UTC()
	expiresAt = now.Add(ttl)
	if principal.TokenID == "" {
		principal.TokenID = kernel.NewID()
	}

	unsigned := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   principal.UserID,
			ID:        principal.TokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Email:      principal.Email,
		Name:       principal.Name,
		Role:       string(principal.Role),
		Properties: principal.PropertyIDs,
	})
	unsigned.Header["kid"] = s.keyID

	token, err = unsigned.SignedString(s.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: assinando token: %w", err)
	}
	return token, expiresAt, nil
}

// PublicJWKS devolve o documento JWKS (JSON Web Key Set, RFC 7517) com a
// chave pública, para ser servido em /.well-known/jwks.json.
func (s *Signer) PublicJWKS() JWKS {
	public := s.privateKey.Public().(ed25519.PublicKey)
	return JWKS{Keys: []JWK{{
		KeyType:   "OKP",
		Curve:     "Ed25519",
		KeyID:     s.keyID,
		Use:       "sig",
		Algorithm: "EdDSA",
		X:         base64.RawURLEncoding.EncodeToString(public),
	}}}
}

// JWKS é um conjunto de chaves públicas.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK é uma chave pública no formato JSON Web Key.
type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	X         string `json:"x"`
}

// KeyID deriva um identificador estável da chave pública (hash truncado).
// O "kid" vai no cabeçalho do token e permite ROTACIONAR chaves: durante a
// troca, o JWKS publica a chave antiga e a nova, e cada token indica qual usar.
func KeyID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

// TokenVerifier valida um token e devolve o usuário autenticado.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (access.Principal, error)
}

// Verifier valida tokens usando as chaves públicas do JWKS do identity.
type Verifier struct {
	jwksURL string
	issuer  string
	client  *http.Client

	mu        sync.RWMutex
	keys      map[string]ed25519.PublicKey
	lastFetch time.Time
}

// NewVerifier cria o validador. As chaves são buscadas sob demanda (o identity
// pode ainda estar subindo quando este serviço inicia) e ficam em memória.
func NewVerifier(jwksURL, issuer string) *Verifier {
	return &Verifier{
		jwksURL: jwksURL,
		issuer:  issuer,
		client:  &http.Client{Timeout: 5 * time.Second},
		keys:    make(map[string]ed25519.PublicKey),
	}
}

// NewStaticVerifier cria um validador com chaves fixas (testes).
func NewStaticVerifier(issuer string, keys ...ed25519.PublicKey) *Verifier {
	verifier := &Verifier{issuer: issuer, keys: make(map[string]ed25519.PublicKey)}
	for _, key := range keys {
		verifier.keys[KeyID(key)] = key
	}
	return verifier
}

// Verify valida assinatura, emissor e validade do token.
func (v *Verifier) Verify(ctx context.Context, token string) (access.Principal, error) {
	var parsed claims
	_, err := jwt.ParseWithClaims(token, &parsed,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return v.publicKey(ctx, kid)
		},
		// Aceitar SOMENTE EdDSA fecha a porta para ataques de confusão de
		// algoritmo (ex.: token com "alg": "none" ou HS256 usando a chave
		// pública como segredo).
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(15*time.Second),
	)
	if err != nil {
		var domainErr *kernel.Error
		if errors.As(err, &domainErr) && domainErr.Kind == kernel.KindUnavailable {
			return access.Principal{}, domainErr
		}
		return access.Principal{}, errInvalidToken.Wrap(err)
	}

	role := access.Role(parsed.Role)
	if !access.ValidRole(role) || parsed.Subject == "" {
		return access.Principal{}, errInvalidToken
	}
	return access.Principal{
		UserID:      parsed.Subject,
		Email:       parsed.Email,
		Name:        parsed.Name,
		Role:        role,
		PropertyIDs: parsed.Properties,
		TokenID:     parsed.ID,
	}, nil
}

// publicKey devolve a chave do `kid`, recarregando o JWKS se ela for
// desconhecida (rotação de chaves). O intervalo mínimo entre recargas impede
// que tokens com `kid` inventado virem um ataque de negação de serviço contra
// o identity.
func (v *Verifier) publicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}
	if v.jwksURL == "" {
		return nil, errInvalidToken
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if key, ok := v.keys[kid]; ok { // outra goroutine pode ter recarregado enquanto esperávamos o lock
		return key, nil
	}
	if time.Since(v.lastFetch) < 5*time.Second {
		return nil, errInvalidToken
	}
	v.lastFetch = time.Now()

	keys, err := v.fetch(ctx)
	if err != nil {
		return nil, kernel.Unavailable("auth.jwks_unavailable", "serviço de autenticação indisponível").Wrap(err)
	}
	v.keys = keys
	if key, ok := keys[kid]; ok {
		return key, nil
	}
	return nil, errInvalidToken
}

func (v *Verifier) fetch(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	// A URL do JWKS vem da configuração do serviço (AUTH_JWKS_URL), nunca de
	// uma requisição: não é um vetor de SSRF.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil) //nolint:gosec // G704: ver o comentário acima
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req) //nolint:gosec // G704: ver o comentário acima
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS respondeu %d", resp.StatusCode)
	}

	var document JWKS
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("JWKS inválido: %w", err)
	}
	keys := make(map[string]ed25519.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		if jwk.KeyType != "OKP" || jwk.Curve != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		keys[jwk.KeyID] = ed25519.PublicKey(raw)
	}
	return keys, nil
}
