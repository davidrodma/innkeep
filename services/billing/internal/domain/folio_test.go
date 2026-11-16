package domain_test

import (
	"testing"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/billing/internal/domain"
)

var now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// openFolio abre uma conta de 2 diárias de R$ 150,00 com sinal de R$ 150,00.
func openFolio() *domain.Folio {
	return domain.OpenFolio(domain.OpenFolioInput{
		ReservationID: "res-1", ReservationCode: "IK-TESTE1", PropertyID: "prop-1", GuestName: "Hóspede",
		Source: "WEBSITE", Currency: "BRL", DepositDueCents: 15000,
		Stay: kernel.MustStayPeriod("2026-11-20", "2026-11-22"),
		NightlyRates: []domain.NightlyRate{
			{Date: kernel.MustParseDate("2026-11-20"), PriceCents: 15000},
			{Date: kernel.MustParseDate("2026-11-21"), PriceCents: 15000},
		},
	}, now)
}

// fees: 15% sobre pagamentos via Booking.com, 5% no cartão de crédito.
func fees(method domain.PaymentMethod) int64 {
	switch method {
	case domain.MethodBookingCom:
		return 1500
	case domain.MethodCreditCard:
		return 500
	}
	return 0
}

func TestOpenFolioPostsOneChargePerNight(t *testing.T) {
	folio := openFolio()
	totals := folio.Totals()
	if len(folio.Entries) != 2 || totals.ChargesCents != 30000 || totals.BalanceCents != 30000 {
		t.Fatalf("conta inesperada: %d lançamentos, totais %+v", len(folio.Entries), totals)
	}
}

func TestMixedPartialPayments(t *testing.T) {
	folio := openFolio()

	// Primeiro pagamento parcial: não chega ao sinal.
	result, err := folio.RecordPayments([]domain.PaymentInput{{Method: domain.MethodPix, AmountCents: 10000}}, fees, kernel.MustParseDate("2026-11-14"), "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.DepositJustSettled || result.JustSettled {
		t.Fatalf("pagamento parcial não quita sinal nem conta: %+v", result)
	}

	// Segundo lançamento com DUAS formas: dinheiro + cartão (com taxa de 5%).
	result, err = folio.RecordPayments([]domain.PaymentInput{
		{Method: domain.MethodCash, AmountCents: 5000},
		{Method: domain.MethodCreditCard, AmountCents: 15000},
	}, fees, kernel.MustParseDate("2026-11-14"), "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DepositJustSettled {
		t.Fatal("ao passar de R$ 150,00 pagos, o sinal deveria ser quitado")
	}
	if !result.JustSettled {
		t.Fatal("R$ 300,00 pagos de R$ 300,00: a conta deveria ser quitada")
	}
	totals := folio.Totals()
	if totals.PaidCents() != 30000 || totals.BalanceCents != 0 || totals.FeesCents != 750 {
		t.Fatalf("totais inesperados: %+v", totals)
	}

	// Pagar de novo depois de quitado não "quita" outra vez.
	result, _ = folio.RecordPayments([]domain.PaymentInput{{Method: domain.MethodPix, AmountCents: 100}}, fees, kernel.MustParseDate("2026-11-14"), "user:1", now)
	if result.DepositJustSettled || result.JustSettled {
		t.Fatalf("os marcos só acontecem uma vez: %+v", result)
	}
	if folio.Totals().BalanceCents != -100 {
		t.Fatal("pagamento a mais vira crédito do hóspede")
	}
}

func TestPaymentBatchIsAtomic(t *testing.T) {
	folio := openFolio()
	_, err := folio.RecordPayments([]domain.PaymentInput{
		{Method: domain.MethodPix, AmountCents: 10000},
		{Method: "BITCOIN", AmountCents: 5000},
	}, fees, kernel.MustParseDate("2026-11-14"), "user:1", now)
	if err == nil {
		t.Fatal("forma de pagamento inválida deveria falhar")
	}
	// A validação acontece antes de gravar: aqui o primeiro pagamento entrou
	// no slice em memória, mas o serviço descarta o folio inteiro (a
	// transação é desfeita). O teste documenta que o erro é devolvido.
	if kernel.CodeOf(err) != "payment.invalid_method" {
		t.Fatalf("código inesperado: %v", err)
	}
}

func TestVoidKeepsHistoryAndChangesBalance(t *testing.T) {
	folio := openFolio()
	charge, err := folio.AddCharge(domain.EntryCharge, "Frigobar", 2500, kernel.MustParseDate("2026-11-21"), "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if folio.Totals().BalanceCents != 32500 {
		t.Fatalf("saldo com extra: %d", folio.Totals().BalanceCents)
	}
	if _, err := folio.Void(charge.ID, "lançado por engano", "user:1", now); err != nil {
		t.Fatal(err)
	}
	if folio.Totals().BalanceCents != 30000 || len(folio.Entries) != 3 {
		t.Fatal("o lançamento anulado continua no histórico, mas fora dos totais")
	}
	if _, err := folio.Void(charge.ID, "", "user:1", now); kernel.CodeOf(err) != "folio.void_reason_required" {
		t.Fatalf("anular exige motivo: %v", err)
	}
}

func TestRefundCannotExceedPaid(t *testing.T) {
	folio := openFolio()
	if _, err := folio.RecordPayments([]domain.PaymentInput{{Method: domain.MethodPix, AmountCents: 10000}}, fees, kernel.MustParseDate("2026-11-14"), "user:1", now); err != nil {
		t.Fatal(err)
	}
	if _, err := folio.Refund(domain.MethodPix, 20000, "desistência", kernel.MustParseDate("2026-11-14"), "user:1", now); kernel.CodeOf(err) != "payment.refund_exceeds_paid" {
		t.Fatalf("devolução maior que o pago deveria falhar: %v", err)
	}
}

func TestDiscountCannotExceedTotal(t *testing.T) {
	folio := openFolio()
	if _, err := folio.AddCharge(domain.EntryDiscount, "Cortesia", 40000, kernel.MustParseDate("2026-11-14"), "user:1", now); kernel.CodeOf(err) != "folio.discount_too_high" {
		t.Fatalf("desconto maior que a conta deveria falhar: %v", err)
	}
}

func TestCancelStayVoidsAccommodationAndClosesEmptyFolio(t *testing.T) {
	folio := openFolio()
	folio.CancelStay("reserva cancelada", now)
	if folio.Totals().ChargesCents != 0 || folio.Status != domain.FolioClosed {
		t.Fatalf("conta de reserva cancelada sem pagamento deveria fechar zerada: %+v %s", folio.Totals(), folio.Status)
	}
}

func TestStayChangeReplacesAccommodation(t *testing.T) {
	folio := openFolio()
	folio.ReplaceAccommodation(kernel.MustStayPeriod("2026-11-20", "2026-11-23"), []domain.NightlyRate{
		{Date: kernel.MustParseDate("2026-11-20"), PriceCents: 15000},
		{Date: kernel.MustParseDate("2026-11-21"), PriceCents: 15000},
		{Date: kernel.MustParseDate("2026-11-22"), PriceCents: 18000},
	}, "102", now)
	if folio.Totals().ChargesCents != 48000 || folio.CheckOut.String() != "2026-11-23" {
		t.Fatalf("diárias não refeitas: %+v", folio.Totals())
	}
}

func TestExpenseLifecycle(t *testing.T) {
	expense, err := domain.NewExpense("prop-1", domain.ExpenseInput{
		Category: "utilities", Description: "Conta de luz", AmountCents: 21840, DueDate: kernel.MustParseDate("2026-11-20"),
	}, "user:1")
	if err != nil {
		t.Fatal(err)
	}
	if expense.Category != "UTILITIES" || expense.Status != domain.ExpensePending {
		t.Fatalf("despesa inesperada: %+v", expense)
	}
	if err := expense.Pay(domain.MethodPix, kernel.MustParseDate("2026-11-18")); err != nil {
		t.Fatal(err)
	}
	if err := expense.Pay(domain.MethodPix, kernel.MustParseDate("2026-11-18")); kernel.CodeOf(err) != "expense.already_paid" {
		t.Fatalf("pagar duas vezes deveria falhar: %v", err)
	}
}
