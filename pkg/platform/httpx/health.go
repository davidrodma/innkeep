package httpx

import (
	"context"
	"net/http"
	"time"
)

// Check verifica uma dependência (banco, Kafka, Redis...).
type Check func(ctx context.Context) error

// Health expõe os dois endpoints de saúde que o Kubernetes consulta:
//
//   - /healthz (liveness): "o processo está vivo?". Se falhar, o Kubernetes
//     REINICIA o contêiner. Por isso não verifica dependências: se o banco
//     cair, reiniciar o serviço não resolve nada.
//   - /readyz (readiness): "posso receber tráfego?". Se falhar, o Kubernetes
//     apenas tira o pod do balanceamento até ele se recuperar. Aqui sim
//     verificamos as dependências.
type Health struct {
	checks map[string]Check
}

// NewHealth cria o registro de verificações.
func NewHealth() *Health { return &Health{checks: make(map[string]Check)} }

// Add registra uma verificação de dependência para o readiness.
func (h *Health) Add(name string, check Check) *Health {
	h.checks[name] = check
	return h
}

// Register publica /healthz e /readyz no roteador.
func (h *Health) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		status := http.StatusOK
		result := make(map[string]string, len(h.checks))
		for name, check := range h.checks {
			if err := check(ctx); err != nil {
				status = http.StatusServiceUnavailable
				result[name] = "indisponível"
				continue
			}
			result[name] = "ok"
		}
		JSON(w, status, map[string]any{"status": http.StatusText(status), "checks": result})
	})
}
