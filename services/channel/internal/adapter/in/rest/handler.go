// Package rest é o adaptador de entrada HTTP do serviço de canais.
package rest

import (
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/channel/internal/adapter/out/ical"
	"github.com/davidrodma/innkeep/services/channel/internal/app"
	"github.com/davidrodma/innkeep/services/channel/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	service   *app.Service
	publicURL string
}

// NewHandler cria o adaptador REST. publicURL é o endereço externo da API,
// usado para montar o link do calendário que o canal vai importar.
func NewHandler(service *app.Service, publicURL string) *Handler {
	return &Handler{service: service, publicURL: publicURL}
}

// Register publica as rotas no roteador.
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard) {
	read := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermChannelRead, fn) }
	write := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermChannelWrite, fn) }

	// Calendário iCal que os canais importam. É público por necessidade (o
	// Airbnb não envia credenciais), mas a URL contém um token aleatório de
	// 192 bits — só quem a recebeu consegue ler. O conteúdo são apenas datas
	// indisponíveis, sem nenhum dado de hóspede.
	mux.Handle("GET /v1/public/channels/ical/{token}", httpx.HandlerFunc(h.calendarFeed))

	mux.Handle("GET /v1/channels/connections", read(h.listConnections))
	mux.Handle("POST /v1/channels/connections", write(h.createConnection))
	mux.Handle("PUT /v1/channels/connections/{id}", write(h.updateConnection))
	mux.Handle("DELETE /v1/channels/connections/{id}", write(h.deleteConnection))
	mux.Handle("POST /v1/channels/connections/{id}/sync", write(h.requestSync))
	mux.Handle("POST /v1/channels/connections/{id}/pull", write(h.pullNow))
	mux.Handle("GET /v1/channels/connections/{id}/mappings", read(h.listMappings))
	mux.Handle("POST /v1/channels/connections/{id}/mappings", write(h.createMapping))
	mux.Handle("GET /v1/channels/connections/{id}/sync-log", read(h.syncLog))
	mux.Handle("DELETE /v1/channels/mappings/{id}", write(h.deleteMapping))
}

// ---- DTOs ----

type connectionResponse struct {
	ID                 string `json:"id"`
	PropertyID         string `json:"property_id"`
	Channel            string `json:"channel"`
	Mode               string `json:"mode"`
	Name               string `json:"name"`
	ExternalPropertyID string `json:"external_property_id"`
	BaseURL            string `json:"base_url"`
	// As credenciais NUNCA são devolvidas pela API (campo somente-escrita);
	// a resposta informa apenas se existem.
	HasCredentials bool       `json:"has_credentials"`
	Active         bool       `json:"active"`
	LastPushAt     *time.Time `json:"last_push_at"`
	LastPullAt     *time.Time `json:"last_pull_at"`
	LastError      string     `json:"last_error"`
}

func toConnectionResponse(c *domain.Connection) connectionResponse {
	return connectionResponse{
		ID: c.ID, PropertyID: c.PropertyID, Channel: string(c.Channel), Mode: string(c.Mode), Name: c.Name,
		ExternalPropertyID: c.ExternalPropertyID, BaseURL: c.BaseURL, HasCredentials: !c.Credentials.Empty(),
		Active: c.Active, LastPushAt: c.LastPushAt, LastPullAt: c.LastPullAt, LastError: c.LastError,
	}
}

type connectionRequest struct {
	PropertyID         string `json:"property_id"`
	Channel            string `json:"channel"`
	Mode               string `json:"mode"`
	Name               string `json:"name"`
	ExternalPropertyID string `json:"external_property_id"`
	ClientID           string `json:"client_id"`
	ClientSecret       string `json:"client_secret"`
	BaseURL            string `json:"base_url"`
	Active             *bool  `json:"active"`
}

func (r connectionRequest) toInput() domain.ConnectionInput {
	input := domain.ConnectionInput{
		Name: r.Name, ExternalPropertyID: r.ExternalPropertyID, BaseURL: r.BaseURL, Active: r.Active == nil || *r.Active,
	}
	// Credenciais em branco = manter as que já estão gravadas.
	if r.ClientID != "" || r.ClientSecret != "" {
		input.Credentials = &domain.Credentials{ClientID: r.ClientID, ClientSecret: r.ClientSecret}
	}
	return input
}

type mappingResponse struct {
	ID             string `json:"id"`
	ConnectionID   string `json:"connection_id"`
	RoomTypeID     string `json:"room_type_id"`
	ExternalRoomID string `json:"external_room_id"`
	ExternalRateID string `json:"external_rate_id"`
	ICalImportURL  string `json:"ical_import_url"`
	// ICalExportURL é o endereço a colar no canal ("importar calendário").
	ICalExportURL string `json:"ical_export_url"`
}

func (h *Handler) toMappingResponse(m domain.Mapping) mappingResponse {
	return mappingResponse{
		ID: m.ID, ConnectionID: m.ConnectionID, RoomTypeID: m.RoomTypeID,
		ExternalRoomID: m.ExternalRoomID, ExternalRateID: m.ExternalRateID, ICalImportURL: m.ICalImportURL,
		ICalExportURL: h.publicURL + "/v1/public/channels/ical/" + m.ICalExportToken + ".ics",
	}
}

// ---- Calendário público ----

func (h *Handler) calendarFeed(w http.ResponseWriter, r *http.Request) error {
	mapping, busy, err := h.service.CalendarFeed(r.Context(), r.PathValue("token"))
	if err != nil {
		return err
	}
	ranges := make([]ical.Range, len(busy))
	for i, period := range busy {
		ranges[i] = ical.Range{From: period.From, To: period.To}
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	// Sem risco de XSS: a resposta é text/calendar (não HTML) e só contém datas
	// e identificadores gerados pelo próprio sistema, nenhum texto do usuário.
	_, _ = w.Write(ical.Encode("InnKeep", mapping.ID[:8], ranges)) //nolint:gosec // G705: ver o comentário acima
	return nil
}

// ---- Conexões ----

func (h *Handler) listConnections(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := httpx.RequiredQueryID(r, "property_id")
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), propertyID); err != nil {
		return err
	}
	connections, err := h.service.Connections(r.Context(), propertyID)
	if err != nil {
		return err
	}
	items := make([]connectionResponse, len(connections))
	for i, connection := range connections {
		items[i] = toConnectionResponse(connection)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request) error {
	var body connectionRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if !kernel.IsID(body.PropertyID) {
		return kernel.Invalid("channel.invalid_property", "informe a propriedade").WithField("property_id", "obrigatório")
	}
	if err := access.EnsureProperty(r.Context(), body.PropertyID); err != nil {
		return err
	}
	connection, err := h.service.CreateConnection(r.Context(), body.PropertyID,
		domain.Channel(body.Channel), domain.Mode(body.Mode), body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toConnectionResponse(connection))
	return nil
}

func (h *Handler) updateConnection(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	var body connectionRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	updated, err := h.service.UpdateConnection(r.Context(), connection.ID, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toConnectionResponse(updated))
	return nil
}

func (h *Handler) deleteConnection(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	if err := h.service.DeleteConnection(r.Context(), connection.ID); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) requestSync(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	count, err := h.service.RequestSync(r.Context(), connection.ID)
	if err != nil {
		return err
	}
	// 202: o reenvio foi PEDIDO; ele acontece de forma assíncrona, por eventos.
	httpx.JSON(w, http.StatusAccepted, map[string]int{"room_types": count})
	return nil
}

func (h *Handler) pullNow(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	published, err := h.service.Pull(r.Context(), connection.ID)
	if err != nil {
		return kernel.Unavailable("channel.pull_failed", "não foi possível consultar o canal: %v", err)
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"changes": published})
	return nil
}

// ---- Mapeamentos ----

func (h *Handler) listMappings(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	mappings, err := h.service.Mappings(r.Context(), connection.ID)
	if err != nil {
		return err
	}
	items := make([]mappingResponse, len(mappings))
	for i, mapping := range mappings {
		items[i] = h.toMappingResponse(mapping)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createMapping(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	var body struct {
		RoomTypeID     string `json:"room_type_id"`
		ExternalRoomID string `json:"external_room_id"`
		ExternalRateID string `json:"external_rate_id"`
		ICalImportURL  string `json:"ical_import_url"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	mapping, err := h.service.CreateMapping(r.Context(), connection.ID, domain.MappingInput{
		RoomTypeID: body.RoomTypeID, ExternalRoomID: body.ExternalRoomID,
		ExternalRateID: body.ExternalRateID, ICalImportURL: body.ICalImportURL,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, h.toMappingResponse(*mapping))
	return nil
}

func (h *Handler) deleteMapping(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	mapping, err := h.service.Mapping(r.Context(), id)
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), mapping.PropertyID); err != nil {
		return err
	}
	if err := h.service.DeleteMapping(r.Context(), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) syncLog(w http.ResponseWriter, r *http.Request) error {
	connection, err := h.authorized(r)
	if err != nil {
		return err
	}
	logs, err := h.service.SyncLogs(r.Context(), connection.ID, httpx.QueryInt(r, "limit", 50, 1, 200))
	if err != nil {
		return err
	}
	type logResponse struct {
		ID        string    `json:"id"`
		Direction string    `json:"direction"`
		Operation string    `json:"operation"`
		Success   bool      `json:"success"`
		Detail    string    `json:"detail"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := make([]logResponse, len(logs))
	for i, log := range logs {
		items[i] = logResponse{ID: log.ID, Direction: log.Direction, Operation: log.Operation,
			Success: log.Success, Detail: log.Detail, CreatedAt: log.CreatedAt}
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

// authorized carrega a conexão da rota e confere o acesso à propriedade dela.
func (h *Handler) authorized(r *http.Request) (*domain.Connection, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return nil, err
	}
	connection, err := h.service.Connection(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return connection, access.EnsureProperty(r.Context(), connection.PropertyID)
}
