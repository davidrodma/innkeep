// Package httpx reúne os utilitários HTTP compartilhados pelos serviços:
// respostas JSON, erros padronizados, middlewares, health checks e servidor
// com desligamento gracioso.
//
// Tudo aqui se apoia em net/http, a biblioteca padrão. Desde o Go 1.22 o
// roteador nativo entende método e parâmetros de rota
// (`mux.Handle("GET /v1/reservations/{id}", ...)`), então não precisamos de
// framework web.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// maxBodyBytes limita o corpo das requisições (proteção contra payloads
// gigantes usados para esgotar memória).
const maxBodyBytes = 1 << 20 // 1 MiB

// HandlerFunc é um handler HTTP que DEVOLVE erro, em vez de escrevê-lo na
// resposta. O tratamento fica centralizado em Error: nenhum handler precisa
// lembrar de qual status usar para cada situação.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP faz HandlerFunc satisfazer http.Handler.
func (fn HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		Error(w, r, err)
	}
}

// JSON escreve payload como JSON com o status informado.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("httpx: falha ao serializar a resposta", slog.Any("erro", err))
	}
}

// NoContent responde 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// List é o envelope padrão das respostas de listagem.
type List[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// NewList monta o envelope garantindo que "items" seja [] (e não null) quando
// a lista estiver vazia — clientes JavaScript agradecem.
func NewList[T any](items []T, total int) List[T] {
	if items == nil {
		items = []T{}
	}
	return List[T]{Items: items, Total: total}
}

// Decode lê o corpo JSON da requisição em dst, rejeitando campos
// desconhecidos (um erro de digitação no nome do campo vira erro claro, em
// vez de ser ignorado em silêncio).
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			return kernel.Invalid("request.body_too_large", "o corpo da requisição excede o limite de 1 MB")
		case errors.Is(err, io.EOF):
			return kernel.Invalid("request.empty_body", "o corpo da requisição está vazio")
		default:
			return kernel.Invalid("request.invalid_json", "JSON inválido: %v", err)
		}
	}
	if decoder.More() {
		return kernel.Invalid("request.invalid_json", "o corpo deve conter um único objeto JSON")
	}
	return nil
}

// Problem é o corpo de erro das APIs, no formato "Problem Details"
// (RFC 9457) — um padrão da IETF para erros em APIs HTTP.
type Problem struct {
	Type    string            `json:"type"`
	Title   string            `json:"title"`
	Status  int               `json:"status"`
	Detail  string            `json:"detail,omitempty"`
	Code    string            `json:"code"`
	Errors  map[string]string `json:"errors,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
}

var kindStatus = map[kernel.ErrorKind]int{
	kernel.KindInvalid:       http.StatusBadRequest,
	kernel.KindNotFound:      http.StatusNotFound,
	kernel.KindConflict:      http.StatusConflict,
	kernel.KindRuleViolation: http.StatusUnprocessableEntity,
	kernel.KindUnauthorized:  http.StatusUnauthorized,
	kernel.KindForbidden:     http.StatusForbidden,
	kernel.KindRateLimited:   http.StatusTooManyRequests,
	kernel.KindUnavailable:   http.StatusServiceUnavailable,
	kernel.KindInternal:      http.StatusInternalServerError,
}

// Error traduz um erro para a resposta HTTP adequada.
//
// É aqui — no adaptador — que o Kind do erro de domínio vira status HTTP. Erros
// inesperados são registrados no log com todos os detalhes, mas o cliente
// recebe uma mensagem genérica: detalhes internos (SQL, caminhos, stack) nunca
// vazam para fora.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	problem := Problem{
		Type:   "about:blank",
		Status: http.StatusInternalServerError,
		Title:  http.StatusText(http.StatusInternalServerError),
		Code:   "internal",
		Detail: "erro interno; tente novamente em instantes",
	}

	var domainErr *kernel.Error
	if errors.As(err, &domainErr) && domainErr.Kind != kernel.KindInternal {
		status, ok := kindStatus[domainErr.Kind]
		if !ok {
			status = http.StatusInternalServerError
		}
		problem.Status = status
		problem.Title = http.StatusText(status)
		problem.Code = domainErr.Code
		problem.Detail = domainErr.Message
		problem.Errors = domainErr.Fields
	}

	if spanCtx := trace.SpanContextFromContext(r.Context()); spanCtx.IsValid() {
		problem.TraceID = spanCtx.TraceID().String()
	}

	if problem.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "erro ao atender requisição",
			slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Any("erro", err))
	}

	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}
