// Comando inventory: disponibilidade, tarifas, bloqueios e alocação de quartos.
//
// Uso:
//
//	inventory          inicia o serviço
//	inventory migrate  aplica as migrações e sai
package main

import (
	"context"
	"net/http"
	"os"

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
	"github.com/davidrodma/innkeep/services/inventory/internal/adapter/in/consumer"
	"github.com/davidrodma/innkeep/services/inventory/internal/adapter/in/rest"
	"github.com/davidrodma/innkeep/services/inventory/internal/adapter/out/cache"
	pgadapter "github.com/davidrodma/innkeep/services/inventory/internal/adapter/out/postgres"
	inventoryapp "github.com/davidrodma/innkeep/services/inventory/internal/app"
	"github.com/davidrodma/innkeep/services/inventory/migrations"
)

const serviceName = "inventory"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
	Redis    config.Redis
	Auth     config.Auth
	// HorizonDays: quantos dias à frente o inventário é anunciado aos canais.
	HorizonDays int `env:"INVENTORY_HORIZON_DAYS" envDefault:"365"`
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
	catalog := pgadapter.NewCatalogRepository(tx)
	allocations := pgadapter.NewAllocationRepository(tx)
	calendar := inventoryapp.NewCalendarService(tx, catalog, allocations,
		pgadapter.NewCalendarRepository(tx),
		cache.NewOfferCache(redisx.NewCache(redisClient, "inventory:offers")),
		events.Publisher, cfg.HorizonDays)
	allocationService := inventoryapp.NewAllocationService(tx, catalog, allocations, calendar, events.Publisher)

	// --- Adaptadores de entrada: eventos ---
	router := messaging.NewRouter(inbox.Middleware(tx, serviceName))
	consumer.Register(router, allocationService, calendar)
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group:  serviceName,
		Topics: []string{contracts.TopicProperty, contracts.TopicReservation, contracts.TopicChannel},
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
	rest.NewHandler(calendar).Register(mux, guard)
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "outbox-relay", Component: events.Relay},
		app.Named{Name: "event-consumer", Component: eventConsumer},
	)
}
