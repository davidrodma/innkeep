// Package rest é o adaptador de entrada HTTP do serviço financeiro.
package rest

import (
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/services/billing/internal/app"
	"github.com/davidrodma/innkeep/services/billing/internal/domain"
)

// Handler agrupa os endpoints do serviço.
type Handler struct {
	folios  *app.FolioService
	finance *app.FinanceService
}

// NewHandler cria o adaptador REST.
func NewHandler(folios *app.FolioService, finance *app.FinanceService) *Handler {
	return &Handler{folios: folios, finance: finance}
}

// Register publica as rotas. `idempotent` protege o lançamento de pagamentos
// contra reenvio (um clique duplo não pode cobrar duas vezes).
func (h *Handler) Register(mux *http.ServeMux, guard *auth.Guard, idempotent httpx.Middleware) {
	billingRead := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermBillingRead, fn) }
	billingWrite := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermBillingWrite, fn) }
	financeRead := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermFinanceRead, fn) }
	financeWrite := func(fn httpx.HandlerFunc) http.Handler { return guard.Require(access.PermFinanceWrite, fn) }

	// Contas dos hóspedes.
	mux.Handle("GET /v1/folios", billingRead(h.listFolios))
	mux.Handle("GET /v1/folios/by-reservation/{reservation_id}", billingRead(h.folioByReservation))
	mux.Handle("GET /v1/folios/{id}", billingRead(h.getFolio))
	mux.Handle("POST /v1/folios/{id}/charges", billingWrite(h.postCharge))
	mux.Handle("POST /v1/folios/{id}/payments", idempotent(billingWrite(h.recordPayments)))
	mux.Handle("POST /v1/folios/{id}/refunds", billingWrite(h.refund))
	mux.Handle("POST /v1/folios/{id}/entries/{entry_id}/void", billingWrite(h.voidEntry))
	mux.Handle("GET /v1/payment-methods", billingRead(h.paymentMethods))

	// Controle financeiro da propriedade.
	mux.Handle("GET /v1/expenses", financeRead(h.listExpenses))
	mux.Handle("POST /v1/expenses", financeWrite(h.createExpense))
	mux.Handle("PUT /v1/expenses/{id}", financeWrite(h.updateExpense))
	mux.Handle("POST /v1/expenses/{id}/pay", financeWrite(h.payExpense))
	mux.Handle("DELETE /v1/expenses/{id}", financeWrite(h.deleteExpense))
	mux.Handle("GET /v1/cash/movements", financeRead(h.listCash))
	mux.Handle("POST /v1/cash/movements", financeWrite(h.addCash))
	mux.Handle("GET /v1/fee-rules", financeRead(h.getFeeRules))
	mux.Handle("PUT /v1/fee-rules", financeWrite(h.setFeeRules))
	mux.Handle("GET /v1/reports/financial", financeRead(h.report))
}

// ---- DTOs ----

type entryResponse struct {
	ID           string      `json:"id"`
	Kind         string      `json:"kind"`
	Category     string      `json:"category"`
	Description  string      `json:"description"`
	AmountCents  int64       `json:"amount_cents"`
	Method       string      `json:"method"`
	FeeCents     int64       `json:"fee_cents"`
	Reference    string      `json:"reference"`
	BusinessDate kernel.Date `json:"business_date"`
	CreatedAt    time.Time   `json:"created_at"`
	CreatedBy    string      `json:"created_by"`
	Voided       bool        `json:"voided"`
	VoidReason   string      `json:"void_reason"`
}

type folioResponse struct {
	ID               string          `json:"id"`
	ReservationID    string          `json:"reservation_id"`
	ReservationCode  string          `json:"reservation_code"`
	PropertyID       string          `json:"property_id"`
	GuestName        string          `json:"guest_name"`
	Source           string          `json:"source"`
	Status           string          `json:"status"`
	Currency         string          `json:"currency"`
	CheckIn          kernel.Date     `json:"check_in"`
	CheckOut         kernel.Date     `json:"check_out"`
	DepositDueCents  int64           `json:"deposit_due_cents"`
	DepositSettledAt *time.Time      `json:"deposit_settled_at"`
	ChargesCents     int64           `json:"charges_cents"`
	DiscountsCents   int64           `json:"discounts_cents"`
	TotalDueCents    int64           `json:"total_due_cents"`
	PaidCents        int64           `json:"paid_cents"`
	FeesCents        int64           `json:"fees_cents"`
	BalanceCents     int64           `json:"balance_cents"`
	Entries          []entryResponse `json:"entries"`
	OpenedAt         time.Time       `json:"opened_at"`
	ClosedAt         *time.Time      `json:"closed_at"`
}

func toFolioResponse(f *domain.Folio) folioResponse {
	totals := f.Totals()
	response := folioResponse{
		ID: f.ID, ReservationID: f.ReservationID, ReservationCode: f.ReservationCode, PropertyID: f.PropertyID,
		GuestName: f.GuestName, Source: f.Source, Status: string(f.Status), Currency: f.Currency,
		CheckIn: f.CheckIn, CheckOut: f.CheckOut,
		DepositDueCents: f.DepositDueCents, DepositSettledAt: f.DepositSettledAt,
		ChargesCents: totals.ChargesCents, DiscountsCents: totals.DiscountsCents,
		TotalDueCents: totals.TotalDueCents(), PaidCents: totals.PaidCents(),
		FeesCents: totals.FeesCents, BalanceCents: totals.BalanceCents,
		Entries: make([]entryResponse, 0, len(f.Entries)), OpenedAt: f.OpenedAt, ClosedAt: f.ClosedAt,
	}
	for _, e := range f.Entries {
		response.Entries = append(response.Entries, entryResponse{
			ID: e.ID, Kind: string(e.Kind), Category: e.Category, Description: e.Description,
			AmountCents: e.AmountCents, Method: string(e.Method), FeeCents: e.FeeCents, Reference: e.Reference,
			BusinessDate: e.BusinessDate, CreatedAt: e.CreatedAt, CreatedBy: e.CreatedBy,
			Voided: e.Voided(), VoidReason: e.VoidReason,
		})
	}
	return response
}

type expenseResponse struct {
	ID          string      `json:"id"`
	PropertyID  string      `json:"property_id"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Supplier    string      `json:"supplier"`
	AmountCents int64       `json:"amount_cents"`
	DueDate     kernel.Date `json:"due_date"`
	Status      string      `json:"status"`
	PaidOn      kernel.Date `json:"paid_on"`
	Method      string      `json:"method"`
}

func toExpenseResponse(e *domain.Expense) expenseResponse {
	return expenseResponse{
		ID: e.ID, PropertyID: e.PropertyID, Category: e.Category, Description: e.Description, Supplier: e.Supplier,
		AmountCents: e.AmountCents, DueDate: e.DueDate, Status: string(e.Status), PaidOn: e.PaidOn, Method: string(e.Method),
	}
}

type expenseRequest struct {
	PropertyID  string `json:"property_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Supplier    string `json:"supplier"`
	AmountCents int64  `json:"amount_cents"`
	DueDate     string `json:"due_date"`
}

func (r expenseRequest) toInput() (domain.ExpenseInput, error) {
	dueDate, err := kernel.ParseDate(r.DueDate)
	if err != nil {
		return domain.ExpenseInput{}, kernel.Invalid("expense.due_date_required", "informe a data da despesa").WithField("due_date", "use AAAA-MM-DD")
	}
	return domain.ExpenseInput{
		Category: r.Category, Description: r.Description, Supplier: r.Supplier, AmountCents: r.AmountCents, DueDate: dueDate,
	}, nil
}

// ---- Folios ----

func (h *Handler) listFolios(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return err
	}
	page := httpx.QueryPage(r)
	summaries, total, err := h.folios.List(r.Context(), app.FolioFilter{
		PropertyID: propertyID, Status: r.URL.Query().Get("status"),
		OnlyOutstanding: r.URL.Query().Get("outstanding") == "true",
		Query:           r.URL.Query().Get("q"), Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(summaries, total))
	return nil
}

func (h *Handler) getFolio(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	folio, err := h.folios.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return h.respondFolio(w, r, folio)
}

func (h *Handler) folioByReservation(w http.ResponseWriter, r *http.Request) error {
	reservationID, err := httpx.PathID(r, "reservation_id")
	if err != nil {
		return err
	}
	folio, err := h.folios.ByReservation(r.Context(), reservationID)
	if err != nil {
		return err
	}
	return h.respondFolio(w, r, folio)
}

func (h *Handler) respondFolio(w http.ResponseWriter, r *http.Request, folio *domain.Folio) error {
	if err := access.EnsureProperty(r.Context(), folio.PropertyID); err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toFolioResponse(folio))
	return nil
}

func (h *Handler) postCharge(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body struct {
		Kind        string `json:"kind"`
		Description string `json:"description"`
		AmountCents int64  `json:"amount_cents"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if body.Kind == "" {
		body.Kind = string(domain.EntryCharge)
	}
	folio, err := h.folios.PostCharge(r.Context(), id, domain.EntryKind(body.Kind), body.Description, body.AmountCents)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toFolioResponse(folio))
	return nil
}

func (h *Handler) recordPayments(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	// O corpo aceita VÁRIOS pagamentos: é assim que o hóspede paga "R$ 200 no
	// Pix e R$ 100 em dinheiro" em uma única operação atômica.
	var body struct {
		Payments []struct {
			Method      string `json:"method"`
			AmountCents int64  `json:"amount_cents"`
			FeeCents    *int64 `json:"fee_cents"`
			Reference   string `json:"reference"`
		} `json:"payments"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	payments := make([]domain.PaymentInput, 0, len(body.Payments))
	for _, payment := range body.Payments {
		payments = append(payments, domain.PaymentInput{
			Method: domain.PaymentMethod(payment.Method), AmountCents: payment.AmountCents,
			FeeCents: payment.FeeCents, Reference: payment.Reference,
		})
	}
	folio, err := h.folios.RecordPayments(r.Context(), id, payments)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toFolioResponse(folio))
	return nil
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body struct {
		Method      string `json:"method"`
		AmountCents int64  `json:"amount_cents"`
		Reason      string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	folio, err := h.folios.Refund(r.Context(), id, domain.PaymentMethod(body.Method), body.AmountCents, body.Reason)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toFolioResponse(folio))
	return nil
}

func (h *Handler) voidEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	entryID, err := httpx.PathID(r, "entry_id")
	if err != nil {
		return err
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	folio, err := h.folios.VoidEntry(r.Context(), id, entryID, body.Reason)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toFolioResponse(folio))
	return nil
}

func (h *Handler) paymentMethods(w http.ResponseWriter, _ *http.Request) error {
	methods := domain.Methods()
	items := make([]string, len(methods))
	for i, method := range methods {
		items[i] = string(method)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"methods": items, "expense_categories": domain.ExpenseCategories})
	return nil
}

// ---- Despesas ----

func (h *Handler) listExpenses(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return err
	}
	from, err := httpx.QueryDate(r, "from")
	if err != nil {
		return err
	}
	to, err := httpx.QueryDate(r, "to")
	if err != nil {
		return err
	}
	expenses, err := h.finance.Expenses(r.Context(), app.ExpenseFilter{
		PropertyID: propertyID, Status: r.URL.Query().Get("status"), From: from, To: to,
	})
	if err != nil {
		return err
	}
	items := make([]expenseResponse, len(expenses))
	for i, expense := range expenses {
		items[i] = toExpenseResponse(expense)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) createExpense(w http.ResponseWriter, r *http.Request) error {
	var body expenseRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := h.ensureProperty(r, body.PropertyID); err != nil {
		return err
	}
	input, err := body.toInput()
	if err != nil {
		return err
	}
	expense, err := h.finance.CreateExpense(r.Context(), body.PropertyID, input)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toExpenseResponse(expense))
	return nil
}

func (h *Handler) updateExpense(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body expenseRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	input, err := body.toInput()
	if err != nil {
		return err
	}
	expense, err := h.finance.UpdateExpense(r.Context(), id, input)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toExpenseResponse(expense))
	return nil
}

func (h *Handler) payExpense(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var body struct {
		Method string `json:"method"`
		PaidOn string `json:"paid_on"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	paidOn, err := kernel.ParseDate(body.PaidOn)
	if err != nil {
		return kernel.Invalid("expense.paid_on_required", "informe a data do pagamento").WithField("paid_on", "use AAAA-MM-DD")
	}
	expense, err := h.finance.PayExpense(r.Context(), id, domain.PaymentMethod(body.Method), paidOn)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toExpenseResponse(expense))
	return nil
}

func (h *Handler) deleteExpense(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.finance.DeleteExpense(r.Context(), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

// ---- Caixa ----

type cashResponse struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	AmountCents int64       `json:"amount_cents"`
	Reason      string      `json:"reason"`
	RefType     string      `json:"ref_type"`
	OccurredOn  kernel.Date `json:"occurred_on"`
	CreatedBy   string      `json:"created_by"`
}

func toCashResponse(m *domain.CashMovement) cashResponse {
	return cashResponse{ID: m.ID, Kind: string(m.Kind), AmountCents: m.AmountCents, Reason: m.Reason,
		RefType: m.RefType, OccurredOn: m.OccurredOn, CreatedBy: m.CreatedBy}
}

func (h *Handler) listCash(w http.ResponseWriter, r *http.Request) error {
	propertyID, from, to, err := h.propertyRange(r)
	if err != nil {
		return err
	}
	movements, err := h.finance.CashMovements(r.Context(), propertyID, from, to)
	if err != nil {
		return err
	}
	items := make([]cashResponse, len(movements))
	for i, movement := range movements {
		items[i] = toCashResponse(movement)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) addCash(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		PropertyID  string `json:"property_id"`
		Kind        string `json:"kind"`
		AmountCents int64  `json:"amount_cents"`
		Reason      string `json:"reason"`
		OccurredOn  string `json:"occurred_on"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := h.ensureProperty(r, body.PropertyID); err != nil {
		return err
	}
	on, err := kernel.ParseDate(body.OccurredOn)
	if err != nil {
		return kernel.Invalid("cash.date_required", "informe a data do movimento").WithField("occurred_on", "use AAAA-MM-DD")
	}
	movement, err := h.finance.AddCashMovement(r.Context(), body.PropertyID, domain.CashKind(body.Kind), body.AmountCents, body.Reason, on)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toCashResponse(movement))
	return nil
}

// ---- Taxas e relatórios ----

type feeRuleDTO struct {
	Method string `json:"method"`
	Bps    int64  `json:"bps"`
}

func (h *Handler) getFeeRules(w http.ResponseWriter, r *http.Request) error {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return err
	}
	rules, err := h.finance.FeeRules(r.Context(), propertyID)
	if err != nil {
		return err
	}
	items := make([]feeRuleDTO, len(rules))
	for i, rule := range rules {
		items[i] = feeRuleDTO{Method: string(rule.Method), Bps: rule.Bps}
	}
	httpx.JSON(w, http.StatusOK, httpx.NewList(items, len(items)))
	return nil
}

func (h *Handler) setFeeRules(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		PropertyID string       `json:"property_id"`
		Rules      []feeRuleDTO `json:"rules"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		return err
	}
	if err := h.ensureProperty(r, body.PropertyID); err != nil {
		return err
	}
	rules := make([]domain.FeeRule, len(body.Rules))
	for i, rule := range body.Rules {
		rules[i] = domain.FeeRule{Method: domain.PaymentMethod(rule.Method), Bps: rule.Bps}
	}
	if err := h.finance.SetFeeRules(r.Context(), body.PropertyID, rules); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) error {
	propertyID, from, to, err := h.propertyRange(r)
	if err != nil {
		return err
	}
	report, err := h.finance.Report(r.Context(), propertyID, from, to)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, report)
	return nil
}

// ---- Auxiliares ----

func (h *Handler) ensureProperty(r *http.Request, propertyID string) error {
	if !kernel.IsID(propertyID) {
		return kernel.Invalid("request.invalid_id", "informe a propriedade").WithField("property_id", "obrigatório")
	}
	return access.EnsureProperty(r.Context(), propertyID)
}

func (h *Handler) propertyFromQuery(r *http.Request) (string, error) {
	propertyID, err := httpx.RequiredQueryID(r, "property_id")
	if err != nil {
		return "", err
	}
	return propertyID, access.EnsureProperty(r.Context(), propertyID)
}

func (h *Handler) propertyRange(r *http.Request) (string, kernel.Date, kernel.Date, error) {
	propertyID, err := h.propertyFromQuery(r)
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	from, err := httpx.RequiredQueryDate(r, "from")
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	to, err := httpx.RequiredQueryDate(r, "to")
	if err != nil {
		return "", kernel.Date{}, kernel.Date{}, err
	}
	return propertyID, from, to, nil
}
