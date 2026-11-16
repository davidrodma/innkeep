// Package messaging é a ponte entre os serviços e o Apache Kafka.
//
// Vocabulário mínimo de Kafka para ler este pacote:
//   - Tópico: um log de mensagens append-only, com nome (ex.: reservas).
//   - Partição: cada tópico é dividido em N partições; a ORDEM só é garantida
//     dentro de uma partição. A partição é escolhida pelo hash da chave da
//     mensagem — por isso usamos o ID do agregado como chave.
//   - Offset: a posição de uma mensagem dentro da partição.
//   - Grupo consumidor: instâncias com o mesmo "group id" dividem as partições
//     entre si (escala horizontal); grupos diferentes recebem, cada um, uma
//     cópia de tudo (é assim que vários serviços reagem ao mesmo evento).
//   - Commit de offset: o consumidor avisa "já processei até aqui". Se ele
//     cair antes do commit, reprocessa — entrega "pelo menos uma vez". Daí a
//     exigência de consumidores idempotentes (veja o pacote inbox).
//
// Usamos o cliente franz-go: Go puro (sem cgo), moderno e completo.
package messaging

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel"

	"github.com/davidrodma/innkeep/pkg/platform/config"
)

// newClient cria um cliente Kafka com as opções comuns (brokers, segurança e
// telemetria) mais as opções específicas de quem chama.
//
// Importante: telemetry.Setup deve rodar ANTES, pois os provedores globais do
// OpenTelemetry são lidos aqui.
func newClient(cfg config.Kafka, clientID string, extra ...kgo.Opt) (*kgo.Client, *kotel.Tracer, error) {
	tracer := kotel.NewTracer(
		kotel.TracerProvider(otel.GetTracerProvider()),
		kotel.TracerPropagator(otel.GetTextMapPropagator()),
	)
	meter := kotel.NewMeter(kotel.MeterProvider(otel.GetMeterProvider()))
	instrumentation := kotel.NewKotel(kotel.WithTracer(tracer), kotel.WithMeter(meter))

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(clientID),
		kgo.WithHooks(instrumentation.Hooks()...),
	}

	if cfg.TLS {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	switch cfg.SASLMechanism {
	case "":
		// Sem autenticação: apenas para o broker local de desenvolvimento.
	case "PLAIN":
		opts = append(opts, kgo.SASL(plain.Auth{User: cfg.Username, Pass: cfg.Password}.AsMechanism()))
	case "SCRAM-SHA-256":
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.Username, Pass: cfg.Password}.AsSha256Mechanism()))
	case "SCRAM-SHA-512":
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.Username, Pass: cfg.Password}.AsSha512Mechanism()))
	default:
		return nil, nil, fmt.Errorf("messaging: mecanismo SASL não suportado: %q", cfg.SASLMechanism)
	}

	client, err := kgo.NewClient(append(opts, extra...)...)
	if err != nil {
		return nil, nil, fmt.Errorf("messaging: criando cliente Kafka: %w", err)
	}
	return client, tracer, nil
}

// EnsureTopics cria os tópicos que ainda não existem (operação idempotente).
//
// Em desenvolvimento isso evita um passo manual. Em produção, o ideal é que
// os tópicos sejam criados por infraestrutura como código, com fator de
// replicação 3 — desligue com KAFKA_ENSURE_TOPICS=false.
func EnsureTopics(ctx context.Context, cfg config.Kafka, topics ...string) error {
	client, _, err := newClient(cfg, "innkeep-admin")
	if err != nil {
		return err
	}
	defer client.Close()

	admin := kadm.NewClient(client)
	responses, err := admin.CreateTopics(ctx, cfg.TopicPartitions, cfg.TopicReplication, nil, topics...)
	if err != nil {
		return fmt.Errorf("messaging: criando tópicos: %w", err)
	}
	for _, response := range responses.Sorted() {
		if response.Err != nil && !errors.Is(response.Err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("messaging: criando tópico %s: %w", response.Topic, response.Err)
		}
	}
	return nil
}
