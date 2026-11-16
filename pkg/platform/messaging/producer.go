package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/config"
)

// Producer publica envelopes de eventos no Kafka.
//
// Atenção: o código de negócio NÃO usa o Producer diretamente. Ele grava os
// eventos na tabela outbox (dentro da transação do banco) e o outbox.Relay é
// quem chama o Producer. Publicar direto no Kafka a partir de um caso de uso
// criaria o problema da "escrita dupla": o banco confirma e o Kafka falha (ou
// o contrário), e os dois ficam inconsistentes para sempre.
type Producer struct {
	client *kgo.Client
}

// NewProducer cria um produtor. Os padrões do franz-go já são os seguros:
// produtor idempotente (o broker descarta duplicatas causadas por reenvio) e
// acks=all (a mensagem só é confirmada após ser replicada).
func NewProducer(cfg config.Kafka, clientID string) (*Producer, error) {
	client, _, err := newClient(cfg, clientID,
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{client: client}, nil
}

// Publish envia os envelopes e espera a confirmação do broker. A ordem dos
// envelopes é preservada dentro de cada partição.
func (p *Producer) Publish(ctx context.Context, envelopes ...contracts.Envelope) error {
	if len(envelopes) == 0 {
		return nil
	}

	propagator := otel.GetTextMapPropagator()
	records := make([]*kgo.Record, 0, len(envelopes))
	for _, envelope := range envelopes {
		value, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("messaging: serializando envelope %s: %w", envelope.ID, err)
		}

		// O envelope guarda o `traceparent` da requisição que originou o
		// evento. Restauramos esse contexto para que o span de publicação — e
		// os spans dos consumidores — façam parte do MESMO trace distribuído,
		// mesmo com a publicação acontecendo depois, pelo relay da outbox.
		recordCtx := ctx
		if envelope.TraceParent != "" {
			recordCtx = propagator.Extract(ctx, propagation.MapCarrier{"traceparent": envelope.TraceParent})
		}

		records = append(records, &kgo.Record{
			Topic: contracts.TopicFor(envelope.Type),
			Key:   []byte(envelope.PartitionKey),
			Value: value,
			Headers: []kgo.RecordHeader{
				// Cabeçalhos permitem inspecionar/rotear sem abrir o corpo.
				{Key: "ce_id", Value: []byte(envelope.ID)},
				{Key: "ce_type", Value: []byte(envelope.Type)},
				{Key: "ce_source", Value: []byte(envelope.Source)},
			},
			Context: recordCtx,
		})
	}

	if err := p.client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		return fmt.Errorf("messaging: publicando eventos: %w", err)
	}
	return nil
}

// Ping verifica se o broker está acessível (usado no readiness probe).
func (p *Producer) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close descarrega mensagens pendentes e fecha as conexões.
func (p *Producer) Close() { p.client.Close() }
