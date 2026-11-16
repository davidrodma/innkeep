// Comando reservation: ciclo de vida das reservas (Event Sourcing + CQRS).
//
// Uso:
//
//	reservation                inicia o serviço
//	reservation migrate        aplica as migrações e sai
//	reservation rebuild-views  reconstrói o modelo de leitura a partir dos eventos
package main

import (
	"context"
	"log/slog"
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
	"github.com/davidrodma/innkeep/services/reservation/internal/adapter/in/consumer"
	"github.com/davidrodma/innkeep/services/reservation/internal/adapter/in/rest"
	pgadapter "github.com/davidrodma/innkeep/services/reservation/internal/adapter/out/postgres"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/command"
	"github.com/davidrodma/innkeep/services/reservation/internal/app/query"
	"github.com/davidrodma/innkeep/services/reservation/migrations"
)

const serviceName = "reservation"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
	Redis    config.Redis
	Auth     config.Auth
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

	subcommand := ""
	if len(os.Args) > 1 {
		subcommand = os.Args[1]
	}

	// --- Infraestrutura ---
	if cfg.Postgres.AutoMigrate || subcommand == "migrate" {
		if err := postgres.Migrate(ctx, cfg.Postgres.URL, migrations.FS); err != nil {
			return err
		}
	}
	if subcommand == "migrate" {
		return nil
	}
	pool, err := postgres.NewPool(ctx, cfg.Postgres.URL, cfg.Postgres.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx := postgres.NewTxManager(pool)

	events, err := outbox.NewPipeline(ctx, cfg.Kafka, serviceName, tx, logger)
	if err != nil {
		return err
	}
	defer events.Close()

	// --- Aplicação: lado de escrita e lado de leitura ---
	store := pgadapter.NewReservationStore(tx)
	views := pgadapter.NewViewRepository(tx)
	commands := command.NewHandler(tx, store, views, pgadapter.NewPolicyRepository(tx), events.Publisher)
	queries := query.NewService(views, store)

	if subcommand == "rebuild-views" {
		rebuilt, err := commands.RebuildViews(ctx)
		if err != nil {
			return err
		}
		logger.Info("modelo de leitura reconstruído a partir do Event Store", slog.Int("reservas", rebuilt))
		return nil
	}

	redisClient, err := redisx.NewClient(ctx, cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	// --- Adaptadores de entrada: eventos ---
	router := messaging.NewRouter(inbox.Middleware(tx, serviceName))
	consumer.Register(router, commands)
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group: serviceName,
		Topics: []string{
			contracts.TopicProperty, contracts.TopicInventory, contracts.TopicBilling,
			contracts.TopicGuest, contracts.TopicChannel,
		},
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
	rest.NewHandler(commands, queries).Register(mux, guard, redisx.Idempotency(redisClient, serviceName, 24*time.Hour))
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "outbox-relay", Component: events.Relay},
		app.Named{Name: "event-consumer", Component: eventConsumer},
		// O "timeout" das sagas: expira reservas sem sinal e pedidos sem resposta.
		app.Named{Name: "expiracao-de-reservas", Component: app.Every(30*time.Second, logger, "expiracao-de-reservas", commands.ExpireDue)},
	)
}
