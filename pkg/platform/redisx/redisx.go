// Package redisx reúne os usos de Redis no InnKeep.
//
// O Redis é um banco em MEMÓRIA, de chave-valor, extremamente rápido
// (sub-milissegundo) e de thread única (cada comando é atômico). Não é o
// "banco de verdade" de ninguém aqui — a fonte da verdade é sempre o
// PostgreSQL. Ele entra onde a memória resolve melhor:
//
//   - cache.go        — cache de leituras caras (padrão cache-aside);
//   - ratelimit.go    — limite de requisições por cliente (no gateway);
//   - idempotency.go  — chaves de idempotência de requisições HTTP;
//   - (identity)      — bloqueio de tentativas de login e revogação de tokens.
//
// Regra de projeto: se o Redis cair ou for apagado, o sistema fica mais lento
// ou menos protegido, mas NÃO perde dados nem para de funcionar.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// NewClient conecta ao Redis a partir de uma URL (redis://usuario:senha@host:6379/0
// ou rediss:// para TLS) e instrumenta os comandos com OpenTelemetry.
func NewClient(ctx context.Context, url string) (*redis.Client, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redisx: URL inválida: %w", err)
	}
	options.DialTimeout = 3 * time.Second
	options.ReadTimeout = 2 * time.Second
	options.WriteTimeout = 2 * time.Second

	client := redis.NewClient(options)
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, fmt.Errorf("redisx: instrumentando traces: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = client.Ping(pingCtx).Err()
		cancel()
		if err == nil {
			return client, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			_ = client.Close()
			return nil, fmt.Errorf("redisx: Redis indisponível: %w", err)
		}
		time.Sleep(time.Second)
	}
}
