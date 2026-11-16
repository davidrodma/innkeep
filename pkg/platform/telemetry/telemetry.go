// Package telemetry liga o serviço ao OpenTelemetry (OTel), o padrão aberto
// de observabilidade da CNCF.
//
// O serviço não conhece Jaeger, Prometheus, Grafana nem AWS X-Ray: ele fala
// apenas o protocolo OTLP com um OpenTelemetry Collector, e é o Collector que
// decide para onde os dados vão. Trocar de ferramenta de observabilidade não
// exige recompilar nada.
//
// Sinais exportados:
//   - traces: o caminho de cada requisição pelos serviços (HTTP -> Kafka -> SQL);
//   - métricas: contadores e histogramas (requisições, latência, runtime do Go).
//
// Os logs vão para a saída padrão (stdout) e são coletados pela plataforma
// (Docker/Kubernetes), já carimbados com o trace_id — veja o pacote logging.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config descreve o serviço para o backend de observabilidade.
type Config struct {
	ServiceName string
	Version     string
	Environment string
	// Endpoint do Collector (variável padrão OTEL_EXPORTER_OTLP_ENDPOINT,
	// ex.: "http://otel-collector:4318"). Vazio desliga a exportação.
	Endpoint string
	// SampleRatio é a fração de traces amostrados (1.0 = todos). Em produção
	// com muito tráfego, reduza para controlar custo.
	SampleRatio float64
}

// Shutdown descarrega os dados pendentes e encerra os exportadores.
type Shutdown func(ctx context.Context) error

// Setup configura os provedores globais de trace e métricas.
//
// O propagador W3C Trace Context é registrado SEMPRE, mesmo com a exportação
// desligada: é ele que carrega o cabeçalho `traceparent` de um serviço a
// outro (via HTTP e via Kafka), costurando o trace distribuído.
func Setup(ctx context.Context, cfg Config) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	if cfg.SampleRatio <= 0 {
		cfg.SampleRatio = 1.0
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.namespace", "innkeep"),
			attribute.String("service.version", cfg.Version),
			attribute.String("deployment.environment.name", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: criando resource: %w", err)
	}

	// Os exportadores leem OTEL_EXPORTER_OTLP_ENDPOINT do ambiente por conta
	// própria; por isso nenhum endereço é passado aqui.
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: criando exportador de traces: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		// ParentBased: se o serviço anterior decidiu amostrar o trace, este
		// também amostra — o trace nunca fica pela metade.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: criando exportador de métricas: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(15*time.Second))),
	)
	otel.SetMeterProvider(meterProvider)

	// Métricas do runtime do Go: goroutines, heap, pausas do GC.
	if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
		return nil, fmt.Errorf("telemetry: iniciando métricas de runtime: %w", err)
	}

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}
