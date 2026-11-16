// Package domain contém o modelo do contexto Financeiro.
//
// O conceito central é o FOLIO: a conta corrente de uma reserva, onde entram
// os débitos (diárias, extras) e os créditos (pagamentos, descontos). É o
// nome usado em toda a hotelaria — "guest folio".
package domain

import (
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// EntryKind é a natureza de um lançamento no folio.
type EntryKind string

const (
	EntryCharge   EntryKind = "CHARGE"   // débito: aumenta o que o hóspede deve
	EntryDiscount EntryKind = "DISCOUNT" // abatimento: reduz o que o hóspede deve
	EntryPayment  EntryKind = "PAYMENT"  // pagamento recebido
	EntryRefund   EntryKind = "REFUND"   // devolução de valor ao hóspede
)

// Categorias de débito.
const (
	CategoryAccommodation = "ACCOMMODATION" // diárias
	CategoryExtra         = "EXTRA"         // consumo, serviços, taxas
)

// PaymentMethod é a forma de pagamento.
type PaymentMethod string

const (
	MethodPix          PaymentMethod = "PIX"
	MethodCash         PaymentMethod = "CASH"
	MethodCreditCard   PaymentMethod = "CREDIT_CARD"
	MethodDebitCard    PaymentMethod = "DEBIT_CARD"
	MethodBankTransfer PaymentMethod = "BANK_TRANSFER"
	MethodPaymentLink  PaymentMethod = "PAYMENT_LINK" // Mercado Pago, PagSeguro etc.
	MethodBookingCom   PaymentMethod = "BOOKING_COM"  // cobrado pelo Booking.com e repassado
	MethodAirbnb       PaymentMethod = "AIRBNB"       // cobrado pelo Airbnb e repassado
	MethodVoucher      PaymentMethod = "VOUCHER"      // crédito / cortesia
	MethodOther        PaymentMethod = "OTHER"
)

var validMethods = map[PaymentMethod]bool{
	MethodPix: true, MethodCash: true, MethodCreditCard: true, MethodDebitCard: true,
	MethodBankTransfer: true, MethodPaymentLink: true, MethodBookingCom: true,
	MethodAirbnb: true, MethodVoucher: true, MethodOther: true,
}

// ValidMethod informa se a forma de pagamento existe.
func ValidMethod(method PaymentMethod) bool { return validMethods[method] }

// Methods lista as formas de pagamento aceitas.
func Methods() []PaymentMethod {
	return []PaymentMethod{MethodPix, MethodCash, MethodCreditCard, MethodDebitCard, MethodBankTransfer,
		MethodPaymentLink, MethodBookingCom, MethodAirbnb, MethodVoucher, MethodOther}
}

// FolioStatus é a situação da conta.
type FolioStatus string

const (
	FolioOpen   FolioStatus = "OPEN"
	FolioClosed FolioStatus = "CLOSED"
)

// Erros de domínio do contexto.
var (
	ErrFolioNotFound = kernel.NotFound("folio.not_found", "conta (folio) não encontrada")
	ErrEntryNotFound = kernel.NotFound("folio.entry_not_found", "lançamento não encontrado")
	ErrFolioClosed   = kernel.Rule("folio.closed", "a conta está fechada e não aceita novos lançamentos")
)

// Entry é um lançamento do folio.
//
// O folio funciona como um LIVRO-RAZÃO: lançamentos nunca são editados nem
// apagados. Um erro se corrige ANULANDO o lançamento (Void), que continua
// visível com o motivo e o responsável. É a mesma disciplina da contabilidade
// — e o que torna a conta auditável.
type Entry struct {
	ID          string
	Kind        EntryKind
	Category    string
	Description string
	// AmountCents é sempre positivo; o sinal vem do Kind.
	AmountCents int64
	Method      PaymentMethod
	// FeeCents é o custo do recebimento: taxa da maquininha ou comissão do
	// canal. O valor líquido de um pagamento é AmountCents - FeeCents.
	FeeCents  int64
	Reference string // NSU, ID da transação Pix, nº do voucher...
	// BusinessDate é a data de competência (a noite da diária, o dia do pagamento).
	BusinessDate kernel.Date
	CreatedAt    time.Time
	CreatedBy    string
	VoidedAt     *time.Time
	VoidedBy     string
	VoidReason   string
}

// Voided informa se o lançamento foi anulado.
func (e Entry) Voided() bool { return e.VoidedAt != nil }

// NightlyRate é o preço de uma diária.
type NightlyRate struct {
	Date       kernel.Date
	PriceCents int64
}

// Folio é a raiz do agregado: a conta de uma reserva.
type Folio struct {
	ID              string
	ReservationID   string
	ReservationCode string
	PropertyID      string
	GuestName       string
	Source          string // canal de venda da reserva
	Status          FolioStatus
	Currency        string
	// DepositDueCents é o sinal exigido para confirmar a reserva (0 = nenhum).
	DepositDueCents  int64
	DepositSettledAt *time.Time
	CheckIn          kernel.Date
	CheckOut         kernel.Date
	Entries          []Entry
	OpenedAt         time.Time
	ClosedAt         *time.Time
}

// OpenFolioInput são os dados para abrir a conta de uma reserva.
type OpenFolioInput struct {
	ReservationID   string
	ReservationCode string
	PropertyID      string
	GuestName       string
	Source          string
	Currency        string
	DepositDueCents int64
	Stay            kernel.StayPeriod
	NightlyRates    []NightlyRate
	RoomName        string
}

// OpenFolio abre a conta e lança uma diária por noite.
func OpenFolio(input OpenFolioInput, now time.Time) *Folio {
	folio := &Folio{
		ID: kernel.NewID(), ReservationID: input.ReservationID, ReservationCode: input.ReservationCode,
		PropertyID: input.PropertyID, GuestName: input.GuestName, Source: input.Source,
		Status: FolioOpen, Currency: input.Currency, DepositDueCents: input.DepositDueCents,
		CheckIn: input.Stay.CheckIn(), CheckOut: input.Stay.CheckOut(), OpenedAt: now,
	}
	folio.postAccommodation(input.NightlyRates, input.RoomName, "system", now)
	return folio
}

func (f *Folio) postAccommodation(rates []NightlyRate, roomName, by string, now time.Time) {
	description := "Diária"
	if roomName != "" {
		description += " — " + roomName
	}
	for _, rate := range rates {
		f.Entries = append(f.Entries, Entry{
			ID: kernel.NewID(), Kind: EntryCharge, Category: CategoryAccommodation,
			Description: description, AmountCents: rate.PriceCents,
			BusinessDate: rate.Date, CreatedAt: now, CreatedBy: by,
		})
	}
}

// Totals é o resumo financeiro de um folio.
type Totals struct {
	ChargesCents   int64 // débitos (diárias + extras)
	DiscountsCents int64
	PaymentsCents  int64 // pagamentos brutos recebidos
	RefundsCents   int64
	FeesCents      int64 // taxas e comissões sobre os pagamentos
	// BalanceCents é quanto falta pagar. Negativo = crédito do hóspede.
	BalanceCents int64
}

// TotalDueCents é o valor total da conta (débitos menos descontos).
func (t Totals) TotalDueCents() int64 { return t.ChargesCents - t.DiscountsCents }

// PaidCents é o total efetivamente pago (pagamentos menos devoluções).
func (t Totals) PaidCents() int64 { return t.PaymentsCents - t.RefundsCents }

// Totals soma os lançamentos válidos (não anulados).
func (f *Folio) Totals() Totals {
	var totals Totals
	for _, entry := range f.Entries {
		if entry.Voided() {
			continue
		}
		switch entry.Kind {
		case EntryCharge:
			totals.ChargesCents += entry.AmountCents
		case EntryDiscount:
			totals.DiscountsCents += entry.AmountCents
		case EntryPayment:
			totals.PaymentsCents += entry.AmountCents
			totals.FeesCents += entry.FeeCents
		case EntryRefund:
			totals.RefundsCents += entry.AmountCents
		}
	}
	totals.BalanceCents = totals.TotalDueCents() - totals.PaidCents()
	return totals
}

func (f *Folio) ensureOpen() error {
	if f.Status != FolioOpen {
		return ErrFolioClosed
	}
	return nil
}

// AddCharge lança um débito (consumo, serviço, taxa) ou um desconto.
func (f *Folio) AddCharge(kind EntryKind, description string, amountCents int64, date kernel.Date, by string, now time.Time) (Entry, error) {
	if err := f.ensureOpen(); err != nil {
		return Entry{}, err
	}
	if kind != EntryCharge && kind != EntryDiscount {
		return Entry{}, kernel.Invalid("folio.invalid_kind", "tipo de lançamento inválido").WithField("kind", "use CHARGE ou DISCOUNT")
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return Entry{}, kernel.Invalid("folio.description_required", "descreva o lançamento").WithField("description", "obrigatório")
	}
	if amountCents <= 0 {
		return Entry{}, kernel.Invalid("folio.invalid_amount", "o valor deve ser maior que zero").WithField("amount_cents", "deve ser maior que zero")
	}
	if kind == EntryDiscount && amountCents > f.Totals().TotalDueCents() {
		return Entry{}, kernel.Rule("folio.discount_too_high", "o desconto não pode superar o valor da conta")
	}
	entry := Entry{
		ID: kernel.NewID(), Kind: kind, Category: CategoryExtra, Description: description,
		AmountCents: amountCents, BusinessDate: date, CreatedAt: now, CreatedBy: by,
	}
	f.Entries = append(f.Entries, entry)
	return entry, nil
}

// PaymentInput é um pagamento a lançar. Um hóspede pode quitar a conta com
// VÁRIOS pagamentos de formas diferentes (parte em Pix, parte em dinheiro...).
type PaymentInput struct {
	Method      PaymentMethod
	AmountCents int64
	// FeeCents: taxa já conhecida. Nulo = calcular pela regra da propriedade.
	FeeCents  *int64
	Reference string
}

// PaymentResult descreve o efeito de um lote de pagamentos.
type PaymentResult struct {
	Entries []Entry
	// DepositJustSettled: o total pago acabou de atingir o sinal exigido.
	DepositJustSettled bool
	// JustSettled: a conta acabou de ser quitada.
	JustSettled bool
}

// RecordPayments lança um lote de pagamentos de forma atômica: ou entram
// todos, ou nenhum. feeBps devolve a taxa (em pontos-base) de cada forma de
// pagamento na propriedade.
func (f *Folio) RecordPayments(payments []PaymentInput, feeBps func(PaymentMethod) int64, date kernel.Date, by string, now time.Time) (PaymentResult, error) {
	if err := f.ensureOpen(); err != nil {
		return PaymentResult{}, err
	}
	if len(payments) == 0 {
		return PaymentResult{}, kernel.Invalid("payment.empty", "informe ao menos um pagamento").WithField("payments", "obrigatório")
	}
	before := f.Totals()
	var result PaymentResult

	for _, payment := range payments {
		if !ValidMethod(payment.Method) {
			return PaymentResult{}, kernel.Invalid("payment.invalid_method", "forma de pagamento inválida").WithField("method", "valor desconhecido")
		}
		if payment.AmountCents <= 0 {
			return PaymentResult{}, kernel.Invalid("payment.invalid_amount", "o valor do pagamento deve ser maior que zero").WithField("amount_cents", "deve ser maior que zero")
		}
		amount := kernel.NewMoney(payment.AmountCents, f.Currency)
		fee := amount.BasisPoints(feeBps(payment.Method)).Cents()
		if payment.FeeCents != nil {
			fee = *payment.FeeCents
		}
		if fee < 0 || fee > payment.AmountCents {
			return PaymentResult{}, kernel.Invalid("payment.invalid_fee", "a taxa deve estar entre zero e o valor do pagamento").WithField("fee_cents", "inválida")
		}
		entry := Entry{
			ID: kernel.NewID(), Kind: EntryPayment, Method: payment.Method,
			Description: "Pagamento", AmountCents: payment.AmountCents, FeeCents: fee,
			Reference: strings.TrimSpace(payment.Reference), BusinessDate: date, CreatedAt: now, CreatedBy: by,
		}
		f.Entries = append(f.Entries, entry)
		result.Entries = append(result.Entries, entry)
	}

	after := f.Totals()
	if f.DepositDueCents > 0 && f.DepositSettledAt == nil && after.PaidCents() >= f.DepositDueCents {
		f.DepositSettledAt = &now
		result.DepositJustSettled = true
	}
	result.JustSettled = before.BalanceCents > 0 && after.BalanceCents <= 0
	return result, nil
}

// Refund lança a devolução de um valor já pago.
func (f *Folio) Refund(method PaymentMethod, amountCents int64, reason string, date kernel.Date, by string, now time.Time) (Entry, error) {
	if !ValidMethod(method) {
		return Entry{}, kernel.Invalid("payment.invalid_method", "forma de devolução inválida").WithField("method", "valor desconhecido")
	}
	if amountCents <= 0 {
		return Entry{}, kernel.Invalid("payment.invalid_amount", "o valor deve ser maior que zero").WithField("amount_cents", "deve ser maior que zero")
	}
	if amountCents > f.Totals().PaidCents() {
		return Entry{}, kernel.Rule("payment.refund_exceeds_paid", "não é possível devolver mais do que foi pago")
	}
	entry := Entry{
		ID: kernel.NewID(), Kind: EntryRefund, Method: method, Description: "Devolução: " + strings.TrimSpace(reason),
		AmountCents: amountCents, BusinessDate: date, CreatedAt: now, CreatedBy: by,
	}
	f.Entries = append(f.Entries, entry)
	return entry, nil
}

// Void anula um lançamento, preservando-o no histórico.
func (f *Folio) Void(entryID, reason, by string, now time.Time) (Entry, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Entry{}, kernel.Invalid("folio.void_reason_required", "informe o motivo da anulação").WithField("reason", "obrigatório")
	}
	for i := range f.Entries {
		if f.Entries[i].ID != entryID {
			continue
		}
		if f.Entries[i].Voided() {
			return f.Entries[i], nil
		}
		f.Entries[i].VoidedAt, f.Entries[i].VoidedBy, f.Entries[i].VoidReason = &now, by, reason
		return f.Entries[i], nil
	}
	return Entry{}, ErrEntryNotFound
}

// ReplaceAccommodation refaz os lançamentos de hospedagem quando a estadia
// muda (datas, quarto ou valor): anula as diárias antigas e lança as novas.
func (f *Folio) ReplaceAccommodation(stay kernel.StayPeriod, rates []NightlyRate, roomName string, now time.Time) {
	f.voidAccommodation("estadia alterada", now)
	f.CheckIn, f.CheckOut = stay.CheckIn(), stay.CheckOut()
	f.postAccommodation(rates, roomName, "system", now)
}

// CancelStay anula as diárias de uma reserva que não vai acontecer
// (cancelada, expirada, no-show). Multas, se houver, são lançadas à mão pela
// equipe. Sem nada pago, a conta é fechada.
func (f *Folio) CancelStay(reason string, now time.Time) {
	f.voidAccommodation(reason, now)
	if f.Totals().PaidCents() == 0 {
		f.close(now)
	}
}

// Settle fecha a conta ao final da estadia, se estiver quitada. Com saldo em
// aberto ela continua aberta — é um valor a receber.
func (f *Folio) Settle(now time.Time) {
	if f.Totals().BalanceCents <= 0 {
		f.close(now)
	}
}

func (f *Folio) voidAccommodation(reason string, now time.Time) {
	for i := range f.Entries {
		entry := &f.Entries[i]
		if entry.Kind == EntryCharge && entry.Category == CategoryAccommodation && !entry.Voided() {
			entry.VoidedAt, entry.VoidedBy, entry.VoidReason = &now, "system", reason
		}
	}
}

func (f *Folio) close(now time.Time) {
	if f.Status == FolioOpen {
		f.Status, f.ClosedAt = FolioClosed, &now
	}
}
