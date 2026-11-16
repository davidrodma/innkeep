// Comando billing: contas dos hóspedes (folios), pagamentos, despesas, caixa
// e relatórios financeiros.
//
// Uso:
//
//	billing          inicia o serviço
//	billing migrate  aplica as migrações e sai
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/app"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/inbox"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/outbox"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
	"github.com/davidrodma/innkeep/pkg/platform/redisx"
	"github.com/davidrodma/innkeep/services/billing/internal/adapter/in/consumer"
	"github.com/davidrodma/innkeep/services/billing/internal/adapter/in/rest"
	pgadapter "github.com/davidrodma/innkeep/services/billing/internal/adapter/out/postgres"
	billingapp "github.com/davidrodma/innkeep/services/billing/internal/app"
	"github.com/davidrodma/innkeep/services/billing/migrations"
)

const serviceName = "billing"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
	Redis    config.Redis
	Auth     config.Auth
	// Timezone define a data de competência dos lançamentos ("hoje").
	Timezone string `env:"APP_TIMEZONE" envDefault:"America/Sao_Paulo"`
}

func main() { app.Main(run) }

func run(ctx context.Context) error {
	cfg, err := config.Load[Config]()
	if err != nil {
		return err
	}
	cfg.ServiceName = serviceName
	logger, cleanup, err := app.Bootstrap(ctx, cfg.Base)
	if err != nil {
		return err
	}
	defer cleanup()

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("APP_TIMEZONE inválido: %w", err)
	}

	// --- Infraestrutura ---
	migrateOnly := len(os.Args) > 1 && os.Args[1] == "migrate"
	if cfg.Postgres.AutoMigrate || migrateOnly {
		if err := postgres.Migrate(ctx, cfg.Postgres.URL, migrations.FS); err != nil {
			return err
		}
	}
	if migrateOnly {
		return nil
	}
	pool, err := postgres.NewPool(ctx, cfg.Postgres.URL, cfg.Postgres.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx := postgres.NewTxManager(pool)

	redisClient, err := redisx.NewClient(ctx, cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	events, err := outbox.NewPipeline(ctx, cfg.Kafka, serviceName, tx, logger)
	if err != nil {
		return err
	}
	defer events.Close()

	// --- Aplicação ---
	financeRepository := pgadapter.NewFinanceRepository(tx)
	folios := billingapp.NewFolioService(tx, pgadapter.NewFolioRepository(tx), financeRepository, events.Publisher, location)
	finance := billingapp.NewFinanceService(tx, financeRepository)

	// --- Adaptadores de entrada: eventos ---
	router := messaging.NewRouter(inbox.Middleware(tx, serviceName))
	consumer.Register(router, folios)
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group:  serviceName,
		Topics: []string{contracts.TopicReservation},
	}, router, logger)
	if err != nil {
		return err
	}

	// --- Adaptadores de entrada: HTTP ---
	mux := http.NewServeMux()
	httpx.NewHealth().
		Add("postgres", pool.Ping).
		Add("kafka", events.Producer.Ping).
		Add("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }).
		Register(mux)
	guard := auth.NewGuard(auth.NewVerifier(cfg.Auth.JWKSURL, cfg.Auth.Issuer))
	rest.NewHandler(folios, finance).Register(mux, guard, redisx.Idempotency(redisClient, serviceName, 24*time.Hour))
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "outbox-relay", Component: events.Relay},
		app.Named{Name: "event-consumer", Component: eventConsumer},
	)
}
