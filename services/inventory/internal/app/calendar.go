package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/inventory/internal/domain"
)

// CalendarService cuida do calendário de disponibilidade e tarifas: consultas
// (lado de leitura), alterações de tarifa, bloqueios e a manutenção da cópia
// local do catálogo.
type CalendarService struct {
	tx          TxRunner
	catalog     CatalogRepository
	allocations AllocationRepository
	calendar    CalendarRepository
	cache       OfferCache
	publisher   EventPublisher
	// horizonDays é até onde, no futuro, o inventário é anunciado aos canais.
	horizonDays int
}

// NewCalendarService monta o serviço.
func NewCalendarService(tx TxRunner, catalog CatalogRepository, allocations AllocationRepository, calendar CalendarRepository,
	cache OfferCache, publisher EventPublisher, horizonDays int) *CalendarService {
	if horizonDays <= 0 {
		horizonDays = 365
	}
	return &CalendarService{tx: tx, catalog: catalog, allocations: allocations, calendar: calendar,
		cache: cache, publisher: publisher, horizonDays: horizonDays}
}

// ---- Consultas ----

// RoomTypeCalendar é a situação diária de um tipo de quarto.
type RoomTypeCalendar struct {
	RoomType domain.RoomType
	Days     []domain.DayStatus
}

// Grid devolve a grade de disponibilidade e tarifas da propriedade em [from, to).
func (s *CalendarService) Grid(ctx context.Context, propertyID string, from, to kernel.Date) ([]RoomTypeCalendar, error) {
	if !to.After(from) || from.DaysUntil(to) > 400 {
		return nil, kernel.Invalid("inventory.invalid_range", "intervalo inválido: informe de 1 a 400 dias")
	}
	roomTypes, err := s.catalog.RoomTypes(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	byType, err := s.calendar.Days(ctx, propertyID, "", from, to)
	if err != nil {
		return nil, err
	}
	grid := make([]RoomTypeCalendar, 0, len(roomTypes))
	for _, roomType := range roomTypes {
		if roomType.Active {
			grid = append(grid, RoomTypeCalendar{RoomType: roomType, Days: byType[roomType.ID]})
		}
	}
	return grid, nil
}

// Offers responde à busca de disponibilidade do site público.
//
// É a consulta mais frequente e mais cara do sistema (agrega quartos,
// ocupações e tarifas para cada data), por isso passa pelo cache no Redis. O
// cache é invalidado sempre que algo muda no inventário da propriedade — veja
// announceAvailability.
func (s *CalendarService) Offers(ctx context.Context, propertyID string, stay kernel.StayPeriod, guests int) ([]domain.Offer, error) {
	key := fmt.Sprintf("%s:%s:%d", stay.CheckIn(), stay.CheckOut(), guests)
	return s.cache.Offers(ctx, propertyID, key, func(ctx context.Context) ([]domain.Offer, error) {
		grid, err := s.Grid(ctx, propertyID, stay.CheckIn(), stay.CheckOut())
		if err != nil {
			return nil, err
		}
		offers := make([]domain.Offer, 0, len(grid))
		for _, entry := range grid {
			offers = append(offers, domain.BuildOffer(entry.RoomType, entry.Days, guests))
		}
		return offers, nil
	})
}

// RoomType devolve o tipo de quarto da cópia local do catálogo.
func (s *CalendarService) RoomType(ctx context.Context, id string) (*domain.RoomType, error) {
	return s.catalog.RoomType(ctx, id)
}

// Room devolve o quarto da cópia local do catálogo.
func (s *CalendarService) Room(ctx context.Context, id string) (*domain.Room, error) {
	return s.catalog.Room(ctx, id)
}

// FreeRooms lista os quartos livres de um tipo no período.
func (s *CalendarService) FreeRooms(ctx context.Context, roomTypeID string, stay kernel.StayPeriod) ([]domain.Room, error) {
	return s.allocations.FreeRooms(ctx, roomTypeID, stay, "")
}

// Allocations lista as ocupações (reservas e bloqueios) que tocam [from, to).
func (s *CalendarService) Allocations(ctx context.Context, propertyID string, from, to kernel.Date, kind domain.AllocationKind) ([]domain.Allocation, error) {
	return s.allocations.List(ctx, propertyID, from, to, kind)
}

func (s *CalendarService) days(ctx context.Context, propertyID, roomTypeID string, stay kernel.StayPeriod) ([]domain.DayStatus, error) {
	byType, err := s.calendar.Days(ctx, propertyID, roomTypeID, stay.CheckIn(), stay.CheckOut())
	if err != nil {
		return nil, err
	}
	return byType[roomTypeID], nil
}

// ---- Tarifas ----

// SetRatesInput descreve uma alteração de tarifas/restrições em [From, To).
type SetRatesInput struct {
	RoomTypeID string
	From       kernel.Date
	To         kernel.Date
	// Weekdays restringe a alteração a alguns dias da semana (0 = domingo).
	// Vazio = todos. Ex.: [5, 6] para tarifa de fim de semana.
	Weekdays []int
	Change   RateChange
}

// SetRates altera preço, estadia mínima e/ou fechamento de venda.
func (s *CalendarService) SetRates(ctx context.Context, input SetRatesInput) (*domain.RoomType, error) {
	if !input.To.After(input.From) || input.From.DaysUntil(input.To) > s.horizonDays+31 {
		return nil, kernel.Invalid("inventory.invalid_range", "intervalo de datas inválido")
	}
	if input.Change.PriceCents == nil && input.Change.MinStay == nil && input.Change.Closed == nil {
		return nil, kernel.Invalid("inventory.empty_change", "informe ao menos um campo para alterar")
	}
	if input.Change.PriceCents != nil && *input.Change.PriceCents <= 0 {
		return nil, kernel.Invalid("inventory.invalid_price", "o preço deve ser maior que zero").WithField("price_cents", "deve ser maior que zero")
	}
	if input.Change.MinStay != nil && (*input.Change.MinStay < 1 || *input.Change.MinStay > 30) {
		return nil, kernel.Invalid("inventory.invalid_min_stay", "estadia mínima inválida").WithField("min_stay", "entre 1 e 30")
	}

	var dates []kernel.Date
	for day := input.From; day.Before(input.To); day = day.AddDays(1) {
		if len(input.Weekdays) == 0 || slices.Contains(input.Weekdays, int(day.Weekday())) {
			dates = append(dates, day)
		}
	}
	if len(dates) == 0 {
		return nil, kernel.Invalid("inventory.no_dates", "nenhuma data do intervalo corresponde aos dias da semana escolhidos")
	}

	var roomType *domain.RoomType
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if roomType, err = s.catalog.RoomType(ctx, input.RoomTypeID); err != nil {
			return err
		}
		if err := s.calendar.SetRates(ctx, roomType.ID, dates, input.Change); err != nil {
			return err
		}
		return s.announceRates(ctx, *roomType, input.From, input.To)
	})
	return roomType, err
}

// ---- Bloqueios ----

// CreateBlock tira um quarto de venda em um período (manutenção, uso do
// proprietário). Falha com ErrRoomTaken se já houver reserva no período.
func (s *CalendarService) CreateBlock(ctx context.Context, roomID string, stay kernel.StayPeriod, reason string) (*domain.Allocation, error) {
	var block domain.Allocation
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		room, err := s.catalog.Room(ctx, roomID)
		if err != nil {
			return err
		}
		block = domain.Allocation{
			ID: kernel.NewID(), PropertyID: room.PropertyID, RoomTypeID: room.RoomTypeID,
			RoomID: room.ID, RoomName: room.Name, Kind: domain.AllocationBlock,
			Stay: stay, Reason: reason, CreatedAt: time.Now().UTC(),
		}
		if err := s.allocations.Insert(ctx, block); err != nil {
			return err
		}
		return s.announceAvailability(ctx, room.PropertyID, room.RoomTypeID, stay.CheckIn(), stay.CheckOut(), "block")
	})
	if err != nil {
		return nil, err
	}
	return &block, nil
}

// Block devolve um bloqueio pelo ID.
func (s *CalendarService) Block(ctx context.Context, id string) (*domain.Allocation, error) {
	allocation, err := s.allocations.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if allocation.Kind != domain.AllocationBlock {
		return nil, domain.ErrAllocationNotFound
	}
	return allocation, nil
}

// DeleteBlock remove um bloqueio, devolvendo o quarto à venda.
func (s *CalendarService) DeleteBlock(ctx context.Context, id string) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		block, err := s.Block(ctx, id)
		if err != nil {
			return err
		}
		if err := s.allocations.Delete(ctx, block.ID); err != nil {
			return err
		}
		return s.announceAvailability(ctx, block.PropertyID, block.RoomTypeID, block.Stay.CheckIn(), block.Stay.CheckOut(), "unblock")
	})
}

// ---- Cópia local do catálogo (reação aos eventos de propriedades) ----

// ApplyRoomType grava o tipo de acomodação recebido por evento. Se a diária
// padrão mudou (ou o tipo é novo), as tarifas do horizonte são reanunciadas.
func (s *CalendarService) ApplyRoomType(ctx context.Context, snapshot contracts.RoomTypeSnapshot) error {
	roomType := domain.RoomType{
		ID: snapshot.RoomTypeID, PropertyID: snapshot.PropertyID, Name: snapshot.Name,
		MaxOccupancy: snapshot.MaxOccupancy, BasePriceCents: snapshot.BasePriceCents,
		Currency: snapshot.Currency, Active: snapshot.Active,
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		previous, err := s.catalog.UpsertRoomType(ctx, roomType)
		if err != nil {
			return err
		}
		from, to := s.horizon()
		if previous == nil || previous.BasePriceCents != roomType.BasePriceCents {
			if err := s.announceRates(ctx, roomType, from, to); err != nil {
				return err
			}
		}
		if previous != nil && previous.Active != roomType.Active {
			return s.announceAvailability(ctx, roomType.PropertyID, roomType.ID, from, to, "room_type_changed")
		}
		s.invalidate(ctx, roomType.PropertyID)
		return nil
	})
}

// ApplyRoom grava o quarto recebido por evento. Um quarto novo, desativado ou
// que mudou de tipo altera a quantidade vendável: a disponibilidade de todo o
// horizonte é reanunciada.
func (s *CalendarService) ApplyRoom(ctx context.Context, snapshot contracts.RoomSnapshot) error {
	room := domain.Room{
		ID: snapshot.RoomID, PropertyID: snapshot.PropertyID, RoomTypeID: snapshot.RoomTypeID,
		Name: snapshot.Name, Active: snapshot.Active,
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		previous, err := s.catalog.UpsertRoom(ctx, room)
		if err != nil {
			return err
		}
		from, to := s.horizon()
		countChanged := previous == nil || previous.Active != room.Active || previous.RoomTypeID != room.RoomTypeID
		if !countChanged {
			return nil
		}
		if previous != nil && previous.RoomTypeID != room.RoomTypeID {
			if err := s.announceAvailability(ctx, room.PropertyID, previous.RoomTypeID, from, to, "room_changed"); err != nil {
				return err
			}
		}
		return s.announceAvailability(ctx, room.PropertyID, room.RoomTypeID, from, to, "room_changed")
	})
}

// Resync reanuncia disponibilidade e tarifas de um tipo de quarto em um
// período, a pedido do serviço de canais (ex.: canal recém-conectado).
//
// Se o tipo de quarto ainda é desconhecido aqui, o pedido é ignorado SEM erro.
// Isso é deliberado: o pedido vem pelo tópico de canais e o cadastro do tipo
// vem pelo tópico de propriedades, e o Kafka não garante ordem ENTRE tópicos.
// Insistir (retry) seria inútil — o consumidor ficaria parado justamente na
// frente do evento que resolveria o problema. E nada se perde: quando o
// cadastro chegar, ApplyRoomType e ApplyRoom anunciam tudo de qualquer forma.
// Regra geral para consumidores: reações a tópicos diferentes devem funcionar
// em qualquer ordem de chegada.
func (s *CalendarService) Resync(ctx context.Context, request contracts.ChannelSyncRequested) error {
	from, errFrom := kernel.ParseDate(request.From)
	to, errTo := kernel.ParseDate(request.To)
	if errFrom != nil || errTo != nil || !to.After(from) {
		from, to = s.horizon()
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		roomType, err := s.catalog.RoomType(ctx, request.RoomTypeID)
		if errors.Is(err, domain.ErrRoomTypeNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.announceAvailability(ctx, roomType.PropertyID, roomType.ID, from, to, "resync"); err != nil {
			return err
		}
		return s.announceRates(ctx, *roomType, from, to)
	})
}

// ---- Anúncio de mudanças ----

// announceAvailability publica "a disponibilidade mudou" para um tipo de
// quarto no intervalo [from, to) e invalida o cache de buscas.
//
// É o coração da sincronização com os canais: TODA operação que altera a
// ocupação passa por aqui, dentro da mesma transação que fez a alteração. O
// serviço de canais consome o evento e replica para Booking.com e Airbnb.
func (s *CalendarService) announceAvailability(ctx context.Context, propertyID, roomTypeID string, from, to kernel.Date, cause string) error {
	// Não faz sentido anunciar datas passadas aos canais.
	if today := kernel.Today(time.UTC).AddDays(-1); from.Before(today) {
		from = today
	}
	s.invalidate(ctx, propertyID)
	if !to.After(from) {
		return nil
	}
	byType, err := s.calendar.Days(ctx, propertyID, roomTypeID, from, to)
	if err != nil {
		return err
	}
	days := byType[roomTypeID]
	if len(days) == 0 {
		return nil
	}
	event := contracts.AvailabilityChanged{PropertyID: propertyID, RoomTypeID: roomTypeID, Cause: cause}
	for _, day := range days {
		event.Days = append(event.Days, contracts.DayAvailability{
			Date: day.Date.String(), Total: day.Total, Available: day.Available(),
		})
	}
	return s.publisher.Publish(ctx, event)
}

func (s *CalendarService) announceRates(ctx context.Context, roomType domain.RoomType, from, to kernel.Date) error {
	s.invalidate(ctx, roomType.PropertyID)
	byType, err := s.calendar.Days(ctx, roomType.PropertyID, roomType.ID, from, to)
	if err != nil {
		return err
	}
	days := byType[roomType.ID]
	if len(days) == 0 {
		return nil
	}
	event := contracts.RateChanged{PropertyID: roomType.PropertyID, RoomTypeID: roomType.ID, Currency: roomType.Currency}
	for _, day := range days {
		event.Days = append(event.Days, contracts.DayRate{
			Date: day.Date.String(), PriceCents: day.PriceCents, MinStay: day.MinStay, Closed: day.Closed,
		})
	}
	return s.publisher.Publish(ctx, event)
}

// invalidate agenda a invalidação do cache para DEPOIS do commit: invalidar
// antes permitiria que outra requisição relesse o dado antigo do banco e o
// recolocasse no cache.
func (s *CalendarService) invalidate(ctx context.Context, propertyID string) {
	s.tx.AfterCommit(ctx, func() { s.cache.Invalidate(context.WithoutCancel(ctx), propertyID) })
}

func (s *CalendarService) horizon() (kernel.Date, kernel.Date) {
	today := kernel.Today(time.UTC)
	return today, today.AddDays(s.horizonDays)
}
