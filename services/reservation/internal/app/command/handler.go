// Package command é o lado de ESCRITA (o "C" de CQRS) do serviço de reservas.
package command

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/eventstore"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/port"
	"github.com/davidrodma/innkeep/services/reservation/internal/domain"
)

// Handler executa os comandos sobre o agregado Reservation.
type Handler struct {
	tx        port.TxRunner
	store     port.ReservationStore
	views     port.ViewRepository
	policies  port.PolicyRepository
	publisher port.EventPublisher
	now       func() time.Time
}

// NewHandler monta o executor de comandos.
func NewHandler(tx port.TxRunner, store port.ReservationStore, views port.ViewRepository,
	policies port.PolicyRepository, publisher port.EventPublisher) *Handler {
	return &Handler{tx: tx, store: store, views: views, policies: policies, publisher: publisher,
		now: func() time.Time { return time.Now().UTC() }}
}

// mutate é o roteiro de TODO comando sobre uma reserva existente:
//
//  1. carrega o agregado (replay dos eventos);
//  2. executa a decisão de negócio (fn), que pode gerar novos eventos;
//  3. grava tudo em UMA transação (commit).
//
// Se outro comando alterou a mesma reserva nesse meio-tempo, o Event Store
// acusa conflito (concorrência otimista) e o roteiro inteiro é repetido com o
// estado atualizado — a decisão é refeita sobre fatos novos, nunca sobre uma
// leitura velha.
func (h *Handler) mutate(ctx context.Context, id string, fn func(reservation *domain.Reservation, policy port.PropertyPolicy) error) (port.ReservationView, error) {
	const maxAttempts = 4
	var view port.ReservationView

	for attempt := 1; ; attempt++ {
		err := h.tx.WithinTx(ctx, func(ctx context.Context) error {
			reservation, err := h.store.Load(ctx, id)
			if err != nil {
				return err
			}
			policy, err := h.policies.ByProperty(ctx, reservation.State().PropertyID)
			if err != nil {
				return err
			}
			if err := fn(reservation, *policy); err != nil {
				return err
			}
			view, err = h.commit(ctx, reservation)
			return err
		})
		// Dentro de uma transação externa (consumidor de eventos) o conflito
		// "envenenou" a transação: quem repete é o próprio consumidor.
		retry := errors.Is(err, eventstore.ErrConcurrency) && attempt < maxAttempts && !h.tx.InTx(ctx)
		if !retry {
			return view, err
		}
	}
}

// commit grava, na MESMA transação:
//
//	(1) os novos eventos de domínio no Event Store — a fonte da verdade;
//	(2) a projeção atualizada em reservation_views — o modelo de leitura;
//	(3) os eventos de integração na outbox — o aviso aos outros serviços.
//
// Como (1), (2) e (3) são atômicos, o modelo de leitura nunca diverge dos
// eventos e nenhum evento de integração se perde.
func (h *Handler) commit(ctx context.Context, reservation *domain.Reservation) (port.ReservationView, error) {
	state := reservation.State()
	view := port.ViewOf(state)
	changes := reservation.Changes()
	if len(changes) == 0 {
		return view, nil // o comando não mudou nada (ex.: repetição idempotente)
	}

	if err := h.store.Save(ctx, reservation, access.ActorFrom(ctx)); err != nil {
		return view, err
	}
	if err := h.views.Upsert(ctx, view); err != nil {
		return view, err
	}
	if err := h.publisher.Publish(ctx, translate(state, changes)...); err != nil {
		return view, err
	}
	return view, nil
}

// today devolve a data corrente no fuso da propriedade.
func (h *Handler) today(policy port.PropertyPolicy) kernel.Date {
	return kernel.DateOf(h.now().In(policy.Location()))
}

// codeAlphabet omite caracteres que se confundem ao ler em voz alta ou
// digitar (0/O, 1/I/L).
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// newCode gera o localizador curto mostrado ao hóspede, ex.: "IK-7F3K9Q".
func newCode() string {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		panic("command: falha ao gerar bytes aleatórios: " + err.Error())
	}
	code := make([]byte, len(raw))
	for i, b := range raw {
		code[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return "IK-" + string(code)
}
