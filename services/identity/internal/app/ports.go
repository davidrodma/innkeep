// Package app é a camada de aplicação do serviço identity: os casos de uso.
//
// Na arquitetura hexagonal, este pacote define as PORTAS — interfaces que
// descrevem do que a aplicação precisa do mundo externo (banco, Redis,
// criptografia). Os ADAPTADORES, em internal/adapter, as implementam. A
// dependência aponta sempre para dentro: os adaptadores conhecem a aplicação;
// a aplicação não conhece os adaptadores (Inversão de Dependência, o "D" do
// SOLID). É por isso que os casos de uso são testáveis com dublês em memória.
package app

import (
	"context"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/identity/internal/domain"
)

// UserRepository persiste usuários.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	ByID(ctx context.Context, id string) (*domain.User, error)
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Count(ctx context.Context) (int, error)
	CountActiveAdmins(ctx context.Context) (int, error)
	TouchLogin(ctx context.Context, userID string, at time.Time) error
}

// SessionRepository persiste sessões (refresh tokens).
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	ByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error)
	MarkUsed(ctx context.Context, sessionID string, at time.Time) error
	RevokeFamily(ctx context.Context, familyID string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID string, at time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// PasswordHasher transforma senhas em hashes e as confere.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encodedHash string) (bool, error)
}

// TokenIssuer emite tokens de acesso (JWT).
type TokenIssuer interface {
	Sign(principal access.Principal, ttl time.Duration) (token string, expiresAt time.Time, err error)
}

// LoginThrottle limita tentativas de login malsucedidas (força bruta).
type LoginThrottle interface {
	// Blocked informa se a chave está bloqueada e por quanto tempo.
	Blocked(ctx context.Context, key string) (bool, time.Duration, error)
	RegisterFailure(ctx context.Context, key string) error
	Reset(ctx context.Context, key string) error
}

// TokenDenylist guarda tokens de acesso revogados antes de expirarem.
type TokenDenylist interface {
	Revoke(ctx context.Context, tokenID string, until time.Time) error
}
