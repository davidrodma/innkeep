// Package access define o vocabulário de controle de acesso compartilhado por
// todos os serviços: papéis (roles), permissões e o usuário autenticado
// (Principal).
//
// Faz parte do Shared Kernel porque é uma linguagem comum a todos os bounded
// contexts — e, como o restante do kernel, só depende da biblioteca padrão.
// A mecânica de tokens (JWT, JWKS, middleware HTTP) fica em pkg/platform/auth.
package access

import (
	"context"
	"slices"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Role é o papel de um usuário interno (funcionário da propriedade).
type Role string

const (
	RoleAdmin        Role = "ADMIN"        // dono/administrador: acesso total
	RoleManager      Role = "MANAGER"      // gerente: tudo, exceto gerenciar usuários
	RoleReceptionist Role = "RECEPTIONIST" // recepção: reservas, hóspedes, recebimentos
	RoleFinance      Role = "FINANCE"      // financeiro: contas, despesas, relatórios
	RoleHousekeeping Role = "HOUSEKEEPING" // governança: situação de limpeza dos quartos
)

// Permission é uma ação protegida, no formato "recurso:ação".
type Permission string

const (
	PermPropertyRead      Permission = "property:read"
	PermPropertyWrite     Permission = "property:write"
	PermHousekeepingWrite Permission = "housekeeping:write"
	PermInventoryRead     Permission = "inventory:read"
	PermInventoryWrite    Permission = "inventory:write"
	PermReservationRead   Permission = "reservation:read"
	PermReservationWrite  Permission = "reservation:write"
	PermGuestRead         Permission = "guest:read"
	PermGuestWrite        Permission = "guest:write"
	PermBillingRead       Permission = "billing:read"
	PermBillingWrite      Permission = "billing:write"
	PermFinanceRead       Permission = "finance:read"
	PermFinanceWrite      Permission = "finance:write"
	PermChannelRead       Permission = "channel:read"
	PermChannelWrite      Permission = "channel:write"
	PermUserManage        Permission = "user:manage"
)

var allPermissions = []Permission{
	PermPropertyRead, PermPropertyWrite, PermHousekeepingWrite,
	PermInventoryRead, PermInventoryWrite,
	PermReservationRead, PermReservationWrite,
	PermGuestRead, PermGuestWrite,
	PermBillingRead, PermBillingWrite,
	PermFinanceRead, PermFinanceWrite,
	PermChannelRead, PermChannelWrite,
	PermUserManage,
}

// rolePermissions é a matriz de autorização (RBAC): o que cada papel pode
// fazer. Princípio do menor privilégio — cada papel recebe só o necessário
// para o seu trabalho.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: allPermissions,
	RoleManager: {
		PermPropertyRead, PermPropertyWrite, PermHousekeepingWrite,
		PermInventoryRead, PermInventoryWrite,
		PermReservationRead, PermReservationWrite,
		PermGuestRead, PermGuestWrite,
		PermBillingRead, PermBillingWrite,
		PermFinanceRead, PermFinanceWrite,
		PermChannelRead, PermChannelWrite,
	},
	RoleReceptionist: {
		PermPropertyRead, PermHousekeepingWrite,
		PermInventoryRead,
		PermReservationRead, PermReservationWrite,
		PermGuestRead, PermGuestWrite,
		PermBillingRead, PermBillingWrite,
	},
	RoleFinance: {
		PermPropertyRead, PermInventoryRead, PermReservationRead, PermGuestRead,
		PermBillingRead, PermBillingWrite,
		PermFinanceRead, PermFinanceWrite,
	},
	RoleHousekeeping: {
		PermPropertyRead, PermHousekeepingWrite,
	},
}

// ValidRole informa se o papel existe.
func ValidRole(role Role) bool {
	_, ok := rolePermissions[role]
	return ok
}

// Roles lista todos os papéis existentes.
func Roles() []Role {
	return []Role{RoleAdmin, RoleManager, RoleReceptionist, RoleFinance, RoleHousekeeping}
}

// PermissionsOf devolve as permissões de um papel (cópia defensiva).
func PermissionsOf(role Role) []Permission {
	return slices.Clone(rolePermissions[role])
}

// Principal é o usuário autenticado que está fazendo a requisição.
type Principal struct {
	UserID string
	Email  string
	Name   string
	Role   Role
	// PropertyIDs restringe o acesso a propriedades específicas. Vazio = todas.
	PropertyIDs []string
	// TokenID é o identificador único do token de acesso (claim "jti").
	TokenID string
}

// Can informa se o usuário possui a permissão.
func (p Principal) Can(permission Permission) bool {
	return slices.Contains(rolePermissions[p.Role], permission)
}

// CanAccessProperty informa se o usuário pode operar na propriedade.
func (p Principal) CanAccessProperty(propertyID string) bool {
	if len(p.PropertyIDs) == 0 {
		return true
	}
	return slices.Contains(p.PropertyIDs, propertyID)
}

// Actor devolve a identificação usada em trilhas de auditoria.
func (p Principal) Actor() string { return "user:" + p.UserID }

type principalKey struct{}

// WithPrincipal devolve um contexto que carrega o usuário autenticado.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// FromContext devolve o usuário autenticado, se houver.
func FromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// MustPrincipal devolve o usuário autenticado. Só deve ser usado em fluxos
// protegidos por autenticação (onde o usuário sempre existe).
func MustPrincipal(ctx context.Context) Principal {
	principal, ok := FromContext(ctx)
	if !ok {
		panic("access: MustPrincipal chamado sem usuário autenticado no contexto")
	}
	return principal
}

// ActorFrom devolve o identificador de auditoria do usuário do contexto, ou
// "system" quando a ação não partiu de uma pessoa (ex.: reação a um evento).
func ActorFrom(ctx context.Context) string {
	if principal, ok := FromContext(ctx); ok {
		return principal.Actor()
	}
	return "system"
}

// EnsureProperty devolve erro se o usuário do contexto não puder operar na
// propriedade. Chame depois de descobrir a qual propriedade o recurso pertence.
func EnsureProperty(ctx context.Context, propertyID string) error {
	principal, ok := FromContext(ctx)
	if !ok {
		return kernel.Unauthorized("auth.missing_token", "autenticação necessária")
	}
	if !principal.CanAccessProperty(propertyID) {
		return kernel.Forbidden("auth.property_forbidden", "você não tem acesso a esta propriedade")
	}
	return nil
}
