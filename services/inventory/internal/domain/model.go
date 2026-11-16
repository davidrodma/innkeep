// Package domain contém o modelo do contexto de Inventário: o que está livre
// e quanto custa em cada data.
//
// No setor, esse trio é conhecido como ARI — Availability, Rates, Inventory —
// e é exatamente o que um channel manager envia às agências online (OTAs).
package domain

import (
	"strconv"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Erros de domínio do contexto.
var (
	ErrRoomTypeNotFound   = kernel.NotFound("inventory.room_type_not_found", "tipo de acomodação não encontrado")
	ErrRoomNotFound       = kernel.NotFound("inventory.room_not_found", "quarto não encontrado")
	ErrAllocationNotFound = kernel.NotFound("inventory.allocation_not_found", "alocação não encontrada")
	// ErrRoomTaken: o quarto já está ocupado ou bloqueado em alguma noite do
	// período. É o banco de dados quem detecta (restrição EXCLUDE).
	ErrRoomTaken = kernel.Conflict("inventory.room_taken", "o quarto já está ocupado ou bloqueado no período")
)

// RoomType é a cópia local de um tipo de acomodação. A fonte da verdade é o
// serviço de propriedades; esta cópia é mantida pelos eventos que ele publica
// (padrão "Event-Carried State Transfer"). O inventário nunca consulta o
// serviço de propriedades — e segue funcionando se ele cair.
type RoomType struct {
	ID             string
	PropertyID     string
	Name           string
	MaxOccupancy   int
	BasePriceCents int64
	Currency       string
	Active         bool
}

// Room é a cópia local de um quarto.
type Room struct {
	ID         string
	PropertyID string
	RoomTypeID string
	Name       string
	Active     bool
}

// AllocationKind distingue por que um quarto está indisponível.
type AllocationKind string

const (
	AllocationReservation AllocationKind = "RESERVATION"
	AllocationBlock       AllocationKind = "BLOCK" // manutenção, uso do proprietário etc.
)

// Allocation é a ocupação de um quarto em um período.
type Allocation struct {
	ID            string
	PropertyID    string
	RoomTypeID    string
	RoomID        string // vazio = overbooking (reserva aceita sem quarto livre)
	RoomName      string // preenchido nas leituras
	ReservationID string // vazio para bloqueios
	Kind          AllocationKind
	Stay          kernel.StayPeriod
	Reason        string
	CreatedAt     time.Time
}

// DayStatus é a situação de um tipo de quarto em uma data.
type DayStatus struct {
	Date       kernel.Date
	Total      int // quartos vendáveis do tipo
	Occupied   int // reservas + bloqueios (inclui overbooking)
	PriceCents int64
	MinStay    int
	Closed     bool // venda fechada (stop-sell)
}

// Available devolve quantos quartos ainda podem ser vendidos. Nunca é
// negativo: em caso de overbooking, os canais devem enxergar zero.
func (d DayStatus) Available() int { return max(d.Total-d.Occupied, 0) }

// Motivos pelos quais uma estadia não pode ser vendida. Os valores coincidem
// com os do contrato de eventos (pkg/contracts), mas o domínio define os seus:
// ele não depende do formato das mensagens trocadas entre serviços.
const (
	RejectNoAvailability  = "NO_AVAILABILITY"
	RejectRoomTaken       = "ROOM_TAKEN"
	RejectClosed          = "CLOSED"
	RejectMinStay         = "MIN_STAY"
	RejectUnknownRoomType = "UNKNOWN_ROOM_TYPE"
	RejectOverCapacity    = "OVER_CAPACITY"
)

// NightlyRate é o preço de uma diária.
type NightlyRate struct {
	Date       kernel.Date `json:"date"`
	PriceCents int64       `json:"price_cents"`
}

// Quote é o preço de uma estadia, noite a noite.
type Quote struct {
	Nightly []NightlyRate
	Total   kernel.Money
}

// QuoteFromRates precifica a estadia pela tarifa vigente de cada noite.
func QuoteFromRates(days []DayStatus, currency string) Quote {
	quote := Quote{Total: kernel.Zero(currency)}
	for _, day := range days {
		quote.Nightly = append(quote.Nightly, NightlyRate{Date: day.Date, PriceCents: day.PriceCents})
		quote.Total = quote.Total.Add(kernel.NewMoney(day.PriceCents, currency))
	}
	return quote
}

// QuoteFixed distribui um preço fechado (negociado pela equipe ou definido
// pelo canal) entre as noites, sem perder centavos na divisão.
func QuoteFixed(stay kernel.StayPeriod, total kernel.Money) Quote {
	nights := stay.NightDates()
	parts := total.Split(len(nights))
	quote := Quote{Total: total}
	for i, night := range nights {
		quote.Nightly = append(quote.Nightly, NightlyRate{Date: night, PriceCents: parts[i].Cents()})
	}
	return quote
}

// Rejection explica por que uma estadia não pode ser vendida.
type Rejection struct {
	Reason string // uma das constantes Reject*
	Detail string
}

// CheckRestrictions aplica as restrições de venda: datas fechadas e estadia
// mínima (regra do setor: vale a estadia mínima do dia de CHEGADA).
func CheckRestrictions(days []DayStatus) *Rejection {
	if len(days) == 0 {
		return nil
	}
	for _, day := range days {
		if day.Closed {
			return &Rejection{Reason: RejectClosed, Detail: "venda fechada em " + day.Date.String()}
		}
	}
	if minStay := days[0].MinStay; len(days) < minStay {
		return &Rejection{Reason: RejectMinStay, Detail: "estadia mínima de " + strconv.Itoa(minStay) + " noite(s)"}
	}
	return nil
}

// MinAvailable devolve a menor disponibilidade entre as noites: uma estadia
// só cabe se houver quarto em TODAS as noites.
func MinAvailable(days []DayStatus) int {
	if len(days) == 0 {
		return 0
	}
	lowest := days[0].Available()
	for _, day := range days[1:] {
		lowest = min(lowest, day.Available())
	}
	return lowest
}

// Offer é uma opção de hospedagem apresentada na busca de disponibilidade.
type Offer struct {
	RoomTypeID   string        `json:"room_type_id"`
	Name         string        `json:"name"`
	MaxOccupancy int           `json:"max_occupancy"`
	Available    int           `json:"available"`
	Nightly      []NightlyRate `json:"nightly_rates"`
	TotalCents   int64         `json:"total_cents"`
	Currency     string        `json:"currency"`
	Bookable     bool          `json:"bookable"`
	Reason       string        `json:"reason,omitempty"` // por que não é reservável
}

// BuildOffer monta a oferta de um tipo de quarto para uma estadia.
func BuildOffer(roomType RoomType, days []DayStatus, guests int) Offer {
	quote := QuoteFromRates(days, roomType.Currency)
	offer := Offer{
		RoomTypeID: roomType.ID, Name: roomType.Name, MaxOccupancy: roomType.MaxOccupancy,
		Available: MinAvailable(days), Nightly: quote.Nightly,
		TotalCents: quote.Total.Cents(), Currency: roomType.Currency,
	}
	switch rejection := CheckRestrictions(days); {
	case guests > roomType.MaxOccupancy:
		offer.Reason = RejectOverCapacity
	case rejection != nil:
		offer.Reason = rejection.Reason
	case offer.Available < 1:
		offer.Reason = RejectNoAvailability
	default:
		offer.Bookable = true
	}
	return offer
}
