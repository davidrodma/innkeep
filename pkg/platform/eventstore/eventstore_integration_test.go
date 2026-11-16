//go:build integration

package eventstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/eventstore"
	"github.com/davidrodma/innkeep/pkg/platform/postgres/pgtest"
	"github.com/davidrodma/innkeep/services/reservation/migrations"
)

// TestOptimisticConcurrency simula dois comandos que leram a MESMA versão de
// um agregado e tentam gravar novos eventos ao mesmo tempo.
func TestOptimisticConcurrency(t *testing.T) {
	tx := pgtest.FreshDatabase(t, migrations.FS)
	ctx := context.Background()
	store := eventstore.New(tx)
	aggregateID := kernel.NewID()

	event := func(kind string) eventstore.Event {
		return eventstore.Event{
			ID: kernel.NewID(), AggregateType: "reservation", AggregateID: aggregateID,
			Type: kind, Payload: []byte(`{}`), OccurredAt: time.Now(),
		}
	}
	if err := store.Append(ctx, 0, []eventstore.Event{event("reservation.requested")}); err != nil {
		t.Fatal(err)
	}

	// Os dois partem da versão 1: só o primeiro pode vencer.
	if err := store.Append(ctx, 1, []eventstore.Event{event("reservation.confirmed")}); err != nil {
		t.Fatalf("o primeiro deveria gravar: %v", err)
	}
	if err := store.Append(ctx, 1, []eventstore.Event{event("reservation.cancelled")}); !errors.Is(err, eventstore.ErrConcurrency) {
		t.Fatalf("o segundo deveria receber ErrConcurrency: %v", err)
	}

	events, err := store.Load(ctx, aggregateID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Type != "reservation.confirmed" || events[1].Version != 2 {
		t.Fatalf("fluxo inesperado: %+v", events)
	}

	// O log global devolve tudo em ordem, para reconstruir projeções.
	all, err := store.LoadAll(ctx, 0, 100)
	if err != nil || len(all) != 2 || all[0].GlobalSeq >= all[1].GlobalSeq {
		t.Fatalf("log global inesperado: %+v %v", all, err)
	}
}
