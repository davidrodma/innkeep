// Package consumer é o adaptador de entrada de EVENTOS do inventário.
package consumer

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/services/inventory/internal/app"
)

// Register associa os eventos de interesse aos casos de uso.
func Register(router *messaging.Router, allocations *app.AllocationService, calendar *app.CalendarService) {
	// --- Catálogo: mantém a cópia local de tipos e quartos ---
	applyRoomType := func(ctx context.Context, snapshot contracts.RoomTypeSnapshot) error {
		return calendar.ApplyRoomType(ctx, snapshot)
	}
	router.On(contracts.TypeRoomTypeDefined, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RoomTypeDefined) error {
			return applyRoomType(ctx, e.RoomTypeSnapshot)
		}))
	router.On(contracts.TypeRoomTypeChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RoomTypeChanged) error {
			return applyRoomType(ctx, e.RoomTypeSnapshot)
		}))
	router.On(contracts.TypeRoomAdded, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RoomAdded) error {
			return calendar.ApplyRoom(ctx, e.RoomSnapshot)
		}))
	router.On(contracts.TypeRoomChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RoomChanged) error {
			return calendar.ApplyRoom(ctx, e.RoomSnapshot)
		}))

	// --- Saga de reserva ---
	router.On(contracts.TypeReservationRequested, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationRequested) error {
			return allocations.Allocate(ctx, e)
		}))
	router.On(contracts.TypeReservationStayChangeRequested, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationStayChangeRequested) error {
			return allocations.Change(ctx, e)
		}))

	// Compensações: todo desfecho que encerra a reserva devolve o quarto.
	router.On(contracts.TypeReservationCancelled, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCancelled) error {
			return allocations.Release(ctx, e.ReservationID)
		}))
	router.On(contracts.TypeReservationExpired, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationExpired) error {
			return allocations.Release(ctx, e.ReservationID)
		}))
	router.On(contracts.TypeReservationNoShow, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationNoShow) error {
			return allocations.Release(ctx, e.ReservationID)
		}))
	router.On(contracts.TypeReservationCheckedOut, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCheckedOut) error {
			return allocations.Shorten(ctx, e.ReservationID, e.ActualCheckOut)
		}))

	// --- Canais: pedido de reenvio completo ---
	router.On(contracts.TypeChannelSyncRequested, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ChannelSyncRequested) error {
			return calendar.Resync(ctx, e)
		}))
}
