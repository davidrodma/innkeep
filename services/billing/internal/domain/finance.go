package domain

import (
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// ---- Despesas ----

// ExpenseStatus é a situação de uma despesa.
type ExpenseStatus string

const (
	ExpensePending ExpenseStatus = "PENDING" // a pagar
	ExpensePaid    ExpenseStatus = "PAID"
)

// ErrExpenseNotFound: despesa inexistente.
var ErrExpenseNotFound = kernel.NotFound("expense.not_found", "despesa não encontrada")

// Categorias sugeridas de despesa (o campo é livre; estas alimentam a tela).
var ExpenseCategories = []string{
	"UTILITIES", "SUPPLIES", "MAINTENANCE", "PAYROLL", "RENT", "COMMISSION", "TAXES", "MARKETING", "OTHER",
}

// Expense é uma conta a pagar (ou já paga) da propriedade.
type Expense struct {
	ID          string
	PropertyID  string
	Category    string
	Description string
	Supplier    string
	AmountCents int64
	DueDate     kernel.Date
	Status      ExpenseStatus
	PaidOn      kernel.Date
	Method      PaymentMethod
	CreatedBy   string
	CreatedAt   time.Time
}

// ExpenseInput reúne os dados editáveis de uma despesa.
type ExpenseInput struct {
	Category    string
	Description string
	Supplier    string
	AmountCents int64
	DueDate     kernel.Date
}

// NewExpense cria uma despesa a pagar.
func NewExpense(propertyID string, input ExpenseInput, by string) (*Expense, error) {
	expense := &Expense{
		ID: kernel.NewID(), PropertyID: propertyID, Status: ExpensePending,
		CreatedBy: by, CreatedAt: time.Now().UTC(),
	}
	if err := expense.Update(input); err != nil {
		return nil, err
	}
	return expense, nil
}

// Update valida e aplica os dados.
func (e *Expense) Update(input ExpenseInput) error {
	input.Description = strings.TrimSpace(input.Description)
	if input.Description == "" {
		return kernel.Invalid("expense.description_required", "descreva a despesa").WithField("description", "obrigatório")
	}
	if input.AmountCents <= 0 {
		return kernel.Invalid("expense.invalid_amount", "o valor deve ser maior que zero").WithField("amount_cents", "deve ser maior que zero")
	}
	if input.DueDate.IsZero() {
		return kernel.Invalid("expense.due_date_required", "informe a data da despesa").WithField("due_date", "obrigatório")
	}
	if input.Category = strings.ToUpper(strings.TrimSpace(input.Category)); input.Category == "" {
		input.Category = "OTHER"
	}
	e.Category, e.Description, e.Supplier = input.Category, input.Description, strings.TrimSpace(input.Supplier)
	e.AmountCents, e.DueDate = input.AmountCents, input.DueDate
	return nil
}

// Pay marca a despesa como paga.
func (e *Expense) Pay(method PaymentMethod, on kernel.Date) error {
	if e.Status == ExpensePaid {
		return kernel.Rule("expense.already_paid", "a despesa já está paga")
	}
	if !ValidMethod(method) {
		return kernel.Invalid("payment.invalid_method", "forma de pagamento inválida").WithField("method", "valor desconhecido")
	}
	if on.IsZero() {
		return kernel.Invalid("expense.paid_on_required", "informe a data do pagamento").WithField("paid_on", "obrigatório")
	}
	e.Status, e.Method, e.PaidOn = ExpensePaid, method, on
	return nil
}

// ---- Caixa em espécie ----

// CashKind é o sentido de um movimento de caixa.
type CashKind string

const (
	CashIn  CashKind = "IN"
	CashOut CashKind = "OUT"
)

// Origens de um movimento de caixa.
const (
	CashRefPayment    = "PAYMENT"    // recebimento em dinheiro de um hóspede
	CashRefRefund     = "REFUND"     // devolução em dinheiro
	CashRefExpense    = "EXPENSE"    // despesa paga em dinheiro
	CashRefWithdrawal = "WITHDRAWAL" // sangria: retirada do caixa
	CashRefDeposit    = "DEPOSIT"    // suprimento: reforço do caixa
	CashRefAdjustment = "ADJUSTMENT" // ajuste de conferência
)

// CashMovement é uma entrada ou saída de DINHEIRO FÍSICO da gaveta. Somando
// os movimentos chega-se ao que deve haver no caixa — a conferência diária de
// qualquer recepção.
type CashMovement struct {
	ID          string
	PropertyID  string
	Kind        CashKind
	AmountCents int64
	Reason      string
	RefType     string
	RefID       string
	OccurredOn  kernel.Date
	CreatedBy   string
	CreatedAt   time.Time
}

// NewCashMovement cria um movimento de caixa válido.
func NewCashMovement(propertyID string, kind CashKind, amountCents int64, reason, refType, refID string, on kernel.Date, by string) (*CashMovement, error) {
	if kind != CashIn && kind != CashOut {
		return nil, kernel.Invalid("cash.invalid_kind", "tipo de movimento inválido").WithField("kind", "use IN ou OUT")
	}
	if amountCents <= 0 {
		return nil, kernel.Invalid("cash.invalid_amount", "o valor deve ser maior que zero").WithField("amount_cents", "deve ser maior que zero")
	}
	if reason = strings.TrimSpace(reason); reason == "" {
		return nil, kernel.Invalid("cash.reason_required", "informe o motivo do movimento").WithField("reason", "obrigatório")
	}
	if on.IsZero() {
		return nil, kernel.Invalid("cash.date_required", "informe a data do movimento").WithField("occurred_on", "obrigatório")
	}
	return &CashMovement{
		ID: kernel.NewID(), PropertyID: propertyID, Kind: kind, AmountCents: amountCents,
		Reason: reason, RefType: refType, RefID: refID, OccurredOn: on, CreatedBy: by, CreatedAt: time.Now().UTC(),
	}, nil
}

// ---- Taxas por forma de pagamento ----

// FeeRule é a taxa cobrada sobre recebimentos de uma forma de pagamento em
// uma propriedade: a taxa da maquininha, a comissão do Booking.com etc.
type FeeRule struct {
	PropertyID string
	Method     PaymentMethod
	// Bps é a taxa em pontos-base (1500 = 15%).
	Bps int64
}

// Validate confere a regra.
func (r FeeRule) Validate() error {
	if !ValidMethod(r.Method) {
		return kernel.Invalid("fee.invalid_method", "forma de pagamento inválida").WithField("method", "valor desconhecido")
	}
	if r.Bps < 0 || r.Bps > 5000 {
		return kernel.Invalid("fee.invalid_rate", "a taxa deve estar entre 0%% e 50%%").WithField("bps", "entre 0 e 5000")
	}
	return nil
}
