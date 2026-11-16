// Comando guest: cadastro de hóspedes e Ficha Nacional de Registro (FNRH).
//
// Uso:
//
//	guest          inicia o serviço
//	guest migrate  aplica as migrações e sai
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
	"github.com/davidrodma/innkeep/pkg/platform/fieldcrypt"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/inbox"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/outbox"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
	"github.com/davidrodma/innkeep/services/guest/internal/adapter/in/rest"
	pgadapter "github.com/davidrodma/innkeep/services/guest/internal/adapter/out/postgres"
	guestapp "github.com/davidrodma/innkeep/services/guest/internal/app"
	"github.com/davidrodma/innkeep/services/guest/migrations"
)

const serviceName = "guest"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
	Auth     config.Auth
	// EncryptionKey: chave mestra (32 bytes em base64) dos dados pessoais.
	EncryptionKey string `env:"PII_ENCRYPTION_KEY"`
	Timezone      string `env:"APP_TIMEZONE" envDefault:"America/Sao_Paulo"`
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

	cipher, usingDevKey, err := fieldcrypt.ForEnvironment(cfg.EncryptionKey, cfg.IsProduction())
	if err != nil {
		return fmt.Errorf("PII_ENCRYPTION_KEY: %w", err)
	}
	if usingDevKey {
		logger.Warn("usando chave de criptografia de DESENVOLVIMENTO; defina PII_ENCRYPTION_KEY")
	}
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

	events, err := outbox.NewPipeline(ctx, cfg.Kafka, serviceName, tx, logger)
	if err != nil {
		return err
	}
	defer events.Close()

	// --- Aplicação ---
	service := guestapp.NewService(tx,
		pgadapter.NewGuestRepository(tx, cipher),
		pgadapter.NewRegistrationRepository(tx),
		events.Publisher, location)

	// --- Adaptadores de entrada: eventos ---
	// Carimba nas fichas os horários reais de entrada e saída.
	router := messaging.NewRouter(inbox.Middleware(tx, serviceName))
	router.On(contracts.TypeReservationCheckedIn, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCheckedIn) error {
			return service.StampCheckIn(ctx, e.ReservationID, e.At)
		}))
	router.On(contracts.TypeReservationCheckedOut, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.ReservationCheckedOut) error {
			return service.StampCheckOut(ctx, e.ReservationID, e.At)
		}))
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
