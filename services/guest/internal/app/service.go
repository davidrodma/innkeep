// Package app é a camada de aplicação do serviço de hóspedes.
package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/guest/internal/domain"
)

// GuestRepository persiste hóspedes.
type GuestRepository interface {
	Create(ctx context.Context, guest *domain.Guest) error
	Update(ctx context.Context, guest *domain.Guest) error
	ByID(ctx context.Context, id string) (*domain.Guest, error)
	// ByDocument busca pelo documento exato (via índice cego).
	ByDocument(ctx context.Context, documentType domain.DocumentType, number string) (*domain.Guest, error)
	Search(ctx context.Context, query string, limit, offset int) ([]*domain.Guest, int, error)
}

// RegistrationRepository persiste as fichas de registro (FNRH).
type RegistrationRepository interface {
	ReplaceForReservation(ctx context.Context, reservationID string, cards []*domain.RegistrationCard) error
	ByReservation(ctx context.Context, reservationID string) ([]*domain.RegistrationCard, error)
	ByGuest(ctx context.Context, guestID string) ([]*domain.RegistrationCard, error)
	StampCheckIn(ctx context.Context, reservationID string, at time.Time) error
	StampCheckOut(ctx context.Context, reservationID string, at time.Time) error
}

// EventPublisher publica eventos de integração (via outbox).
type EventPublisher interface {
	Publish(ctx context.Context, events ...contracts.Event) error
}

// TxRunner executa uma função dentro de uma transação (Unit of Work).
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Service reúne os casos de uso do contexto de hóspedes.
type Service struct {
	tx            TxRunner
	guests        GuestRepository
	registrations RegistrationRepository
	publisher     EventPublisher
	location      *time.Location
}

// NewService monta o serviço.
func NewService(tx TxRunner, guests GuestRepository, registrations RegistrationRepository, publisher EventPublisher, location *time.Location) *Service {
	return &Service{tx: tx, guests: guests, registrations: registrations, publisher: publisher, location: location}
}

func (s *Service) today() kernel.Date { return kernel.Today(s.location) }

// ---- Cadastro de hóspedes ----

// Create cadastra um hóspede.
func (s *Service) Create(ctx context.Context, input domain.GuestInput) (*domain.Guest, error) {
	guest, err := domain.NewGuest(input, s.today())
	if err != nil {
		return nil, err
	}
	return guest, s.guests.Create(ctx, guest)
}

// Update altera o cadastro de um hóspede.
func (s *Service) Update(ctx context.Context, id string, input domain.GuestInput) (*domain.Guest, error) {
	guest, err := s.guests.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := guest.Update(input, s.today()); err != nil {
		return nil, err
	}
	return guest, s.guests.Update(ctx, guest)
}

// Get devolve o cadastro completo de um hóspede.
//
// Toda leitura de dado pessoal fica registrada: quem acessou e qual cadastro
// — sem gravar os dados em si no log. É a trilha de auditoria que a LGPD
// espera de quem trata dados pessoais.
func (s *Service) Get(ctx context.Context, id string) (*domain.Guest, error) {
	guest, err := s.guests.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "cadastro de hóspede consultado",
		slog.String("guest_id", id), slog.String("actor", access.ActorFrom(ctx)))
	return guest, nil
}

// Search busca hóspedes por nome, e-mail ou telefone.
func (s *Service) Search(ctx context.Context, query string, limit, offset int) ([]*domain.Guest, int, error) {
	return s.guests.Search(ctx, query, limit, offset)
}

// FindByDocument busca um hóspede pelo documento exato.
func (s *Service) FindByDocument(ctx context.Context, documentType domain.DocumentType, number string) (*domain.Guest, error) {
	return s.guests.ByDocument(ctx, documentType, domain.NormalizeDocument(documentType, number))
}

// Anonymize elimina os dados pessoais de um hóspede (LGPD, art. 18, VI).
func (s *Service) Anonymize(ctx context.Context, id string) error {
	guest, err := s.guests.ByID(ctx, id)
	if err != nil {
		return err
	}
	guest.Anonymize(time.Now().UTC())
	slog.InfoContext(ctx, "hóspede anonimizado", slog.String("guest_id", id), slog.String("actor", access.ActorFrom(ctx)))
	return s.guests.Update(ctx, guest)
}

// Stays devolve as fichas (estadias) de um hóspede.
func (s *Service) Stays(ctx context.Context, guestID string) ([]*domain.RegistrationCard, error) {
	return s.registrations.ByGuest(ctx, guestID)
}

// ---- Ficha de registro (FNRH) ----

// GuestRegistration é um hóspede informado na ficha de uma estadia.
type GuestRegistration struct {
	Guest   domain.GuestInput
	Primary bool
	Trip    domain.TripDetails
}

// RegistrationInput são os dados para registrar os hóspedes de uma reserva.
type RegistrationInput struct {
	ReservationID string
	PropertyID    string
	Stay          kernel.StayPeriod
	Guests        []GuestRegistration
}

// RegisteredGuest é um hóspede com a sua ficha em uma estadia.
type RegisteredGuest struct {
	Guest *domain.Guest
	Card  *domain.RegistrationCard
}

// Register grava a FNRH de uma reserva e anuncia que o registro está completo
// — é esse evento que libera o check-in no serviço de reservas.
//
// Para cada pessoa: se o documento já é conhecido, o cadastro existente é
// atualizado (hóspede recorrente); senão, é criado. As fichas anteriores da
// reserva são substituídas, então corrigir e reenviar é seguro.
func (s *Service) Register(ctx context.Context, input RegistrationInput) ([]RegisteredGuest, error) {
	if len(input.Guests) == 0 {
		return nil, kernel.Invalid("registration.guests_required", "informe ao menos um hóspede").WithField("guests", "obrigatório")
	}
	primaryIndex := 0
	for i, entry := range input.Guests {
		if entry.Primary {
			primaryIndex = i
			break
		}
	}

	var registered []RegisteredGuest
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		now, today, actor := time.Now().UTC(), s.today(), access.ActorFrom(ctx)
		guests := make([]*domain.Guest, 0, len(input.Guests))
		cards := make([]*domain.RegistrationCard, 0, len(input.Guests))
		registered = registered[:0]

		for i, entry := range input.Guests {
			guest, err := s.upsertGuest(ctx, entry.Guest, today)
			if err != nil {
				return err
			}
			card, err := domain.NewRegistrationCard(input.ReservationID, input.PropertyID, guest.ID,
				i == primaryIndex, input.Stay, entry.Trip, actor, now)
			if err != nil {
				return err
			}
			guests, cards = append(guests, guest), append(cards, card)
			registered = append(registered, RegisteredGuest{Guest: guest, Card: card})
		}
		if err := domain.ValidateParty(guests, input.Stay.CheckIn()); err != nil {
			return err
		}
		if err := s.registrations.ReplaceForReservation(ctx, input.ReservationID, cards); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, contracts.RegistrationCompleted{
			ReservationID: input.ReservationID, PropertyID: input.PropertyID,
			PrimaryGuestID: guests[primaryIndex].ID, GuestCount: len(guests), At: now,
		})
	})
	return registered, err
}

func (s *Service) upsertGuest(ctx context.Context, input domain.GuestInput, today kernel.Date) (*domain.Guest, error) {
	documentType := input.DocumentType
	if documentType == "" {
		documentType = domain.DocumentCPF
	}
	existing, err := s.guests.ByDocument(ctx, documentType, domain.NormalizeDocument(documentType, input.DocumentNumber))
	switch {
	case err == nil:
		if err := existing.Update(input, today); err != nil {
			return nil, err
		}
		return existing, s.guests.Update(ctx, existing)
	case errors.Is(err, domain.ErrGuestNotFound):
		guest, err := domain.NewGuest(input, today)
		if err != nil {
			return nil, err
		}
		return guest, s.guests.Create(ctx, guest)
	default:
		return nil, err
	}
}

// Registration devolve os hóspedes registrados em uma reserva.
func (s *Service) Registration(ctx context.Context, reservationID string) ([]RegisteredGuest, error) {
	cards, err := s.registrations.ByReservation(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	registered := make([]RegisteredGuest, 0, len(cards))
	for _, card := range cards {
		guest, err := s.guests.ByID(ctx, card.GuestID)
		if err != nil {
			return nil, err
		}
		registered = append(registered, RegisteredGuest{Guest: guest, Card: card})
	}
	return registered, nil
}

// StampCheckIn grava nas fichas o instante real da entrada.
func (s *Service) StampCheckIn(ctx context.Context, reservationID string, at time.Time) error {
	return s.registrations.StampCheckIn(ctx, reservationID, at)
}

// StampCheckOut grava nas fichas o instante real da saída.
func (s *Service) StampCheckOut(ctx context.Context, reservationID string, at time.Time) error {
	return s.registrations.StampCheckOut(ctx, reservationID, at)
}
