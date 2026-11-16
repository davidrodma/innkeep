// Package inbox implementa o padrão Inbox (consumidor idempotente).
//
// O Kafka (e a nossa outbox) entregam cada evento "pelo menos uma vez": de vez
// em quando o MESMO evento chega duas vezes. Sem cuidado, isso significaria
// cobrar duas vezes ou alocar dois quartos.
//
// A defesa: cada consumidor anota, em uma tabela `inbox`, os IDs dos eventos
// que já processou — NA MESMA TRANSAÇÃO em que aplica os efeitos do evento.
// Se o ID já estiver lá, o evento é ignorado. O resultado prático é
// "exatamente uma vez" do ponto de vista dos efeitos no banco.
//
// DDL da tabela (cada serviço a cria na sua primeira migração):
//
//	CREATE TABLE inbox (
//	    consumer     TEXT        NOT NULL,
//	    event_id     UUID        NOT NULL,
//	    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    PRIMARY KEY (consumer, event_id)
//	);
package inbox

import (
	"context"
	"fmt"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
)

// Middleware torna idempotente e transacional todo handler do roteador:
//
//	router := messaging.NewRouter(inbox.Middleware(tx, "billing"))
//
// Tudo o que o handler fizer no banco (incluindo gravar novos eventos na
// outbox) participa da mesma transação que registra o evento como processado.
// Se o handler falhar, nada é gravado e o evento será tentado de novo.
func Middleware(tx *postgres.TxManager, consumer string) messaging.Middleware {
	return func(next messaging.Handler) messaging.Handler {
		return func(ctx context.Context, envelope contracts.Envelope) error {
			return tx.WithinTx(ctx, func(ctx context.Context) error {
				tag, err := tx.DB(ctx).Exec(ctx,
					`INSERT INTO inbox (consumer, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					consumer, envelope.ID)
				if err != nil {
					return fmt.Errorf("inbox: registrando evento %s: %w", envelope.ID, err)
				}
				if tag.RowsAffected() == 0 {
					return nil // duplicata: os efeitos deste evento já foram aplicados
				}
				return next(ctx, envelope)
			})
		}
	}
}
