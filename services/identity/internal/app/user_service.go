package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/identity/internal/domain"
)

// UserService reúne os casos de uso de gestão de usuários.
type UserService struct {
	users    UserRepository
	sessions SessionRepository
	hasher   PasswordHasher
}

// NewUserService monta o serviço de usuários.
func NewUserService(users UserRepository, sessions SessionRepository, hasher PasswordHasher) *UserService {
	return &UserService{users: users, sessions: sessions, hasher: hasher}
}

// CreateUserInput são os dados para criar um usuário.
type CreateUserInput struct {
	Email       string
	Name        string
	Password    string
	Role        access.Role
	PropertyIDs []string
}

// Create cadastra um novo usuário.
func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	if err := domain.ValidatePassword(input.Password); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return nil, err
	}
	user, err := domain.NewUser(input.Email, input.Name, input.Role, input.PropertyIDs, hash)
	if err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "usuário criado", slog.String("user_id", user.ID), slog.String("role", string(user.Role)))
	return user, nil
}

// UpdateUserInput são os dados alteráveis de um usuário.
type UpdateUserInput struct {
	Name        string
	Role        access.Role
	PropertyIDs []string
	Active      bool
}

// Update altera um usuário. Protege a regra "sempre deve existir ao menos um
// administrador ativo" — sem ela, um clique errado trancaria todos para fora.
func (s *UserService) Update(ctx context.Context, id string, input UpdateUserInput) (*domain.User, error) {
	user, err := s.users.ByID(ctx, id)
	if err != nil {
		return nil, err
	}

	losingAdmin := user.Role == access.RoleAdmin && user.Active &&
		(input.Role != access.RoleAdmin || !input.Active)
	if losingAdmin {
		admins, err := s.users.CountActiveAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if admins <= 1 {
			return nil, domain.ErrLastAdmin
		}
	}

	wasActive := user.Active
	if err := user.Update(input.Name, input.Role, input.PropertyIDs, input.Active); err != nil {
		return nil, err
	}
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	if wasActive && !user.Active {
		// Usuário desativado perde o acesso imediatamente (dentro do TTL do
		// token de acesso, que é curto).
		if err := s.sessions.RevokeAllForUser(ctx, user.ID, time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	return user, nil
}

// ResetPassword define uma nova senha para um usuário (ação de administrador)
// e encerra todas as sessões dele.
func (s *UserService) ResetPassword(ctx context.Context, id, newPassword string) error {
	if _, err := s.users.ByID(ctx, id); err != nil {
		return err
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, id, hash); err != nil {
		return err
	}
	return s.sessions.RevokeAllForUser(ctx, id, time.Now().UTC())
}

// Get devolve um usuário pelo ID.
func (s *UserService) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.users.ByID(ctx, id)
}

// List devolve todos os usuários.
func (s *UserService) List(ctx context.Context) ([]*domain.User, error) {
	return s.users.List(ctx)
}

// EnsureBootstrapAdmin cria o primeiro administrador quando o banco ainda não
// tem nenhum usuário. É como alguém consegue entrar no sistema pela primeira
// vez. Se já houver usuários, não faz nada.
func (s *UserService) EnsureBootstrapAdmin(ctx context.Context, email, name, password string) error {
	count, err := s.users.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if email == "" || password == "" {
		slog.WarnContext(ctx, "nenhum usuário cadastrado e BOOTSTRAP_ADMIN_EMAIL/PASSWORD não definidos: ninguém conseguirá fazer login")
		return nil
	}
	_, err = s.Create(ctx, CreateUserInput{Email: email, Name: name, Password: password, Role: access.RoleAdmin})
	if kernel.IsKind(err, kernel.KindConflict) {
		return nil // outra réplica criou primeiro
	}
	if err == nil {
		slog.InfoContext(ctx, "administrador inicial criado — troque a senha no primeiro acesso")
	}
	return err
}
