package command

import (
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// staleRequestAfter é quanto tempo uma reserva pode aguardar o inventário
// antes de ser rejeitada por timeout.
const staleRequestAfter = 3 * time.Minute

// translate converte os eventos de DOMÍNIO recém-gravados nos eventos de
// INTEGRAÇÃO publicados para os outros serviços.
//
// Manter os dois modelos separados dá liberdade: o domínio pode refatorar
// seus eventos internos sem quebrar ninguém, e o contrato público só expõe o
// que os outros serviços realmente precisam. Alguns fatos internos
// (RegistrationCompleted, StayChangeRejected) nem viram evento de integração.
func translate(state domain.State, events []domain.Event) []contracts.Event {
	summary := summaryOf(state)
	rates := toContractRates(state.NightlyRates)
	var out []contracts.Event

	for _, event := range events {
		switch e := event.(type) {
		case domain.Requested:
			out = append(out, contracts.ReservationRequested{
				ReservationSummary:  summary,
				PreferredRoomID:     e.PreferredRoomID,
				EnforceRestrictions: e.EnforceRestrictions,
				AllowOverbooking:    e.AllowOverbooking,
				QuotedTotalCents:    e.QuotedTotalCents,
			})
		case domain.RoomAllocated:
			out = append(out, contracts.ReservationAccepted{
				ReservationSummary: summary,
				NightlyRates:       rates,
				DepositDueCents:    e.DepositDueCents,
				HoldExpiresAt:      e.HoldExpiresAt,
				PaymentCollect:     state.PaymentCollect,
				Overbooked:         e.Overbooked,
				Confirmed:          state.Status == domain.StatusConfirmed,
			})
		case domain.Rejected:
			out = append(out, contracts.ReservationRejected{ReservationSummary: summary, Reason: e.Reason, Detail: e.Detail})
		case domain.Confirmed:
			out = append(out, contracts.ReservationConfirmed{ReservationSummary: summary, Reason: e.Reason})
		case domain.Cancelled:
			out = append(out, contracts.ReservationCancelled{ReservationSummary: summary, Reason: e.Reason, CancelledBy: e.By})
		case domain.Expired:
			out = append(out, contracts.ReservationExpired{ReservationSummary: summary})
		case domain.MarkedNoShow:
			out = append(out, contracts.ReservationNoShow{ReservationSummary: summary})
		case domain.CheckedIn:
			out = append(out, contracts.ReservationCheckedIn{ReservationSummary: summary, At: e.At})
		case domain.CheckedOut:
			out = append(out, contracts.ReservationCheckedOut{
				ReservationSummary: summary, At: e.At, ActualCheckOut: e.ActualCheckOut.String(),
			})
		case domain.StayChangeRequested:
			out = append(out, contracts.ReservationStayChangeRequested{
				ReservationID: state.ID, PropertyID: state.PropertyID, RoomTypeID: state.RoomTypeID,
				NewRoomID: e.NewRoomID, NewCheckIn: e.NewCheckIn.String(), NewCheckOut: e.NewCheckOut.String(),
				QuotedTotalCents: e.QuotedTotalCents,
			})
		case domain.StayChanged:
			out = append(out, contracts.ReservationStayChanged{ReservationSummary: summary, NightlyRates: rates})
		}
	}
	return out
}

func summaryOf(state domain.State) contracts.ReservationSummary {
	return contracts.ReservationSummary{
		ReservationID: state.ID, Code: state.Code, PropertyID: state.PropertyID,
		RoomTypeID: state.RoomTypeID, RoomID: state.RoomID, RoomName: state.RoomName,
		CheckIn: state.Stay.CheckIn().String(), CheckOut: state.Stay.CheckOut().String(),
		Adults: state.Adults, Children: state.Children,
		Holder: contracts.Holder{Name: state.Holder.Name, Email: state.Holder.Email, Phone: state.Holder.Phone},
		Source: state.Source, ExternalRef: state.ExternalRef,
		TotalCents: state.TotalCents, Currency: state.Currency,
	}
}

func toContractRates(rates []domain.NightlyRate) []contracts.NightlyRate {
	converted := make([]contracts.NightlyRate, len(rates))
	for i, rate := range rates {
		converted[i] = contracts.NightlyRate{Date: rate.Date.String(), PriceCents: rate.PriceCents}
	}
	return converted
}
