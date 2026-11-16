// Comando property: catálogo de propriedades, tipos de acomodação e quartos.
//
// Uso:
//
//	property          inicia o serviço
//	property migrate  aplica as migrações e sai
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
	"github.com/davidrodma/innkeep/services/property/internal/adapter/in/consumer"
	"github.com/davidrodma/innkeep/services/property/internal/adapter/in/rest"
	pgadapter "github.com/davidrodma/innkeep/services/property/internal/adapter/out/postgres"
	propertyapp "github.com/davidrodma/innkeep/services/property/internal/app"
	"github.com/davidrodma/innkeep/services/property/migrations"
)

const serviceName = "property"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
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

	events, err := outbox.NewPipeline(ctx, cfg.Kafka, serviceName, tx, logger)
	if err != nil {
		return err
	}
	defer events.Close()

	// --- Aplicação ---
	service := propertyapp.NewService(tx,
		pgadapter.NewPropertyRepository(tx),
		pgadapter.NewRoomTypeRepository(tx),
		pgadapter.NewRoomRepository(tx),
		events.Publisher,
	)

	// --- Adaptadores de entrada: eventos ---
	router := messaging.NewRouter(inbox.Middleware(tx, serviceName))
	consumer.Register(router, service)
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group:  serviceName,
		Topics: []string{contracts.TopicReservation},
	}, router, logger)
	if err != nil {
		return err
	}

	// --- Adaptadores de entrada: HTTP ---
	mux := http.NewServeMux()
	httpx.NewHealth().Add("postgres", pool.Ping).Add("kafka", events.Producer.Ping).Register(mux)
	guard := auth.NewGuard(auth.NewVerifier(cfg.Auth.JWKSURL, cfg.Auth.Issuer))
	rest.NewHandler(service).Register(mux, guard)
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "outbox-relay", Component: events.Relay},
		app.Named{Name: "event-consumer", Component: eventConsumer},
	)
}
