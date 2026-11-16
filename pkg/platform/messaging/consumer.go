package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/config"
)

// ConsumerConfig define como um serviço consome eventos.
type ConsumerConfig struct {
	// Group é o "group id" do Kafka. Convenção: o nome do serviço.
	Group string
	// Topics são os tópicos assinados.
	Topics []string
	// MaxAttempts é o número de tentativas por mensagem antes de enviá-la para
	// a Dead Letter Queue. Padrão: 5.
	MaxAttempts int
	// Backoff é a espera antes da segunda tentativa; dobra a cada nova
	// tentativa (backoff exponencial). Padrão: 200ms.
	Backoff time.Duration
}

// Consumer lê eventos do Kafka e os entrega ao Router.
//
// Garantias e decisões:
//   - Entrega "pelo menos uma vez": o offset só é confirmado DEPOIS do
//     processamento. Uma queda no meio causa reprocessamento, nunca perda.
//   - Processamento sequencial por lote: preserva a ordem dos eventos de cada
//     agregado (mesma chave => mesma partição => mesma ordem).
//   - Retry com backoff exponencial para erros transitórios.
//   - Dead Letter Queue: a mensagem que falha repetidamente ("poison pill") é
//     desviada para um tópico à parte, e o consumo continua. Sem isso, uma
//     única mensagem problemática travaria a partição para sempre.
type Consumer struct {
	client   *kgo.Client
	tracer   *kotel.Tracer
	router   *Router
	cfg      ConsumerConfig
	dlqTopic string
	logger   *slog.Logger
}

// NewConsumer cria o consumidor e garante a existência do tópico de DLQ.
func NewConsumer(ctx context.Context, kafka config.Kafka, cfg ConsumerConfig, router *Router, logger *slog.Logger) (*Consumer, error) {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 200 * time.Millisecond
	}
	dlqTopic := contracts.DeadLetterTopic(cfg.Group)

	if kafka.EnsureTopics {
		topics := append([]string{dlqTopic}, cfg.Topics...)
		if err := EnsureTopics(ctx, kafka, topics...); err != nil {
			return nil, err
		}
	}

	client, tracer, err := newClient(kafka, cfg.Group,
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topics...),
		// Um grupo novo começa do INÍCIO do tópico: um serviço recém-criado
		// consegue montar sua cópia local de dados lendo todo o histórico.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		// O commit é manual, depois do processamento (veja Run).
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:   client,
		tracer:   tracer,
		router:   router,
		cfg:      cfg,
		dlqTopic: dlqTopic,
		logger:   logger.With(slog.String("consumer_group", cfg.Group)),
	}, nil
}

// Run consome até o contexto ser cancelado.
func (c *Consumer) Run(ctx context.Context) error {
	defer c.client.Close()

	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			c.logger.ErrorContext(ctx, "falha ao buscar mensagens",
				slog.String("topic", topic), slog.Int("partition", int(partition)), slog.Any("erro", err))
		})

		var fatal error
		fetches.EachRecord(func(record *kgo.Record) {
			if fatal == nil {
				fatal = c.handleRecord(ctx, record)
			}
		})
		if fatal != nil {
			if ctx.Err() != nil {
				return nil // desligamento em andamento: os offsets não confirmados serão reprocessados
			}
			return fmt.Errorf("messaging: consumidor %s: %w", c.cfg.Group, fatal)
		}

		if err := c.client.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
			c.logger.ErrorContext(ctx, "falha ao confirmar offsets", slog.Any("erro", err))
		}
	}
}

// handleRecord processa uma mensagem. Só devolve erro em situação fatal (não
// foi possível nem mandar para a DLQ): nesse caso o serviço encerra e o
// orquestrador o reinicia, retomando do último offset confirmado.
func (c *Consumer) handleRecord(ctx context.Context, record *kgo.Record) error {
	// Continua o trace iniciado por quem publicou: o kotel lê o contexto de
	// trace dos headers da mensagem e abre o span "process" a partir dele.
	// O contexto que o kotel devolve, porém, NÃO carrega o cancelamento do
	// consumidor. Por isso o span é anexado ao ctx recebido: num desligamento,
	// as novas tentativas e as consultas em andamento são interrompidas (a
	// mensagem será reprocessada, sem efeito duplicado graças à inbox) em vez
	// de seguirem até o fim.
	_, span := c.tracer.WithProcessSpan(record)
	defer span.End()
	ctx = trace.ContextWithSpan(ctx, span)

	var envelope contracts.Envelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		return c.deadLetter(ctx, record, "envelope ilegível", err)
	}
	if envelope.ID == "" || envelope.Type == "" {
		return c.deadLetter(ctx, record, "envelope sem id ou type", nil)
	}
	span.SetAttributes(
		attribute.String("innkeep.event.id", envelope.ID),
		attribute.String("innkeep.event.type", envelope.Type),
	)

	correlationID := envelope.CorrelationID
	if correlationID == "" {
		correlationID = envelope.ID
	}
	ctx = WithMetadata(ctx, Metadata{CorrelationID: correlationID, CausationID: envelope.ID})

	backoff := c.cfg.Backoff
	for attempt := 1; ; attempt++ {
		handled, err := c.dispatch(ctx, envelope)
		if err == nil {
			if handled {
				c.logger.DebugContext(ctx, "evento processado",
					slog.String("event_type", envelope.Type), slog.String("event_id", envelope.ID))
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retryable(err) || attempt >= c.cfg.MaxAttempts {
			span.RecordError(err)
			span.SetStatus(codes.Error, "evento enviado para a DLQ")
			return c.deadLetter(ctx, record, fmt.Sprintf("falhou após %d tentativa(s)", attempt), err)
		}

		c.logger.WarnContext(ctx, "falha ao processar evento; nova tentativa",
			slog.String("event_type", envelope.Type), slog.String("event_id", envelope.ID),
			slog.Int("tentativa", attempt), slog.Any("erro", err))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// dispatch chama o handler convertendo pânico em erro: um bug em um handler
// não pode derrubar o consumidor inteiro.
func (c *Consumer) dispatch(ctx context.Context, envelope contracts.Envelope) (handled bool, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("pânico no handler de %s: %v", envelope.Type, p)
		}
	}()
	return c.router.Dispatch(ctx, envelope)
}

// retryable decide se vale a pena tentar de novo. Erros de validação e de
// regra de negócio são determinísticos: tentar outra vez dá no mesmo. Já
// "não encontrado" pode ser só questão de tempo (o dado chega por outro
// tópico e ainda não foi consumido), então merece novas tentativas.
func retryable(err error) bool {
	var domainErr *kernel.Error
	if errors.As(err, &domainErr) {
		switch domainErr.Kind {
		case kernel.KindInvalid, kernel.KindRuleViolation, kernel.KindForbidden, kernel.KindUnauthorized:
			return false
		}
	}
	return true
}

// deadLetter desvia a mensagem para a DLQ com o contexto da falha nos headers.
func (c *Consumer) deadLetter(ctx context.Context, record *kgo.Record, reason string, cause error) error {
	headers := append([]kgo.RecordHeader{}, record.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "dlq_reason", Value: []byte(reason)},
		kgo.RecordHeader{Key: "dlq_consumer_group", Value: []byte(c.cfg.Group)},
		kgo.RecordHeader{Key: "dlq_original_topic", Value: []byte(record.Topic)},
		kgo.RecordHeader{Key: "dlq_original_partition", Value: []byte(strconv.Itoa(int(record.Partition)))},
		kgo.RecordHeader{Key: "dlq_original_offset", Value: []byte(strconv.FormatInt(record.Offset, 10))},
		kgo.RecordHeader{Key: "dlq_failed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
	)
	if cause != nil {
		headers = append(headers, kgo.RecordHeader{Key: "dlq_error", Value: []byte(cause.Error())})
	}

	dead := &kgo.Record{Topic: c.dlqTopic, Key: record.Key, Value: record.Value, Headers: headers}
	if err := c.client.ProduceSync(ctx, dead).FirstErr(); err != nil {
		return fmt.Errorf("enviando mensagem para a DLQ %s: %w", c.dlqTopic, err)
	}

	c.logger.ErrorContext(ctx, "mensagem enviada para a DLQ",
		slog.String("dlq_topic", c.dlqTopic), slog.String("motivo", reason),
		slog.String("topic", record.Topic), slog.Int64("offset", record.Offset), slog.Any("erro", cause))
	return nil
}
