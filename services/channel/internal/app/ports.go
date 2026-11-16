// Package app é a camada de aplicação do serviço de canais de venda.
package app

import (
	"context"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/channel/internal/domain"
)

// Connector é a PORTA para um canal de venda. Cada canal/modo tem o seu
// adaptador (padrão Strategy): o restante do serviço fala só com esta
// interface e não sabe se do outro lado há XML, JSON ou um arquivo .ics.
//
// Acrescentar um canal novo (Expedia, Decolar, a API do Airbnb...) é
// escrever mais um adaptador e registrá-lo — nada mais muda (princípio
// Aberto/Fechado, o "O" do SOLID).
type Connector interface {
	// PushAvailability envia ao canal quantos quartos estão à venda por data.
	PushAvailability(ctx context.Context, connection domain.Connection, mapping domain.Mapping, days []domain.DayAvailability) error
	// PushRates envia ao canal os preços e restrições por data.
	PushRates(ctx context.Context, connection domain.Connection, mapping domain.Mapping, currency string, days []domain.DayRate) error
	// Pull busca no canal as reservas novas, alteradas e canceladas.
	Pull(ctx context.Context, connection domain.Connection, mappings []domain.Mapping) (domain.PullResult, error)
	// Acknowledge confirma ao canal que as reservas foram recebidas.
	Acknowledge(ctx context.Context, connection domain.Connection, reservations []domain.ExternalReservation) error
}

// ConnectorRegistry localiza o adaptador de um canal/modo.
type ConnectorRegistry interface {
	For(channel domain.Channel, mode domain.Mode) (Connector, bool)
}

// KnownReservation é o que já sabemos de uma reserva do canal.
type KnownReservation struct {
	ExternalID  string
	MappingID   string
	Fingerprint string
	Cancelled   bool
	CheckOut    kernel.Date
}

// Repository persiste o estado do serviço de canais.
type Repository interface {
	CreateConnection(ctx context.Context, connection *domain.Connection) error
	UpdateConnection(ctx context.Context, connection *domain.Connection) error
	DeleteConnection(ctx context.Context, id string) error
	Connection(ctx context.Context, id string) (*domain.Connection, error)
	Connections(ctx context.Context, propertyID string) ([]*domain.Connection, error)
	ActiveConnections(ctx context.Context) ([]*domain.Connection, error)
	MarkPush(ctx context.Context, connectionID string, at time.Time, lastError string) error
	MarkPull(ctx context.Context, connectionID string, at time.Time, lastError string) error

	CreateMapping(ctx context.Context, mapping *domain.Mapping) error
	DeleteMapping(ctx context.Context, id string) error
	Mapping(ctx context.Context, id string) (*domain.Mapping, error)
	MappingByToken(ctx context.Context, token string) (*domain.Mapping, error)
	MappingsByConnection(ctx context.Context, connectionID string) ([]domain.Mapping, error)
	MappingsByRoomType(ctx context.Context, roomTypeID string) ([]domain.Mapping, error)

	// Retrato local da disponibilidade (alimentado pelos eventos do inventário).
	SaveAvailability(ctx context.Context, roomTypeID string, days []domain.DayAvailability) error
	SoldOutDays(ctx context.Context, roomTypeID string, from, to kernel.Date) ([]kernel.Date, error)

	KnownReservations(ctx context.Context, connectionID string) ([]KnownReservation, error)
	SaveKnownReservation(ctx context.Context, connectionID string, known KnownReservation) error

	AddSyncLog(ctx context.Context, log domain.SyncLog) error
	SyncLogs(ctx context.Context, connectionID string, limit int) ([]domain.SyncLog, error)

	// TryPullLock tenta obter a trava que elege a réplica responsável pela
	// rodada de busca de reservas (válida até o fim da transação).
	TryPullLock(ctx context.Context) (bool, error)
}

// EventPublisher publica eventos de integração (via outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner executa uma função dentro de uma transação (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
