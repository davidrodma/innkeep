// Package eventstore é um Event Store sobre PostgreSQL, a base do Event
// Sourcing.
//
// Em vez de guardar o ESTADO ATUAL de um agregado (uma linha que é
// sobrescrita a cada mudança), guardamos a SEQUÊNCIA DE FATOS que levou a
// ele: "reserva solicitada", "quarto alocado", "confirmada", "check-in"...
// O estado atual é obtido reaplicando os eventos em ordem ("replay").
//
// O que se ganha:
//   - trilha de auditoria completa e à prova de adulteração (append-only);
//   - "viagem no tempo": dá para saber o estado em qualquer momento passado;
//   - novas visões dos dados podem ser construídas depois, relendo o histórico.
//
// O que custa: mais conceitos, consultas exigem modelos de leitura separados
// (CQRS) e os eventos, uma vez gravados, são contratos eternos. Por isso o
// InnKeep usa Event Sourcing APENAS no serviço de reservas, onde o histórico
// é o próprio negócio — os demais serviços persistem estado comum.
//
// DDL (criada pela migração do serviço que usa o event store):
//
//	CREATE TABLE events (
//	    global_seq     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
//	    event_id       UUID        NOT NULL UNIQUE,
//	    aggregate_type TEXT        NOT NULL,
//	    aggregate_id   UUID        NOT NULL,
//	    version        INT         NOT NULL,
//	    event_type     TEXT        NOT NULL,
//	    payload        JSONB       NOT NULL,
//	    metadata       JSONB       NOT NULL DEFAULT '{}',
//	    occurred_at    TIMESTAMPTZ NOT NULL,
//	    UNIQUE (aggregate_id, version)
//	);
package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
)

// ErrConcurrency indica que outro comando gravou eventos no mesmo agregado
// entre a leitura e a escrita. O chamador deve recarregar o agregado e tentar
// de novo (controle de concorrência otimista).
var ErrConcurrency = kernel.Conflict("eventstore.concurrency_conflict",
	"o registro foi alterado por outra operação; tente novamente")

// Metadata descreve o contexto em que o evento foi produzido.
type Metadata struct {
	Actor         string `json:"actor,omitempty"`          // quem causou (ex.: "user:<id>", "system", "channel:BOOKING_COM")
	CorrelationID string `json:"correlation_id,omitempty"` // fluxo de negócio
	CausationID   string `json:"causation_id,omitempty"`   // evento que causou este
}

// Event é um evento de domínio persistido.
type Event struct {
	ID            string
	AggregateType string
	AggregateID   string
	// Version é a posição do evento no fluxo do agregado (1, 2, 3...).
	Version    int
	Type       string
	Payload    json.RawMessage
	Metadata   Metadata
	OccurredAt time.Time
	// GlobalSeq é a posição do evento no log global (preenchido na leitura).
	GlobalSeq int64
}

// Store lê e grava eventos.
type Store struct {
	tx *postgres.TxManager
}

// New cria o event store.
func New(tx *postgres.TxManager) *Store { return &Store{tx: tx} }

// Append grava novos eventos ao final do fluxo de um agregado.
//
// expectedVersion é a versão do agregado que o chamador leu antes de decidir.
// Os eventos recebem as versões expectedVersion+1, +2... A restrição
// UNIQUE (aggregate_id, version) é o que implementa a concorrência otimista:
// se dois comandos concorrentes partirem da mesma versão, o segundo INSERT
// viola a restrição e recebe ErrConcurrency — sem nenhum lock explícito.
func (s *Store) Append(ctx context.Context, expectedVersion int, events []Event) error {
	db := s.tx.DB(ctx)
	for i, event := range events {
		metadata, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("eventstore: serializando metadados: %w", err)
		}
		_, err = db.Exec(ctx, `
			INSERT INTO events (event_id, aggregate_type, aggregate_id, version, event_type, payload, metadata, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			event.ID, event.AggregateType, event.AggregateID, expectedVersion+i+1,
			event.Type, []byte(event.Payload), metadata, event.OccurredAt)
		if err != nil {
			if postgres.IsUniqueViolation(err) {
				return ErrConcurrency.Wrap(err)
			}
			return fmt.Errorf("eventstore: gravando evento %s: %w", event.Type, err)
		}
	}
	return nil
}

// Load devolve todos os eventos de um agregado, em ordem de versão.
func (s *Store) Load(ctx context.Context, aggregateID string) ([]Event, error) {
	return s.query(ctx, `
		SELECT global_seq, event_id, aggregate_type, aggregate_id, version, event_type, payload, metadata, occurred_at
		FROM events WHERE aggregate_id = $1 ORDER BY version`, aggregateID)
}

// LoadAll devolve eventos de todos os agregados em ordem global, a partir de
// afterSeq (exclusive). Serve para reconstruir projeções do zero (replay).
func (s *Store) LoadAll(ctx context.Context, afterSeq int64, limit int) ([]Event, error) {
	return s.query(ctx, `
		SELECT global_seq, event_id, aggregate_type, aggregate_id, version, event_type, payload, metadata, occurred_at
		FROM events WHERE global_seq > $1 ORDER BY global_seq LIMIT $2`, afterSeq, limit)
}

func (s *Store) query(ctx context.Context, sql string, args ...any) ([]Event, error) {
	rows, err := s.tx.DB(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("eventstore: consultando eventos: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var (
			event    Event
			payload  []byte
			metadata []byte
		)
		if err := rows.Scan(&event.GlobalSeq, &event.ID, &event.AggregateType, &event.AggregateID,
			&event.Version, &event.Type, &payload, &metadata, &event.OccurredAt); err != nil {
			return nil, fmt.Errorf("eventstore: lendo evento: %w", err)
		}
		event.Payload = payload
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
				return nil, fmt.Errorf("eventstore: metadados inválidos: %w", err)
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
