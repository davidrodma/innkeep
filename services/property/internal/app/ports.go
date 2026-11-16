// Package app é a camada de aplicação do serviço de propriedades.
package app

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/services/property/internal/domain"
)

// PropertyRepository persiste propriedades.
type PropertyRepository interface {
	Create(ctx context.Context, property *domain.Property) error
	Update(ctx context.Context, property *domain.Property) error
	ByID(ctx context.Context, id string) (*domain.Property, error)
	BySlug(ctx context.Context, slug string) (*domain.Property, error)
	List(ctx context.Context, onlyPublished bool) ([]*domain.Property, error)
}

// RoomTypeRepository persiste tipos de acomodação.
type RoomTypeRepository interface {
	Create(ctx context.Context, roomType *domain.RoomType) error
	Update(ctx context.Context, roomType *domain.RoomType) error
	ByID(ctx context.Context, id string) (*domain.RoomType, error)
	ListByProperty(ctx context.Context, propertyID string, onlyActive bool) ([]*domain.RoomType, error)
}

// RoomRepository persiste quartos.
type RoomRepository interface {
	Create(ctx context.Context, room *domain.Room) error
	Update(ctx context.Context, room *domain.Room) error
	ByID(ctx context.Context, id string) (*domain.Room, error)
	ListByProperty(ctx context.Context, propertyID string) ([]*domain.Room, error)
}

// EventPublisher publica eventos de integração. A implementação grava na
// tabela outbox dentro da transação corrente (padrão Transactional Outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner executa uma função dentro de uma transação (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
