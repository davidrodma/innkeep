// Package port define as portas (interfaces) e os modelos de leitura da camada
// de aplicação do serviço de reservas.
//
// Este serviço aplica CQRS (Command Query Responsibility Segregation):
//
//   - ESCRITA (app/command): comandos carregam o agregado do Event Store,
//     aplicam regras e gravam novos eventos. Otimizado para consistência.
//   - LEITURA (app/query): consultas leem a tabela `reservation_views`, um
//     modelo desnormalizado e indexado para listar, filtrar e buscar.
//     Otimizado para as telas.
//
// O modelo de leitura é uma PROJEÇÃO dos eventos: pode ser apagado e
// reconstruído a qualquer momento a partir do Event Store (comando
// `reservation rebuild-views`).
package port

import (
	"context"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// ReservationStore é o repositório do agregado, sobre o Event Store.
type ReservationStore interface {
	// Load reconstrói a reserva reaplicando seus eventos.
	Load(ctx context.Context, id string) (*domain.Reservation, error)
	// Save grava os eventos pendentes do agregado. Devolve
	// eventstore.ErrConcurrency se outro comando gravou antes.
	Save(ctx context.Context, reservation *domain.Reservation, actor string) error
	// History devolve o histórico de eventos para exibição.
	History(ctx context.Context, id string) ([]HistoryEntry, error)
	// AllIDs lista todos os agregados (para reconstruir as projeções).
	AllIDs(ctx context.Context) ([]string, error)
}

// HistoryEntry é uma linha da linha do tempo de uma reserva.
type HistoryEntry struct {
	Version    int       `json:"version"`
	Type       string    `json:"type"`
	Actor      string    `json:"actor"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       any       `json:"data"`
}

// ReservationView é o modelo de leitura de uma reserva — e também o formato
// devolvido pela API. No lado de consulta do CQRS não há entidades com
// comportamento: são apenas dados prontos para a tela.
type ReservationView struct {
	ID                    string               `json:"id"`
	Code                  string               `json:"code"`
	PropertyID            string               `json:"property_id"`
	RoomTypeID            string               `json:"room_type_id"`
	RoomID                string               `json:"room_id"`
	RoomName              string               `json:"room_name"`
	CheckIn               kernel.Date          `json:"check_in"`
	CheckOut              kernel.Date          `json:"check_out"`
	Nights                int                  `json:"nights"`
	Adults                int                  `json:"adults"`
	Children              int                  `json:"children"`
	Holder                domain.Holder        `json:"holder"`
	Source                string               `json:"source"`
	ExternalRef           string               `json:"external_ref"`
	GroupCode             string               `json:"group_code"`
	Notes                 string               `json:"notes"`
	Status                string               `json:"status"`
	NightlyRates          []domain.NightlyRate `json:"nightly_rates"`
	TotalCents            int64                `json:"total_cents"`
	DepositDueCents       int64                `json:"deposit_due_cents"`
	Currency              string               `json:"currency"`
	HoldExpiresAt         *time.Time           `json:"hold_expires_at"`
	PaymentCollect        string               `json:"payment_collect"`
	Overbooked            bool                 `json:"overbooked"`
	RegistrationCompleted bool                 `json:"registration_completed"`
	PrimaryGuestID        string               `json:"primary_guest_id"`
	PendingChange         *domain.StayChange   `json:"pending_change"`
	CancelReason          string               `json:"cancel_reason"`
	RejectReason          string               `json:"reject_reason"`
	RejectDetail          string               `json:"reject_detail"`
	RequestedAt           time.Time            `json:"requested_at"`
	ConfirmedAt           *time.Time           `json:"confirmed_at"`
	CheckedInAt           *time.Time           `json:"checked_in_at"`
	CheckedOutAt          *time.Time           `json:"checked_out_at"`
	ClosedAt              *time.Time           `json:"closed_at"`
	Version               int                  `json:"version"`
}

// ViewOf projeta o estado do agregado no modelo de leitura.
func ViewOf(state domain.State) ReservationView {
	rates := state.NightlyRates
	if rates == nil {
		rates = []domain.NightlyRate{}
	}
	return ReservationView{
		ID: state.ID, Code: state.Code, PropertyID: state.PropertyID,
		RoomTypeID: state.RoomTypeID, RoomID: state.RoomID, RoomName: state.RoomName,
		CheckIn: state.Stay.CheckIn(), CheckOut: state.Stay.CheckOut(), Nights: state.Stay.Nights(),
		Adults: state.Adults, Children: state.Children, Holder: state.Holder,
		Source: state.Source, ExternalRef: state.ExternalRef, GroupCode: state.GroupCode, Notes: state.Notes,
		Status: string(state.Status), NightlyRates: rates,
		TotalCents: state.TotalCents, DepositDueCents: state.DepositDueCents, Currency: state.Currency,
		HoldExpiresAt: state.HoldExpiresAt, PaymentCollect: state.PaymentCollect, Overbooked: state.Overbooked,
		RegistrationCompleted: state.RegistrationCompleted, PrimaryGuestID: state.PrimaryGuestID,
		PendingChange: state.PendingChange,
		CancelReason:  state.CancelReason, RejectReason: state.RejectReason, RejectDetail: state.RejectDetail,
		RequestedAt: state.RequestedAt, ConfirmedAt: state.ConfirmedAt,
		CheckedInAt: state.CheckedInAt, CheckedOutAt: state.CheckedOutAt, ClosedAt: state.ClosedAt,
		Version: state.Version,
	}
}

// ListFilter são os filtros da listagem de reservas.
type ListFilter struct {
	PropertyID string
	Statuses   []string
	// From/To: reservas cuja estadia toca o intervalo [From, To).
	From, To kernel.Date
	// CheckInOn/CheckOutOn: chegadas/saídas em uma data específica.
	CheckInOn, CheckOutOn kernel.Date
	// Query busca por nome do titular, localizador ou nº externo.
	Query  string
	Limit  int
	Offset int
}

// ViewRepository persiste e consulta o modelo de leitura.
type ViewRepository interface {
	Upsert(ctx context.Context, view ReservationView) error
	ByID(ctx context.Context, id string) (*ReservationView, error)
	ByCode(ctx context.Context, code string) (*ReservationView, error)
	List(ctx context.Context, filter ListFilter) ([]ReservationView, int, error)
	// ExpiredHolds: reservas PENDING com prazo do sinal vencido.
	ExpiredHolds(ctx context.Context, now time.Time, limit int) ([]string, error)
	// StaleRequests: reservas REQUESTED sem resposta do inventário desde `before`.
	StaleRequests(ctx context.Context, before time.Time, limit int) ([]string, error)
	Truncate(ctx context.Context) error
}

// PropertyPolicy é a cópia local das políticas de uma propriedade, mantida
// pelos eventos do serviço de propriedades.
type PropertyPolicy struct {
	PropertyID     string
	Name           string
	Timezone       string
	Currency       string
	DepositPercent int
	HoldMinutes    int
	Published      bool
}

// Location devolve o fuso da propriedade (UTC se o fuso for desconhecido).
func (p PropertyPolicy) Location() *time.Location {
	location, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.UTC
	}
	return location
}

// PolicyRepository guarda as políticas das propriedades.
type PolicyRepository interface {
	Upsert(ctx context.Context, policy PropertyPolicy) error
	ByProperty(ctx context.Context, propertyID string) (*PropertyPolicy, error)
}

// EventPublisher publica eventos de integração (via outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner controla transações (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
	InTx(ctx context.Context) bool
}
