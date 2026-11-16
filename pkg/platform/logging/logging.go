// Package logging configura logs estruturados com log/slog (biblioteca padrão).
//
// Logs estruturados são pares chave=valor (JSON), não frases soltas: isso
// permite filtrar no agregador de logs por "reservation_id" ou "trace_id".
// Cada linha ganha automaticamente o trace_id da requisição em andamento, o
// que liga os três pilares da observabilidade: do log você pula para o trace.
//
// ATENÇÃO (LGPD): nunca registre dados pessoais em logs — nome, e-mail, CPF,
// telefone. Registre IDs.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// New cria o logger do serviço e o define como logger padrão do processo.
func New(service, level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(traceHandler{Handler: handler}).With(slog.String("service", service))
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// traceHandler é um Decorator sobre o handler do slog: antes de escrever cada
// registro, acrescenta os identificadores do trace OpenTelemetry presentes no
// contexto.
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: h.Handler.WithGroup(name)}
}
