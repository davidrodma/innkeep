package app

import (
	"context"
	"errors"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/services/billing/internal/domain"
)

// FolioService reúne os casos de uso das contas dos hóspedes.
//
// Concorrência: toda alteração carrega o folio com SELECT ... FOR UPDATE
// (trava PESSIMISTA). Dois recepcionistas lançando pagamento na mesma conta
// ao mesmo tempo são enfileirados pelo banco: o segundo só lê depois que o
// primeiro confirma, e enxerga o saldo atualizado.
//
// Compare com o serviço de reservas, que usa trava OTIMISTA (versão do
// agregado). Lá conflitos são raros e repetir o comando é barato; aqui a
// conta envolve dinheiro e é mais simples impedir o conflito do que tratá-lo.
type FolioService struct {
	tx        TxRunner
	folios    FolioRepository
	finance   FinanceRepository
	publisher EventPublisher
	location  *time.Location
}

// NewFolioService monta o serviço. location é o fuso usado para definir a
// data de competência dos lançamentos.
func NewFolioService(tx TxRunner, folios FolioRepository, finance FinanceRepository, publisher EventPublisher, location *time.Location) *FolioService {
	return &FolioService{tx: tx, folios: folios, finance: finance, publisher: publisher, location: location}
}

func (s *FolioService) today() kernel.Date { return kernel.Today(s.location) }

// ---- Reações aos eventos de reserva ----

// OpenForReservation abre a conta quando a reserva é aceita. Idempotente: se
// a conta já existe, não faz nada.
func (s *FolioService) OpenForReservation(ctx context.Context, event contracts.ReservationAccepted) error {
	stay, err := kernel.ParseStayPeriod(event.CheckIn, event.CheckOut)
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.folios.ByReservation(ctx, event.ReservationID, false); err == nil {
			return nil
		} else if !errors.Is(err, domain.ErrFolioNotFound) {
			return err
		}
		folio := domain.OpenFolio(domain.OpenFolioInput{
			ReservationID: event.ReservationID, ReservationCode: event.Code, PropertyID: event.PropertyID,
			GuestName: event.Holder.Name, Source: event.Source, Currency: event.Currency,
			DepositDueCents: event.DepositDueCents, Stay: stay,
			NightlyRates: toDomainRates(event.NightlyRates), RoomName: event.RoomName,
		}, time.Now().UTC())
		return s.folios.Create(ctx, folio)
	})
}

// ApplyStayChange refaz as diárias quando datas, quarto ou valor mudam.
func (s *FolioService) ApplyStayChange(ctx context.Context, event contracts.ReservationStayChanged) error {
	stay, err := kernel.ParseStayPeriod(event.CheckIn, event.CheckOut)
	if err != nil {
		return err
	}
	return s.update(ctx, event.ReservationID, func(folio *domain.Folio) {
		folio.ReplaceAccommodation(stay, toDomainRates(event.NightlyRates), event.RoomName, time.Now().UTC())
	})
}

// CancelForReservation anula as diárias de uma reserva que não vai acontecer.
func (s *FolioService) CancelForReservation(ctx context.Context, reservationID, reason string) error {
	return s.update(ctx, reservationID, func(folio *domain.Folio) {
		folio.CancelStay(reason, time.Now().UTC())
	})
}

// SettleForReservation fecha a conta no check-out, se estiver quitada.
func (s *FolioService) SettleForReservation(ctx context.Context, reservationID string) error {
	return s.update(ctx, reservationID, func(folio *domain.Folio) {
		folio.Settle(time.Now().UTC())
	})
}

func (s *FolioService) update(ctx context.Context, reservationID string, fn func(folio *domain.Folio)) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		folio, err := s.folios.ByReservation(ctx, reservationID, true)
		if errors.Is(err, domain.ErrFolioNotFound) {
			return nil // reserva que nunca chegou a ter conta (ex.: rejeitada)
		}
		if err != nil {
			return err
		}
		fn(folio)
		return s.folios.Save(ctx, folio)
	})
}

// ---- Consultas ----

// Get devolve um folio com seus lançamentos.
func (s *FolioService) Get(ctx context.Context, id string) (*domain.Folio, error) {
	return s.folios.ByID(ctx, id, false)
}

// ByReservation devolve o folio de uma reserva.
func (s *FolioService) ByReservation(ctx context.Context, reservationID string) (*domain.Folio, error) {
	return s.folios.ByReservation(ctx, reservationID, false)
}

// List lista os folios de uma propriedade.
func (s *FolioService) List(ctx context.Context, filter FolioFilter) ([]FolioSummary, int, error) {
	return s.folios.List(ctx, filter)
}

// ---- Comandos da equipe ----

// PostCharge lança um débito (extra) ou desconto na conta.
func (s *FolioService) PostCharge(ctx context.Context, folioID string, kind domain.EntryKind, description string, amountCents int64) (*domain.Folio, error) {
	return s.change(ctx, folioID, func(ctx context.Context, folio *domain.Folio) error {
		_, err := folio.AddCharge(kind, description, amountCents, s.today(), access.ActorFrom(ctx), time.Now().UTC())
		return err
	})
}

// RecordPayments lança um ou mais pagamentos — o hóspede pode combinar várias
// formas de pagamento e pagar em partes (sinal agora, restante no check-in).
//
// Efeitos, todos na mesma transação:
//   - os lançamentos entram no folio;
//   - pagamentos em dinheiro geram entrada no caixa em espécie;
//   - eventos são publicados: "pagamento registrado" e, quando for o caso,
//     "sinal quitado" (que confirma a reserva) e "conta quitada".
func (s *FolioService) RecordPayments(ctx context.Context, folioID string, payments []domain.PaymentInput) (*domain.Folio, error) {
	return s.change(ctx, folioID, func(ctx context.Context, folio *domain.Folio) error {
		rules, err := s.finance.FeeRules(ctx, folio.PropertyID)
		if err != nil {
			return err
		}
		feeBps := func(method domain.PaymentMethod) int64 {
			for _, rule := range rules {
				if rule.Method == method {
					return rule.Bps
				}
			}
			return 0
		}

		now, today, actor := time.Now().UTC(), s.today(), access.ActorFrom(ctx)
		result, err := folio.RecordPayments(payments, feeBps, today, actor, now)
		if err != nil {
			return err
		}

		totals := folio.Totals()
		var events []contracts.Event
		for _, entry := range result.Entries {
			if entry.Method == domain.MethodCash {
				if err := s.cash(ctx, folio, domain.CashIn, entry.AmountCents, "Pagamento em dinheiro — reserva "+folio.ReservationCode, domain.CashRefPayment, entry.ID); err != nil {
					return err
				}
			}
			events = append(events, contracts.PaymentRecorded{
				FolioID: folio.ID, ReservationID: folio.ReservationID, PropertyID: folio.PropertyID, EntryID: entry.ID,
				Method: string(entry.Method), AmountCents: entry.AmountCents, FeeCents: entry.FeeCents,
				Currency: folio.Currency, PaidCents: totals.PaidCents(), BalanceCents: totals.BalanceCents, At: now,
			})
		}
		if result.DepositJustSettled {
			events = append(events, contracts.DepositSettled{
				FolioID: folio.ID, ReservationID: folio.ReservationID, PropertyID: folio.PropertyID,
				DepositDueCents: folio.DepositDueCents, PaidCents: totals.PaidCents(), At: now,
			})
		}
		if result.JustSettled {
			events = append(events, contracts.FolioSettled{
				FolioID: folio.ID, ReservationID: folio.ReservationID, PropertyID: folio.PropertyID,
				TotalCents: totals.TotalDueCents(), At: now,
			})
		}
		return s.publisher.Publish(ctx, events...)
	})
}

// Refund devolve ao hóspede um valor já pago.
func (s *FolioService) Refund(ctx context.Context, folioID string, method domain.PaymentMethod, amountCents int64, reason string) (*domain.Folio, error) {
	return s.change(ctx, folioID, func(ctx context.Context, folio *domain.Folio) error {
		entry, err := folio.Refund(method, amountCents, reason, s.today(), access.ActorFrom(ctx), time.Now().UTC())
		if err != nil {
			return err
		}
		if method == domain.MethodCash {
			return s.cash(ctx, folio, domain.CashOut, entry.AmountCents, "Devolução em dinheiro — reserva "+folio.ReservationCode, domain.CashRefRefund, entry.ID)
		}
		return nil
	})
}

// VoidEntry anula um lançamento. Anular um pagamento em dinheiro gera o
// movimento inverso no caixa.
func (s *FolioService) VoidEntry(ctx context.Context, folioID, entryID, reason string) (*domain.Folio, error) {
	return s.change(ctx, folioID, func(ctx context.Context, folio *domain.Folio) error {
		var wasVoided bool
		for _, entry := range folio.Entries {
			if entry.ID == entryID {
				wasVoided = entry.Voided()
			}
		}
		entry, err := folio.Void(entryID, reason, access.ActorFrom(ctx), time.Now().UTC())
		if err != nil || wasVoided || entry.Method != domain.MethodCash {
			return err
		}
		switch entry.Kind {
		case domain.EntryPayment:
			return s.cash(ctx, folio, domain.CashOut, entry.AmountCents, "Anulação de pagamento em dinheiro — reserva "+folio.ReservationCode, domain.CashRefAdjustment, entry.ID)
		case domain.EntryRefund:
			return s.cash(ctx, folio, domain.CashIn, entry.AmountCents, "Anulação de devolução em dinheiro — reserva "+folio.ReservationCode, domain.CashRefAdjustment, entry.ID)
		}
		return nil
	})
}

// change é o roteiro das alterações de folio: trava, altera e grava.
func (s *FolioService) change(ctx context.Context, folioID string, fn func(ctx context.Context, folio *domain.Folio) error) (*domain.Folio, error) {
	var folio *domain.Folio
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if folio, err = s.folios.ByID(ctx, folioID, true); err != nil {
			return err
		}
		if err := access.EnsureProperty(ctx, folio.PropertyID); err != nil {
			return err
		}
		if err := fn(ctx, folio); err != nil {
			return err
		}
		return s.folios.Save(ctx, folio)
	})
	if err != nil {
		return nil, err
	}
	return folio, nil
}

func (s *FolioService) cash(ctx context.Context, folio *domain.Folio, kind domain.CashKind, amountCents int64, reason, refType, refID string) error {
	movement, err := domain.NewCashMovement(folio.PropertyID, kind, amountCents, reason, refType, refID, s.today(), access.ActorFrom(ctx))
	if err != nil {
		return err
	}
	return s.finance.AddCashMovement(ctx, movement)
}

func toDomainRates(rates []contracts.NightlyRate) []domain.NightlyRate {
	converted := make([]domain.NightlyRate, 0, len(rates))
	for _, rate := range rates {
		if date, err := kernel.ParseDate(rate.Date); err == nil {
			converted = append(converted, domain.NightlyRate{Date: date, PriceCents: rate.PriceCents})
		}
	}
	return converted
}
