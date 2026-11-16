package redisx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
)

// HeaderIdempotencyKey é o cabeçalho que o cliente envia para tornar um POST
// seguro para repetição.
const HeaderIdempotencyKey = "Idempotency-Key"

const idempotencyInProgress = "in-progress"

type storedResponse struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
}

// Idempotency devolve um middleware que torna requisições de escrita
// repetíveis com segurança.
//
// Cenário: o hóspede clica em "Reservar", a rede cai antes da resposta chegar
// e o navegador (ou ele mesmo) tenta de novo. Sem proteção, nascem duas
// reservas. Com o cabeçalho Idempotency-Key (um UUID gerado pelo cliente para
// AQUELA intenção), a segunda requisição recebe a MESMA resposta da primeira,
// sem executar nada outra vez.
//
// Como funciona, usando o Redis:
//  1. SET chave "in-progress" NX — só grava se a chave não existir (atômico).
//     Quem conseguir gravar é o "dono" da execução.
//  2. O dono executa o handler, captura a resposta e a guarda na chave.
//  3. Uma repetição encontra a resposta guardada e a devolve.
//  4. Uma repetição que chegue DURANTE a execução recebe 409 (tente depois).
//
// Requisições sem o cabeçalho passam direto. Só respostas de sucesso (2xx)
// são guardadas: depois de um erro, o cliente pode corrigir e tentar de novo
// com a mesma chave.
func Idempotency(client *redis.Client, scope string, ttl time.Duration) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(HeaderIdempotencyKey)
			if key == "" || r.Method == http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			if len(key) > 128 {
				httpx.Error(w, r, kernel.Invalid("idempotency.invalid_key", "Idempotency-Key deve ter no máximo 128 caracteres"))
				return
			}
			ctx := r.Context()
			redisKey := "idem:" + scope + ":" + r.Method + ":" + r.URL.Path + ":" + key

			acquired, err := client.SetNX(ctx, redisKey, idempotencyInProgress, time.Minute).Result()
			if err != nil {
				// Redis fora do ar: seguimos sem a proteção (disponibilidade
				// acima de tudo), mas deixamos registrado.
				slog.WarnContext(ctx, "idempotência indisponível; processando sem proteção", slog.Any("erro", err))
				next.ServeHTTP(w, r)
				return
			}

			if !acquired {
				raw, err := client.Get(ctx, redisKey).Bytes()
				if errors.Is(err, redis.Nil) || string(raw) == idempotencyInProgress {
					httpx.Error(w, r, kernel.Conflict("idempotency.in_progress",
						"uma requisição com esta Idempotency-Key ainda está em processamento"))
					return
				}
				var stored storedResponse
				if err != nil || json.Unmarshal(raw, &stored) != nil {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Content-Type", stored.ContentType)
				w.Header().Set("Idempotent-Replayed", "true")
				w.WriteHeader(stored.Status)
				_, _ = w.Write(stored.Body)
				return
			}

			recorder := &bodyRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)

			if recorder.status >= 200 && recorder.status < 300 {
				encoded, _ := json.Marshal(storedResponse{
					Status:      recorder.status,
					ContentType: recorder.Header().Get("Content-Type"),
					Body:        recorder.body.Bytes(),
				})
				if err := client.Set(ctx, redisKey, encoded, ttl).Err(); err != nil {
					slog.WarnContext(ctx, "falha ao guardar resposta idempotente", slog.Any("erro", err))
				}
				return
			}
			// Erro: libera a chave para o cliente poder tentar novamente.
			_ = client.Del(ctx, redisKey).Err()
		})
	}
}

// bodyRecorder copia a resposta enquanto ela é escrita para o cliente.
type bodyRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *bodyRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *bodyRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
