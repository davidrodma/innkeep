package redisx

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript implementa um limitador de taxa por "contador de janela
// deslizante".
//
// Ideia: contamos requisições em janelas fixas (ex.: minuto 12:00, minuto
// 12:01...). Para estimar o uso nos ÚLTIMOS 60 segundos, somamos a janela
// atual com uma fração da janela anterior, proporcional ao quanto dela ainda
// está dentro do intervalo. Isso elimina o defeito da janela fixa simples,
// que deixa passar o dobro do limite na virada entre duas janelas, e custa só
// dois contadores por cliente.
//
// Por que um script Lua? O Redis executa scripts de forma ATÔMICA: nenhum
// outro comando roda no meio. Sem isso, duas requisições simultâneas poderiam
// ler o mesmo contador e ambas passarem (condição de corrida).
//
//	KEYS[1] = contador da janela atual     ARGV[1] = limite
//	KEYS[2] = contador da janela anterior  ARGV[2] = tamanho da janela (ms)
//	                                       ARGV[3] = ms decorridos na janela atual
var slidingWindowScript = redis.NewScript(`
local limit   = tonumber(ARGV[1])
local window  = tonumber(ARGV[2])
local elapsed = tonumber(ARGV[3])

local current  = tonumber(redis.call('GET', KEYS[1]) or '0')
local previous = tonumber(redis.call('GET', KEYS[2]) or '0')
local weight   = (window - elapsed) / window
local estimate = previous * weight + current

if estimate >= limit then
  return {0, math.ceil(estimate)}
end

current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], window * 2)
end
return {1, math.ceil(previous * weight + current)}
`)

// RateLimiter limita requisições por chave (IP, usuário, rota...).
type RateLimiter struct {
	client *redis.Client
	prefix string
}

// NewRateLimiter cria o limitador.
func NewRateLimiter(client *redis.Client, prefix string) *RateLimiter {
	return &RateLimiter{client: client, prefix: prefix}
}

// Decision é o resultado de uma consulta ao limitador.
type Decision struct {
	Allowed bool
	Limit   int
	// Remaining é uma estimativa de quantas requisições ainda cabem na janela.
	Remaining int
	// RetryAfter indica quanto esperar quando a requisição foi negada.
	RetryAfter time.Duration
}

// Allow registra uma requisição da chave e informa se ela cabe no limite.
func (l *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error) {
	now := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	currentWindow := now / windowMs
	elapsed := now % windowMs

	keys := []string{
		fmt.Sprintf("%s:%s:%d", l.prefix, key, currentWindow),
		fmt.Sprintf("%s:%s:%d", l.prefix, key, currentWindow-1),
	}
	result, err := slidingWindowScript.Run(ctx, l.client, keys, limit, windowMs, elapsed).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("redisx: rate limit: %w", err)
	}

	used := int(result[1])
	decision := Decision{Allowed: result[0] == 1, Limit: limit, Remaining: max(limit-used, 0)}
	if !decision.Allowed {
		decision.RetryAfter = time.Duration(windowMs-elapsed) * time.Millisecond
	}
	return decision, nil
}

// Headers devolve os cabeçalhos HTTP que informam o cliente sobre o limite.
func (d Decision) Headers() map[string]string {
	headers := map[string]string{
		"RateLimit-Limit":     strconv.Itoa(d.Limit),
		"RateLimit-Remaining": strconv.Itoa(d.Remaining),
	}
	if !d.Allowed {
		headers["Retry-After"] = strconv.Itoa(int(d.RetryAfter.Seconds()) + 1)
	}
	return headers
}
