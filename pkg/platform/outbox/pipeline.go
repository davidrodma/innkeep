package outbox

import (
	"context"
	"log/slog"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
)

// Pipeline reúne as três peças da publicação de eventos de um serviço:
//
//	caso de uso -> Publisher (grava na outbox) -> Relay (lê a outbox) -> Producer (Kafka)
type Pipeline struct {
	Publisher *Publisher
	Relay     *Relay
	Producer  *messaging.Producer
}

// NewPipeline monta a publicação de eventos de um serviço e, se configurado,
// garante que os tópicos de negócio existam.
func NewPipeline(ctx context.Context, kafka config.Kafka, service string, tx *postgres.TxManager, logger *slog.Logger) (*Pipeline, error) {
	if kafka.EnsureTopics {
		if err := messaging.EnsureTopics(ctx, kafka, contracts.AllTopics()...); err != nil {
			return nil, err
		}
	}
	producer, err := messaging.NewProducer(kafka, service)
	if err != nil {
		return nil, err
	}
	return &Pipeline{
		Publisher: NewPublisher(tx, service),
		Relay:     NewRelay(tx.Pool(), producer, logger),
		Producer:  producer,
	}, nil
}

// Close encerra o produtor Kafka.
func (p *Pipeline) Close() { p.Producer.Close() }
