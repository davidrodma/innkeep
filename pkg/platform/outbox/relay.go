package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidrodma/innkeep/pkg/contracts"
)

// EnvelopePublisher é a porta de saída do relay (implementada por
// messaging.Producer). Depender da interface permite testar o relay sem Kafka.
type EnvelopePublisher interface {
	Publish(ctx context.Context, envelopes ...contracts.Envelope) error
}

// relayLockKey identifica o advisory lock que elege o relay ativo. O valor é
// arbitrário; como cada serviço tem o seu banco, não há colisão entre serviços.
const relayLockKey int64 = 7266353

// Relay lê a tabela outbox e publica os eventos pendentes no Kafka.
//
// Como ele garante a ORDEM dos eventos de um mesmo agregado mesmo com várias
// réplicas do serviço rodando?
//   - Um advisory lock transacional do PostgreSQL funciona como eleição de
//     líder: só uma réplica por vez consegue publicar.
//   - O lote é lido em ordem de inserção (ORDER BY id) e publicado de forma
//     síncrona.
//
// Latência: em vez de consultar a tabela em intervalos curtos, o relay fica
// "ouvindo" o canal LISTEN/NOTIFY e acorda no instante em que há novidade. A
// consulta periódica (Interval) existe apenas como rede de segurança.
type Relay struct {
	pool      *pgxpool.Pool
	publisher EnvelopePublisher
	logger    *slog.Logger

	// Interval é a frequência da consulta de segurança. Padrão: 2s.
	Interval time.Duration
	// BatchSize é o máximo de eventos publicados por rodada. Padrão: 200.
	BatchSize int
	// Retention é por quanto tempo eventos já publicados são mantidos (para
	// auditoria e depuração) antes de serem apagados. Padrão: 7 dias.
	Retention time.Duration
}

// NewRelay cria o relay com os padrões recomendados.
func NewRelay(pool *pgxpool.Pool, publisher EnvelopePublisher, logger *slog.Logger) *Relay {
	return &Relay{
		pool:      pool,
		publisher: publisher,
		logger:    logger,
		Interval:  2 * time.Second,
		BatchSize: 200,
		Retention: 7 * 24 * time.Hour,
	}
}

// Run executa o relay até o contexto ser cancelado.
func (r *Relay) Run(ctx context.Context) error {
	wake := make(chan struct{}, 1)
	go r.listen(ctx, wake)

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()

	for {
		r.drain(ctx)

		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-ticker.C:
		case <-cleanup.C:
			r.purge(ctx)
		}
	}
}

// drain publica lotes até esvaziar a fila (ou até ocorrer um erro).
func (r *Relay) drain(ctx context.Context) {
	for ctx.Err() == nil {
		published, err := r.flush(ctx)
		if err != nil {
			if ctx.Err() == nil {
				r.logger.ErrorContext(ctx, "outbox: falha ao publicar lote", slog.Any("erro", err))
			}
			return
		}
		if published < r.BatchSize {
			return
		}
	}
}

// flush publica um lote dentro de uma transação e devolve quantos eventos
// foram publicados.
func (r *Relay) flush(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("iniciando transação: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// Eleição de líder: se outra réplica estiver publicando, esta desiste da
	// rodada. O lock é liberado sozinho ao final da transação.
	var leader bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, relayLockKey).Scan(&leader); err != nil {
		return 0, fmt.Errorf("obtendo advisory lock: %w", err)
	}
	if !leader {
		return 0, nil
	}

	rows, err := tx.Query(ctx,
		`SELECT id, payload FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT $1`, r.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("lendo eventos pendentes: %w", err)
	}
	var (
		ids       []int64
		envelopes []contracts.Envelope
	)
	for rows.Next() {
		var (
			id      int64
			payload []byte
		)
		if err := rows.Scan(&id, &payload); err != nil {
			rows.Close()
			return 0, fmt.Errorf("lendo linha da outbox: %w", err)
		}
		var envelope contracts.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			rows.Close()
			return 0, fmt.Errorf("envelope inválido na outbox (id=%d): %w", id, err)
		}
		ids = append(ids, id)
		envelopes = append(envelopes, envelope)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("lendo eventos pendentes: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// Se a publicação falhar, a transação é desfeita e os eventos continuam
	// pendentes para a próxima rodada. Se a publicação der certo e o COMMIT
	// abaixo falhar, eles serão publicados de novo: entrega "pelo menos uma
	// vez" — os consumidores descartam a duplicata pelo ID do evento.
	if err := r.publisher.Publish(ctx, envelopes...); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return 0, fmt.Errorf("marcando eventos como publicados: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(ids), nil
}

// listen mantém uma conexão dedicada em LISTEN e sinaliza `wake` a cada
// notificação. Se a conexão cair, reconecta; enquanto isso, a consulta
// periódica de Run mantém o relay funcionando.
func (r *Relay) listen(ctx context.Context, wake chan<- struct{}) {
	for ctx.Err() == nil {
		if err := r.listenOnce(ctx, wake); err != nil && ctx.Err() == nil {
			r.logger.WarnContext(ctx, "outbox: conexão LISTEN perdida; reconectando", slog.Any("erro", err))
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func (r *Relay) listenOnce(ctx context.Context, wake chan<- struct{}) error {
	pooled, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// Hijack retira a conexão do pool de vez: ela fica dedicada ao LISTEN e é
	// fechada ao sair, em vez de voltar ao pool "ouvindo" o canal.
	conn := pooled.Hijack()
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return err
		}
		select {
		case wake <- struct{}{}:
		default: // já existe um sinal pendente; não precisa acumular
		}
	}
}

// purge apaga eventos publicados há mais tempo que Retention, para a tabela
// não crescer indefinidamente.
func (r *Relay) purge(ctx context.Context) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM outbox WHERE published_at IS NOT NULL AND published_at < now() - make_interval(secs => $1)`,
		r.Retention.Seconds())
	if err != nil {
		r.logger.WarnContext(ctx, "outbox: falha na limpeza", slog.Any("erro", err))
		return
	}
	if tag.RowsAffected() > 0 {
		r.logger.InfoContext(ctx, "outbox: eventos antigos removidos", slog.Int64("quantidade", tag.RowsAffected()))
	}
}
