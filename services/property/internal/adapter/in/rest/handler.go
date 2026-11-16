// Package rest é o adaptador de entrada HTTP do serviço de propriedades.
package rest

import (
	"net/http"

	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/property/internal/app"
	"github.com/davidrodma/innkeep/services/property/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	service *app.Service
}

// NewHandler cria o adaptador REST.
func NewHandler(service *app.Service) *Handler { return &Handler{service: service} }

// Register publica as rotas no roteador.
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard) {
	read := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermPropertyRead, fn) }
	write := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermPropertyWrite, fn) }

	// Site público (sem autenticação): só propriedades publicadas.
	mux.Handle("GET /v1/public/properties", httpx.HandlerFunc(h.publicList))
	mux.Handle("GET /v1/public/properties/{slug}", httpx.HandlerFunc(h.publicGet))

	// Painel administrativo.
	mux.Handle("GET /v1/properties", read(h.listProperties))
	mux.Handle("POST /v1/properties", write(h.createProperty))
	mux.Handle("GET /v1/properties/{id}", read(h.getProperty))
	mux.Handle("PUT /v1/properties/{id}", write(h.updateProperty))

	mux.Handle("GET /v1/properties/{id}/room-types", read(h.listRoomTypes))
	mux.Handle("POST /v1/properties/{id}/room-types", write(h.createRoomType))
	mux.Handle("PUT /v1/room-types/{id}", write(h.updateRoomType))

	mux.Handle("GET /v1/properties/{id}/rooms", read(h.listRooms))
	mux.Handle("POST /v1/properties/{id}/rooms", write(h.createRoom))
	mux.Handle("PUT /v1/rooms/{id}", write(h.updateRoom))
	mux.Handle("PUT /v1/rooms/{id}/housekeeping",
		guard.Require(access.PermHousekeepingWrite, httpx.HandlerFunc(h.setHousekeeping)))
}

// ---- Site público ----

func (h *Handler) publicList(w http.ResponseWriter, r *http.Request) error {
	properties, err := h.service.Properties(r.Context(), true)
	if err != nil {
		return err
	}
	items := make([]publicPropertyResponse, 0, len(properties))
	for _, property := range properties {
		items = append(items, toPublicProperty(property, nil))
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) error {
	property, err := h.service.PropertyBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		return err
	}
	if !property.Published {
		// Não revelamos que existe uma propriedade não publicada.
		return domain.ErrPropertyNotFound
	}
	roomTypes, err := h.service.RoomTypes(r.Context(), property.ID, true)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toPublicProperty(property, roomTypes))
	return nil
}

// ---- Propriedades ----

func (h *Handler) listProperties(w http.ResponseWriter, r *http.Request) error {
	properties, err := h.service.Properties(r.Context(), false)
	if err != nil {
		return err
	}
	principal := access.MustPrincipal(r.Context())
	items := make([]propertyResponse, 0, len(properties))
	for _, property := range properties {
		if principal.CanAccessProperty(property.ID) {
			items = append(items, toPropertyResponse(property))
		}
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createProperty(w http.ResponseWriter, r *http.Request) error {
	var body propertyRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	property, err := h.service.CreateProperty(r.Context(), body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toPropertyResponse(property))
	return nil
}

func (h *Handler) getProperty(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	property, err := h.service.Property(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toPropertyResponse(property))
	return nil
}

func (h *Handler) updateProperty(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	var body propertyRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	property, err := h.service.UpdateProperty(r.Context(), id, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toPropertyResponse(property))
	return nil
}

// ---- Tipos de acomodação ----

func (h *Handler) listRoomTypes(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	roomTypes, err := h.service.RoomTypes(r.Context(), id, false)
	if err != nil {
		return err
	}
	items := make([]roomTypeResponse, len(roomTypes))
	for i, roomType := range roomTypes {
		items[i] = toRoomTypeResponse(roomType)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createRoomType(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	var body roomTypeRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	roomType, err := h.service.CreateRoomType(r.Context(), id, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toRoomTypeResponse(roomType))
	return nil
}

func (h *Handler) updateRoomType(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	current, err := h.service.RoomType(r.Context(), id)
	if err != nil {
		return err
	}
	if err := access.EnsureProperty(r.Context(), current.PropertyID); err != nil {
		return err
	}
	var body roomTypeRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	roomType, err := h.service.UpdateRoomType(r.Context(), id, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toRoomTypeResponse(roomType))
	return nil
}

// ---- Quartos ----

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	rooms, err := h.service.Rooms(r.Context(), id)
	if err != nil {
		return err
	}
	items := make([]roomResponse, len(rooms))
	for i, room := range rooms {
		items[i] = toRoomResponse(room)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request) error {
	id, err := h.propertyID(r)
	if err != nil {
		return err
	}
	var body roomRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	room, err := h.service.CreateRoom(r.Context(), id, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toRoomResponse(room))
	return nil
}

func (h *Handler) updateRoom(w http.ResponseWriter, r *http.Request) error {
	id, err := h.roomID(r)
	if err != nil {
		return err
	}
	var body roomRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	room, err := h.service.UpdateRoom(r.Context(), id, body.toInput())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toRoomResponse(room))
	return nil
}

func (h *Handler) setHousekeeping(w http.ResponseWriter, r *http.Request) error {
	id, err := h.roomID(r)
	if err != nil {
		return err
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	room, err := h.service.SetHousekeeping(r.Context(), id, domain.HousekeepingStatus(body.Status))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toRoomResponse(room))
	return nil
}

// propertyID lê o ID da propriedade da rota e confere o acesso do usuário.
func (h *Handler) propertyID(r *http.Request) (string, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return "", err
	}
	return id, access.EnsureProperty(r.Context(), id)
}

// roomID lê o ID do quarto da rota e confere o acesso à propriedade dele.
func (h *Handler) roomID(r *http.Request) (string, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return "", err
	}
	room, err := h.service.Room(r.Context(), id)
	if err != nil {
		return "", err
	}
	return id, access.EnsureProperty(r.Context(), room.PropertyID)
}
