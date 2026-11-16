package app

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/billing/internal/domain"
)

// FinanceService reúne os casos de uso do controle financeiro da propriedade:
// despesas, caixa em espécie, taxas e relatórios.
type FinanceService struct {
	tx      TxRunner
	finance FinanceRepository
}

// NewFinanceService monta o serviço.
func NewFinanceService(tx TxRunner, finance FinanceRepository) *FinanceService {
	return &FinanceService{tx: tx, finance: finance}
}

// ---- Despesas ----

// CreateExpense registra uma despesa a pagar.
func (s *FinanceService) CreateExpense(ctx context.Context, propertyID string, input domain.ExpenseInput) (*domain.Expense, error) {
	expense, err := domain.NewExpense(propertyID, input, access.ActorFrom(ctx))
	if err != nil {
		return nil, err
	}
	return expense, s.finance.CreateExpense(ctx, expense)
}

// UpdateExpense altera uma despesa ainda não paga.
func (s *FinanceService) UpdateExpense(ctx context.Context, id string, input domain.ExpenseInput) (*domain.Expense, error) {
	expense, err := s.authorizedExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	if expense.Status == domain.ExpensePaid {
		return nil, kernel.Rule("expense.already_paid", "despesa paga não pode ser alterada")
	}
	if err := expense.Update(input); err != nil {
		return nil, err
	}
	return expense, s.finance.UpdateExpense(ctx, expense)
}

// PayExpense marca a despesa como paga. Pagamento em dinheiro sai do caixa.
func (s *FinanceService) PayExpense(ctx context.Context, id string, method domain.PaymentMethod, on kernel.Date) (*domain.Expense, error) {
	var expense *domain.Expense
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if expense, err = s.authorizedExpense(ctx, id); err != nil {
			return err
		}
		if err := expense.Pay(method, on); err != nil {
			return err
		}
		if err := s.finance.UpdateExpense(ctx, expense); err != nil {
			return err
		}
		if method != domain.MethodCash {
			return nil
		}
		movement, err := domain.NewCashMovement(expense.PropertyID, domain.CashOut, expense.AmountCents,
			"Despesa: "+expense.Description, domain.CashRefExpense, expense.ID, on, access.ActorFrom(ctx))
		if err != nil {
			return err
		}
		return s.finance.AddCashMovement(ctx, movement)
	})
	return expense, err
}

// DeleteExpense apaga uma despesa ainda não paga (lançada por engano).
func (s *FinanceService) DeleteExpense(ctx context.Context, id string) error {
	expense, err := s.authorizedExpense(ctx, id)
	if err != nil {
		return err
	}
	if expense.Status == domain.ExpensePaid {
		return kernel.Rule("expense.already_paid", "despesa paga não pode ser apagada")
	}
	return s.finance.DeleteExpense(ctx, id)
}

// Expenses lista as despesas.
func (s *FinanceService) Expenses(ctx context.Context, filter ExpenseFilter) ([]*domain.Expense, error) {
	return s.finance.Expenses(ctx, filter)
}

func (s *FinanceService) authorizedExpense(ctx context.Context, id string) (*domain.Expense, error) {
	expense, err := s.finance.Expense(ctx, id)
	if err != nil {
		return nil, err
	}
	return expense, access.EnsureProperty(ctx, expense.PropertyID)
}

// ---- Caixa em espécie ----

// AddCashMovement registra uma sangria, um suprimento ou um ajuste de caixa.
func (s *FinanceService) AddCashMovement(ctx context.Context, propertyID string, kind domain.CashKind, amountCents int64, reason string, on kernel.Date) (*domain.CashMovement, error) {
	refType := domain.CashRefWithdrawal
	if kind == domain.CashIn {
		refType = domain.CashRefDeposit
	}
	movement, err := domain.NewCashMovement(propertyID, kind, amountCents, reason, refType, "", on, access.ActorFrom(ctx))
	if err != nil {
		return nil, err
	}
	return movement, s.finance.AddCashMovement(ctx, movement)
}

// CashMovements lista os movimentos de caixa do período [from, to).
func (s *FinanceService) CashMovements(ctx context.Context, propertyID string, from, to kernel.Date) ([]*domain.CashMovement, error) {
	return s.finance.CashMovements(ctx, propertyID, from, to)
}

// ---- Taxas ----

// FeeRules devolve a taxa de cada forma de pagamento (0 quando não definida).
func (s *FinanceService) FeeRules(ctx context.Context, propertyID string) ([]domain.FeeRule, error) {
	defined, err := s.finance.FeeRules(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	rules := make([]domain.FeeRule, 0, len(domain.Methods()))
	for _, method := range domain.Methods() {
		rule := domain.FeeRule{PropertyID: propertyID, Method: method}
		for _, candidate := range defined {
			if candidate.Method == method {
				rule.Bps = candidate.Bps
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// SetFeeRules grava as taxas informadas.
func (s *FinanceService) SetFeeRules(ctx context.Context, propertyID string, rules []domain.FeeRule) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		for _, rule := range rules {
			rule.PropertyID = propertyID
			if err := rule.Validate(); err != nil {
				return err
			}
			if err := s.finance.SetFeeRule(ctx, rule); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- Relatórios ----

// Report calcula o relatório financeiro do período [from, to).
func (s *FinanceService) Report(ctx context.Context, propertyID string, from, to kernel.Date) (*Report, error) {
	if !to.After(from) || from.DaysUntil(to) > 400 {
		return nil, kernel.Invalid("report.invalid_range", "intervalo inválido: informe de 1 a 400 dias")
	}
	return s.finance.Report(ctx, propertyID, from, to)
}
