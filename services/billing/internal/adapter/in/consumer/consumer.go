// Package consumer é o adaptador de entrada de EVENTOS do serviço financeiro.
package consumer

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/services/billing/internal/app"
)

// Register associa os eventos de reserva aos casos de uso do folio.
//
// O financeiro nunca é chamado pelo serviço de reservas: ele observa os
// fatos da reserva e mantém a conta em dia por conta própria.
func Register(router *messaging.Router, folios *app.FolioService) {
	// Reserva aceita => abre a conta e lança as diárias.
	router.On(contracts.TypeReservationAccepted, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationAccepted) error {
			return folios.OpenForReservation(ctx, e)
		}))
	// Estadia alterada => refaz as diárias.
	router.On(contracts.TypeReservationStayChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationStayChanged) error {
			return folios.ApplyStayChange(ctx, e)
		}))
	// Reserva que não vai acontecer => anula as diárias.
	router.On(contracts.TypeReservationCancelled, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCancelled) error {
			return folios.CancelForReservation(ctx, e.ReservationID, "reserva cancelada")
		}))
	router.On(contracts.TypeReservationExpired, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationExpired) error {
			return folios.CancelForReservation(ctx, e.ReservationID, "reserva expirada")
		}))
	router.On(contracts.TypeReservationNoShow, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationNoShow) error {
			return folios.CancelForReservation(ctx, e.ReservationID, "no-show")
		}))
	// Check-out => fecha a conta, se quitada.
	router.On(contracts.TypeReservationCheckedOut, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCheckedOut) error {
			return folios.SettleForReservation(ctx, e.ReservationID)
		}))
}
