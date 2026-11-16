// Package rest é o adaptador de entrada HTTP do serviço de inventário.
package rest

import (
	"net/http"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/inventory/internal/app"
	"github.com/davidrodma/innkeep/services/inventory/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	calendar *app.CalendarService
}

// NewHandler cria o adaptador REST.
func NewHandler(calendar *app.CalendarService) *Handler { return &Handler{calendar: calendar} }

// Register publica as rotas no roteador.
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard) {
	read := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermInventoryRead, fn) }
	write := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermInventoryWrite, fn) }

	// Busca de disponibilidade do site público (sem autenticação).
	mux.Handle("GET /v1/public/availability", httpx.HandlerFunc(h.publicAvailability))

	mux.Handle("GET /v1/availability", read(h.grid))
	mux.Handle("GET /v1/free-rooms", read(h.freeRooms))
	mux.Handle("PUT /v1/rates", write(h.setRates))
	mux.Handle("GET /v1/blocks", read(h.listBlocks))
	mux.Handle("POST /v1/blocks", write(h.createBlock))
	mux.Handle("DELETE /v1/blocks/{id}", write(h.deleteBlock))
}

type dayResponse struct {
	Date       string `json:"date"`
	Total      int    `json:"total"`
	Available  int    `json:"available"`
	PriceCents int64  `json:"price_cents"`
	MinStay    int    `json:"min_stay"`
	Closed     bool   `json:"closed"`
}

type roomTypeCalendarResponse struct {
	RoomTypeID     string        `json:"room_type_id"`
	Name           string        `json:"name"`
	MaxOccupancy   int           `json:"max_occupancy"`
	BasePriceCents int64         `json:"base_price_cents"`
	Currency       string        `json:"currency"`
	Days           []dayResponse `json:"days"`
}

type blockResponse struct {
	ID         string `json:"id"`
	PropertyID string `json:"property_id"`
	RoomTypeID string `json:"room_type_id"`
	RoomID     string `json:"room_id"`
	RoomName   string `json:"room_name"`
	From       string `json:"from"`
	To         string `json:"to"`
	Reason     string `json:"reason"`
}

func toBlockResponse(a domain.Allocation) blockResponse {
	return blockResponse{
		ID: a.ID, PropertyID: a.PropertyID, RoomTypeID: a.RoomTypeID, RoomID: a.RoomID, RoomName: a.RoomName,
		From: a.Stay.CheckIn().String(), To: a.Stay.CheckOut().String(), Reason: a.Reason,
	}
}

func (h *Handler) publicAvailability(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := httpx.RequiredQueryID(r, "property_id")
	if err != nil {
		return err
	}
	stay, err := kernel.ParseStayPeriod(r.URL.Query().Get("check_in"), r.URL.Query().Get("check_out"))
	if err != nil {
		return err
	}
	guests := httpx.QueryInt(r, "adults", 2, 1, 50) + httpx.QueryInt(r, "children", 0, 0, 50)
	offers, err := h.calendar.Offers(r.Context(), propertyID, stay, guests)
	if err != nil {
		return err
	}
	// A busca pode ser cacheada por instantes também no navegador/CDN.
	w.Header().Set("Cache-Control", "public, max-age=15")
	httpx.JSON(w, http.StatusOK, map[string]any{
		"check_in": stay.CheckIn().String(), "check_out": stay.CheckOut().String(),
		"nights": stay.Nights(), "offers": offers,
	})
	return nil
}

func (h *Handler) grid(w http.ResponseWriter, r *http.Request) error {
	propertyID, from, to, err := h.propertyRange(r)
	if err != nil {
		return err
	}
	grid, err := h.calendar.Grid(r.Context(), propertyID, from, to)
	if err != nil {
		return err
	}
	items := make([]roomTypeCalendarResponse, 0, len(grid))
	for _, entry := range grid {
		item := roomTypeCalendarResponse{
			RoomTypeID: entry.RoomType.ID, Name: entry.RoomType.Name, MaxOccupancy: entry.RoomType.MaxOccupancy,
			BasePriceCents: entry.RoomType.BasePriceCents, Currency: entry.RoomType.Currency,
			Days: make([]dayResponse, 0, len(entry.Days)),
		}
		for _, day := range entry.Days {
			item.Days = append(item.Days, dayResponse{
				Date: day.Date.String(), Total: day.Total, Available: day.Available(),
				PriceCents: day.PriceCents, MinStay: day.MinStay, Closed: day.Closed,
			})
		}
		items = append(items, item)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"from": from.String(), "to": to.String(), "room_types": items})
	return nil
}

func (h *Handler) freeRooms(w http.ResponseWriter, r *http.Request) error {
	roomTypeID, err := httpx.RequiredQueryID(r, "room_type_id")
	if err != nil {
		return err
	}
	stay, err := kernel.ParseStayPeriod(r.URL.Query().Get("check_in"), r.URL.Query().Get("check_out"))
	if err != nil {
		return err
	}
	rooms, err := h.calendar.FreeRooms(r.Context(), roomTypeID, stay)
	if err != nil {
		return err
	}
	type roomResponse struct {
		ID         string `json:"id"`
		RoomTypeID string `json:"room_type_id"`
		Name       string `json:"name"`
	}
	items := make([]roomResponse, 0, len(rooms))
	for _, room := range rooms {
		if err := access.EnsureProperty(r.Context(), room.PropertyID); err != nil {
			return err
		}
		items = append(items, roomResponse{ID: room.ID, RoomTypeID: room.RoomTypeID, Name: room.Name})
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) setRates(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		RoomTypeID string `json:"room_type_id"`
		From       string `json:"from"`
		To         string `json:"to"`
		Weekdays   []int  `json:"weekdays"`
		PriceCents *int64 `json:"price_cents"`
		MinStay    *int   `json:"min_stay"`
		Closed     *bool  `json:"closed"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if !kernel.IsID(body.RoomTypeID) {
		return kernel.Invalid("inventory.invalid_room_type", "informe o tipo de acomodação").WithField("room_type_id", "obrigatório")
	}
	from, err := kernel.ParseDate(body.From)
	if err != nil {
		return kernel.Invalid("inventory.invalid_range", "data inicial inválida").WithField("from", "use AAAA-MM-DD")
	}
	to, err := kernel.ParseDate(body.To)
	if err != nil {
		return kernel.Invalid("inventory.invalid_range", "data final inválida").WithField("to", "use AAAA-MM-DD")
	}
	// A autorização vem ANTES da alteração: primeiro descobrimos a qual
	// propriedade o tipo de quarto pertence e conferimos o acesso.
	roomType, err := h.calendar.RoomType(r.Context(), body.RoomTypeID)
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), roomType.PropertyID); err != nil {
		return err
	}
	if _, err := h.calendar.SetRates(r.Context(), app.SetRatesInput{
		RoomTypeID: body.RoomTypeID, From: from, To: to, Weekdays: body.Weekdays,
		Change: app.RateChange{PriceCents: body.PriceCents, MinStay: body.MinStay, Closed: body.Closed},
	}); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) listBlocks(w http.ResponseWriter, r *http.Request) error {
	propertyID, from, to, err := h.propertyRange(r)
	if err != nil {
		return err
	}
	blocks, err := h.calendar.Allocations(r.Context(), propertyID, from, to, domain.AllocationBlock)
	if err != nil {
		return err
	}
	items := make([]blockResponse, len(blocks))
	for i, block := range blocks {
		items[i] = toBlockResponse(block)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createBlock(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		RoomID string `json:"room_id"`
		From   string `json:"from"`
		To     string `json:"to"`
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if !kernel.IsID(body.RoomID) {
		return kernel.Invalid("inventory.invalid_room", "informe o quarto").WithField("room_id", "obrigatório")
	}
	stay, err := kernel.ParseStayPeriod(body.From, body.To)
	if err != nil {
		return err
	}
	room, err := h.calendar.Room(r.Context(), body.RoomID)
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), room.PropertyID); err != nil {
		return err
	}
	block, err := h.calendar.CreateBlock(r.Context(), body.RoomID, stay, body.Reason)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toBlockResponse(*block))
	return nil
}

func (h *Handler) deleteBlock(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	block, err := h.calendar.Block(r.Context(), id)
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), block.PropertyID); err != nil {
		return err
	}
	if err := h.calendar.DeleteBlock(r.Context(), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

// propertyRange lê property_id, from e to da query e confere o acesso.
func (h *Handler) propertyRange(r *http.Request) (string, kernel.Date, kernel.Date, error) {
	propertyID, err := httpx.RequiredQueryID(r, "property_id")
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	if err := access.EnsureProperty(r.Context(), propertyID); err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	from, err := httpx.RequiredQueryDate(r, "from")
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	to, err := httpx.RequiredQueryDate(r, "to")
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	return propertyID, from, to, nil
}
