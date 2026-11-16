package auth

import (
	"net/http"
	"strings"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
)

// Guard protege rotas HTTP.
type Guard struct {
	verifier TokenVerifier
}

// NewGuard cria o protetor de rotas.
func NewGuard(verifier TokenVerifier) *Guard { return &Guard{verifier: verifier} }

// Authenticated exige um token válido e coloca o usuário no contexto.
func (g *Guard) Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := BearerToken(r)
		if token == "" {
			httpx.Error(w, r, kernel.Unauthorized("auth.missing_token", "autenticação necessária"))
			return
		}
		principal, err := g.verifier.Verify(r.Context(), token)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(access.WithPrincipal(r.Context(), principal)))
	})
}

// Optional coloca o usuário no contexto SE houver um token válido, mas deixa
// a requisição seguir mesmo sem ele. Serve a rotas que funcionam nos dois
// casos — como o logout, que precisa conseguir encerrar uma sessão expirada.
func (g *Guard) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := BearerToken(r); token != "" {
			if principal, err := g.verifier.Verify(r.Context(), token); err == nil {
				r = r.WithContext(access.WithPrincipal(r.Context(), principal))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Require exige um token válido E a permissão informada.
//
//	mux.Handle("POST /v1/reservations", guard.Require(access.PermReservationWrite, handler))
func (g *Guard) Require(permission access.Permission, next http.Handler) http.Handler {
	return g.Authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := access.FromContext(r.Context())
		if !principal.Can(permission) {
			httpx.Error(w, r, kernel.Forbidden("auth.forbidden", "você não tem permissão para esta operação"))
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// BearerToken extrai o token do cabeçalho "Authorization: Bearer <token>".
func BearerToken(r *http.Request) string {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
