package contracts

import "time"

// Eventos do contexto de Reservas. Chave de partição: o ID da reserva.
const (
	TypeReservationRequested           = "innkeep.reservation.requested.v1"
	TypeReservationAccepted            = "innkeep.reservation.accepted.v1"
	TypeReservationRejected            = "innkeep.reservation.rejected.v1"
	TypeReservationConfirmed           = "innkeep.reservation.confirmed.v1"
	TypeReservationCancelled           = "innkeep.reservation.cancelled.v1"
	TypeReservationExpired             = "innkeep.reservation.expired.v1"
	TypeReservationNoShow              = "innkeep.reservation.no_show.v1"
	TypeReservationStayChangeRequested = "innkeep.reservation.stay_change_requested.v1"
	TypeReservationStayChanged         = "innkeep.reservation.stay_changed.v1"
	TypeReservationCheckedIn           = "innkeep.reservation.checked_in.v1"
	TypeReservationCheckedOut          = "innkeep.reservation.checked_out.v1"
)

// Origens (canais de venda) de uma reserva.
const (
	SourceDirect     = "DIRECT"      // lançada pela equipe: balcão, telefone, WhatsApp
	SourceWebsite    = "WEBSITE"     // motor de reservas público do InnKeep
	SourceBookingCom = "BOOKING_COM" // Booking.com
	SourceAirbnb     = "AIRBNB"      // Airbnb
)

// Quem recebe o pagamento de uma reserva vinda de canal externo.
const (
	CollectProperty = "PROPERTY" // o hóspede paga na propriedade
	CollectChannel  = "CHANNEL"  // o canal cobra o hóspede e repassa o valor líquido
)

// Motivos de confirmação.
const (
	ConfirmedByDeposit = "DEPOSIT_PAID"        // o sinal foi pago
	ConfirmedManually  = "MANUAL"              // a equipe confirmou sem sinal
	ConfirmedByChannel = "CHANNEL"             // já veio confirmada do canal externo
	ConfirmedNoDeposit = "NO_DEPOSIT_REQUIRED" // a propriedade não exige sinal
)

// Holder é o titular da reserva: a pessoa de contato. Guardamos só o mínimo
// necessário para operar (LGPD: minimização de dados). Documentos e endereço
// ficam exclusivamente no serviço de hóspedes, criptografados.
type Holder struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// ReservationSummary é o resumo da reserva embutido na maioria dos eventos,
// para que cada consumidor tenha o contexto de que precisa sem consultar o
// serviço de reservas.
type ReservationSummary struct {
	ReservationID string `json:"reservation_id"`
	Code          string `json:"code"` // localizador curto exibido ao hóspede, ex.: "IK-7F3K9Q"
	PropertyID    string `json:"property_id"`
	RoomTypeID    string `json:"room_type_id"`
	RoomID        string `json:"room_id,omitempty"`
	RoomName      string `json:"room_name,omitempty"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
	Adults        int    `json:"adults"`
	Children      int    `json:"children"`
	Holder        Holder `json:"holder"`
	Source        string `json:"source"`
	ExternalRef   string `json:"external_ref,omitempty"` // nº da reserva no canal externo
	TotalCents    int64  `json:"total_cents"`
	Currency      string `json:"currency"`
}

// ReservationRequested: alguém pediu uma reserva. É o primeiro passo da saga:
// o inventário reage tentando alocar um quarto.
type ReservationRequested struct {
	ReservationSummary
	// PreferredRoomID: quarto específico escolhido pela equipe (opcional).
	PreferredRoomID string `json:"preferred_room_id,omitempty"`
	// EnforceRestrictions: respeitar estadia mínima e datas fechadas. Verdadeiro
	// para o site público; a equipe e os canais externos podem ignorá-las.
	EnforceRestrictions bool `json:"enforce_restrictions"`
	// AllowOverbooking: aceitar mesmo sem quarto livre (reservas de canais).
	AllowOverbooking bool `json:"allow_overbooking"`
	// QuotedTotalCents: preço fechado fora da tabela (negociado pela equipe ou
	// definido pelo canal). Nulo = usar a tarifa vigente.
	QuotedTotalCents *int64 `json:"quoted_total_cents,omitempty"`
}

func (ReservationRequested) EventType() string      { return TypeReservationRequested }
func (e ReservationRequested) PartitionKey() string { return e.ReservationID }

// ReservationAccepted: o quarto foi alocado e o preço definido. A reserva pode
// ainda depender do pagamento do sinal (Confirmed=false) ou já estar
// confirmada. O financeiro reage abrindo a conta (folio) do hóspede.
type ReservationAccepted struct {
	ReservationSummary
	NightlyRates    []NightlyRate `json:"nightly_rates"`
	DepositDueCents int64         `json:"deposit_due_cents"`
	HoldExpiresAt   *time.Time    `json:"hold_expires_at,omitempty"`
	PaymentCollect  string        `json:"payment_collect"`
	Overbooked      bool          `json:"overbooked"`
	Confirmed       bool          `json:"confirmed"`
}

func (ReservationAccepted) EventType() string      { return TypeReservationAccepted }
func (e ReservationAccepted) PartitionKey() string { return e.ReservationID }

// ReservationRejected: a reserva não pôde ser atendida (sem disponibilidade).
type ReservationRejected struct {
	ReservationSummary
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

func (ReservationRejected) EventType() string      { return TypeReservationRejected }
func (e ReservationRejected) PartitionKey() string { return e.ReservationID }

// ReservationConfirmed: a reserva está garantida.
type ReservationConfirmed struct {
	ReservationSummary
	Reason string `json:"reason"` // uma das constantes ConfirmedBy*
}

func (ReservationConfirmed) EventType() string      { return TypeReservationConfirmed }
func (e ReservationConfirmed) PartitionKey() string { return e.ReservationID }

// ReservationCancelled: a reserva foi cancelada; o inventário libera o quarto.
type ReservationCancelled struct {
	ReservationSummary
	Reason      string `json:"reason,omitempty"`
	CancelledBy string `json:"cancelled_by,omitempty"` // "staff:<id>", "channel", "guest"
}

func (ReservationCancelled) EventType() string      { return TypeReservationCancelled }
func (e ReservationCancelled) PartitionKey() string { return e.ReservationID }

// ReservationExpired: o prazo do sinal venceu sem pagamento.
type ReservationExpired struct{ ReservationSummary }

func (ReservationExpired) EventType() string      { return TypeReservationExpired }
func (e ReservationExpired) PartitionKey() string { return e.ReservationID }

// ReservationNoShow: o hóspede não apareceu.
type ReservationNoShow struct{ ReservationSummary }

func (ReservationNoShow) EventType() string      { return TypeReservationNoShow }
func (e ReservationNoShow) PartitionKey() string { return e.ReservationID }

// ReservationStayChangeRequested: pedido de mudança de datas e/ou quarto.
// Abre uma mini-saga: o inventário tenta mover a alocação e responde com
// AllocationChanged ou AllocationChangeRejected.
type ReservationStayChangeRequested struct {
	ReservationID    string `json:"reservation_id"`
	PropertyID       string `json:"property_id"`
	RoomTypeID       string `json:"room_type_id"`
	NewRoomID        string `json:"new_room_id,omitempty"` // vazio = manter o quarto atual, se possível
	NewCheckIn       string `json:"new_check_in"`
	NewCheckOut      string `json:"new_check_out"`
	QuotedTotalCents *int64 `json:"quoted_total_cents,omitempty"`
}

func (ReservationStayChangeRequested) EventType() string      { return TypeReservationStayChangeRequested }
func (e ReservationStayChangeRequested) PartitionKey() string { return e.ReservationID }

// ReservationStayChanged: datas, quarto e/ou valor foram alterados. O
// financeiro reage reajustando os lançamentos de hospedagem.
type ReservationStayChanged struct {
	ReservationSummary
	NightlyRates []NightlyRate `json:"nightly_rates"`
}

func (ReservationStayChanged) EventType() string      { return TypeReservationStayChanged }
func (e ReservationStayChanged) PartitionKey() string { return e.ReservationID }

// ReservationCheckedIn: o hóspede entrou.
type ReservationCheckedIn struct {
	ReservationSummary
	At time.Time `json:"at"`
}

func (ReservationCheckedIn) EventType() string      { return TypeReservationCheckedIn }
func (e ReservationCheckedIn) PartitionKey() string { return e.ReservationID }

// ReservationCheckedOut: o hóspede saiu. ActualCheckOut pode ser anterior ao
// previsto (saída antecipada): o inventário libera as noites restantes.
type ReservationCheckedOut struct {
	ReservationSummary
	At             time.Time `json:"at"`
	ActualCheckOut string    `json:"actual_check_out"`
}

func (ReservationCheckedOut) EventType() string      { return TypeReservationCheckedOut }
func (e ReservationCheckedOut) PartitionKey() string { return e.ReservationID }
