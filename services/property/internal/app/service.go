package app

import (
	"context"
	"errors"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/services/property/internal/domain"
)

// Service reúne os casos de uso do catálogo de propriedades.
//
// Padrão de TODO caso de uso de escrita deste serviço:
//
//	tx.WithinTx(ctx, func(ctx) error {
//	    repositório.Salvar(...)          // muda o estado
//	    publisher.Publish(ctx, evento)   // registra o fato na outbox
//	})
//
// As duas operações estão na mesma transação: ou o estado muda E o evento é
// registrado, ou nada acontece.
type Service struct {
	tx         TxRunner
	properties PropertyRepository
	roomTypes  RoomTypeRepository
	rooms      RoomRepository
	publisher  EventPublisher
}

// NewService monta o serviço de aplicação.
func NewService(tx TxRunner, properties PropertyRepository, roomTypes RoomTypeRepository, rooms RoomRepository, publisher EventPublisher) *Service {
	return &Service{tx: tx, properties: properties, roomTypes: roomTypes, rooms: rooms, publisher: publisher}
}

// ---- Propriedades ----

// CreateProperty cadastra uma propriedade.
func (s *Service) CreateProperty(ctx context.Context, input domain.PropertyInput) (*domain.Property, error) {
	property, err := domain.NewProperty(input)
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.properties.Create(ctx, property); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.PropertyRegistered{PropertySnapshot: propertySnapshot(property)})
	})
	if err != nil {
		return nil, err
	}
	return property, nil
}

// UpdateProperty altera os dados e as políticas de uma propriedade.
func (s *Service) UpdateProperty(ctx context.Context, id string, input domain.PropertyInput) (*domain.Property, error) {
	var property *domain.Property
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if property, err = s.properties.ByID(ctx, id); err != nil {
			return err
		}
		if err := property.Update(input); err != nil {
			return err
		}
		if err := s.properties.Update(ctx, property); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.PropertyUpdated{PropertySnapshot: propertySnapshot(property)})
	})
	if err != nil {
		return nil, err
	}
	return property, nil
}

// Property devolve uma propriedade pelo ID.
func (s *Service) Property(ctx context.Context, id string) (*domain.Property, error) {
	return s.properties.ByID(ctx, id)
}

// PropertyBySlug devolve uma propriedade pelo slug.
func (s *Service) PropertyBySlug(ctx context.Context, slug string) (*domain.Property, error) {
	return s.properties.BySlug(ctx, slug)
}

// Properties lista as propriedades (todas, ou só as publicadas no site).
func (s *Service) Properties(ctx context.Context, onlyPublished bool) ([]*domain.Property, error) {
	return s.properties.List(ctx, onlyPublished)
}

// ---- Tipos de acomodação ----

// CreateRoomType define um novo tipo de acomodação.
func (s *Service) CreateRoomType(ctx context.Context, propertyID string, input domain.RoomTypeInput) (*domain.RoomType, error) {
	var roomType *domain.RoomType
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		property, err := s.properties.ByID(ctx, propertyID)
		if err != nil {
			return err
		}
		if roomType, err = domain.NewRoomType(propertyID, input); err != nil {
			return err
		}
		if err := s.roomTypes.Create(ctx, roomType); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.RoomTypeDefined{RoomTypeSnapshot: roomTypeSnapshot(roomType, property.Currency)})
	})
	if err != nil {
		return nil, err
	}
	return roomType, nil
}

// UpdateRoomType altera um tipo de acomodação.
func (s *Service) UpdateRoomType(ctx context.Context, id string, input domain.RoomTypeInput) (*domain.RoomType, error) {
	var roomType *domain.RoomType
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if roomType, err = s.roomTypes.ByID(ctx, id); err != nil {
			return err
		}
		property, err := s.properties.ByID(ctx, roomType.PropertyID)
		if err != nil {
			return err
		}
		if err := roomType.Update(input); err != nil {
			return err
		}
		if err := s.roomTypes.Update(ctx, roomType); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.RoomTypeChanged{RoomTypeSnapshot: roomTypeSnapshot(roomType, property.Currency)})
	})
	if err != nil {
		return nil, err
	}
	return roomType, nil
}

// RoomType devolve um tipo de acomodação pelo ID.
func (s *Service) RoomType(ctx context.Context, id string) (*domain.RoomType, error) {
	return s.roomTypes.ByID(ctx, id)
}

// RoomTypes lista os tipos de acomodação de uma propriedade.
func (s *Service) RoomTypes(ctx context.Context, propertyID string, onlyActive bool) ([]*domain.RoomType, error) {
	return s.roomTypes.ListByProperty(ctx, propertyID, onlyActive)
}

// ---- Quartos ----

// CreateRoom adiciona um quarto.
func (s *Service) CreateRoom(ctx context.Context, propertyID string, input domain.RoomInput) (*domain.Room, error) {
	var room *domain.Room
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.ensureRoomTypeOf(ctx, input.RoomTypeID, propertyID); err != nil {
			return err
		}
		var err error
		if room, err = domain.NewRoom(propertyID, input); err != nil {
			return err
		}
		if err := s.rooms.Create(ctx, room); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.RoomAdded{RoomSnapshot: roomSnapshot(room)})
	})
	if err != nil {
		return nil, err
	}
	return room, nil
}

// UpdateRoom altera um quarto.
func (s *Service) UpdateRoom(ctx context.Context, id string, input domain.RoomInput) (*domain.Room, error) {
	var room *domain.Room
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if room, err = s.rooms.ByID(ctx, id); err != nil {
			return err
		}
		if err := s.ensureRoomTypeOf(ctx, input.RoomTypeID, room.PropertyID); err != nil {
			return err
		}
		if err := room.Update(input); err != nil {
			return err
		}
		if err := s.rooms.Update(ctx, room); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.RoomChanged{RoomSnapshot: roomSnapshot(room)})
	})
	if err != nil {
		return nil, err
	}
	return room, nil
}

// Room devolve um quarto pelo ID.
func (s *Service) Room(ctx context.Context, id string) (*domain.Room, error) {
	return s.rooms.ByID(ctx, id)
}

// Rooms lista os quartos de uma propriedade.
func (s *Service) Rooms(ctx context.Context, propertyID string) ([]*domain.Room, error) {
	return s.rooms.ListByProperty(ctx, propertyID)
}

// SetHousekeeping altera a situação de limpeza de um quarto. É um dado
// puramente operacional deste contexto, por isso não gera evento de integração.
func (s *Service) SetHousekeeping(ctx context.Context, roomID string, status domain.HousekeepingStatus) (*domain.Room, error) {
	room, err := s.rooms.ByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if err := room.SetHousekeeping(status); err != nil {
		return nil, err
	}
	if err := s.rooms.Update(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// MarkRoomDirty é a reação ao evento "check-out realizado": o quarto precisa
// de limpeza antes do próximo hóspede. Quarto inexistente é ignorado — a
// reserva pode ter sido um overbooking sem quarto atribuído.
func (s *Service) MarkRoomDirty(ctx context.Context, roomID string) error {
	if roomID == "" {
		return nil
	}
	room, err := s.rooms.ByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, domain.ErrRoomNotFound) {
			return nil
		}
		return err
	}
	if err := room.SetHousekeeping(domain.HousekeepingDirty); err != nil {
		return err
	}
	return s.rooms.Update(ctx, room)
}

// ensureRoomTypeOf garante que o tipo de acomodação existe e pertence à
// mesma propriedade do quarto.
func (s *Service) ensureRoomTypeOf(ctx context.Context, roomTypeID, propertyID string) error {
	roomType, err := s.roomTypes.ByID(ctx, roomTypeID)
	if err != nil {
		return err
	}
	if roomType.PropertyID != propertyID {
		return domain.ErrRoomTypeNotFound
	}
	return nil
}

// ---- Tradução do modelo de domínio para os contratos de eventos ----

func propertySnapshot(p *domain.Property) contracts.PropertySnapshot {
	return contracts.PropertySnapshot{
		PropertyID: p.ID, Slug: p.Slug, Name: p.Name, Kind: string(p.Kind),
		Timezone: p.Timezone, Currency: p.Currency,
		CheckInTime: p.CheckInTime, CheckOutTime: p.CheckOutTime,
		DepositPercent: p.DepositPercent, HoldMinutes: p.HoldMinutes,
		Email: p.Email, Phone: p.Phone,
		AddressLine: p.Address.Line, City: p.Address.City, State: p.Address.State,
		PixKey: p.PixKey, Published: p.Published,
	}
}

func roomTypeSnapshot(t *domain.RoomType, currency string) contracts.RoomTypeSnapshot {
	return contracts.RoomTypeSnapshot{
		RoomTypeID: t.ID, PropertyID: t.PropertyID, Name: t.Name,
		MaxOccupancy: t.MaxOccupancy, BasePriceCents: t.BasePriceCents,
		Currency: currency, Active: t.Active,
	}
}

func roomSnapshot(r *domain.Room) contracts.RoomSnapshot {
	return contracts.RoomSnapshot{
		RoomID: r.ID, PropertyID: r.PropertyID, RoomTypeID: r.RoomTypeID, Name: r.Name, Active: r.Active,
	}
}
