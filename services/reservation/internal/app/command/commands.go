package command

import (
	"context"
	"errors"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/port"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// RequestReservation são os dados de um pedido de reserva.
type RequestReservation struct {
	ID               string // opcional (reservas de canal usam ID determinístico)
	PropertyID       string
	RoomTypeID       string
	PreferredRoomID  string
	CheckIn          string
	CheckOut         string
	Adults           int
	Children         int
	Holder           domain.Holder
	Source           string
	ExternalRef      string
	GroupCode        string
	Notes            string
	QuotedTotalCents *int64
	ConfirmNow       bool
	PaymentCollect   string
}

// Request inicia a saga de reserva.
//
// O resultado NÃO é uma reserva confirmada: é uma reserva "REQUESTED". A
// confirmação depende do inventário, que responde por evento instantes depois
// (consistência eventual). Por isso a API devolve 202 Accepted, e o cliente
// consulta a reserva até a situação mudar.
func (h *Handler) Request(ctx context.Context, cmd RequestReservation) (port.ReservationView, error) {
	stay, err := kernel.ParseStayPeriod(cmd.CheckIn, cmd.CheckOut)
	if err != nil {
		return port.ReservationView{}, err
	}
	policy, err := h.policies.ByProperty(ctx, cmd.PropertyID)
	if err != nil {
		return port.ReservationView{}, err
	}
	if cmd.Source == domain.SourceWebsite && !policy.Published {
		return port.ReservationView{}, kernel.NotFound("property.not_found", "propriedade não encontrada")
	}

	reservation, err := domain.Request(domain.RequestInput{
		ID: cmd.ID, Code: newCode(),
		PropertyID: cmd.PropertyID, RoomTypeID: cmd.RoomTypeID, PreferredRoomID: cmd.PreferredRoomID,
		Stay: stay, Adults: cmd.Adults, Children: cmd.Children, Holder: cmd.Holder,
		Source: cmd.Source, ExternalRef: cmd.ExternalRef, GroupCode: cmd.GroupCode, Notes: cmd.Notes,
		QuotedTotalCents: cmd.QuotedTotalCents, ConfirmNow: cmd.ConfirmNow, PaymentCollect: cmd.PaymentCollect,
	}, domain.Policy{
		DepositPercent: policy.DepositPercent, HoldMinutes: policy.HoldMinutes, Currency: policy.Currency,
	}, h.now(), h.today(*policy))
	if err != nil {
		return port.ReservationView{}, err
	}

	var view port.ReservationView
	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		// Idempotência para IDs determinísticos (reservas de canal): se a
		// reserva já existe, o pedido repetido é ignorado.
		if cmd.ID != "" {
			if existing, err := h.views.ByID(ctx, cmd.ID); err == nil {
				view = *existing
				return nil
			} else if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
		}
		view, err = h.commit(ctx, reservation)
		return err
	})
	return view, err
}

// Confirm confirma manualmente uma reserva pendente (sem exigir o sinal).
func (h *Handler) Confirm(ctx context.Context, id string) (port.ReservationView, error) {
	return h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		return r.Confirm(domain.ConfirmedManually, h.now())
	})
}

// Cancel cancela uma reserva.
func (h *Handler) Cancel(ctx context.Context, id, reason, by string) (port.ReservationView, error) {
	return h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		return r.Cancel(reason, by, h.now())
	})
}

// CheckIn registra a entrada do hóspede.
func (h *Handler) CheckIn(ctx context.Context, id string) (port.ReservationView, error) {
	return h.mutate(ctx, id, func(r *domain.Reservation, policy port.PropertyPolicy) error {
		return r.CheckIn(h.now(), h.today(policy))
	})
}

// CheckOut registra a saída do hóspede.
func (h *Handler) CheckOut(ctx context.Context, id string) (port.ReservationView, error) {
	return h.mutate(ctx, id, func(r *domain.Reservation, policy port.PropertyPolicy) error {
		return r.CheckOut(h.now(), h.today(policy))
	})
}

// MarkNoShow registra o não comparecimento.
func (h *Handler) MarkNoShow(ctx context.Context, id string) (port.ReservationView, error) {
	return h.mutate(ctx, id, func(r *domain.Reservation, policy port.PropertyPolicy) error {
		return r.MarkNoShow(h.now(), h.today(policy))
	})
}

// ChangeStay pede a mudança de datas e/ou quarto (mini-saga com o inventário).
func (h *Handler) ChangeStay(ctx context.Context, id, checkIn, checkOut, roomID string, quotedTotalCents *int64) (port.ReservationView, error) {
	stay, err := kernel.ParseStayPeriod(checkIn, checkOut)
	if err != nil {
		return port.ReservationView{}, err
	}
	return h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		return r.RequestStayChange(stay, roomID, quotedTotalCents, h.now())
	})
}

// ---------------------------------------------------------------------------
// Reações a eventos de outros serviços (continuação das sagas). São comandos
// como os demais — só que quem os dispara é um evento, não uma pessoa.
// ---------------------------------------------------------------------------

// RecordAllocation: o inventário reservou o quarto.
func (h *Handler) RecordAllocation(ctx context.Context, id string, allocation domain.Allocation) error {
	_, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		r.RecordAllocation(allocation, h.now())
		return nil
	})
	return err
}

// RecordRejection: o inventário não conseguiu atender.
func (h *Handler) RecordRejection(ctx context.Context, id, reason, detail string) error {
	_, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		r.RecordRejection(reason, detail, h.now())
		return nil
	})
	return err
}

// RecordDeposit: o financeiro avisou que o sinal foi pago.
func (h *Handler) RecordDeposit(ctx context.Context, id string) error {
	_, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		// Sinal pago em reserva já encerrada (cancelada/expirada) não a
		// ressuscita: a equipe trata o estorno pelo financeiro.
		if r.State().Status != domain.StatusPending {
			return nil
		}
		return r.Confirm(domain.ConfirmedByDeposit, h.now())
	})
	return err
}

// RecordRegistration: o serviço de hóspedes avisou que a FNRH foi preenchida.
func (h *Handler) RecordRegistration(ctx context.Context, id, primaryGuestID string, guestCount int) error {
	_, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		r.RecordRegistration(primaryGuestID, guestCount, h.now())
		return nil
	})
	return err
}

// RecordStayChange: o inventário efetivou a mudança de estadia.
func (h *Handler) RecordStayChange(ctx context.Context, id string, allocation domain.Allocation, checkIn, checkOut string) error {
	stay, err := kernel.ParseStayPeriod(checkIn, checkOut)
	if err != nil {
		return err
	}
	_, err = h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		r.RecordStayChange(allocation, stay, h.now())
		return nil
	})
	return err
}

// RecordStayChangeRejection: o inventário recusou a mudança de estadia.
func (h *Handler) RecordStayChangeRejection(ctx context.Context, id, reason, detail string) error {
	_, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
		r.RecordStayChangeRejection(reason, detail, h.now())
		return nil
	})
	return err
}

// ApplyPolicy grava a cópia local das políticas de uma propriedade.
func (h *Handler) ApplyPolicy(ctx context.Context, policy port.PropertyPolicy) error {
	return h.policies.Upsert(ctx, policy)
}

// ---------------------------------------------------------------------------
// Tarefas periódicas
// ---------------------------------------------------------------------------

// ExpireDue encerra as reservas cujo prazo de pagamento do sinal venceu e as
// que ficaram sem resposta do inventário.
//
// É o "timeout" da saga: nenhuma reserva pode ficar para sempre em um estado
// intermediário segurando um quarto. Cada expiração publica um evento, o
// inventário libera o quarto, e a disponibilidade volta aos canais de venda.
func (h *Handler) ExpireDue(ctx context.Context) error {
	now := h.now()

	expired, err := h.views.ExpiredHolds(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, id := range expired {
		if _, err := h.mutate(ctx, id, func(r *domain.Reservation, _ port.PropertyPolicy) error {
			r.Expire(h.now())
			return nil
		}); err != nil {
			return err
		}
	}

	stale, err := h.views.StaleRequests(ctx, now.Add(-staleRequestAfter), 100)
	if err != nil {
		return err
	}
	for _, id := range stale {
		if err := h.RecordRejection(ctx, id, "TIMEOUT", "o inventário não respondeu a tempo"); err != nil {
			return err
		}
	}
	return nil
}

// RebuildViews apaga o modelo de leitura e o reconstrói a partir do Event
// Store. Demonstra a propriedade central do Event Sourcing: os eventos são a
// verdade; toda tabela de consulta é descartável e recalculável.
func (h *Handler) RebuildViews(ctx context.Context) (int, error) {
	ids, err := h.store.AllIDs(ctx)
	if err != nil {
		return 0, err
	}
	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := h.views.Truncate(ctx); err != nil {
			return err
		}
		for _, id := range ids {
			reservation, err := h.store.Load(ctx, id)
			if err != nil {
				return err
			}
			if err := h.views.Upsert(ctx, port.ViewOf(reservation.State())); err != nil {
				return err
			}
		}
		return nil
	})
	return len(ids), err
}
