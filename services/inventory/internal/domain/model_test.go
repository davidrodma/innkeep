package domain_test

import (
	"testing"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/inventory/internal/domain"
)

func day(date string, total, occupied int, price int64, minStay int, closed bool) domain.DayStatus {
	return domain.DayStatus{Date: kernel.MustParseDate(date), Total: total, Occupied: occupied, PriceCents: price, MinStay: minStay, Closed: closed}
}

func TestAvailableIsNeverNegative(t *testing.T) {
	// Overbooking: 3 ocupações para 2 quartos. Os canais devem ver zero, não -1.
	if got := day("2026-11-20", 2, 3, 10000, 1, false).Available(); got != 0 {
		t.Fatalf("Available = %d; esperado 0", got)
	}
}

func TestStayFitsOnlyIfEveryNightHasRoom(t *testing.T) {
	days := []domain.DayStatus{
		day("2026-11-20", 2, 0, 10000, 1, false),
		day("2026-11-21", 2, 2, 10000, 1, false), // noite esgotada no meio da estadia
		day("2026-11-22", 2, 1, 10000, 1, false),
	}
	if got := domain.MinAvailable(days); got != 0 {
		t.Fatalf("MinAvailable = %d; esperado 0", got)
	}
}

func TestRestrictions(t *testing.T) {
	// Estadia mínima de 3 noites definida no dia da CHEGADA.
	if rejection := domain.CheckRestrictions([]domain.DayStatus{
		day("2026-12-30", 1, 0, 30000, 3, false),
		day("2026-12-31", 1, 0, 30000, 1, false),
	}); rejection == nil || rejection.Reason != domain.RejectMinStay {
		t.Fatalf("esperada rejeição por estadia mínima: %+v", rejection)
	}

	if rejection := domain.CheckRestrictions([]domain.DayStatus{
		day("2026-12-24", 1, 0, 30000, 1, false),
		day("2026-12-25", 1, 0, 30000, 1, true),
	}); rejection == nil || rejection.Reason != domain.RejectClosed {
		t.Fatalf("esperada rejeição por venda fechada: %+v", rejection)
	}
}

func TestQuotes(t *testing.T) {
	days := []domain.DayStatus{
		day("2026-11-20", 1, 0, 10000, 1, false),
		day("2026-11-21", 1, 0, 15000, 1, false), // sábado mais caro
	}
	if quote := domain.QuoteFromRates(days, "BRL"); quote.Total.Cents() != 25000 || len(quote.Nightly) != 2 {
		t.Fatalf("cotação pela tarifa inesperada: %+v", quote)
	}

	// Preço fechado de R$ 100,00 em 3 noites: 33,34 + 33,33 + 33,33.
	fixed := domain.QuoteFixed(kernel.MustStayPeriod("2026-11-20", "2026-11-23"), kernel.BRL(10000))
	var sum int64
	for _, night := range fixed.Nightly {
		sum += night.PriceCents
	}
	if sum != 10000 || fixed.Nightly[0].PriceCents != 3334 {
		t.Fatalf("divisão do preço fechado perdeu centavos: %+v", fixed.Nightly)
	}
}

func TestBuildOffer(t *testing.T) {
	roomType := domain.RoomType{ID: "t1", Name: "Suíte", MaxOccupancy: 2, Currency: "BRL"}
	days := []domain.DayStatus{day("2026-11-20", 1, 0, 15000, 1, false)}

	if offer := domain.BuildOffer(roomType, days, 2); !offer.Bookable || offer.TotalCents != 15000 {
		t.Fatalf("oferta deveria ser reservável: %+v", offer)
	}
	if offer := domain.BuildOffer(roomType, days, 3); offer.Bookable || offer.Reason != domain.RejectOverCapacity {
		t.Fatalf("3 hóspedes não cabem na suíte: %+v", offer)
	}
	full := []domain.DayStatus{day("2026-11-20", 1, 1, 15000, 1, false)}
	if offer := domain.BuildOffer(roomType, full, 2); offer.Bookable || offer.Reason != domain.RejectNoAvailability {
		t.Fatalf("sem quarto livre: %+v", offer)
	}
}
