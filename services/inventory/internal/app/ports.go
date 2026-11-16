// Package app é a camada de aplicação do serviço de inventário.
package app

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/inventory/internal/domain"
)

// CatalogRepository guarda a cópia local do catálogo (tipos e quartos).
type CatalogRepository interface {
	// UpsertRoomType grava o tipo e devolve o estado anterior (nil se era novo).
	UpsertRoomType(ctx context.Context, roomType domain.RoomType) (*domain.RoomType, error)
	// UpsertRoom grava o quarto e devolve o estado anterior (nil se era novo).
	UpsertRoom(ctx context.Context, room domain.Room) (*domain.Room, error)
	RoomType(ctx context.Context, id string) (*domain.RoomType, error)
	Room(ctx context.Context, id string) (*domain.Room, error)
	RoomTypes(ctx context.Context, propertyID string) ([]domain.RoomType, error)
}

// AllocationRepository guarda as ocupações de quartos.
type AllocationRepository interface {
	// Insert devolve domain.ErrRoomTaken se o quarto estiver ocupado no período.
	Insert(ctx context.Context, allocation domain.Allocation) error
	// Update muda quarto e/ou datas; também devolve domain.ErrRoomTaken.
	Update(ctx context.Context, allocation domain.Allocation) error
	Delete(ctx context.Context, id string) error
	ByID(ctx context.Context, id string) (*domain.Allocation, error)
	ByReservation(ctx context.Context, reservationID string) (*domain.Allocation, error)
	// FreeRooms lista os quartos ativos do tipo sem ocupação no período,
	// ignorando a alocação excludeID (útil ao mover uma reserva).
	FreeRooms(ctx context.Context, roomTypeID string, stay kernel.StayPeriod, excludeID string) ([]domain.Room, error)
	List(ctx context.Context, propertyID string, from, to kernel.Date, kind domain.AllocationKind) ([]domain.Allocation, error)
}

// RateChange descreve uma alteração de tarifa/restrições. Campos nulos não
// são alterados.
type RateChange struct {
	PriceCents *int64
	MinStay    *int
	Closed     *bool
}

// CalendarRepository responde "quanto há e quanto custa" por data.
type CalendarRepository interface {
	// Days devolve a situação diária, por tipo de quarto, no intervalo
	// [from, to). roomTypeID vazio = todos os tipos ativos da propriedade.
	Days(ctx context.Context, propertyID, roomTypeID string, from, to kernel.Date) (map[string][]domain.DayStatus, error)
	SetRates(ctx context.Context, roomTypeID string, dates []kernel.Date, change RateChange) error
}

// OfferCache guarda resultados da busca pública de disponibilidade.
type OfferCache interface {
	Offers(ctx context.Context, propertyID, key string, load func(ctx context.Context) ([]domain.Offer, error)) ([]domain.Offer, error)
	Invalidate(ctx context.Context, propertyID string)
}

// EventPublisher publica eventos de integração (via outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner controla transações (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
	WithinSavepoint(ctx context.Context, fn func(ctx context.Context) error) error
	AfterCommit(ctx context.Context, fn func())
}
