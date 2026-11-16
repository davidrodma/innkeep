package redisx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache implementa o padrão cache-aside ("cache ao lado"):
//
//  1. procura no cache;
//  2. se achou (hit), devolve;
//  3. se não achou (miss), busca na fonte da verdade, grava no cache com um
//     prazo de validade (TTL) e devolve.
//
// A parte difícil de todo cache é a INVALIDAÇÃO: quando o dado muda na fonte,
// como evitar servir a cópia velha? Aqui usamos invalidação por VERSÃO: cada
// "escopo" (ex.: a disponibilidade de uma propriedade) tem um contador no
// Redis, e esse contador faz parte da chave de cache. Mudou algo? Incrementa
// o contador (Invalidate). Todas as chaves antigas ficam órfãs de uma vez —
// ninguém mais as lê e o TTL as remove. Não é preciso saber quais chaves
// existem, nem apagar uma a uma.
type Cache struct {
	client *redis.Client
	prefix string
}

// NewCache cria um cache com um prefixo de chaves (namespace do serviço).
func NewCache(client *redis.Client, prefix string) *Cache {
	return &Cache{client: client, prefix: prefix}
}

func (c *Cache) versionKey(scope string) string { return c.prefix + ":ver:" + scope }

// Invalidate torna obsoletas todas as entradas de um escopo.
func (c *Cache) Invalidate(ctx context.Context, scope string) error {
	if err := c.client.Incr(ctx, c.versionKey(scope)).Err(); err != nil {
		return fmt.Errorf("redisx: invalidando escopo %s: %w", scope, err)
	}
	return nil
}

// GetOrLoad devolve o valor em cache de (scope, key) ou executa load, guarda
// o resultado por ttl e o devolve.
//
// O cache é uma OTIMIZAÇÃO: se o Redis falhar, a função registra o problema e
// segue pela fonte da verdade. Um cache fora do ar nunca derruba a leitura.
func GetOrLoad[T any](ctx context.Context, c *Cache, scope, key string, ttl time.Duration, load func(ctx context.Context) (T, error)) (T, error) {
	var zero T

	version, err := c.client.Get(ctx, c.versionKey(scope)).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		slog.WarnContext(ctx, "cache indisponível; lendo da fonte", slog.Any("erro", err))
		return load(ctx)
	}
	fullKey := c.prefix + ":" + scope + ":v" + strconv.FormatInt(version, 10) + ":" + key

	cached, err := c.client.Get(ctx, fullKey).Bytes()
	if err == nil {
		var value T
		if err := json.Unmarshal(cached, &value); err == nil {
			return value, nil // hit
		}
	}

	value, err := load(ctx) // miss
	if err != nil {
		return zero, err
	}
	if encoded, err := json.Marshal(value); err == nil {
		if err := c.client.Set(ctx, fullKey, encoded, ttl).Err(); err != nil {
			slog.WarnContext(ctx, "falha ao gravar no cache", slog.Any("erro", err))
		}
	}
	return value, nil
}
