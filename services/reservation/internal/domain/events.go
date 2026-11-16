package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Event é um evento de DOMÍNIO: um fato que aconteceu com uma reserva.
//
// Estes eventos são o que o Event Store guarda — eles SÃO o estado da reserva.
// Não confunda com os eventos de INTEGRAÇÃO (pkg/contracts), que são o
// contrato público entre serviços: os de domínio são internos e podem ter
// granularidade e formato próprios; a camada de aplicação traduz de um para o
// outro (veja app/command/translate.go).
//
// Regra de ouro do Event Sourcing: um evento gravado é IMUTÁVEL e eterno.
// Nunca altere o significado de um campo; para evoluir, acrescente campos
// opcionais ou crie um novo tipo de evento.
type Event interface {
	EventName() string
	// OccurredAt é o instante em que o fato aconteceu.
	OccurredAt() time.Time
}

// Nomes persistidos dos eventos (coluna event_type do Event Store).
const (
	EventRequested             = "reservation.requested"
	EventRoomAllocated         = "reservation.room_allocated"
	EventRejected              = "reservation.rejected"
	EventConfirmed             = "reservation.confirmed"
	EventCancelled             = "reservation.cancelled"
	EventExpired               = "reservation.expired"
	EventMarkedNoShow          = "reservation.marked_no_show"
	EventRegistrationCompleted = "reservation.registration_completed"
	EventCheckedIn             = "reservation.checked_in"
	EventCheckedOut            = "reservation.checked_out"
	EventStayChangeRequested   = "reservation.stay_change_requested"
	EventStayChanged           = "reservation.stay_changed"
	EventStayChangeRejected    = "reservation.stay_change_rejected"
)

// NightlyRate é o preço de uma diária.
type NightlyRate struct {
	Date       kernel.Date `json:"date"`
	PriceCents int64       `json:"price_cents"`
}

// Requested: a reserva foi solicitada (primeiro evento de todo fluxo).
type Requested struct {
	ReservationID   string      `json:"reservation_id"`
	Code            string      `json:"code"`
	PropertyID      string      `json:"property_id"`
	RoomTypeID      string      `json:"room_type_id"`
	PreferredRoomID string      `json:"preferred_room_id,omitempty"`
	CheckIn         kernel.Date `json:"check_in"`
	CheckOut        kernel.Date `json:"check_out"`
	Adults          int         `json:"adults"`
	Children        int         `json:"children"`
	Holder          Holder      `json:"holder"`
	Source          string      `json:"source"`
	ExternalRef     string      `json:"external_ref,omitempty"`
	GroupCode       string      `json:"group_code,omitempty"`
	Notes           string      `json:"notes,omitempty"`
	// Política da propriedade no momento do pedido. Fica registrada no evento
	// para que reaplicar o histórico dê sempre o mesmo resultado, mesmo que a
	// propriedade mude a política depois.
	DepositPercent int    `json:"deposit_percent"`
	HoldMinutes    int    `json:"hold_minutes"`
	Currency       string `json:"currency"`

	QuotedTotalCents    *int64    `json:"quoted_total_cents,omitempty"`
	ConfirmOnAllocation bool      `json:"confirm_on_allocation"`
	PaymentCollect      string    `json:"payment_collect"`
	EnforceRestrictions bool      `json:"enforce_restrictions"`
	AllowOverbooking    bool      `json:"allow_overbooking"`
	At                  time.Time `json:"at"`
}

// RoomAllocated: o inventário reservou o quarto e definiu o preço.
type RoomAllocated struct {
	RoomTypeID      string        `json:"room_type_id"`
	RoomID          string        `json:"room_id,omitempty"`
	RoomName        string        `json:"room_name,omitempty"`
	NightlyRates    []NightlyRate `json:"nightly_rates"`
	TotalCents      int64         `json:"total_cents"`
	DepositDueCents int64         `json:"deposit_due_cents"`
	HoldExpiresAt   *time.Time    `json:"hold_expires_at,omitempty"`
	Overbooked      bool          `json:"overbooked"`
	At              time.Time     `json:"at"`
}

// Rejected: não houve como atender o pedido.
type Rejected struct {
	Reason string    `json:"reason"`
	Detail string    `json:"detail,omitempty"`
	At     time.Time `json:"at"`
}

// Confirmed: a reserva está garantida.
type Confirmed struct {
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// Cancelled: a reserva foi cancelada.
type Cancelled struct {
	Reason string    `json:"reason,omitempty"`
	By     string    `json:"by,omitempty"`
	At     time.Time `json:"at"`
}

// Expired: o prazo do sinal venceu.
type Expired struct {
	At time.Time `json:"at"`
}

// MarkedNoShow: o hóspede não compareceu.
type MarkedNoShow struct {
	At time.Time `json:"at"`
}

// RegistrationCompleted: a FNRH foi preenchida (pré-requisito do check-in).
type RegistrationCompleted struct {
	PrimaryGuestID string    `json:"primary_guest_id"`
	GuestCount     int       `json:"guest_count"`
	At             time.Time `json:"at"`
}

// CheckedIn: o hóspede entrou.
type CheckedIn struct {
	At time.Time `json:"at"`
}

// CheckedOut: o hóspede saiu.
type CheckedOut struct {
	ActualCheckOut kernel.Date `json:"actual_check_out"`
	At             time.Time   `json:"at"`
}

// StayChangeRequested: pedido de mudança de datas e/ou quarto.
type StayChangeRequested struct {
	NewRoomID        string      `json:"new_room_id,omitempty"`
	NewCheckIn       kernel.Date `json:"new_check_in"`
	NewCheckOut      kernel.Date `json:"new_check_out"`
	QuotedTotalCents *int64      `json:"quoted_total_cents,omitempty"`
	At               time.Time   `json:"at"`
}

// StayChanged: a mudança foi efetivada pelo inventário.
type StayChanged struct {
	RoomTypeID   string        `json:"room_type_id"`
	RoomID       string        `json:"room_id,omitempty"`
	RoomName     string        `json:"room_name,omitempty"`
	CheckIn      kernel.Date   `json:"check_in"`
	CheckOut     kernel.Date   `json:"check_out"`
	NightlyRates []NightlyRate `json:"nightly_rates"`
	TotalCents   int64         `json:"total_cents"`
	At           time.Time     `json:"at"`
}

// StayChangeRejected: a mudança não pôde ser atendida; nada mudou.
type StayChangeRejected struct {
	Reason string    `json:"reason"`
	Detail string    `json:"detail,omitempty"`
	At     time.Time `json:"at"`
}

func (Requested) EventName() string             { return EventRequested }
func (RoomAllocated) EventName() string         { return EventRoomAllocated }
func (Rejected) EventName() string              { return EventRejected }
func (Confirmed) EventName() string             { return EventConfirmed }
func (Cancelled) EventName() string             { return EventCancelled }
func (Expired) EventName() string               { return EventExpired }
func (MarkedNoShow) EventName() string          { return EventMarkedNoShow }
func (RegistrationCompleted) EventName() string { return EventRegistrationCompleted }
func (CheckedIn) EventName() string             { return EventCheckedIn }
func (CheckedOut) EventName() string            { return EventCheckedOut }
func (StayChangeRequested) EventName() string   { return EventStayChangeRequested }
func (StayChanged) EventName() string           { return EventStayChanged }
func (StayChangeRejected) EventName() string    { return EventStayChangeRejected }

func (e Requested) OccurredAt() time.Time             { return e.At }
func (e RoomAllocated) OccurredAt() time.Time         { return e.At }
func (e Rejected) OccurredAt() time.Time              { return e.At }
func (e Confirmed) OccurredAt() time.Time             { return e.At }
func (e Cancelled) OccurredAt() time.Time             { return e.At }
func (e Expired) OccurredAt() time.Time               { return e.At }
func (e MarkedNoShow) OccurredAt() time.Time          { return e.At }
func (e RegistrationCompleted) OccurredAt() time.Time { return e.At }
func (e CheckedIn) OccurredAt() time.Time             { return e.At }
func (e CheckedOut) OccurredAt() time.Time            { return e.At }
func (e StayChangeRequested) OccurredAt() time.Time   { return e.At }
func (e StayChanged) OccurredAt() time.Time           { return e.At }
func (e StayChangeRejected) OccurredAt() time.Time    { return e.At }

// eventFactories mapeia o nome persistido para o tipo Go correspondente. É o
// que permite reconstruir o evento certo ao ler o Event Store.
var eventFactories = map[string]func() Event{
	EventRequested:             func() Event { return &Requested{} },
	EventRoomAllocated:         func() Event { return &RoomAllocated{} },
	EventRejected:              func() Event { return &Rejected{} },
	EventConfirmed:             func() Event { return &Confirmed{} },
	EventCancelled:             func() Event { return &Cancelled{} },
	EventExpired:               func() Event { return &Expired{} },
	EventMarkedNoShow:          func() Event { return &MarkedNoShow{} },
	EventRegistrationCompleted: func() Event { return &RegistrationCompleted{} },
	EventCheckedIn:             func() Event { return &CheckedIn{} },
	EventCheckedOut:            func() Event { return &CheckedOut{} },
	EventStayChangeRequested:   func() Event { return &StayChangeRequested{} },
	EventStayChanged:           func() Event { return &StayChanged{} },
	EventStayChangeRejected:    func() Event { return &StayChangeRejected{} },
}

// DecodeEvent reconstrói um evento de domínio a partir do nome e do JSON
// gravados no Event Store.
func DecodeEvent(name string, payload []byte) (Event, error) {
	factory, ok := eventFactories[name]
	if !ok {
		return nil, fmt.Errorf("evento de domínio desconhecido: %q", name)
	}
	event := factory()
	if err := json.Unmarshal(payload, event); err != nil {
		return nil, fmt.Errorf("decodificando evento %s: %w", name, err)
	}
	return derefEvent(event), nil
}

// derefEvent devolve o evento por valor (as fábricas criam ponteiros só para
// o json.Unmarshal poder preenchê-los).
func derefEvent(event Event) Event {
	switch e := event.(type) {
	case *Requested:
		return *e
	case *RoomAllocated:
		return *e
	case *Rejected:
		return *e
	case *Confirmed:
		return *e
	case *Cancelled:
		return *e
	case *Expired:
		return *e
	case *MarkedNoShow:
		return *e
	case *RegistrationCompleted:
		return *e
	case *CheckedIn:
		return *e
	case *CheckedOut:
		return *e
	case *StayChangeRequested:
		return *e
	case *StayChanged:
		return *e
	case *StayChangeRejected:
		return *e
	}
	return event
}
