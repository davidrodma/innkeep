// Package app cuida do ciclo de vida de um serviço: iniciar os componentes de
// longa duração (servidor HTTP, consumidores Kafka, relay da outbox, tarefas
// periódicas) e desligá-los de forma coordenada.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	// Embute a base de fusos horários (IANA) no binário. A imagem Docker final
	// é mínima e pode não trazer /usr/share/zoneinfo; sem isto,
	// time.LoadLocation("America/Sao_Paulo") falharia em produção.
	_ "time/tzdata"

	"golang.org/x/sync/errgroup"

	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/logging"
	"github.com/davidrodma/innkeep/pkg/platform/telemetry"
)

// Component é algo que roda até o contexto ser cancelado.
type Component interface {
	Run(ctx context.Context) error
}

// ComponentFunc adapta uma função simples para a interface Component.
type ComponentFunc func(ctx context.Context) error

// Run chama a função.
func (fn ComponentFunc) Run(ctx context.Context) error { return fn(ctx) }

// Named dá um nome ao componente para os logs.
type Named struct {
	Name string
	Component
}

// Main é o ponto de entrada comum dos serviços: cria um contexto que é
// cancelado ao receber SIGINT/SIGTERM (Ctrl+C, `docker stop`, encerramento de
// pod no Kubernetes) e converte erro em código de saída.
func Main(run func(ctx context.Context) error) {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("serviço encerrado com erro", slog.Any("erro", err))
		os.Exit(1)
	}
}

// healthcheck consulta o /readyz do próprio serviço e devolve o código de
// saída (0 = saudável). Existe porque a imagem Docker final é "distroless":
// não tem shell, curl nem wget para o HEALTHCHECK do Docker Compose usar —
// então o próprio binário faz esse papel (`/app healthcheck`).
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// O endereço é local e vem da configuração do próprio serviço (HTTP_ADDR),
	// não de uma requisição: ninguém de fora consegue desviar esta chamada (SSRF).
	url := "http://127.0.0.1" + addr[strings.LastIndex(addr, ":"):] + "/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // G704: ver o comentário acima
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: ver o comentário acima
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// Bootstrap configura o que todo serviço precisa antes de qualquer outra
// coisa: logs e telemetria. Devolve o logger e a função de encerramento da
// telemetria (chame com defer).
func Bootstrap(ctx context.Context, base config.Base) (*slog.Logger, func(), error) {
	logger := logging.New(base.ServiceName, base.LogLevel, base.LogFormat)

	shutdownTelemetry, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName: base.ServiceName,
		Version:     base.Version,
		Environment: base.Environment,
		Endpoint:    base.OTLPEndpoint,
	})
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		// Contexto novo: o da aplicação já foi cancelado quando chegamos aqui,
		// e ainda precisamos de alguns segundos para descarregar a telemetria.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			logger.Warn("falha ao encerrar a telemetria", slog.Any("erro", err))
		}
	}
	logger.Info("serviço iniciando",
		slog.String("versao", base.Version), slog.String("ambiente", base.Environment))
	return logger, cleanup, nil
}

// Run executa todos os componentes em paralelo e espera todos terminarem.
//
// Política "um por todos": se QUALQUER componente falhar, o contexto dos
// demais é cancelado e o serviço inteiro encerra. É melhor um processo morto
// (que o Kubernetes reinicia e alerta) do que um serviço "meio vivo" — por
// exemplo, respondendo HTTP com o consumidor de eventos parado.
func Run(ctx context.Context, logger *slog.Logger, components ...Named) error {
	group, ctx := errgroup.WithContext(ctx)
	for _, component := range components {
		group.Go(func() error {
			logger.Info("componente iniciado", slog.String("componente", component.Name))
			err := component.Run(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("%s: %w", component.Name, err)
			}
			logger.Info("componente encerrado", slog.String("componente", component.Name))
			return nil
		})
	}
	return group.Wait()
}

// Every executa fn a cada intervalo, até o contexto ser cancelado. Erros são
// registrados e não interrompem o ciclo: a próxima execução tenta de novo.
func Every(interval time.Duration, logger *slog.Logger, name string, fn func(ctx context.Context) error) Component {
	return ComponentFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := fn(ctx); err != nil && ctx.Err() == nil {
					logger.ErrorContext(ctx, "falha em tarefa periódica",
						slog.String("tarefa", name), slog.Any("erro", err))
				}
			}
		}
	})
}
