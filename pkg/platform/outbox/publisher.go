// Package outbox implementa o padrão Transactional Outbox.
//
// O PROBLEMA (escrita dupla): um caso de uso precisa (1) gravar no banco e
// (2) publicar um evento no Kafka. São dois sistemas diferentes; não existe
// transação que cubra os dois. Se o processo cair entre (1) e (2), o banco
// diz "reserva confirmada" e ninguém mais fica sabendo.
//
// A SOLUÇÃO: gravar o evento em uma tabela `outbox` do PRÓPRIO banco, na
// mesma transação da mudança de estado. Ou as duas coisas acontecem, ou
// nenhuma. Um processo separado (o Relay) lê a tabela e publica no Kafka,
// repetindo até dar certo.
//
// Consequência: a entrega é "pelo menos uma vez" (o relay pode publicar e
// cair antes de marcar como publicado). Por isso todo consumidor é
// idempotente — veja o pacote inbox.
//
// DDL da tabela (cada serviço a cria na sua primeira migração):
//
//	CREATE TABLE outbox (
//	    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
//	    event_id      UUID        NOT NULL UNIQUE,
//	    event_type    TEXT        NOT NULL,
//	    partition_key TEXT        NOT NULL,
//	    payload       JSONB       NOT NULL,
//	    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    published_at  TIMESTAMPTZ
//	);
//	CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
)

// notifyChannel é o canal LISTEN/NOTIFY do PostgreSQL usado para acordar o
// relay assim que a transação que gravou eventos é confirmada.
const notifyChannel = "innkeep_outbox"

// Publisher grava eventos de integração na tabela outbox.
//
// Do ponto de vista da camada de aplicação, ele "publica eventos" — ela não
// sabe (nem precisa saber) que por trás existe uma tabela e um relay.
type Publisher struct {
	tx     *postgres.TxManager
	source string
}

// NewPublisher cria o publicador. source é o nome do serviço emissor.
func NewPublisher(tx *postgres.TxManager, source string) *Publisher {
	return &Publisher{tx: tx, source: source}
}

// Publish grava os eventos usando a transação presente no contexto. Chame
// SEMPRE dentro de TxManager.WithinTx, junto da mudança de estado.
func (p *Publisher) Publish(ctx context.Context, events ...contracts.Event) error {
	if len(events) == 0 {
		return nil
	}

	metadata := messaging.MetadataFrom(ctx)
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	db := p.tx.DB(ctx)
	for _, event := range events {
		envelope, err := contracts.NewEnvelope(p.source, event)
		if err != nil {
			return err
		}
		envelope.CorrelationID = metadata.CorrelationID
		envelope.CausationID = metadata.CausationID
		if envelope.CorrelationID == "" {
			envelope.CorrelationID = envelope.ID // este evento inicia um novo fluxo
		}
		// Guarda o contexto de tracing: o relay publica depois, fora da
		// requisição original, e precisa continuar o mesmo trace.
		envelope.TraceParent = carrier.Get("traceparent")

		payload, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("outbox: serializando envelope: %w", err)
		}
		_, err = db.Exec(ctx,
			`INSERT INTO outbox (event_id, event_type, partition_key, payload) VALUES ($1, $2, $3, $4)`,
			envelope.ID, envelope.Type, envelope.PartitionKey, payload)
		if err != nil {
			return fmt.Errorf("outbox: gravando evento %s: %w", envelope.Type, err)
		}
	}

	// O NOTIFY só é entregue quando a transação é confirmada — exatamente o
	// instante em que os eventos passam a existir para o relay.
	if _, err := db.Exec(ctx, `SELECT pg_notify($1, '')`, notifyChannel); err != nil {
		return fmt.Errorf("outbox: notificando o relay: %w", err)
	}
	return nil
}
