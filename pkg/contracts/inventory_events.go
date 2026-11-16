package contracts

// Eventos do contexto de Inventário (disponibilidade, tarifas e alocações).
//
// Dois grupos convivem no mesmo tópico, com chaves de partição diferentes:
//   - allocation_*: respostas da saga de reserva (chave = ID da reserva);
//   - availability_changed / rate_changed: o que os canais de venda (Booking,
//     Airbnb) precisam saber (chave = ID do tipo de quarto).
const (
	TypeAllocationConfirmed      = "innkeep.inventory.allocation_confirmed.v1"
	TypeAllocationRejected       = "innkeep.inventory.allocation_rejected.v1"
	TypeAllocationReleased       = "innkeep.inventory.allocation_released.v1"
	TypeAllocationChanged        = "innkeep.inventory.allocation_changed.v1"
	TypeAllocationChangeRejected = "innkeep.inventory.allocation_change_rejected.v1"
	TypeAvailabilityChanged      = "innkeep.inventory.availability_changed.v1"
	TypeRateChanged              = "innkeep.inventory.rate_changed.v1"
)

// Motivos de rejeição de uma alocação.
const (
	RejectNoAvailability  = "NO_AVAILABILITY"   // nenhum quarto livre do tipo pedido
	RejectRoomTaken       = "ROOM_TAKEN"        // o quarto escolhido já está ocupado no período
	RejectClosed          = "CLOSED"            // venda fechada (stop-sell) em alguma data
	RejectMinStay         = "MIN_STAY"          // estadia menor que o mínimo exigido
	RejectUnknownRoomType = "UNKNOWN_ROOM_TYPE" // tipo de quarto inexistente ou inativo
	RejectOverCapacity    = "OVER_CAPACITY"     // mais hóspedes do que o quarto comporta
)

// NightlyRate é o preço de uma diária.
type NightlyRate struct {
	Date       string `json:"date"` // AAAA-MM-DD (a noite que começa nesta data)
	PriceCents int64  `json:"price_cents"`
}

// AllocationConfirmed: o inventário reservou um quarto para a reserva e
// calculou o preço. É a resposta "sim" da saga de criação de reserva.
type AllocationConfirmed struct {
	ReservationID string        `json:"reservation_id"`
	PropertyID    string        `json:"property_id"`
	RoomTypeID    string        `json:"room_type_id"`
	RoomID        string        `json:"room_id,omitempty"` // vazio quando houve overbooking
	RoomName      string        `json:"room_name,omitempty"`
	CheckIn       string        `json:"check_in"`
	CheckOut      string        `json:"check_out"`
	NightlyRates  []NightlyRate `json:"nightly_rates"`
	TotalCents    int64         `json:"total_cents"`
	Currency      string        `json:"currency"`
	// Overbooked indica que a reserva foi aceita SEM quarto livre. Só acontece
	// com reservas vindas de canais externos, que já estão confirmadas do lado
	// de lá: recusar não é opção, então registramos e alertamos a equipe.
	Overbooked bool `json:"overbooked"`
}

func (AllocationConfirmed) EventType() string      { return TypeAllocationConfirmed }
func (e AllocationConfirmed) PartitionKey() string { return e.ReservationID }

// AllocationRejected: não foi possível reservar um quarto. É a resposta "não"
// da saga; o serviço de reservas compensa marcando a reserva como rejeitada.
type AllocationRejected struct {
	ReservationID string `json:"reservation_id"`
	PropertyID    string `json:"property_id"`
	RoomTypeID    string `json:"room_type_id"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
	Reason        string `json:"reason"` // uma das constantes Reject*
	Detail        string `json:"detail,omitempty"`
}

func (AllocationRejected) EventType() string      { return TypeAllocationRejected }
func (e AllocationRejected) PartitionKey() string { return e.ReservationID }

// AllocationReleased: o quarto voltou a ficar livre (cancelamento, expiração,
// no-show ou check-out antecipado).
type AllocationReleased struct {
	ReservationID string `json:"reservation_id"`
	PropertyID    string `json:"property_id"`
	RoomTypeID    string `json:"room_type_id"`
	RoomID        string `json:"room_id,omitempty"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
}

func (AllocationReleased) EventType() string      { return TypeAllocationReleased }
func (e AllocationReleased) PartitionKey() string { return e.ReservationID }

// AllocationChanged: a alocação mudou de datas e/ou de quarto.
type AllocationChanged struct {
	ReservationID    string        `json:"reservation_id"`
	PropertyID       string        `json:"property_id"`
	RoomTypeID       string        `json:"room_type_id"`
	RoomID           string        `json:"room_id,omitempty"`
	RoomName         string        `json:"room_name,omitempty"`
	CheckIn          string        `json:"check_in"`
	CheckOut         string        `json:"check_out"`
	NightlyRates     []NightlyRate `json:"nightly_rates"`
	TotalCents       int64         `json:"total_cents"`
	Currency         string        `json:"currency"`
	PreviousRoomID   string        `json:"previous_room_id,omitempty"`
	PreviousCheckIn  string        `json:"previous_check_in"`
	PreviousCheckOut string        `json:"previous_check_out"`
}

func (AllocationChanged) EventType() string      { return TypeAllocationChanged }
func (e AllocationChanged) PartitionKey() string { return e.ReservationID }

// AllocationChangeRejected: a mudança de datas/quarto não pôde ser atendida;
// a alocação anterior continua valendo.
type AllocationChangeRejected struct {
	ReservationID string `json:"reservation_id"`
	PropertyID    string `json:"property_id"`
	Reason        string `json:"reason"`
	Detail        string `json:"detail,omitempty"`
}

func (AllocationChangeRejected) EventType() string      { return TypeAllocationChangeRejected }
func (e AllocationChangeRejected) PartitionKey() string { return e.ReservationID }

// DayAvailability é a disponibilidade de um tipo de quarto em uma data.
type DayAvailability struct {
	Date      string `json:"date"`
	Total     int    `json:"total"`     // quartos vendáveis do tipo
	Available int    `json:"available"` // quantos ainda podem ser vendidos (nunca negativo)
}

// AvailabilityChanged: a disponibilidade de um tipo de quarto mudou em uma ou
// mais datas. É ESTE o evento que mantém Booking.com e Airbnb sincronizados:
// o serviço de canais o consome e replica a mudança para fora.
type AvailabilityChanged struct {
	PropertyID string            `json:"property_id"`
	RoomTypeID string            `json:"room_type_id"`
	Days       []DayAvailability `json:"days"`
	// Cause documenta o que provocou a mudança (ex.: "reservation", "block",
	// "room_added"). Informativo — útil ao investigar o log de sincronização.
	Cause string `json:"cause,omitempty"`
}

func (AvailabilityChanged) EventType() string      { return TypeAvailabilityChanged }
func (e AvailabilityChanged) PartitionKey() string { return e.RoomTypeID }

// DayRate é a tarifa e as restrições de venda de um tipo de quarto em uma data.
type DayRate struct {
	Date       string `json:"date"`
	PriceCents int64  `json:"price_cents"`
	MinStay    int    `json:"min_stay"`
	Closed     bool   `json:"closed"` // stop-sell: não vender nesta data
}

// RateChanged: preço ou restrições mudaram em uma ou mais datas.
type RateChanged struct {
	PropertyID string    `json:"property_id"`
	RoomTypeID string    `json:"room_type_id"`
	Currency   string    `json:"currency"`
	Days       []DayRate `json:"days"`
}

func (RateChanged) EventType() string      { return TypeRateChanged }
func (e RateChanged) PartitionKey() string { return e.RoomTypeID }
