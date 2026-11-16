// Package query é o lado de LEITURA (o "Q" de CQRS) do serviço de reservas.
//
// Nada aqui passa pelo agregado nem pelo Event Store: as consultas leem
// direto a projeção `reservation_views`, feita sob medida para as telas.
package query

import (
	"context"
	"strings"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/port"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// Service responde às consultas sobre reservas.
type Service struct {
	views port.ViewRepository
	store port.ReservationStore
}

// NewService monta o serviço de consultas.
func NewService(views port.ViewRepository, store port.ReservationStore) *Service {
	return &Service{views: views, store: store}
}

// Get devolve uma reserva pelo ID.
func (s *Service) Get(ctx context.Context, id string) (*port.ReservationView, error) {
	return s.views.ByID(ctx, id)
}

// List devolve as reservas que atendem ao filtro e o total sem paginação.
func (s *Service) List(ctx context.Context, filter port.ListFilter) ([]port.ReservationView, int, error) {
	return s.views.List(ctx, filter)
}

// History devolve a linha do tempo da reserva — o próprio fluxo de eventos
// do Event Store, exposto como trilha de auditoria.
func (s *Service) History(ctx context.Context, id string) ([]port.HistoryEntry, error) {
	return s.store.History(ctx, id)
}

// Lookup é a consulta do hóspede no site público: exige o localizador E o
// e-mail do titular. Um localizador sozinho não basta — códigos curtos podem
// ser adivinhados por tentativa.
func (s *Service) Lookup(ctx context.Context, code, email string) (*port.ReservationView, error) {
	view, err := s.views.ByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || view.Holder.Email != email {
		// Mesma resposta de "não encontrada": não confirmamos a existência.
		return nil, domain.ErrNotFound
	}
	return view, nil
}

// Dashboard resume o movimento do dia em uma propriedade.
type Dashboard struct {
	Date       kernel.Date            `json:"date"`
	Arrivals   []port.ReservationView `json:"arrivals"`   // chegadas previstas
	Departures []port.ReservationView `json:"departures"` // saídas previstas
	InHouse    []port.ReservationView `json:"in_house"`   // hóspedes na casa
	Pending    []port.ReservationView `json:"pending"`    // aguardando sinal
}

// DashboardFor monta o resumo do dia.
func (s *Service) DashboardFor(ctx context.Context, propertyID string, date kernel.Date) (*Dashboard, error) {
	dashboard := &Dashboard{Date: date}
	sections := []struct {
		target *[]port.ReservationView
		filter port.ListFilter
	}{
		{&dashboard.Arrivals, port.ListFilter{CheckInOn: date,
			Statuses: []string{string(domain.StatusConfirmed), string(domain.StatusPending), string(domain.StatusCheckedIn)}}},
		{&dashboard.Departures, port.ListFilter{CheckOutOn: date,
			Statuses: []string{string(domain.StatusCheckedIn), string(domain.StatusCheckedOut)}}},
		{&dashboard.InHouse, port.ListFilter{Statuses: []string{string(domain.StatusCheckedIn)}}},
		{&dashboard.Pending, port.ListFilter{Statuses: []string{string(domain.StatusPending), string(domain.StatusRequested)}}},
	}
	for _, section := range sections {
		section.filter.PropertyID = propertyID
		section.filter.Limit = 200
		items, _, err := s.views.List(ctx, section.filter)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []port.ReservationView{}
		}
		*section.target = items
	}
	return dashboard, nil
}
