package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// ReuseGrace é a tolerância para o mesmo refresh token ser apresentado duas
// vezes quase ao mesmo tempo (duas abas do navegador renovando em paralelo).
// Passado esse prazo, reapresentar um token já usado é tratado como roubo.
const ReuseGrace = 30 * time.Second

// Session representa um refresh token emitido para um usuário.
//
// Dois tokens participam da autenticação:
//   - access token (JWT): vida curta (minutos), validado sem ir ao banco;
//   - refresh token: opaco, vida longa (dias), serve só para obter um novo
//     access token. É este que fica registrado aqui.
//
// A cada renovação o refresh token é TROCADO por um novo (rotação). Todos os
// tokens nascidos do mesmo login compartilham a FamilyID. Se um token já
// usado reaparecer, alguém o copiou: a família inteira é revogada e tanto o
// ladrão quanto o usuário legítimo precisam fazer login de novo.
type Session struct {
	ID       string
	UserID   string
	FamilyID string
	// TokenHash é o SHA-256 do refresh token. O token em si nunca é gravado:
	// um vazamento do banco não entrega sessões válidas.
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// NewSession cria uma sessão e devolve também o refresh token em texto puro
// (que só existe neste momento, para ser entregue ao cliente).
func NewSession(userID, familyID string, ttl time.Duration) (*Session, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("gerando refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if familyID == "" {
		familyID = kernel.NewID()
	}
	now := time.Now().UTC()
	return &Session{
		ID:        kernel.NewID(),
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: HashToken(token),
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}, token, nil
}

// HashToken devolve o SHA-256 (hex) de um refresh token.
//
// Aqui um hash rápido é suficiente (diferente das senhas): o token tem 256
// bits de entropia aleatória, impossível de adivinhar por força bruta.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RefreshOutcome é a decisão sobre uma tentativa de renovação.
type RefreshOutcome int

const (
	RefreshAllowed RefreshOutcome = iota // pode renovar
	RefreshDenied                        // token expirado ou revogado
	RefreshReused                        // token já usado fora da tolerância: possível roubo
)

// EvaluateRefresh decide se a sessão pode ser renovada agora.
func (s *Session) EvaluateRefresh(now time.Time) RefreshOutcome {
	if s.RevokedAt != nil || !now.Before(s.ExpiresAt) {
		return RefreshDenied
	}
	if s.UsedAt != nil && now.Sub(*s.UsedAt) > ReuseGrace {
		return RefreshReused
	}
	return RefreshAllowed
}
