// Package consumer é o adaptador de entrada de EVENTOS do serviço de
// reservas: é por aqui que as sagas continuam.
package consumer

import (
	"context"
	"errors"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/command"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/port"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// Register associa os eventos de interesse aos comandos.
func Register(router *messaging.Router, commands *command.Handler) {
	// --- Propriedades: cópia local das políticas ---
	applyPolicy := func(ctx context.Context, s contracts.PropertySnapshot) error {
		return commands.ApplyPolicy(ctx, port.PropertyPolicy{
			PropertyID: s.PropertyID, Name: s.Name, Timezone: s.Timezone, Currency: s.Currency,
			DepositPercent: s.DepositPercent, HoldMinutes: s.HoldMinutes, Published: s.Published,
		})
	}
	router.On(contracts.TypePropertyRegistered, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.PropertyRegistered) error {
			return applyPolicy(ctx, e.PropertySnapshot)
		}))
	router.On(contracts.TypePropertyUpdated, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.PropertyUpdated) error {
			return applyPolicy(ctx, e.PropertySnapshot)
		}))

	// --- Inventário: respostas da saga de reserva ---
	router.On(contracts.TypeAllocationConfirmed, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.AllocationConfirmed) error {
			return ignoreMissing(commands.RecordAllocation(ctx, e.ReservationID, domain.Allocation{
				RoomTypeID: e.RoomTypeID, RoomID: e.RoomID, RoomName: e.RoomName,
				NightlyRates: toDomainRates(e.NightlyRates), TotalCents: e.TotalCents, Overbooked: e.Overbooked,
			}))
		}))
	router.On(contracts.TypeAllocationRejected, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.AllocationRejected) error {
			return ignoreMissing(commands.RecordRejection(ctx, e.ReservationID, e.Reason, e.Detail))
		}))
	router.On(contracts.TypeAllocationChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.AllocationChanged) error {
			return ignoreMissing(commands.RecordStayChange(ctx, e.ReservationID, domain.Allocation{
				RoomTypeID: e.RoomTypeID, RoomID: e.RoomID, RoomName: e.RoomName,
				NightlyRates: toDomainRates(e.NightlyRates), TotalCents: e.TotalCents,
			}, e.CheckIn, e.CheckOut))
		}))
	router.On(contracts.TypeAllocationChangeRejected, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.AllocationChangeRejected) error {
			return ignoreMissing(commands.RecordStayChangeRejection(ctx, e.ReservationID, e.Reason, e.Detail))
		}))

	// --- Financeiro: sinal pago => reserva confirmada ---
	router.On(contracts.TypeDepositSettled, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.DepositSettled) error {
			return ignoreMissing(commands.RecordDeposit(ctx, e.ReservationID))
		}))

	// --- Hóspedes: FNRH preenchida => check-in liberado ---
	router.On(contracts.TypeRegistrationCompleted, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RegistrationCompleted) error {
			return ignoreMissing(commands.RecordRegistration(ctx, e.ReservationID, e.PrimaryGuestID, e.GuestCount))
		}))

	// --- Canais de venda: reservas feitas no Booking.com / Airbnb ---
	router.On(contracts.TypeChannelReservationReceived, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ChannelReservationReceived) error {
			return requestFromChannel(ctx, commands, e.ChannelReservation)
		}))
	router.On(contracts.TypeChannelReservationModified, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ChannelReservationModified) error {
			id := contracts.ReservationIDForChannel(e.PropertyID, e.Channel, e.ExternalID)
			_, err := commands.ChangeStay(ctx, id, e.CheckIn, e.CheckOut, e.RoomID, quotedTotal(e.TotalCents))
			switch {
			case errors.Is(err, domain.ErrNotFound):
				// A modificação chegou antes da criação (ou a criação se
				// perdeu): cria a reserva já com os dados novos.
				return requestFromChannel(ctx, commands, e.ChannelReservation)
			case kernel.CodeOf(err) == "reservation.nothing_to_change":
				return nil
			}
			return err
		}))
	router.On(contracts.TypeChannelReservationCancelled, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ChannelReservationCancelled) error {
			id := contracts.ReservationIDForChannel(e.PropertyID, e.Channel, e.ExternalID)
			_, err := commands.Cancel(ctx, id, e.Reason, "channel:"+e.Channel)
			return ignoreMissing(err)
		}))
}

// requestFromChannel cria a reserva interna correspondente a uma reserva de
// canal. O ID é determinístico (derivado do canal + nº externo), de modo que
// receber a mesma reserva duas vezes não a duplica.
func requestFromChannel(ctx context.Context, commands *command.Handler, r contracts.ChannelReservation) error {
	_, err := commands.Request(ctx, command.RequestReservation{
		ID:         contracts.ReservationIDForChannel(r.PropertyID, r.Channel, r.ExternalID),
		PropertyID: r.PropertyID, RoomTypeID: r.RoomTypeID, PreferredRoomID: r.RoomID,
		CheckIn: r.CheckIn, CheckOut: r.CheckOut, Adults: max(r.Adults, 1), Children: r.Children,
		Holder: domain.Holder{Name: r.Holder.Name, Email: r.Holder.Email, Phone: r.Holder.Phone},
		Source: r.Channel, ExternalRef: r.ExternalID, Notes: r.Notes,
		QuotedTotalCents: quotedTotal(r.TotalCents), PaymentCollect: r.PaymentCollect,
	})
	return err
}

// quotedTotal devolve o preço informado pelo canal, ou nil quando ele não
// informou (caso do iCal) — e então vale a tarifa vigente do inventário.
func quotedTotal(totalCents int64) *int64 {
	if totalCents <= 0 {
		return nil
	}
	return &totalCents
}

// ignoreMissing trata "reserva não encontrada" como sucesso: um evento sobre
// uma reserva que este serviço não conhece não tem o que alterar, e tentar de
// novo não mudaria isso.
func ignoreMissing(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}

func toDomainRates(rates []contracts.NightlyRate) []domain.NightlyRate {
	converted := make([]domain.NightlyRate, 0, len(rates))
	for _, rate := range rates {
		date, err := kernel.ParseDate(rate.Date)
		if err != nil {
			continue
		}
		converted = append(converted, domain.NightlyRate{Date: date, PriceCents: rate.PriceCents})
	}
	return converted
}
