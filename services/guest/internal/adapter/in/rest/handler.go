// Package rest é o adaptador de entrada HTTP do serviço de hóspedes.
package rest

import (
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/guest/internal/app"
	"github.com/davidrodma/innkeep/services/guest/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	service *app.Service
}

// NewHandler cria o adaptador REST.
func NewHandler(service *app.Service) *Handler { return &Handler{service: service} }

// Register publica as rotas. Não há NENHUMA rota pública neste serviço: dados
// pessoais só são servidos a usuários autenticados e autorizados.
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard) {
	read := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermGuestRead, fn) }
	write := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermGuestWrite, fn) }

	mux.Handle("GET /v1/guests", read(h.search))
	mux.Handle("POST /v1/guests", write(h.create))
	mux.Handle("GET /v1/guests/{id}", read(h.get))
	mux.Handle("PUT /v1/guests/{id}", write(h.update))
	mux.Handle("GET /v1/guests/{id}/stays", read(h.stays))
	mux.Handle("POST /v1/guests/{id}/anonymize", write(h.anonymize))

	mux.Handle("GET /v1/registrations/{reservation_id}", read(h.registration))
	mux.Handle("PUT /v1/registrations/{reservation_id}", write(h.register))
	mux.Handle("GET /v1/registration-options", read(h.options))
}

// ---- DTOs ----

type addressDTO struct {
	Line       string `json:"line"`
	District   string `json:"district"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type guestRequest struct {
	FullName         string     `json:"full_name"`
	Email            string     `json:"email"`
	Phone            string     `json:"phone"`
	BirthDate        string     `json:"birth_date"`
	Gender           string     `json:"gender"`
	Nationality      string     `json:"nationality"`
	Profession       string     `json:"profession"`
	DocumentType     string     `json:"document_type"`
	DocumentNumber   string     `json:"document_number"`
	DocumentIssuer   string     `json:"document_issuer"`
	Address          addressDTO `json:"address"`
	Notes            string     `json:"notes"`
	MarketingConsent bool       `json:"marketing_consent"`
}

func (r guestRequest) toInput() (domain.GuestInput, error) {
	var birthDate kernel.Date
	if r.BirthDate != "" {
		parsed, err := kernel.ParseDate(r.BirthDate)
		if err != nil {
			return domain.GuestInput{}, kernel.Invalid("guest.invalid_birth_date", "data de nascimento inválida").WithField("birth_date", "use AAAA-MM-DD")
		}
		birthDate = parsed
	}
	return domain.GuestInput{
		FullName: r.FullName, Email: r.Email, Phone: r.Phone, BirthDate: birthDate, Gender: r.Gender,
		Nationality: r.Nationality, Profession: r.Profession,
		DocumentType: domain.DocumentType(r.DocumentType), DocumentNumber: r.DocumentNumber, DocumentIssuer: r.DocumentIssuer,
		Address: domain.Address{Line: r.Address.Line, District: r.Address.District, City: r.Address.City,
			State: r.Address.State, PostalCode: r.Address.PostalCode, Country: r.Address.Country},
		Notes: r.Notes, MarketingConsent: r.MarketingConsent,
	}, nil
}

type guestResponse struct {
	ID string `json:"id"`
	guestRequest
	Anonymized bool      `json:"anonymized"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// toGuestResponse monta a resposta. Com masked=true (listagens) o documento
// sai mascarado e o endereço é omitido: mostra-se o mínimo necessário para
// reconhecer a pessoa (LGPD: princípio da necessidade).
func toGuestResponse(g *domain.Guest, masked bool) guestResponse {
	response := guestResponse{
		ID: g.ID,
		guestRequest: guestRequest{
			FullName: g.FullName, Email: g.Email, Phone: g.Phone, BirthDate: g.BirthDate.String(),
			Gender: g.Gender, Nationality: g.Nationality, Profession: g.Profession,
			DocumentType: string(g.DocumentType), DocumentNumber: g.DocumentNumber, DocumentIssuer: g.DocumentIssuer,
			Address: addressDTO{Line: g.Address.Line, District: g.Address.District, City: g.Address.City,
				State: g.Address.State, PostalCode: g.Address.PostalCode, Country: g.Address.Country},
			Notes: g.Notes, MarketingConsent: g.MarketingConsent,
		},
		Anonymized: g.AnonymizedAt != nil, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
	if masked {
		response.DocumentNumber = domain.MaskDocument(g.DocumentNumber)
		response.Address = addressDTO{City: g.Address.City, State: g.Address.State, Country: g.Address.Country}
		response.Notes, response.BirthDate = "", ""
	}
	return response
}

type placeDTO struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type cardResponse struct {
	ID            string      `json:"id"`
	ReservationID string      `json:"reservation_id"`
	PropertyID    string      `json:"property_id"`
	GuestID       string      `json:"guest_id"`
	Primary       bool        `json:"primary"`
	PurposeOfTrip string      `json:"purpose_of_trip"`
	Transport     string      `json:"transport"`
	Origin        placeDTO    `json:"origin"`
	Destination   placeDTO    `json:"destination"`
	VehiclePlate  string      `json:"vehicle_plate"`
	CheckIn       kernel.Date `json:"check_in"`
	CheckOut      kernel.Date `json:"check_out"`
	CheckedInAt   *time.Time  `json:"checked_in_at"`
	CheckedOutAt  *time.Time  `json:"checked_out_at"`
}

func toCardResponse(c *domain.RegistrationCard) cardResponse {
	return cardResponse{
		ID: c.ID, ReservationID: c.ReservationID, PropertyID: c.PropertyID, GuestID: c.GuestID, Primary: c.Primary,
		PurposeOfTrip: c.PurposeOfTrip, Transport: c.Transport,
		Origin:       placeDTO{City: c.Origin.City, State: c.Origin.State, Country: c.Origin.Country},
		Destination:  placeDTO{City: c.Destination.City, State: c.Destination.State, Country: c.Destination.Country},
		VehiclePlate: c.VehiclePlate, CheckIn: c.CheckIn, CheckOut: c.CheckOut,
		CheckedInAt: c.CheckedInAt, CheckedOutAt: c.CheckedOutAt,
	}
}

type registeredGuestResponse struct {
	Guest guestResponse `json:"guest"`
	Card  cardResponse  `json:"card"`
}

func toRegistered(items []app.RegisteredGuest) []registeredGuestResponse {
	response := make([]registeredGuestResponse, 0, len(items))
	for _, item := range items {
		response = append(response, registeredGuestResponse{Guest: toGuestResponse(item.Guest, false), Card: toCardResponse(item.Card)})
	}
	return response
}

// ---- Hóspedes ----

func (h *Handler) search(w http.ResponseWriter, r *http.Request) error {
	// Busca por documento exato: ?document_type=CPF&document_number=...
	if number := r.URL.Query().Get("document_number"); number != "" {
		documentType := domain.DocumentType(r.URL.Query().Get("document_type"))
		if documentType == "" {
			documentType = domain.DocumentCPF
		}
		guest, err := h.service.FindByDocument(r.Context(), documentType, number)
		if kernel.IsKind(err, kernel.KindNotFound) {
			httpx.JSON(w, http.StatusOK, httpx.NewList([]guestResponse{}, 0))
			return nil
		}
		if err != nil {
			return err
		}
		if ok, err := h.visible(r, guest.ID); err != nil || !ok {
			httpx.JSON(w, http.StatusOK, httpx.NewList([]guestResponse{}, 0))
			return err
		}
		httpx.JSON(w, http.StatusOK, httpx.NewList([]guestResponse{toGuestResponse(guest, false)}, 1))
		return nil
	}

	page := httpx.QueryPage(r)
	guests, total, err := h.service.Search(r.Context(), r.URL.Query().Get("q"), page.Limit, page.Offset)
	if err != nil {
		return err
	}
	items := make([]guestResponse, 0, len(guests))
	for _, guest := range guests {
		ok, err := h.visible(r, guest.ID)
		if err != nil {
			return err
		}
		if !ok {
			total--
			continue
		}
		items = append(items, toGuestResponse(guest, true))
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, total))
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var body guestRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	input, err := body.toInput()
	if err != nil {
		return err
	}
	guest, err := h.service.Create(r.Context(), input)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toGuestResponse(guest, false))
	return nil
}

// visible informa se o usuário pode ver o hóspede. O cadastro de hóspedes é
// compartilhado entre as propriedades da instalação; um usuário restrito a
// algumas propriedades só enxerga quem se hospedou nelas.
func (h *Handler) visible(r *http.Request, guestID string) (bool, error) {
	principal := access.MustPrincipal(r.Context())
	if len(principal.PropertyIDs) == 0 {
		return true, nil
	}
	stays, err := h.service.Stays(r.Context(), guestID)
	if err != nil {
		return false, err
	}
	for _, stay := range stays {
		if principal.CanAccessProperty(stay.PropertyID) {
			return true, nil
		}
	}
	return false, nil
}

// ensureVisible devolve "não encontrado" para hóspedes fora do alcance do
// usuário (não revelamos que o cadastro existe).
func (h *Handler) ensureVisible(r *http.Request, guestID string) error {
	ok, err := h.visible(r, guestID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrGuestNotFound
	}
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.ensureVisible(r, id); err != nil {
		return err
	}
	guest, err := h.service.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toGuestResponse(guest, false))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.ensureVisible(r, id); err != nil {
		return err
	}
	var body guestRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	input, err := body.toInput()
	if err != nil {
		return err
	}
	guest, err := h.service.Update(r.Context(), id, input)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toGuestResponse(guest, false))
	return nil
}

func (h *Handler) stays(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.ensureVisible(r, id); err != nil {
		return err
	}
	cards, err := h.service.Stays(r.Context(), id)
	if err != nil {
		return err
	}
	principal := access.MustPrincipal(r.Context())
	items := make([]cardResponse, 0, len(cards))
	for _, card := range cards {
		if principal.CanAccessProperty(card.PropertyID) {
			items = append(items, toCardResponse(card))
		}
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) anonymize(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.ensureVisible(r, id); err != nil {
		return err
	}
	if err := h.service.Anonymize(r.Context(), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

// ---- Ficha de registro (FNRH) ----

func (h *Handler) registration(w http.ResponseWriter, r *http.Request) error {
	reservationID, err := httpx.PathID(r, "reservation_id")
	if err != nil {
		return err
	}
	registered, err := h.service.Registration(r.Context(), reservationID)
	if err != nil {
		return err
	}
	for _, item := range registered {
		if err := access.EnsureProperty(r.Context(), item.Card.PropertyID); err != nil {
			return err
		}
	}
	items := toRegistered(registered)
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	reservationID, err := httpx.PathID(r, "reservation_id")
	if err != nil {
		return err
	}
	var body struct {
		PropertyID string `json:"property_id"`
		CheckIn    string `json:"check_in"`
		CheckOut   string `json:"check_out"`
		Guests     []struct {
			guestRequest
			Primary       bool     `json:"primary"`
			PurposeOfTrip string   `json:"purpose_of_trip"`
			Transport     string   `json:"transport"`
			Origin        placeDTO `json:"origin"`
			Destination   placeDTO `json:"destination"`
			VehiclePlate  string   `json:"vehicle_plate"`
		} `json:"guests"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if !kernel.IsID(body.PropertyID) {
		return kernel.Invalid("request.invalid_id", "informe a propriedade").WithField("property_id", "obrigatório")
	}
	if err := access.EnsureProperty(r.Context(), body.PropertyID); err != nil {
		return err
	}
	stay, err := kernel.ParseStayPeriod(body.CheckIn, body.CheckOut)
	if err != nil {
		return err
	}

	input := app.RegistrationInput{ReservationID: reservationID, PropertyID: body.PropertyID, Stay: stay}
	for _, entry := range body.Guests {
		guestInput, err := entry.toInput()
		if err != nil {
			return err
		}
		input.Guests = append(input.Guests, app.GuestRegistration{
			Guest: guestInput, Primary: entry.Primary,
			Trip: domain.TripDetails{
				PurposeOfTrip: entry.PurposeOfTrip, Transport: entry.Transport,
				Origin:       domain.Place{City: entry.Origin.City, State: entry.Origin.State, Country: entry.Origin.Country},
				Destination:  domain.Place{City: entry.Destination.City, State: entry.Destination.State, Country: entry.Destination.Country},
				VehiclePlate: entry.VehiclePlate,
			},
		})
	}
	registered, err := h.service.Register(r.Context(), input)
	if err != nil {
		return err
	}
	items := toRegistered(registered)
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) options(w http.ResponseWriter, _ *http.Request) error {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"document_types": []string{"CPF", "RG", "PASSPORT", "CNH", "RNE", "OTHER"},
		"trip_purposes":  domain.TripPurposes,
		"transports":     domain.Transports,
	})
	return nil
}
