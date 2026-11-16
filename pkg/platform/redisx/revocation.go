package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenRevocations é a lista de tokens de acesso revogados.
//
// Um JWT é válido até expirar — não há como "apagá-lo" depois de emitido.
// Para o logout valer de imediato, o identity grava aqui o ID do token (claim
// "jti") com um TTL igual ao tempo de vida restante do token, e o gateway
// consulta a lista a cada requisição. Como os tokens expiram em minutos, a
// lista se mantém pequena sozinha: o Redis apaga as entradas vencidas.
type TokenRevocations struct {
	client *redis.Client
}

// NewTokenRevocations cria a lista.
func NewTokenRevocations(client *redis.Client) *TokenRevocations {
	return &TokenRevocations{client: client}
}

func revokedKey(tokenID string) string { return "auth:revoked:" + tokenID }

// Revoke marca o token como revogado até `until` (o instante em que ele
// expiraria de qualquer forma).
func (t *TokenRevocations) Revoke(ctx context.Context, tokenID string, until time.Time) error {
	ttl := time.Until(until)
	if ttl <= 0 || tokenID == "" {
		return nil
	}
	return t.client.Set(ctx, revokedKey(tokenID), "1", ttl).Err()
}

// IsRevoked informa se o token foi revogado.
func (t *TokenRevocations) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	if tokenID == "" {
		return false, nil
	}
	exists, err := t.client.Exists(ctx, revokedKey(tokenID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
