// Package app é a camada de aplicação do serviço financeiro.
package app

import (
	"context"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/billing/internal/domain"
)

// FolioSummary é a linha de um folio nas listagens.
type FolioSummary struct {
	ID              string      `json:"id"`
	ReservationID   string      `json:"reservation_id"`
	ReservationCode string      `json:"reservation_code"`
	PropertyID      string      `json:"property_id"`
	GuestName       string      `json:"guest_name"`
	Source          string      `json:"source"`
	Status          string      `json:"status"`
	CheckIn         kernel.Date `json:"check_in"`
	CheckOut        kernel.Date `json:"check_out"`
	TotalDueCents   int64       `json:"total_due_cents"`
	PaidCents       int64       `json:"paid_cents"`
	BalanceCents    int64       `json:"balance_cents"`
	DepositDueCents int64       `json:"deposit_due_cents"`
	Currency        string      `json:"currency"`
	OpenedAt        time.Time   `json:"opened_at"`
}

// FolioFilter são os filtros da listagem de folios.
type FolioFilter struct {
	PropertyID string
	Status     string
	// OnlyOutstanding: apenas contas com saldo a receber.
	OnlyOutstanding bool
	Query           string
	Limit           int
	Offset          int
}

// FolioRepository persiste folios.
type FolioRepository interface {
	Create(ctx context.Context, folio *domain.Folio) error
	Save(ctx context.Context, folio *domain.Folio) error
	// ByID e ByReservation aceitam forUpdate: quando verdadeiro, a linha do
	// folio é travada (SELECT ... FOR UPDATE) até o fim da transação.
	ByID(ctx context.Context, id string, forUpdate bool) (*domain.Folio, error)
	ByReservation(ctx context.Context, reservationID string, forUpdate bool) (*domain.Folio, error)
	List(ctx context.Context, filter FolioFilter) ([]FolioSummary, int, error)
}

// ExpenseFilter são os filtros da listagem de despesas.
type ExpenseFilter struct {
	PropertyID string
	Status     string
	From, To   kernel.Date // intervalo [From, To) sobre a data de vencimento
}

// Report é o relatório financeiro de um período.
type Report struct {
	From kernel.Date `json:"from"`
	To   kernel.Date `json:"to"`

	// Receita = pagamentos recebidos no período (regime de caixa).
	GrossCents   int64 `json:"gross_cents"`   // bruto recebido
	FeesCents    int64 `json:"fees_cents"`    // taxas e comissões
	RefundsCents int64 `json:"refunds_cents"` // devoluções
	NetCents     int64 `json:"net_cents"`     // bruto - taxas - devoluções

	ExpensesPaidCents    int64 `json:"expenses_paid_cents"`
	ExpensesPendingCents int64 `json:"expenses_pending_cents"`
	// ResultCents é o resultado do período: líquido menos despesas pagas.
	ResultCents int64 `json:"result_cents"`
	// ReceivablesCents: total a receber de contas em aberto (posição atual).
	ReceivablesCents int64 `json:"receivables_cents"`

	ByMethod   []MethodTotal   `json:"by_method"`
	BySource   []SourceTotal   `json:"by_source"`
	ByCategory []CategoryTotal `json:"expenses_by_category"`
	Daily      []DailyTotal    `json:"daily"`
	Cash       CashSummary     `json:"cash"`
}

// MethodTotal é o recebido por forma de pagamento.
type MethodTotal struct {
	Method     string `json:"method"`
	Count      int    `json:"count"`
	GrossCents int64  `json:"gross_cents"`
	FeesCents  int64  `json:"fees_cents"`
	NetCents   int64  `json:"net_cents"`
}

// SourceTotal é o recebido por canal de venda.
type SourceTotal struct {
	Source     string `json:"source"`
	GrossCents int64  `json:"gross_cents"`
	FeesCents  int64  `json:"fees_cents"`
	NetCents   int64  `json:"net_cents"`
}

// CategoryTotal é o gasto por categoria de despesa.
type CategoryTotal struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
}

// DailyTotal é o movimento de um dia.
type DailyTotal struct {
	Date          kernel.Date `json:"date"`
	GrossCents    int64       `json:"gross_cents"`
	NetCents      int64       `json:"net_cents"`
	ExpensesCents int64       `json:"expenses_cents"`
}

// CashSummary é a posição do caixa em espécie no período.
type CashSummary struct {
	OpeningCents int64 `json:"opening_cents"` // saldo antes do período
	InCents      int64 `json:"in_cents"`
	OutCents     int64 `json:"out_cents"`
	ClosingCents int64 `json:"closing_cents"`
}

// FinanceRepository persiste despesas, caixa, taxas e calcula relatórios.
type FinanceRepository interface {
	FeeRules(ctx context.Context, propertyID string) ([]domain.FeeRule, error)
	SetFeeRule(ctx context.Context, rule domain.FeeRule) error

	CreateExpense(ctx context.Context, expense *domain.Expense) error
	UpdateExpense(ctx context.Context, expense *domain.Expense) error
	DeleteExpense(ctx context.Context, id string) error
	Expense(ctx context.Context, id string) (*domain.Expense, error)
	Expenses(ctx context.Context, filter ExpenseFilter) ([]*domain.Expense, error)

	AddCashMovement(ctx context.Context, movement *domain.CashMovement) error
	CashMovements(ctx context.Context, propertyID string, from, to kernel.Date) ([]*domain.CashMovement, error)

	Report(ctx context.Context, propertyID string, from, to kernel.Date) (*Report, error)
}

// EventPublisher publica eventos de integração (via outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner executa uma função dentro de uma transação (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
