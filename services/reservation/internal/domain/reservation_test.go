package domain_test

// Testes do agregado Reservation. Com Event Sourcing, o teste segue um
// formato muito legível — "Dado / Quando / Então":
//
//	Dado    um histórico de eventos (o passado),
//	Quando  um comando é executado,
//	Então   estes novos eventos são produzidos (ou este erro acontece).
//
// Nenhum banco, Kafka ou HTTP: o domínio é código puro.

import (
	"testing"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

var (
	now   = time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	today = kernel.MustParseDate("2026-11-14")
)

const (
	propertyID = "01a1dab0-0000-7000-8000-000000000001"
	roomTypeID = "01a1dab0-0000-7000-8000-000000000002"
	roomID     = "01a1dab0-0000-7000-8000-000000000003"
)

func request(t *testing.T, policy domain.Policy, mutate ...func(*domain.RequestInput)) *domain.Reservation {
	t.Helper()
	input := domain.RequestInput{
		Code: "IK-TESTE1", PropertyID: propertyID, RoomTypeID: roomTypeID,
		Stay: kernel.MustStayPeriod("2026-11-20", "2026-11-22"), Adults: 2,
		Holder: domain.Holder{Name: "Hóspede de Teste", Email: "hospede@example.com"},
		Source: domain.SourceWebsite,
	}
	for _, fn := range mutate {
		fn(&input)
	}
	reservation, err := domain.Request(input, policy, now, today)
	if err != nil {
		t.Fatalf("Request falhou: %v", err)
	}
	return reservation
}

var halfDeposit = domain.Policy{DepositPercent: 50, HoldMinutes: 60, Currency: "BRL"}

func allocate(r *domain.Reservation, totalCents int64) {
	r.RecordAllocation(domain.Allocation{RoomTypeID: roomTypeID, RoomID: roomID, RoomName: "101", TotalCents: totalCents}, now)
}

func eventNames(events []domain.Event) []string {
	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.EventName()
	}
	return names
}

func assertEvents(t *testing.T, got []domain.Event, want ...string) {
	t.Helper()
	names := eventNames(got)
	if len(names) != len(want) {
		t.Fatalf("eventos = %v; esperado %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("eventos = %v; esperado %v", names, want)
		}
	}
}

func TestRequestFromWebsiteWaitsForDeposit(t *testing.T) {
	r := request(t, halfDeposit)
	allocate(r, 30000)

	state := r.State()
	if state.Status != domain.StatusPending {
		t.Fatalf("status = %s; esperado PENDING", state.Status)
	}
	if state.DepositDueCents != 15000 {
		t.Fatalf("sinal = %d; esperado 15000 (50%% de 30000)", state.DepositDueCents)
	}
	if state.HoldExpiresAt == nil || !state.HoldExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("prazo do sinal inesperado: %v", state.HoldExpiresAt)
	}
	assertEvents(t, r.Changes(), domain.EventRequested, domain.EventRoomAllocated)
}

func TestNoDepositPolicyConfirmsImmediately(t *testing.T) {
	r := request(t, domain.Policy{Currency: "BRL"})
	allocate(r, 30000)
	if r.State().Status != domain.StatusConfirmed {
		t.Fatalf("sem sinal exigido a reserva deveria confirmar: %s", r.State().Status)
	}
	assertEvents(t, r.Changes(), domain.EventRequested, domain.EventRoomAllocated, domain.EventConfirmed)
}

func TestChannelReservationConfirmsAndAllowsOverbooking(t *testing.T) {
	r := request(t, halfDeposit, func(in *domain.RequestInput) { in.Source = domain.SourceBookingCom })
	state := r.State()
	if !state.AllowOverbooking || state.EnforceRestrictions {
		t.Fatal("reserva de canal deve aceitar overbooking e ignorar restrições de venda")
	}
	allocate(r, 30000)
	if r.State().Status != domain.StatusConfirmed {
		t.Fatalf("reserva de canal já vem confirmada: %s", r.State().Status)
	}
}

func TestWebsiteRequestValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.RequestInput)
		code   string
	}{
		{"sem contato", func(in *domain.RequestInput) { in.Holder.Email, in.Holder.Phone = "", "" }, "reservation.contact_required"},
		{"no passado", func(in *domain.RequestInput) { in.Stay = kernel.MustStayPeriod("2026-11-10", "2026-11-12") }, "stay.in_the_past"},
		{"sem adultos", func(in *domain.RequestInput) { in.Adults = 0 }, "reservation.invalid_guests"},
		{"nome curto", func(in *domain.RequestInput) { in.Holder.Name = "A" }, "reservation.invalid_holder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := domain.RequestInput{
				PropertyID: propertyID, RoomTypeID: roomTypeID, Stay: kernel.MustStayPeriod("2026-11-20", "2026-11-22"),
				Adults: 2, Holder: domain.Holder{Name: "Hóspede", Email: "h@example.com"}, Source: domain.SourceWebsite,
			}
			tt.mutate(&input)
			_, err := domain.Request(input, halfDeposit, now, today)
			if kernel.CodeOf(err) != tt.code {
				t.Fatalf("código = %q; esperado %q", kernel.CodeOf(err), tt.code)
			}
		})
	}
}

func TestCheckInRequiresRegistration(t *testing.T) {
	r := request(t, domain.Policy{Currency: "BRL"}, func(in *domain.RequestInput) {
		in.Stay = kernel.MustStayPeriod("2026-11-14", "2026-11-16")
	})
	allocate(r, 20000)

	// Sem a FNRH, o check-in é recusado (Lei Geral do Turismo).
	if err := r.CheckIn(now, today); kernel.CodeOf(err) != "reservation.registration_required" {
		t.Fatalf("esperado erro de FNRH; obtido %v", err)
	}

	r.RecordRegistration("guest-1", 2, now)
	if err := r.CheckIn(now, today); err != nil {
		t.Fatalf("check-in com FNRH deveria funcionar: %v", err)
	}
	if r.State().Status != domain.StatusCheckedIn {
		t.Fatalf("status = %s", r.State().Status)
	}
}

func TestCheckInDateRules(t *testing.T) {
	r := request(t, domain.Policy{Currency: "BRL"})
	allocate(r, 20000)
	r.RecordRegistration("guest-1", 2, now)
	if err := r.CheckIn(now, today); kernel.CodeOf(err) != "reservation.check_in_too_early" {
		t.Fatalf("check-in antes da data deveria falhar: %v", err)
	}
}

func TestInvalidTransitions(t *testing.T) {
	r := request(t, domain.Policy{Currency: "BRL"})
	r.RecordRejection("NO_AVAILABILITY", "", now)

	if err := r.Cancel("teste", "user:1", now); kernel.CodeOf(err) != "reservation.invalid_transition" {
		t.Fatalf("cancelar uma reserva rejeitada deveria falhar: %v", err)
	}
	if err := r.Confirm(domain.ConfirmedManually, now); kernel.CodeOf(err) != "reservation.invalid_transition" {
		t.Fatalf("confirmar uma reserva rejeitada deveria falhar: %v", err)
	}
}

func TestLateEventsAreIgnored(t *testing.T) {
	// A alocação chega DEPOIS do cancelamento (eventos de serviços diferentes
	// não têm ordem garantida): o fato tardio não ressuscita a reserva.
	r := request(t, halfDeposit)
	if err := r.Cancel("desistiu", "guest", now); err != nil {
		t.Fatal(err)
	}
	r.ClearChanges()
	allocate(r, 30000)
	if len(r.Changes()) != 0 || r.State().Status != domain.StatusCancelled {
		t.Fatalf("alocação tardia não deveria gerar eventos: %v", eventNames(r.Changes()))
	}
}

func TestConfirmIsIdempotent(t *testing.T) {
	r := request(t, halfDeposit)
	allocate(r, 30000)
	if err := r.Confirm(domain.ConfirmedByDeposit, now); err != nil {
		t.Fatal(err)
	}
	r.ClearChanges()
	if err := r.Confirm(domain.ConfirmedByDeposit, now); err != nil || len(r.Changes()) != 0 {
		t.Fatalf("confirmar de novo deveria ser inócuo: %v %v", err, eventNames(r.Changes()))
	}
}

func TestExpireOnlyAfterDeadline(t *testing.T) {
	r := request(t, halfDeposit)
	allocate(r, 30000)
	r.Expire(now.Add(30 * time.Minute))
	if r.State().Status != domain.StatusPending {
		t.Fatal("não deveria expirar antes do prazo")
	}
	r.Expire(now.Add(61 * time.Minute))
	if r.State().Status != domain.StatusExpired {
		t.Fatalf("deveria expirar depois do prazo: %s", r.State().Status)
	}
}

func TestEarlyCheckOutReleasesRemainingNights(t *testing.T) {
	r := request(t, domain.Policy{Currency: "BRL"}, func(in *domain.RequestInput) {
		// A equipe pode lançar uma entrada do dia anterior (hoje é 14/11).
		in.Stay = kernel.MustStayPeriod("2026-11-13", "2026-11-18")
		in.Source = domain.SourceDirect
	})
	allocate(r, 60000)
	r.RecordRegistration("guest-1", 1, now)
	if err := r.CheckIn(now, kernel.MustParseDate("2026-11-13")); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckOut(now, today); err != nil {
		t.Fatal(err)
	}
	last := r.Changes()[len(r.Changes())-1].(domain.CheckedOut)
	if last.ActualCheckOut.String() != "2026-11-14" {
		t.Fatalf("saída antecipada deveria registrar a data real: %s", last.ActualCheckOut)
	}
}

// TestReplayRebuildsTheSameState é a propriedade fundamental do Event
// Sourcing: reaplicar o histórico reconstrói exatamente o mesmo estado.
func TestReplayRebuildsTheSameState(t *testing.T) {
	original := request(t, halfDeposit)
	allocate(original, 30000)
	if err := original.Confirm(domain.ConfirmedByDeposit, now); err != nil {
		t.Fatal(err)
	}
	newStay := kernel.MustStayPeriod("2026-11-21", "2026-11-24")
	if err := original.RequestStayChange(newStay, "", nil, now); err != nil {
		t.Fatal(err)
	}
	original.RecordStayChange(domain.Allocation{RoomTypeID: roomTypeID, RoomID: roomID, RoomName: "101", TotalCents: 45000}, newStay, now)

	replayed := domain.FromHistory(original.Changes())

	a, b := original.State(), replayed.State()
	if a.Status != b.Status || a.TotalCents != b.TotalCents || a.Stay != b.Stay || a.Version != b.Version || a.RoomID != b.RoomID {
		t.Fatalf("estado reconstruído difere do original:\n original:   %+v\n reconstruído: %+v", a, b)
	}
	if b.Stay.CheckIn().String() != "2026-11-21" || b.TotalCents != 45000 || b.PendingChange != nil {
		t.Fatalf("mudança de estadia não reaplicada corretamente: %+v", b)
	}
}

func TestDecodeEventRoundTrip(t *testing.T) {
	r := request(t, halfDeposit)
	for _, event := range r.Changes() {
		payload := mustJSON(t, event)
		decoded, err := domain.DecodeEvent(event.EventName(), payload)
		if err != nil {
			t.Fatalf("decodificando %s: %v", event.EventName(), err)
		}
		if decoded.EventName() != event.EventName() {
			t.Fatalf("tipo decodificado diferente: %s", decoded.EventName())
		}
	}
	if _, err := domain.DecodeEvent("reservation.unknown", []byte(`{}`)); err == nil {
		t.Fatal("evento desconhecido deveria falhar")
	}
}
