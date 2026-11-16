package app

import (
	"context"
	"errors"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/inventory/internal/domain"
)

// AllocationService é o participante do inventário na SAGA de reserva.
//
// Uma saga é uma transação de negócio que atravessa vários serviços. Como não
// existe transação distribuída entre bancos diferentes, cada serviço executa a
// SUA parte em uma transação local e anuncia o resultado por evento; se algo
// der errado, os outros compensam. Aqui a saga é COREOGRAFADA: não há um
// maestro central — cada serviço sabe a que eventos reagir.
//
//	reservation: "reserva solicitada"      (reservation.requested)
//	inventory:   aloca o quarto e responde (inventory.allocation_confirmed | _rejected)
//	reservation: confirma ou rejeita a reserva
type AllocationService struct {
	tx          TxRunner
	catalog     CatalogRepository
	allocations AllocationRepository
	calendar    *CalendarService
	publisher   EventPublisher
}

// NewAllocationService monta o serviço.
func NewAllocationService(tx TxRunner, catalog CatalogRepository, allocations AllocationRepository, calendar *CalendarService, publisher EventPublisher) *AllocationService {
	return &AllocationService{tx: tx, catalog: catalog, allocations: allocations, calendar: calendar, publisher: publisher}
}

// Allocate reage a "reserva solicitada": tenta reservar um quarto e publica a
// resposta. Nunca devolve erro por falta de disponibilidade — isso é um
// resultado de negócio legítimo, comunicado por evento. Erro aqui significa
// falha técnica (banco fora do ar), e o consumidor tentará de novo.
func (s *AllocationService) Allocate(ctx context.Context, request contracts.ReservationRequested) error {
	stay, err := kernel.ParseStayPeriod(request.CheckIn, request.CheckOut)
	if err != nil {
		return s.reject(ctx, request, domain.Rejection{Reason: domain.RejectNoAvailability, Detail: "período inválido"})
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		// Segunda linha de defesa da idempotência (a primeira é a inbox): se a
		// reserva já tem alocação, não há nada a fazer.
		if existing, err := s.allocations.ByReservation(ctx, request.ReservationID); err == nil && existing != nil {
			return nil
		} else if err != nil && !errors.Is(err, domain.ErrAllocationNotFound) {
			return err
		}

		roomType, err := s.catalog.RoomType(ctx, request.RoomTypeID)
		if errors.Is(err, domain.ErrRoomTypeNotFound) || (err == nil && (!roomType.Active || roomType.PropertyID != request.PropertyID)) {
			return s.reject(ctx, request, domain.Rejection{Reason: domain.RejectUnknownRoomType})
		}
		if err != nil {
			return err
		}

		days, err := s.calendar.days(ctx, request.PropertyID, roomType.ID, stay)
		if err != nil {
			return err
		}
		if request.EnforceRestrictions {
			if request.Adults+request.Children > roomType.MaxOccupancy {
				return s.reject(ctx, request, domain.Rejection{Reason: domain.RejectOverCapacity})
			}
			if rejection := domain.CheckRestrictions(days); rejection != nil {
				return s.reject(ctx, request, *rejection)
			}
		}

		allocation := domain.Allocation{
			ID: kernel.NewID(), PropertyID: request.PropertyID, RoomTypeID: roomType.ID,
			ReservationID: request.ReservationID, Kind: domain.AllocationReservation,
			Stay: stay, CreatedAt: time.Now().UTC(),
		}
		room, rejection, err := s.pickRoom(ctx, &allocation, request.PreferredRoomID, request.AllowOverbooking)
		if err != nil {
			return err
		}
		if rejection != nil {
			return s.reject(ctx, request, *rejection)
		}

		quote := domain.QuoteFromRates(days, roomType.Currency)
		if request.QuotedTotalCents != nil {
			quote = domain.QuoteFixed(stay, kernel.NewMoney(*request.QuotedTotalCents, roomType.Currency))
		}

		confirmed := contracts.AllocationConfirmed{
			ReservationID: request.ReservationID, PropertyID: request.PropertyID, RoomTypeID: roomType.ID,
			CheckIn: request.CheckIn, CheckOut: request.CheckOut,
			NightlyRates: toContractRates(quote.Nightly), TotalCents: quote.Total.Cents(), Currency: roomType.Currency,
			Overbooked: room == nil,
		}
		if room != nil {
			confirmed.RoomID, confirmed.RoomName = room.ID, room.Name
		}
		if err := s.publisher.Publish(ctx, confirmed); err != nil {
			return err
		}
		return s.calendar.announceAvailability(ctx, request.PropertyID, roomType.ID, stay.CheckIn(), stay.CheckOut(), "reservation")
	})
}

// pickRoom grava a alocação em um quarto livre e o devolve. Devolve room=nil
// (sem rejeição) quando a reserva foi aceita em overbooking.
//
// O ponto central: NÃO fazemos "verifica se está livre e depois grava" — entre
// as duas etapas outra transação poderia ocupar o quarto (condição de
// corrida). Em vez disso, tentamos gravar e deixamos a restrição EXCLUDE do
// PostgreSQL decidir. Se dois pedidos disputarem o mesmo quarto no mesmo
// instante, o banco garante que exatamente um vence; o outro recebe
// ErrRoomTaken e tenta o próximo quarto. O savepoint mantém a transação viva
// depois do erro esperado.
func (s *AllocationService) pickRoom(ctx context.Context, allocation *domain.Allocation, preferredRoomID string, allowOverbooking bool) (*domain.Room, *domain.Rejection, error) {
	try := func(room domain.Room) error {
		allocation.RoomID = room.ID
		return s.tx.WithinSavepoint(ctx, func(ctx context.Context) error {
			return s.allocations.Insert(ctx, *allocation)
		})
	}

	if preferredRoomID != "" {
		room, err := s.catalog.Room(ctx, preferredRoomID)
		if errors.Is(err, domain.ErrRoomNotFound) || (err == nil && (!room.Active || room.RoomTypeID != allocation.RoomTypeID)) {
			return nil, &domain.Rejection{Reason: domain.RejectRoomTaken, Detail: "quarto inexistente, fora de venda ou de outro tipo"}, nil
		}
		if err != nil {
			return nil, nil, err
		}
		switch err := try(*room); {
		case err == nil:
			return room, nil, nil
		case !errors.Is(err, domain.ErrRoomTaken):
			return nil, nil, err
		case !allowOverbooking:
			return nil, &domain.Rejection{Reason: domain.RejectRoomTaken, Detail: "o quarto " + room.Name + " já está ocupado no período"}, nil
		}
	} else {
		const maxAttempts = 5
		for attempt := 0; attempt < maxAttempts; attempt++ {
			free, err := s.allocations.FreeRooms(ctx, allocation.RoomTypeID, allocation.Stay, "")
			if err != nil {
				return nil, nil, err
			}
			if len(free) == 0 {
				break
			}
			switch err := try(free[0]); {
			case err == nil:
				return &free[0], nil, nil
			case !errors.Is(err, domain.ErrRoomTaken):
				return nil, nil, err
			}
			// Outro pedido ocupou o quarto entre a consulta e a gravação:
			// consulta de novo e tenta o próximo.
		}
		if !allowOverbooking {
			return nil, &domain.Rejection{Reason: domain.RejectNoAvailability}, nil
		}
	}

	// Overbooking: a reserva já está confirmada no canal externo, então é
	// registrada sem quarto (room_id nulo) para a equipe resolver.
	allocation.RoomID = ""
	if err := s.allocations.Insert(ctx, *allocation); err != nil {
		return nil, nil, err
	}
	return nil, nil, nil
}

func (s *AllocationService) reject(ctx context.Context, request contracts.ReservationRequested, rejection domain.Rejection) error {
	return s.publisher.Publish(ctx, contracts.AllocationRejected{
		ReservationID: request.ReservationID, PropertyID: request.PropertyID, RoomTypeID: request.RoomTypeID,
		CheckIn: request.CheckIn, CheckOut: request.CheckOut,
		Reason: rejection.Reason, Detail: rejection.Detail,
	})
}

// Release devolve o quarto ao inventário (cancelamento, expiração, no-show).
// É a AÇÃO DE COMPENSAÇÃO da saga. Liberar o que não existe não é erro: a
// operação é idempotente por natureza.
func (s *AllocationService) Release(ctx context.Context, reservationID string) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		allocation, err := s.allocations.ByReservation(ctx, reservationID)
		if errors.Is(err, domain.ErrAllocationNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.allocations.Delete(ctx, allocation.ID); err != nil {
			return err
		}
		if err := s.publisher.Publish(ctx, contracts.AllocationReleased{
			ReservationID: reservationID, PropertyID: allocation.PropertyID, RoomTypeID: allocation.RoomTypeID,
			RoomID: allocation.RoomID, CheckIn: allocation.Stay.CheckIn().String(), CheckOut: allocation.Stay.CheckOut().String(),
		}); err != nil {
			return err
		}
		return s.calendar.announceAvailability(ctx, allocation.PropertyID, allocation.RoomTypeID,
			allocation.Stay.CheckIn(), allocation.Stay.CheckOut(), "release")
	})
}

// Shorten encurta a alocação quando o hóspede sai antes do previsto,
// devolvendo à venda as noites que sobraram.
func (s *AllocationService) Shorten(ctx context.Context, reservationID, actualCheckOut string) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		allocation, err := s.allocations.ByReservation(ctx, reservationID)
		if errors.Is(err, domain.ErrAllocationNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		actual, err := kernel.ParseDate(actualCheckOut)
		if err != nil {
			// Data ilegível no evento: é erro de validação, que vai para a DLQ
			// sem novas tentativas, em vez de ser ignorado em silêncio.
			return err
		}
		if !actual.Before(allocation.Stay.CheckOut()) {
			return nil // saída na data prevista (ou depois): nada a liberar
		}
		previousCheckOut := allocation.Stay.CheckOut()
		shortened, err := kernel.NewStayPeriod(allocation.Stay.CheckIn(), actual)
		if err != nil {
			// Saiu no mesmo dia em que entrou: não sobra nenhuma noite.
			return s.allocations.Delete(ctx, allocation.ID)
		}
		allocation.Stay = shortened
		if err := s.allocations.Update(ctx, *allocation); err != nil {
			return err
		}
		return s.calendar.announceAvailability(ctx, allocation.PropertyID, allocation.RoomTypeID, actual, previousCheckOut, "early_checkout")
	})
}

// Change reage a "mudança de estadia solicitada": move a alocação para as
// novas datas e/ou o novo quarto, ou responde que não é possível (e nesse
// caso nada muda — a alocação original continua valendo).
func (s *AllocationService) Change(ctx context.Context, request contracts.ReservationStayChangeRequested) error {
	rejectChange := func(ctx context.Context, reason, detail string) error {
		return s.publisher.Publish(ctx, contracts.AllocationChangeRejected{
			ReservationID: request.ReservationID, PropertyID: request.PropertyID, Reason: reason, Detail: detail,
		})
	}
	newStay, err := kernel.ParseStayPeriod(request.NewCheckIn, request.NewCheckOut)
	if err != nil {
		return rejectChange(ctx, domain.RejectNoAvailability, "período inválido")
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.allocations.ByReservation(ctx, request.ReservationID)
		if errors.Is(err, domain.ErrAllocationNotFound) {
			return rejectChange(ctx, domain.RejectNoAvailability, "a reserva não possui alocação")
		}
		if err != nil {
			return err
		}
		previous := *current

		// Candidatos, em ordem de preferência: o quarto pedido; senão o quarto
		// atual; senão qualquer quarto livre do mesmo tipo.
		var candidates []domain.Room
		if request.NewRoomID != "" {
			room, err := s.catalog.Room(ctx, request.NewRoomID)
			if errors.Is(err, domain.ErrRoomNotFound) || (err == nil && (!room.Active || room.PropertyID != current.PropertyID)) {
				return rejectChange(ctx, domain.RejectRoomTaken, "quarto inexistente ou fora de venda")
			}
			if err != nil {
				return err
			}
			candidates = []domain.Room{*room}
		} else {
			if current.RoomID != "" {
				if room, err := s.catalog.Room(ctx, current.RoomID); err == nil {
					candidates = append(candidates, *room)
				}
			}
			free, err := s.allocations.FreeRooms(ctx, current.RoomTypeID, newStay, current.ID)
			if err != nil {
				return err
			}
			candidates = append(candidates, free...)
		}

		var chosen *domain.Room
		for i := range candidates {
			moved := *current
			moved.Stay, moved.RoomID, moved.RoomTypeID = newStay, candidates[i].ID, candidates[i].RoomTypeID
			err := s.tx.WithinSavepoint(ctx, func(ctx context.Context) error { return s.allocations.Update(ctx, moved) })
			if err == nil {
				chosen, *current = &candidates[i], moved
				break
			}
			if !errors.Is(err, domain.ErrRoomTaken) {
				return err
			}
		}
		if chosen == nil {
			return rejectChange(ctx, domain.RejectNoAvailability, "não há quarto livre para o novo período")
		}

		roomType, err := s.catalog.RoomType(ctx, current.RoomTypeID)
		if err != nil {
			return err
		}
		days, err := s.calendar.days(ctx, current.PropertyID, current.RoomTypeID, newStay)
		if err != nil {
			return err
		}
		quote := domain.QuoteFromRates(days, roomType.Currency)
		if request.QuotedTotalCents != nil {
			quote = domain.QuoteFixed(newStay, kernel.NewMoney(*request.QuotedTotalCents, roomType.Currency))
		}

		if err := s.publisher.Publish(ctx, contracts.AllocationChanged{
			ReservationID: request.ReservationID, PropertyID: current.PropertyID, RoomTypeID: current.RoomTypeID,
			RoomID: chosen.ID, RoomName: chosen.Name,
			CheckIn: request.NewCheckIn, CheckOut: request.NewCheckOut,
			NightlyRates: toContractRates(quote.Nightly), TotalCents: quote.Total.Cents(), Currency: roomType.Currency,
			PreviousRoomID:  previous.RoomID,
			PreviousCheckIn: previous.Stay.CheckIn().String(), PreviousCheckOut: previous.Stay.CheckOut().String(),
		}); err != nil {
			return err
		}

		// A disponibilidade mudou no período antigo e no novo (e nos dois
		// tipos, se o quarto novo for de outro tipo).
		if err := s.calendar.announceAvailability(ctx, previous.PropertyID, previous.RoomTypeID,
			previous.Stay.CheckIn(), previous.Stay.CheckOut(), "stay_change"); err != nil {
			return err
		}
		return s.calendar.announceAvailability(ctx, current.PropertyID, current.RoomTypeID,
			newStay.CheckIn(), newStay.CheckOut(), "stay_change")
	})
}

// toContractRates traduz as diárias do domínio para o contrato de eventos.
func toContractRates(rates []domain.NightlyRate) []contracts.NightlyRate {
	converted := make([]contracts.NightlyRate, len(rates))
	for i, rate := range rates {
		converted[i] = contracts.NightlyRate{Date: rate.Date.String(), PriceCents: rate.PriceCents}
	}
	return converted
}
