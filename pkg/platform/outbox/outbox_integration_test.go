//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/outbox"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
	"github.com/davidrodma/innkeep/pkg/platform/postgres/pgtest"
	"github.com/davidrodma/innkeep/services/reservation/migrations"
)

func pending(t *testing.T, tx *postgres.TxManager) int {
	t.Helper()
	var count int
	if err := tx.Pool().QueryRow(context.Background(), `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestPublishIsAtomicWithTheTransaction prova a garantia central do padrão
// Transactional Outbox: se a transação é desfeita, o evento some junto.
func TestPublishIsAtomicWithTheTransaction(t *testing.T) {
	tx := pgtest.FreshDatabase(t, migrations.FS)
	ctx := context.Background()
	publisher := outbox.NewPublisher(tx, "integration-test")
	event := contracts.ReservationExpired{ReservationSummary: contracts.ReservationSummary{ReservationID: kernel.NewID()}}

	failure := errors.New("falha simulada depois de publicar")
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := publisher.Publish(ctx, event); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("erro inesperado: %v", err)
	}
	if count := pending(t, tx); count != 0 {
		t.Fatalf("evento de transação desfeita vazou para a outbox: %d", count)
	}

	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return publisher.Publish(ctx, event) }); err != nil {
		t.Fatal(err)
	}
	if count := pending(t, tx); count != 1 {
		t.Fatalf("esperado 1 evento pendente, há %d", count)
	}
}

// fakeBroker guarda o que o relay "publicou" e pode falhar sob demanda.
type fakeBroker struct {
	mu        sync.Mutex
	published []contracts.Envelope
	fail      bool
}

func (b *fakeBroker) Publish(_ context.Context, envelopes ...contracts.Envelope) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return errors.New("broker fora do ar")
	}
	b.published = append(b.published, envelopes...)
	return nil
}

func (b *fakeBroker) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.published)
}

// TestRelayDeliversInOrderAndRetries: com o broker fora do ar, nada se perde;
// quando ele volta, os eventos saem na ordem em que foram gravados.
func TestRelayDeliversInOrderAndRetries(t *testing.T) {
	tx := pgtest.FreshDatabase(t, migrations.FS)
	publisher := outbox.NewPublisher(tx, "integration-test")
	broker := &fakeBroker{fail: true}

	ids := make([]string, 5)
	for i := range ids {
		ids[i] = kernel.NewID()
		if err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
			return publisher.Publish(ctx, contracts.ReservationExpired{ReservationSummary: contracts.ReservationSummary{ReservationID: ids[i]}})
		}); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	relay := outbox.NewRelay(tx.Pool(), broker, slog.New(slog.DiscardHandler))
	relay.Interval = 100 * time.Millisecond
	go func() { _ = relay.Run(ctx) }()

	time.Sleep(400 * time.Millisecond)
	if broker.count() != 0 || pending(t, tx) != 5 {
		t.Fatal("com o broker fora do ar, os eventos devem continuar pendentes")
	}

	broker.mu.Lock()
	broker.fail = false
	broker.mu.Unlock()

	// Espera a outbox esvaziar — e não só o broker receber: o relay publica
	// ANTES de marcar os eventos como publicados (e confirmar a transação).
	deadline := time.Now().Add(5 * time.Second)
	for pending(t, tx) > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if broker.count() != 5 || pending(t, tx) != 0 {
		t.Fatalf("esperados 5 publicados e 0 pendentes; obtido %d e %d", broker.count(), pending(t, tx))
	}
	for i, envelope := range broker.published {
		if envelope.PartitionKey != ids[i] {
			t.Fatalf("ordem de publicação alterada na posição %d", i)
		}
	}
}
