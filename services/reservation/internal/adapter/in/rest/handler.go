// Package rest é o adaptador de entrada HTTP do serviço de reservas.
package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/command"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/port"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/query"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// Handler agrupa os endpoints do serviço. Repare que ele tem DUAS
// dependências — comandos e consultas — espelhando a separação do CQRS.
type Handler struct {
	commands *command.Handler
	queries  *query.Service
}

// NewHandler cria o adaptador REST.
func NewHandler(commands *command.Handler, queries *query.Service) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// Register publica as rotas. `idempotent` é o middleware de Idempotency-Key,
// aplicado às criações (que não podem ser executadas duas vezes).
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard, idempotent httpx.Middleware) {
	read := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermReservationRead, fn) }
	write := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermReservationWrite, fn) }

	// Site público (sem autenticação).
	mux.Handle("POST /v1/public/reservations", idempotent(httpx.HandlerFunc(h.publicRequest)))
	mux.Handle("POST /v1/public/reservations/lookup", httpx.HandlerFunc(h.publicLookup))

	// Painel administrativo — consultas.
	mux.Handle("GET /v1/reservations", read(h.list))
	mux.Handle("GET /v1/reservations/dashboard", read(h.dashboard))
	mux.Handle("GET /v1/reservations/{id}", read(h.get))
	mux.Handle("GET /v1/reservations/{id}/history", read(h.history))

	// Painel administrativo — comandos.
	mux.Handle("POST /v1/reservations", idempotent(write(h.request)))
	mux.Handle("POST /v1/reservations/{id}/confirm", write(h.confirm))
	mux.Handle("POST /v1/reservations/{id}/cancel", write(h.cancel))
	mux.Handle("POST /v1/reservations/{id}/check-in", write(h.checkIn))
	mux.Handle("POST /v1/reservations/{id}/check-out", write(h.checkOut))
	mux.Handle("POST /v1/reservations/{id}/no-show", write(h.noShow))
	mux.Handle("POST /v1/reservations/{id}/change-stay", write(h.changeStay))
}

type holderDTO struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type requestBody struct {
	PropertyID       string    `json:"property_id"`
	RoomTypeID       string    `json:"room_type_id"`
	RoomID           string    `json:"room_id"`
	CheckIn          string    `json:"check_in"`
	CheckOut         string    `json:"check_out"`
	Adults           int       `json:"adults"`
	Children         int       `json:"children"`
	Holder           holderDTO `json:"holder"`
	GroupCode        string    `json:"group_code"`
	Notes            string    `json:"notes"`
	QuotedTotalCents *int64    `json:"quoted_total_cents"`
	ConfirmNow       bool      `json:"confirm_now"`
}

func (b requestBody) holder() domain.Holder {
	return domain.Holder{Name: b.Holder.Name, Email: b.Holder.Email, Phone: b.Holder.Phone}
}

// ---- Site público ----

// publicView é o recorte da reserva mostrado ao hóspede: sem observações
// internas nem identificadores de sistema desnecessários.
type publicView struct {
	ID              string               `json:"id"`
	Code            string               `json:"code"`
	PropertyID      string               `json:"property_id"`
	Status          string               `json:"status"`
	RoomTypeID      string               `json:"room_type_id"`
	CheckIn         kernel.Date          `json:"check_in"`
	CheckOut        kernel.Date          `json:"check_out"`
	Nights          int                  `json:"nights"`
	Adults          int                  `json:"adults"`
	Children        int                  `json:"children"`
	HolderName      string               `json:"holder_name"`
	NightlyRates    []domain.NightlyRate `json:"nightly_rates"`
	TotalCents      int64                `json:"total_cents"`
	DepositDueCents int64                `json:"deposit_due_cents"`
	Currency        string               `json:"currency"`
	HoldExpiresAt   *time.Time           `json:"hold_expires_at"`
	RejectReason    string               `json:"reject_reason"`
}

func toPublicView(v port.ReservationView) publicView {
	return publicView{
		ID: v.ID, Code: v.Code, PropertyID: v.PropertyID, Status: v.Status, RoomTypeID: v.RoomTypeID,
		CheckIn: v.CheckIn, CheckOut: v.CheckOut, Nights: v.Nights, Adults: v.Adults, Children: v.Children,
		HolderName: v.Holder.Name, NightlyRates: v.NightlyRates, TotalCents: v.TotalCents,
		DepositDueCents: v.DepositDueCents, Currency: v.Currency, HoldExpiresAt: v.HoldExpiresAt,
		RejectReason: v.RejectReason,
	}
}

func (h *Handler) publicRequest(w http.ResponseWriter, r *http.Request) error {
	var body requestBody
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	// O site público nunca define preço, quarto específico nem confirmação:
	// esses campos são ignorados mesmo que venham no JSON. Nunca confie em
	// valores de negócio enviados pelo cliente.
	view, err := h.commands.Request(r.Context(), command.RequestReservation{
		PropertyID: body.PropertyID, RoomTypeID: body.RoomTypeID,
		CheckIn: body.CheckIn, CheckOut: body.CheckOut,
		Adults: body.Adults, Children: body.Children, Holder: body.holder(),
		Source: domain.SourceWebsite, Notes: body.Notes,
	})
	if err != nil {
		return err
	}
	// 202 Accepted: o pedido foi registrado; a alocação do quarto acontece
	// de forma assíncrona (saga) e o cliente acompanha pelo /lookup.
	httpx.JSON(w, http.StatusAccepted, toPublicView(view))
	return nil
}

func (h *Handler) publicLookup(w http.ResponseWriter, r *http.Request) error {
	// POST (e não GET) para que o e-mail viaje no corpo, e não na URL, onde
	// ficaria gravado em logs de acesso e no histórico do navegador.
	var body struct {
		Code  string `json:"code"`
		Email string `json:"email"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	view, err := h.queries.Lookup(r.Context(), body.Code, body.Email)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toPublicView(*view))
	return nil
}

// ---- Consultas ----

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return err
	}
	from, err := httpx.QueryDate(r, "from")
	if err != nil {
		return err
	}
	to, err := httpx.QueryDate(r, "to")
	if err != nil {
		return err
	}
	page := httpx.QueryPage(r)
	filter := port.ListFilter{
		PropertyID: propertyID, From: from, To: to, Query: r.URL.Query().Get("q"),
		Limit: page.Limit, Offset: page.Offset,
	}
	if statuses := r.URL.Query().Get("status"); statuses != "" {
		filter.Statuses = strings.Split(strings.ToUpper(statuses), ",")
	}
	views, total, err := h.queries.List(r.Context(), filter)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(views, total))
	return nil
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return err
	}
	date, err := httpx.RequiredQueryDate(r, "date")
	if err != nil {
		return err
	}
	dashboard, err := h.queries.DashboardFor(r.Context(), propertyID, date)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, dashboard)
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	view, err := h.authorized(r)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, view)
	return nil
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) error {
	view, err := h.authorized(r)
	if err != nil {
		return err
	}
	entries, err := h.queries.History(r.Context(), view.ID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(entries, len(entries)))
	return nil
}

// ---- Comandos ----

func (h *Handler) request(w http.ResponseWriter, r *http.Request) error {
	var body requestBody
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), body.PropertyID); err != nil {
		return err
	}
	view, err := h.commands.Request(r.Context(), command.RequestReservation{
		PropertyID: body.PropertyID, RoomTypeID: body.RoomTypeID, PreferredRoomID: body.RoomID,
		CheckIn: body.CheckIn, CheckOut: body.CheckOut,
		Adults: body.Adults, Children: body.Children, Holder: body.holder(),
		Source: domain.SourceDirect, GroupCode: body.GroupCode, Notes: body.Notes,
		QuotedTotalCents: body.QuotedTotalCents, ConfirmNow: body.ConfirmNow,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusAccepted, view)
	return nil
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) error {
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.Confirm(r.Context(), id)
	})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.Cancel(r.Context(), id, body.Reason, access.ActorFrom(r.Context()))
	})
}

func (h *Handler) checkIn(w http.ResponseWriter, r *http.Request) error {
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.CheckIn(r.Context(), id)
	})
}

func (h *Handler) checkOut(w http.ResponseWriter, r *http.Request) error {
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.CheckOut(r.Context(), id)
	})
}

func (h *Handler) noShow(w http.ResponseWriter, r *http.Request) error {
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.MarkNoShow(r.Context(), id)
	})
}

func (h *Handler) changeStay(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		CheckIn          string `json:"check_in"`
		CheckOut         string `json:"check_out"`
		RoomID           string `json:"room_id"`
		QuotedTotalCents *int64 `json:"quoted_total_cents"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	return h.run(w, r, func(id string) (port.ReservationView, error) {
		return h.commands.ChangeStay(r.Context(), id, body.CheckIn, body.CheckOut, body.RoomID, body.QuotedTotalCents)
	})
}

// run confere o acesso à reserva, executa o comando e devolve o estado novo.
func (h *Handler) run(w http.ResponseWriter, r *http.Request, execute func(id string) (port.ReservationView, error)) error {
	current, err := h.authorized(r)
	if err != nil {
		return err
	}
	view, err := execute(current.ID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, view)
	return nil
}

// authorized carrega a reserva da rota e confere se o usuário tem acesso à
// propriedade dela.
func (h *Handler) authorized(r *http.Request) (*port.ReservationView, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return nil, err
	}
	view, err := h.queries.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if err := access.EnsureProperty(r.Context(), view.PropertyID); err != nil {
		return nil, err
	}
	return view, nil
}

func (h *Handler) propertyFromQuery(r *http.Request) (string, error) {
	propertyID, err := httpx.RequiredQueryID(r, "property_id")
	if err != nil {
		return "", err
	}
	return propertyID, access.EnsureProperty(r.Context(), propertyID)
}
