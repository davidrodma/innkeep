// Package domain contém o modelo de domínio do contexto de Identidade:
// usuários internos (funcionários) e suas sessões.
//
// Como em todos os serviços, o pacote domain é o núcleo da arquitetura
// hexagonal: não importa banco, HTTP nem Kafka — só o Shared Kernel.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
)

// Limites da política de senhas. Seguimos o NIST SP 800-63B: o que protege é
// o COMPRIMENTO; regras de composição ("um símbolo, uma maiúscula...") só
// levam as pessoas a escolher "Senha@123".
const (
	MinPasswordLength = 10
	MaxPasswordLength = 128
)

// Erros de domínio do contexto.
var (
	ErrUserNotFound       = kernel.NotFound("user.not_found", "usuário não encontrado")
	ErrEmailTaken         = kernel.Conflict("user.email_taken", "já existe um usuário com este e-mail")
	ErrInvalidCredentials = kernel.Unauthorized("auth.invalid_credentials", "e-mail ou senha incorretos")
	ErrInvalidRefresh     = kernel.Unauthorized("auth.invalid_refresh_token", "sessão inválida ou expirada; faça login novamente")
	ErrLastAdmin          = kernel.Rule("user.last_admin", "não é possível desativar ou rebaixar o último administrador ativo")
)

// User é um funcionário com acesso ao painel administrativo.
type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Role         access.Role
	// PropertyIDs restringe o usuário a propriedades específicas. Vazio = todas.
	PropertyIDs []string
	Active      bool
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewUser cria um usuário válido. A senha chega já como hash: o domínio nunca
// vê a senha em texto puro além da validação de política (ValidatePassword).
func NewUser(email, name string, role access.Role, propertyIDs []string, passwordHash string) (*User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	user := &User{
		ID:           kernel.NewID(),
		Email:        normalized,
		PasswordHash: passwordHash,
		Active:       true,
		CreatedAt:    time.Now().UTC(),
	}
	if err := user.Update(name, role, propertyIDs, true); err != nil {
		return nil, err
	}
	return user, nil
}

// Update altera os dados cadastrais validando as regras.
func (u *User) Update(name string, role access.Role, propertyIDs []string, active bool) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 120 {
		return kernel.Invalid("user.invalid_name", "informe o nome do usuário").WithField("name", "entre 2 e 120 caracteres")
	}
	if !access.ValidRole(role) {
		return kernel.Invalid("user.invalid_role", "papel inválido").WithField("role", "papel desconhecido")
	}
	for _, id := range propertyIDs {
		if !kernel.IsID(id) {
			return kernel.Invalid("user.invalid_property", "propriedade inválida").WithField("property_ids", "ID inválido")
		}
	}
	u.Name = name
	u.Role = role
	u.PropertyIDs = propertyIDs
	u.Active = active
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// Principal converte o usuário na identidade usada para autorizar requisições.
func (u *User) Principal() access.Principal {
	return access.Principal{
		UserID:      u.ID,
		Email:       u.Email,
		Name:        u.Name,
		Role:        u.Role,
		PropertyIDs: u.PropertyIDs,
	}
}

// NormalizeEmail valida e normaliza um e-mail (minúsculas, sem espaços).
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", kernel.Invalid("user.invalid_email", "e-mail inválido").WithField("email", "formato inválido")
	}
	return email, nil
}

// ValidatePassword aplica a política de senhas.
func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength {
		return kernel.Invalid("user.weak_password", "a senha deve ter pelo menos %d caracteres", MinPasswordLength).
			WithField("password", "muito curta")
	}
	if length > MaxPasswordLength {
		return kernel.Invalid("user.weak_password", "a senha deve ter no máximo %d caracteres", MaxPasswordLength).
			WithField("password", "muito longa")
	}
	return nil
}
