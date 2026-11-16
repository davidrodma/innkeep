package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
)

// Middleware envolve um http.Handler para acrescentar comportamento
// transversal. É o padrão Decorator (ou "Chain of Responsibility"): cada
// camada faz o seu trabalho e passa a requisição adiante.
type Middleware func(next http.Handler) http.Handler

// Chain aplica os middlewares na ordem informada: o primeiro é o mais externo
// (o primeiro a ver a requisição e o último a ver a resposta).
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Standard monta a pilha padrão de middlewares de um serviço.
func Standard(service string, logger *slog.Logger, handler http.Handler) http.Handler {
	return Chain(handler,
		Recover(logger),
		SecurityHeaders,
		RequestID,
		Tracing(service),
		AccessLog(logger),
	)
}

// HeaderRequestID é o cabeçalho que identifica a requisição de ponta a ponta.
const HeaderRequestID = "X-Request-Id"

type requestIDKey struct{}

// RequestID garante que toda requisição tenha um identificador: reaproveita o
// que vier do cliente/gateway ou gera um novo. O ID também vira o
// CorrelationID dos eventos disparados por esta requisição.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !kernel.IsID(id) {
			id = kernel.NewID()
		}
		w.Header().Set(HeaderRequestID, id)

		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		ctx = messaging.WithMetadata(ctx, messaging.Metadata{CorrelationID: id})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom devolve o ID da requisição em andamento.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Tracing cria um span OpenTelemetry por requisição e propaga o contexto de
// trace recebido no cabeçalho `traceparent`.
func Tracing(service string) Middleware {
	return func(next http.Handler) http.Handler {
		// O nome definitivo do span só é conhecido depois que o roteador
		// casa a rota; routeTagger o ajusta para "GET /v1/reservations/{id}".
		// Usar o padrão da rota (e não a URL real) mantém a cardinalidade
		// baixa: um nome por rota, não um por reserva.
		tagged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			if r.Pattern != "" {
				span := trace.SpanFromContext(r.Context())
				span.SetName(r.Pattern)
				span.SetAttributes(attribute.String("http.route", r.Pattern))
			}
		})
		return otelhttp.NewHandler(tagged, service,
			otelhttp.WithFilter(func(r *http.Request) bool { return !isProbe(r) }),
		)
	}
}

// AccessLog registra uma linha por requisição atendida.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isProbe(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)

			route := r.Pattern
			if route == "" {
				route = r.Method + " (sem rota)"
			}
			level := slog.LevelInfo
			if recorder.status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			// Registramos a ROTA, não a URL: query strings podem conter dados
			// pessoais e não devem ir para os logs.
			logger.Log(r.Context(), level, "requisição atendida",
				slog.String("route", route),
				slog.Int("status", recorder.status),
				slog.Duration("duracao", time.Since(start)),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)
		})
	}
}

// Recover transforma um pânico em resposta 500, mantendo o serviço de pé.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(p) // sinal legítimo do net/http para abortar a resposta
					}
					logger.ErrorContext(r.Context(), "pânico ao atender requisição",
						slog.Any("panico", p), slog.String("stack", string(debug.Stack())))
					Error(w, r, kernel.Internal(fmt.Errorf("%v", p), "pânico"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders acrescenta cabeçalhos de segurança às respostas da API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff") // o navegador não "adivinha" o tipo do conteúdo
		header.Set("X-Frame-Options", "DENY")           // impede embutir em iframe (clickjacking)
		header.Set("Referrer-Policy", "no-referrer")    // não vaza URLs para terceiros
		header.Set("Cache-Control", "no-store")         // respostas de API não são cacheadas por padrão
		header.Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
}

// CORS libera chamadas de navegador vindas das origens permitidas. O front-end
// do InnKeep chama a API pelo servidor do Next.js (sem CORS); isto existe para
// integrações de terceiros que rodem no navegador.
func CORS(allowedOrigins []string) Middleware {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				header := w.Header()
				header.Set("Access-Control-Allow-Origin", origin)
				header.Add("Vary", "Origin")
				header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-Id")
				header.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isProbe(r *http.Request) bool {
	return r.URL.Path == "/healthz" || r.URL.Path == "/readyz"
}

// statusRecorder captura o status da resposta para o log de acesso.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

// Flush repassa o flush (necessário para streaming e para o proxy do gateway).
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap permite que http.ResponseController alcance o writer original.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
