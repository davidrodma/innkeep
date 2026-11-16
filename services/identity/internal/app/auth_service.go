package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/identity/internal/domain"
)

// Tokens é o resultado de um login ou de uma renovação.
type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             *domain.User
}

// AuthService reúne os casos de uso de autenticação.
type AuthService struct {
	users      UserRepository
	sessions   SessionRepository
	hasher     PasswordHasher
	issuer     TokenIssuer
	throttle   LoginThrottle
	denylist   TokenDenylist
	accessTTL  time.Duration
	refreshTTL time.Duration
	// dummyHash é comparado quando o e-mail não existe, para que a resposta
	// demore o mesmo tempo em ambos os casos (veja Login).
	dummyHash string
}

// NewAuthService monta o serviço de autenticação.
func NewAuthService(users UserRepository, sessions SessionRepository, hasher PasswordHasher, issuer TokenIssuer,
	throttle LoginThrottle, denylist TokenDenylist, accessTTL, refreshTTL time.Duration) (*AuthService, error) {
	dummyHash, err := hasher.Hash("innkeep-dummy-password")
	if err != nil {
		return nil, err
	}
	return &AuthService{
		users: users, sessions: sessions, hasher: hasher, issuer: issuer,
		throttle: throttle, denylist: denylist,
		accessTTL: accessTTL, refreshTTL: refreshTTL, dummyHash: dummyHash,
	}, nil
}

// Login autentica por e-mail e senha.
//
// Cuidados de segurança neste fluxo:
//   - Mensagem única para "e-mail não existe" e "senha errada": a API não
//     revela quais e-mails estão cadastrados (enumeração de usuários).
//   - Tempo constante: quando o e-mail não existe, ainda assim calculamos um
//     hash, senão a diferença de tempo de resposta entregaria a informação.
//   - Bloqueio temporário após tentativas seguidas com erro (força bruta).
func (s *AuthService) Login(ctx context.Context, email, password string) (Tokens, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return Tokens{}, domain.ErrInvalidCredentials
	}

	blocked, retryAfter, err := s.throttle.Blocked(ctx, email)
	if err != nil {
		// Redis fora do ar: o login continua funcionando, sem o bloqueio.
		slog.WarnContext(ctx, "controle de tentativas de login indisponível", slog.Any("erro", err))
	}
	if blocked {
		return Tokens{}, &kernel.Error{
			Kind:    kernel.KindRateLimited,
			Code:    "auth.too_many_attempts",
			Message: "muitas tentativas de login; aguarde " + retryAfter.Round(time.Minute).String() + " e tente novamente",
		}
	}

	user, err := s.users.ByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return Tokens{}, err
	}

	hash := s.dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	matches, err := s.hasher.Verify(password, hash)
	if err != nil {
		return Tokens{}, err
	}
	if user == nil || !matches || !user.Active {
		if err := s.throttle.RegisterFailure(ctx, email); err != nil {
			slog.WarnContext(ctx, "falha ao registrar tentativa de login", slog.Any("erro", err))
		}
		return Tokens{}, domain.ErrInvalidCredentials
	}

	if err := s.throttle.Reset(ctx, email); err != nil {
		slog.WarnContext(ctx, "falha ao zerar tentativas de login", slog.Any("erro", err))
	}
	now := time.Now().UTC()
	if err := s.users.TouchLogin(ctx, user.ID, now); err != nil {
		return Tokens{}, err
	}
	slog.InfoContext(ctx, "login realizado", slog.String("user_id", user.ID))
	return s.issue(ctx, user, "")
}

// Refresh troca um refresh token válido por um novo par de tokens (rotação).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	if refreshToken == "" {
		return Tokens{}, domain.ErrInvalidRefresh
	}
	session, err := s.sessions.ByTokenHash(ctx, domain.HashToken(refreshToken))
	if err != nil {
		return Tokens{}, err
	}
	now := time.Now().UTC()

	switch session.EvaluateRefresh(now) {
	case domain.RefreshDenied:
		return Tokens{}, domain.ErrInvalidRefresh
	case domain.RefreshReused:
		// Um token já trocado reapareceu: ou o cliente tem um bug, ou o token
		// foi roubado. Na dúvida, encerramos todas as sessões desse login.
		slog.WarnContext(ctx, "reuso de refresh token detectado; família revogada",
			slog.String("user_id", session.UserID), slog.String("family_id", session.FamilyID))
		if err := s.sessions.RevokeFamily(ctx, session.FamilyID, now); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, domain.ErrInvalidRefresh
	}

	user, err := s.users.ByID(ctx, session.UserID)
	if err != nil || !user.Active {
		_ = s.sessions.RevokeFamily(ctx, session.FamilyID, now)
		return Tokens{}, domain.ErrInvalidRefresh
	}
	if session.UsedAt == nil {
		if err := s.sessions.MarkUsed(ctx, session.ID, now); err != nil {
			return Tokens{}, err
		}
	}
	return s.issue(ctx, user, session.FamilyID)
}

// Logout encerra a sessão: revoga a família do refresh token e coloca o token
// de acesso atual na lista de revogados até ele expirar.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	now := time.Now().UTC()
	if refreshToken != "" {
		session, err := s.sessions.ByTokenHash(ctx, domain.HashToken(refreshToken))
		if err == nil {
			if err := s.sessions.RevokeFamily(ctx, session.FamilyID, now); err != nil {
				return err
			}
		} else if !errors.Is(err, domain.ErrInvalidRefresh) {
			return err
		}
	}
	if principal, ok := access.FromContext(ctx); ok && principal.TokenID != "" {
		// Não sabemos a expiração exata do token aqui, mas ela nunca passa de
		// "agora + TTL": guardar por esse prazo é suficiente.
		if err := s.denylist.Revoke(ctx, principal.TokenID, now.Add(s.accessTTL)); err != nil {
			slog.WarnContext(ctx, "falha ao revogar token de acesso", slog.Any("erro", err))
		}
	}
	return nil
}

// ChangePassword troca a senha do próprio usuário, exigindo a senha atual, e
// encerra todas as outras sessões.
func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	matches, err := s.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil {
		return err
	}
	if !matches {
		return kernel.Invalid("auth.wrong_current_password", "a senha atual não confere").
			WithField("current_password", "senha incorreta")
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.sessions.RevokeAllForUser(ctx, userID, time.Now().UTC())
}

// PurgeExpiredSessions remove sessões vencidas (tarefa periódica).
func (s *AuthService) PurgeExpiredSessions(ctx context.Context) error {
	// Mantemos por mais um dia após o vencimento para a detecção de reuso.
	_, err := s.sessions.DeleteExpired(ctx, time.Now().UTC().Add(-24*time.Hour))
	return err
}

func (s *AuthService) issue(ctx context.Context, user *domain.User, familyID string) (Tokens, error) {
	accessToken, accessExpiresAt, err := s.issuer.Sign(user.Principal(), s.accessTTL)
	if err != nil {
		return Tokens{}, err
	}
	session, refreshToken, err := domain.NewSession(user.ID, familyID, s.refreshTTL)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return Tokens{}, err
	}
	return Tokens{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: session.ExpiresAt,
		User:             user,
	}, nil
}
