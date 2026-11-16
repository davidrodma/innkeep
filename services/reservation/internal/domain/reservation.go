// Package domain contém o agregado Reservation, modelado com Event Sourcing.
//
// Como ler este arquivo:
//
//  1. Os MÉTODOS DE COMANDO (Request, Confirm, Cancel, CheckIn...) validam as
//     regras de negócio contra o estado atual e, se tudo estiver certo,
//     registram um evento com `raise`. Eles NUNCA alteram campos diretamente.
//  2. O método `apply` é o único lugar que muda o estado — sempre a partir de
//     um evento. Ele não valida nada: se o evento existe, o fato aconteceu.
//  3. `FromHistory` reconstrói a reserva reaplicando seus eventos em ordem.
//
// Separar "decidir" (comando) de "mudar" (apply) é o que garante que o estado
// reconstruído a partir do histórico seja idêntico ao estado original.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Status é a etapa do ciclo de vida de uma reserva.
//
//	REQUESTED ──▶ PENDING ──▶ CONFIRMED ──▶ CHECKED_IN ──▶ CHECKED_OUT
//	    │            │            │
//	    ▼            ▼            ▼
//	 REJECTED   EXPIRED /     CANCELLED /
//	            CANCELLED      NO_SHOW
type Status string

const (
	StatusRequested  Status = "REQUESTED"   // aguardando o inventário alocar um quarto
	StatusPending    Status = "PENDING"     // quarto reservado; aguardando o sinal
	StatusConfirmed  Status = "CONFIRMED"   // garantida
	StatusCheckedIn  Status = "CHECKED_IN"  // hóspede na casa
	StatusCheckedOut Status = "CHECKED_OUT" // estadia encerrada
	StatusCancelled  Status = "CANCELLED"
	StatusRejected   Status = "REJECTED" // sem disponibilidade
	StatusExpired    Status = "EXPIRED"  // sinal não pago no prazo
	StatusNoShow     Status = "NO_SHOW"  // hóspede não compareceu
)

// Origens (canais de venda) de uma reserva.
const (
	SourceDirect     = "DIRECT"      // lançada pela equipe: balcão, telefone, WhatsApp
	SourceWebsite    = "WEBSITE"     // motor de reservas público
	SourceBookingCom = "BOOKING_COM" // Booking.com
	SourceAirbnb     = "AIRBNB"      // Airbnb
)

// Quem recebe o pagamento de uma reserva vinda de canal.
const (
	CollectProperty = "PROPERTY"
	CollectChannel  = "CHANNEL"
)

// Motivos de confirmação.
const (
	ConfirmedByDeposit = "DEPOSIT_PAID"
	ConfirmedManually  = "MANUAL"
	ConfirmedByChannel = "CHANNEL"
	ConfirmedNoDeposit = "NO_DEPOSIT_REQUIRED"
)

// Erros de domínio do contexto.
var (
	ErrNotFound = kernel.NotFound("reservation.not_found", "reserva não encontrada")
)

func errTransition(action string, status Status) *kernel.Error {
	return kernel.Rule("reservation.invalid_transition", "não é possível %s uma reserva com situação %s", action, status)
}

// Holder é o titular: a pessoa de contato da reserva.
type Holder struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Policy é a política comercial da propriedade vigente no momento do pedido.
type Policy struct {
	DepositPercent int
	HoldMinutes    int
	Currency       string
}

// StayChange é uma mudança de estadia aguardando resposta do inventário.
type StayChange struct {
	NewRoomID   string      `json:"new_room_id,omitempty"`
	NewCheckIn  kernel.Date `json:"new_check_in"`
	NewCheckOut kernel.Date `json:"new_check_out"`
}

// State é a fotografia do estado atual da reserva. É derivado dos eventos e
// serve de base para o modelo de leitura e para os eventos de integração.
type State struct {
	ID              string
	Code            string
	PropertyID      string
	RoomTypeID      string
	PreferredRoomID string
	RoomID          string
	RoomName        string
	Stay            kernel.StayPeriod
	Adults          int
	Children        int
	Holder          Holder
	Source          string
	ExternalRef     string
	GroupCode       string
	Notes           string
	Status          Status

	NightlyRates    []NightlyRate
	TotalCents      int64
	DepositDueCents int64
	Currency        string
	DepositPercent  int
	HoldMinutes     int
	HoldExpiresAt   *time.Time
	PaymentCollect  string
	Overbooked      bool

	QuotedTotalCents    *int64
	ConfirmOnAllocation bool
	EnforceRestrictions bool
	AllowOverbooking    bool

	RegistrationCompleted bool
	PrimaryGuestID        string
	GuestCount            int
	PendingChange         *StayChange

	CancelReason string
	CancelledBy  string
	RejectReason string
	RejectDetail string

	RequestedAt  time.Time
	ConfirmedAt  *time.Time
	CheckedInAt  *time.Time
	CheckedOutAt *time.Time
	ClosedAt     *time.Time // cancelada, rejeitada, expirada ou no-show

	// Version é o número de eventos já aplicados (controle de concorrência).
	Version int
}

// Reservation é a raiz do agregado.
type Reservation struct {
	state   State
	changes []Event // eventos novos, ainda não gravados no Event Store
}

// State devolve uma cópia do estado atual.
func (r *Reservation) State() State { return r.state }

// ID devolve o identificador da reserva.
func (r *Reservation) ID() string { return r.state.ID }

// Changes devolve os eventos produzidos desde a última gravação.
func (r *Reservation) Changes() []Event { return r.changes }

// PersistedVersion é a versão que estava gravada quando o agregado foi
// carregado — o "expectedVersion" da concorrência otimista.
func (r *Reservation) PersistedVersion() int { return r.state.Version - len(r.changes) }

// ClearChanges marca os eventos pendentes como gravados.
func (r *Reservation) ClearChanges() { r.changes = nil }

// FromHistory reconstrói uma reserva a partir do seu histórico de eventos.
func FromHistory(events []Event) *Reservation {
	reservation := &Reservation{}
	for _, event := range events {
		reservation.apply(event)
	}
	return reservation
}

// raise registra um novo evento: aplica-o ao estado e o guarda para gravação.
func (r *Reservation) raise(event Event) {
	r.apply(event)
	r.changes = append(r.changes, event)
}

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

// RequestInput são os dados de um pedido de reserva.
type RequestInput struct {
	ID              string // opcional; gerado se vazio (reservas de canal usam ID determinístico)
	Code            string
	PropertyID      string
	RoomTypeID      string
	PreferredRoomID string
	Stay            kernel.StayPeriod
	Adults          int
	Children        int
	Holder          Holder
	Source          string
	ExternalRef     string
	GroupCode       string
	Notes           string
	// QuotedTotalCents: preço fechado fora da tabela (equipe ou canal).
	QuotedTotalCents *int64
	// ConfirmNow: confirmar assim que o quarto for alocado, sem exigir sinal.
	ConfirmNow     bool
	PaymentCollect string
}

// Request cria uma reserva a partir de um pedido válido.
//
// `today` é a data corrente NO FUSO DA PROPRIEDADE: o domínio não consulta o
// relógio por conta própria — receber o tempo como parâmetro mantém as regras
// determinísticas e fáceis de testar.
func Request(input RequestInput, policy Policy, now time.Time, today kernel.Date) (*Reservation, error) {
	input.Holder.Name = strings.TrimSpace(input.Holder.Name)
	input.Holder.Email = strings.ToLower(strings.TrimSpace(input.Holder.Email))
	input.Holder.Phone = strings.TrimSpace(input.Holder.Phone)

	if !kernel.IsID(input.PropertyID) {
		return nil, kernel.Invalid("reservation.invalid_property", "informe a propriedade").WithField("property_id", "obrigatório")
	}
	if !kernel.IsID(input.RoomTypeID) {
		return nil, kernel.Invalid("reservation.invalid_room_type", "informe o tipo de acomodação").WithField("room_type_id", "obrigatório")
	}
	if input.Stay.IsZero() {
		return nil, kernel.Invalid("stay.dates_required", "informe as datas de check-in e check-out")
	}
	if n := utf8.RuneCountInString(input.Holder.Name); n < 2 || n > 120 {
		return nil, kernel.Invalid("reservation.invalid_holder", "informe o nome do titular da reserva").WithField("holder.name", "entre 2 e 120 caracteres")
	}
	if input.Adults < 1 || input.Adults > 50 {
		return nil, kernel.Invalid("reservation.invalid_guests", "informe ao menos um adulto").WithField("adults", "entre 1 e 50")
	}
	if input.Children < 0 || input.Children > 50 {
		return nil, kernel.Invalid("reservation.invalid_guests", "número de crianças inválido").WithField("children", "entre 0 e 50")
	}
	if input.QuotedTotalCents != nil && *input.QuotedTotalCents < 0 {
		return nil, kernel.Invalid("reservation.invalid_price", "o valor negociado não pode ser negativo").WithField("quoted_total_cents", "inválido")
	}

	fromChannel := input.Source == SourceBookingCom || input.Source == SourceAirbnb
	switch input.Source {
	case SourceWebsite:
		// O hóspede precisa ser contatável e não pode reservar para o passado.
		if input.Holder.Email == "" && input.Holder.Phone == "" {
			return nil, kernel.Invalid("reservation.contact_required", "informe um e-mail ou telefone para contato").WithField("holder.email", "obrigatório")
		}
		if input.Stay.CheckIn().Before(today) {
			return nil, kernel.Invalid("stay.in_the_past", "a data de check-in já passou").WithField("check_in", "não pode ser no passado")
		}
	case SourceDirect:
		// A equipe pode lançar uma entrada do dia anterior (walk-in de madrugada).
		if input.Stay.CheckIn().Before(today.AddDays(-1)) {
			return nil, kernel.Invalid("stay.in_the_past", "a data de check-in já passou").WithField("check_in", "não pode ser no passado")
		}
	default:
		if !fromChannel {
			return nil, kernel.Invalid("reservation.invalid_source", "origem da reserva inválida").WithField("source", "valor desconhecido")
		}
	}

	if input.ID == "" {
		input.ID = kernel.NewID()
	}
	if input.PaymentCollect == "" {
		input.PaymentCollect = CollectProperty
	}

	reservation := &Reservation{}
	reservation.raise(Requested{
		ReservationID: input.ID, Code: input.Code,
		PropertyID: input.PropertyID, RoomTypeID: input.RoomTypeID, PreferredRoomID: input.PreferredRoomID,
		CheckIn: input.Stay.CheckIn(), CheckOut: input.Stay.CheckOut(),
		Adults: input.Adults, Children: input.Children, Holder: input.Holder,
		Source: input.Source, ExternalRef: input.ExternalRef, GroupCode: input.GroupCode,
		Notes:          strings.TrimSpace(input.Notes),
		DepositPercent: policy.DepositPercent, HoldMinutes: policy.HoldMinutes, Currency: policy.Currency,
		QuotedTotalCents: input.QuotedTotalCents,
		// Reservas de canal já chegam confirmadas do lado de lá.
		ConfirmOnAllocation: input.ConfirmNow || fromChannel,
		PaymentCollect:      input.PaymentCollect,
		// Só o site público obedece estadia mínima e datas fechadas; só
		// reservas de canal podem ser aceitas em overbooking.
		EnforceRestrictions: input.Source == SourceWebsite,
		AllowOverbooking:    fromChannel,
		At:                  now,
	})
	return reservation, nil
}

// Allocation é o resultado da alocação feita pelo inventário.
type Allocation struct {
	RoomTypeID   string
	RoomID       string
	RoomName     string
	NightlyRates []NightlyRate
	TotalCents   int64
	Overbooked   bool
}

// RecordAllocation registra que o inventário reservou o quarto. Define o
// sinal exigido e o prazo para pagá-lo — ou confirma direto, quando não há
// sinal a cobrar. Chegar fora de hora (reserva já cancelada, evento repetido)
// não é erro: o fato simplesmente não se aplica mais.
func (r *Reservation) RecordAllocation(allocation Allocation, now time.Time) {
	if r.state.Status != StatusRequested {
		return
	}
	deposit := kernel.NewMoney(allocation.TotalCents, r.state.Currency).Percent(r.state.DepositPercent)
	confirmNow := r.state.ConfirmOnAllocation || deposit.IsZero()

	event := RoomAllocated{
		RoomTypeID: allocation.RoomTypeID, RoomID: allocation.RoomID, RoomName: allocation.RoomName,
		NightlyRates: allocation.NightlyRates, TotalCents: allocation.TotalCents,
		Overbooked: allocation.Overbooked, At: now,
	}
	if !confirmNow {
		event.DepositDueCents = deposit.Cents()
		if r.state.HoldMinutes > 0 {
			expires := now.Add(time.Duration(r.state.HoldMinutes) * time.Minute)
			event.HoldExpiresAt = &expires
		}
	}
	r.raise(event)

	if confirmNow {
		reason := ConfirmedNoDeposit
		switch {
		case r.state.AllowOverbooking:
			reason = ConfirmedByChannel
		case r.state.ConfirmOnAllocation:
			reason = ConfirmedManually
		}
		r.raise(Confirmed{Reason: reason, At: now})
	}
}

// RecordRejection registra que o inventário não conseguiu atender o pedido.
func (r *Reservation) RecordRejection(reason, detail string, now time.Time) {
	if r.state.Status != StatusRequested {
		return
	}
	r.raise(Rejected{Reason: reason, Detail: detail, At: now})
}

// Confirm garante a reserva. Idempotente: confirmar o que já está confirmado
// não faz nada (o evento "sinal pago" pode chegar depois de uma confirmação
// manual, por exemplo).
func (r *Reservation) Confirm(reason string, now time.Time) error {
	switch r.state.Status {
	case StatusConfirmed, StatusCheckedIn, StatusCheckedOut:
		return nil
	case StatusPending:
		r.raise(Confirmed{Reason: reason, At: now})
		return nil
	default:
		return errTransition("confirmar", r.state.Status)
	}
}

// Cancel cancela a reserva.
func (r *Reservation) Cancel(reason, by string, now time.Time) error {
	switch r.state.Status {
	case StatusCancelled:
		return nil
	case StatusRequested, StatusPending, StatusConfirmed:
		r.raise(Cancelled{Reason: strings.TrimSpace(reason), By: by, At: now})
		return nil
	default:
		return errTransition("cancelar", r.state.Status)
	}
}

// Expire encerra a reserva cujo prazo de pagamento do sinal venceu.
func (r *Reservation) Expire(now time.Time) {
	if r.state.Status != StatusPending || r.state.HoldExpiresAt == nil || now.Before(*r.state.HoldExpiresAt) {
		return
	}
	r.raise(Expired{At: now})
}

// MarkNoShow registra que o hóspede não compareceu.
func (r *Reservation) MarkNoShow(now time.Time, today kernel.Date) error {
	if r.state.Status != StatusConfirmed && r.state.Status != StatusPending {
		return errTransition("marcar como no-show", r.state.Status)
	}
	if today.Before(r.state.Stay.CheckIn()) {
		return kernel.Rule("reservation.no_show_too_early", "só é possível marcar no-show a partir da data de check-in")
	}
	r.raise(MarkedNoShow{At: now})
	return nil
}

// RecordRegistration registra que a FNRH foi preenchida.
func (r *Reservation) RecordRegistration(primaryGuestID string, guestCount int, now time.Time) {
	if r.state.RegistrationCompleted && r.state.PrimaryGuestID == primaryGuestID && r.state.GuestCount == guestCount {
		return
	}
	r.raise(RegistrationCompleted{PrimaryGuestID: primaryGuestID, GuestCount: guestCount, At: now})
}

// CheckIn registra a entrada do hóspede.
//
// A FNRH (Ficha Nacional de Registro de Hóspedes) é exigência legal no Brasil
// (Lei 11.771/2008, a Lei Geral do Turismo): o check-in só é permitido depois
// que o serviço de hóspedes avisa, por evento, que a ficha foi preenchida.
func (r *Reservation) CheckIn(now time.Time, today kernel.Date) error {
	if r.state.Status != StatusConfirmed {
		return errTransition("fazer check-in de", r.state.Status)
	}
	if today.Before(r.state.Stay.CheckIn()) {
		return kernel.Rule("reservation.check_in_too_early", "o check-in só pode ser feito a partir de %s", r.state.Stay.CheckIn())
	}
	if !today.Before(r.state.Stay.CheckOut()) {
		return kernel.Rule("reservation.check_in_too_late", "o período da reserva já terminou")
	}
	if r.state.RoomID == "" {
		return kernel.Rule("reservation.no_room_assigned", "a reserva está em overbooking: atribua um quarto antes do check-in")
	}
	if !r.state.RegistrationCompleted {
		return kernel.Rule("reservation.registration_required", "preencha a ficha de registro do hóspede (FNRH) antes do check-in")
	}
	r.raise(CheckedIn{At: now})
	return nil
}

// CheckOut registra a saída do hóspede.
func (r *Reservation) CheckOut(now time.Time, today kernel.Date) error {
	if r.state.Status != StatusCheckedIn {
		return errTransition("fazer check-out de", r.state.Status)
	}
	// Saída antecipada libera as noites restantes; saída "atrasada" não
	// estende a ocupação (para isso existe a mudança de estadia).
	actual := today
	if actual.After(r.state.Stay.CheckOut()) {
		actual = r.state.Stay.CheckOut()
	}
	if actual.Before(r.state.Stay.CheckIn()) {
		actual = r.state.Stay.CheckIn()
	}
	r.raise(CheckedOut{ActualCheckOut: actual, At: now})
	return nil
}

// RequestStayChange pede a mudança de datas e/ou de quarto. A mudança só vale
// depois que o inventário confirmar (StayChanged).
func (r *Reservation) RequestStayChange(newStay kernel.StayPeriod, newRoomID string, quotedTotalCents *int64, now time.Time) error {
	switch r.state.Status {
	case StatusPending, StatusConfirmed, StatusCheckedIn:
	default:
		return errTransition("alterar a estadia de", r.state.Status)
	}
	if r.state.PendingChange != nil {
		return kernel.Conflict("reservation.change_in_progress", "já existe uma alteração de estadia em andamento")
	}
	if r.state.Status == StatusCheckedIn && !newStay.CheckIn().Equal(r.state.Stay.CheckIn()) {
		return kernel.Rule("reservation.check_in_already_done", "o hóspede já entrou: a data de check-in não pode mudar")
	}
	if newStay.Equal(r.state.Stay) && (newRoomID == "" || newRoomID == r.state.RoomID) && quotedTotalCents == nil {
		return kernel.Invalid("reservation.nothing_to_change", "nenhuma alteração informada")
	}
	if quotedTotalCents != nil && *quotedTotalCents < 0 {
		return kernel.Invalid("reservation.invalid_price", "o valor negociado não pode ser negativo")
	}
	r.raise(StayChangeRequested{
		NewRoomID: newRoomID, NewCheckIn: newStay.CheckIn(), NewCheckOut: newStay.CheckOut(),
		QuotedTotalCents: quotedTotalCents, At: now,
	})
	return nil
}

// RecordStayChange registra que o inventário efetivou a mudança.
func (r *Reservation) RecordStayChange(allocation Allocation, stay kernel.StayPeriod, now time.Time) {
	if r.state.PendingChange == nil {
		return
	}
	r.raise(StayChanged{
		RoomTypeID: allocation.RoomTypeID, RoomID: allocation.RoomID, RoomName: allocation.RoomName,
		CheckIn: stay.CheckIn(), CheckOut: stay.CheckOut(),
		NightlyRates: allocation.NightlyRates, TotalCents: allocation.TotalCents, At: now,
	})
}

// RecordStayChangeRejection registra que a mudança não pôde ser atendida.
func (r *Reservation) RecordStayChangeRejection(reason, detail string, now time.Time) {
	if r.state.PendingChange == nil {
		return
	}
	r.raise(StayChangeRejected{Reason: reason, Detail: detail, At: now})
}

// ---------------------------------------------------------------------------
// Aplicação de eventos: o único ponto que altera o estado.
// ---------------------------------------------------------------------------

func (r *Reservation) apply(event Event) {
	s := &r.state
	switch e := event.(type) {
	case Requested:
		s.ID, s.Code = e.ReservationID, e.Code
		s.PropertyID, s.RoomTypeID, s.PreferredRoomID = e.PropertyID, e.RoomTypeID, e.PreferredRoomID
		// Os eventos gravados já passaram pela validação; o erro é impossível.
		s.Stay, _ = kernel.NewStayPeriod(e.CheckIn, e.CheckOut)
		s.Adults, s.Children, s.Holder = e.Adults, e.Children, e.Holder
		s.Source, s.ExternalRef, s.GroupCode, s.Notes = e.Source, e.ExternalRef, e.GroupCode, e.Notes
		s.DepositPercent, s.HoldMinutes, s.Currency = e.DepositPercent, e.HoldMinutes, e.Currency
		s.QuotedTotalCents, s.ConfirmOnAllocation = e.QuotedTotalCents, e.ConfirmOnAllocation
		s.PaymentCollect = e.PaymentCollect
		s.EnforceRestrictions, s.AllowOverbooking = e.EnforceRestrictions, e.AllowOverbooking
		s.Status, s.RequestedAt = StatusRequested, e.At
		if e.QuotedTotalCents != nil {
			s.TotalCents = *e.QuotedTotalCents
		}
	case RoomAllocated:
		s.RoomTypeID, s.RoomID, s.RoomName = e.RoomTypeID, e.RoomID, e.RoomName
		s.NightlyRates, s.TotalCents = e.NightlyRates, e.TotalCents
		s.DepositDueCents, s.HoldExpiresAt, s.Overbooked = e.DepositDueCents, e.HoldExpiresAt, e.Overbooked
		s.Status = StatusPending
	case Rejected:
		s.Status, s.RejectReason, s.RejectDetail, s.ClosedAt = StatusRejected, e.Reason, e.Detail, &e.At
	case Confirmed:
		s.Status, s.ConfirmedAt, s.HoldExpiresAt = StatusConfirmed, &e.At, nil
	case Cancelled:
		s.Status, s.CancelReason, s.CancelledBy, s.ClosedAt = StatusCancelled, e.Reason, e.By, &e.At
		s.PendingChange, s.HoldExpiresAt = nil, nil
	case Expired:
		s.Status, s.ClosedAt, s.HoldExpiresAt = StatusExpired, &e.At, nil
	case MarkedNoShow:
		s.Status, s.ClosedAt = StatusNoShow, &e.At
	case RegistrationCompleted:
		s.RegistrationCompleted, s.PrimaryGuestID, s.GuestCount = true, e.PrimaryGuestID, e.GuestCount
	case CheckedIn:
		s.Status, s.CheckedInAt = StatusCheckedIn, &e.At
	case CheckedOut:
		s.Status, s.CheckedOutAt = StatusCheckedOut, &e.At
	case StayChangeRequested:
		s.PendingChange = &StayChange{NewRoomID: e.NewRoomID, NewCheckIn: e.NewCheckIn, NewCheckOut: e.NewCheckOut}
	case StayChanged:
		s.RoomTypeID, s.RoomID, s.RoomName = e.RoomTypeID, e.RoomID, e.RoomName
		s.Stay, _ = kernel.NewStayPeriod(e.CheckIn, e.CheckOut)
		s.NightlyRates, s.TotalCents = e.NightlyRates, e.TotalCents
		s.Overbooked = e.RoomID == ""
		s.PendingChange = nil
	case StayChangeRejected:
		s.PendingChange = nil
	}
	s.Version++
}
