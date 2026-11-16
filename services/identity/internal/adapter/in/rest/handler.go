// Package rest é o adaptador de ENTRADA HTTP do identity: traduz requisições
// REST em chamadas aos casos de uso e os resultados em respostas JSON.
//
// Um handler não contém regra de negócio. Ele faz três coisas: ler e validar o
// formato da entrada, chamar a aplicação e formatar a saída.
package rest

import (
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/identity/internal/app"
	"github.com/davidrodma/innkeep/services/identity/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	auth  *app.AuthService
	users *app.UserService
	jwks  auth.JWKS
}

// NewHandler cria o adaptador REST.
func NewHandler(authService *app.AuthService, userService *app.UserService, jwks auth.JWKS) *Handler {
	return &Handler{auth: authService, users: userService, jwks: jwks}
}

// Register publica as rotas no roteador.
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard) {
	// Chaves públicas para os demais serviços validarem os tokens.
	mux.HandleFunc("GET /.well-known/jwks.json", h.serveJWKS)

	// Autenticação (rotas abertas: é por elas que se obtém um token).
	mux.Handle("POST /v1/auth/login", httpx.HandlerFunc(h.login))
	mux.Handle("POST /v1/auth/refresh", httpx.HandlerFunc(h.refresh))
	mux.Handle("POST /v1/auth/logout", guard.Optional(httpx.HandlerFunc(h.logout)))

	// Conta do próprio usuário.
	mux.Handle("GET /v1/auth/me", guard.Authenticated(httpx.HandlerFunc(h.me)))
	mux.Handle("PUT /v1/auth/password", guard.Authenticated(httpx.HandlerFunc(h.changePassword)))

	// Gestão de usuários (somente administradores).
	mux.Handle("GET /v1/users", guard.Require(access.PermUserManage, httpx.HandlerFunc(h.listUsers)))
	mux.Handle("POST /v1/users", guard.Require(access.PermUserManage, httpx.HandlerFunc(h.createUser)))
	mux.Handle("GET /v1/users/{id}", guard.Require(access.PermUserManage, httpx.HandlerFunc(h.getUser)))
	mux.Handle("PUT /v1/users/{id}", guard.Require(access.PermUserManage, httpx.HandlerFunc(h.updateUser)))
	mux.Handle("PUT /v1/users/{id}/password", guard.Require(access.PermUserManage, httpx.HandlerFunc(h.resetPassword)))
}

// ---- DTOs (Data Transfer Objects): o formato do JSON na fronteira HTTP ----
//
// Os DTOs são separados das entidades de domínio de propósito: a API pública
// pode evoluir sem mexer no domínio (e vice-versa), e campos internos — como
// o hash da senha — nunca vazam por descuido.

type userResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	PropertyIDs []string   `json:"property_ids"`
	Permissions []string   `json:"permissions"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toUserResponse(user *domain.User) userResponse {
	permissions := access.PermissionsOf(user.Role)
	names := make([]string, len(permissions))
	for i, permission := range permissions {
		names[i] = string(permission)
	}
	propertyIDs := user.PropertyIDs
	if propertyIDs == nil {
		propertyIDs = []string{}
	}
	return userResponse{
		ID: user.ID, Email: user.Email, Name: user.Name, Role: string(user.Role),
		PropertyIDs: propertyIDs, Permissions: names, Active: user.Active,
		LastLoginAt: user.LastLoginAt, CreatedAt: user.CreatedAt,
	}
}

type tokensResponse struct {
	AccessToken      string       `json:"access_token"`
	TokenType        string       `json:"token_type"`
	ExpiresAt        time.Time    `json:"expires_at"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	User             userResponse `json:"user"`
}

func toTokensResponse(tokens app.Tokens) tokensResponse {
	return tokensResponse{
		AccessToken: tokens.AccessToken, TokenType: "Bearer", ExpiresAt: tokens.AccessExpiresAt,
		RefreshToken: tokens.RefreshToken, RefreshExpiresAt: tokens.RefreshExpiresAt,
		User: toUserResponse(tokens.User),
	}
}

// ---- Handlers ----

func (h *Handler) serveJWKS(w http.ResponseWriter, _ *http.Request) {
	// Chaves públicas podem (e devem) ser cacheadas por quem consome.
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.JSON(w, http.StatusOK, h.jwks)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	tokens, err := h.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toTokensResponse(tokens))
	return nil
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	tokens, err := h.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toTokensResponse(tokens))
	return nil
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := h.auth.Logout(r.Context(), body.RefreshToken); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	principal := access.MustPrincipal(r.Context())
	user, err := h.users.Get(r.Context(), principal.UserID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	principal := access.MustPrincipal(r.Context())
	if err := h.auth.ChangePassword(r.Context(), principal.UserID, body.CurrentPassword, body.NewPassword); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) error {
	users, err := h.users.List(r.Context())
	if err != nil {
		return err
	}
	items := make([]userResponse, len(users))
	for i, user := range users {
		items[i] = toUserResponse(user)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Email       string   `json:"email"`
		Name        string   `json:"name"`
		Password    string   `json:"password"`
		Role        string   `json:"role"`
		PropertyIDs []string `json:"property_ids"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	user, err := h.users.Create(r.Context(), app.CreateUserInput{
		Email: body.Email, Name: body.Name, Password: body.Password,
		Role: access.Role(body.Role), PropertyIDs: body.PropertyIDs,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toUserResponse(user))
	return nil
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	user, err := h.users.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body struct {
		Name        string   `json:"name"`
		Role        string   `json:"role"`
		PropertyIDs []string `json:"property_ids"`
		Active      *bool    `json:"active"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if body.Active == nil {
		return kernel.Invalid("user.active_required", "informe se o usuário está ativo").WithField("active", "obrigatório")
	}
	user, err := h.users.Update(r.Context(), id, app.UpdateUserInput{
		Name: body.Name, Role: access.Role(body.Role), PropertyIDs: body.PropertyIDs, Active: *body.Active,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := h.users.ResetPassword(r.Context(), id, body.NewPassword); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
