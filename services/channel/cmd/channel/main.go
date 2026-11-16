// Comando channel: o channel manager — mantém Booking.com e Airbnb em dia
// com a disponibilidade e traz para dentro as reservas feitas neles.
//
// Uso:
//
//	channel          inicia o serviço
//	channel migrate  aplica as migrações e sai
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/app"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/fieldcrypt"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/outbox"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
	"github.com/davidrodma/innkeep/services/channel/internal/adapter/in/rest"
	"github.com/davidrodma/innkeep/services/channel/internal/adapter/out/bookingcom"
	"github.com/davidrodma/innkeep/services/channel/internal/adapter/out/ical"
	pgadapter "github.com/davidrodma/innkeep/services/channel/internal/adapter/out/postgres"
	"github.com/davidrodma/innkeep/services/channel/internal/adapter/out/safehttp"
	channelapp "github.com/davidrodma/innkeep/services/channel/internal/app"
	"github.com/davidrodma/innkeep/services/channel/internal/domain"
	"github.com/davidrodma/innkeep/services/channel/migrations"
)

const serviceName = "channel"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Kafka    config.Kafka
	Auth     config.Auth
	// EncryptionKey: chave mestra (32 bytes em base64) das credenciais dos canais.
	EncryptionKey string `env:"CHANNEL_ENCRYPTION_KEY"`
	// PublicAPIURL: endereço externo da API (compõe os links dos calendários).
	PublicAPIURL string `env:"PUBLIC_API_URL" envDefault:"http://localhost:8080"`
	// AllowPrivateURLs libera chamadas a endereços de rede interna. Só para
	// desenvolvimento (simulador de OTA na rede do Docker). Veja safehttp.
	AllowPrivateURLs bool          `env:"CHANNEL_ALLOW_PRIVATE_URLS" envDefault:"false"`
	APIPullInterval  time.Duration `env:"CHANNEL_API_PULL_INTERVAL" envDefault:"60s"`
	ICalPullInterval time.Duration `env:"CHANNEL_ICAL_PULL_INTERVAL" envDefault:"15m"`
}

// registry implementa app.ConnectorRegistry: escolhe o adaptador pelo par
// canal + modo (padrão Strategy).
type registry map[string]channelapp.Connector

func (r registry) For(channel domain.Channel, mode domain.Mode) (channelapp.Connector, bool) {
	connector, ok := r[string(channel)+"/"+string(mode)]
	return connector, ok
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
		return fmt.Errorf("CHANNEL_ENCRYPTION_KEY: %w", err)
	}
	if usingDevKey {
		logger.Warn("usando chave de criptografia de DESENVOLVIMENTO; defina CHANNEL_ENCRYPTION_KEY")
	}
	if cfg.AllowPrivateURLs && cfg.IsProduction() {
		logger.Warn("CHANNEL_ALLOW_PRIVATE_URLS=true em produção desliga a proteção contra SSRF")
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

	// --- Adaptadores de saída: um conector por canal/modo ---
	httpClient := safehttp.New(cfg.AllowPrivateURLs)
	icalConnector := ical.New(httpClient)
	connectors := registry{
		string(domain.ChannelBookingCom) + "/" + string(domain.ModeAPI):  bookingcom.New(httpClient),
		string(domain.ChannelBookingCom) + "/" + string(domain.ModeICal): icalConnector,
		string(domain.ChannelAirbnb) + "/" + string(domain.ModeICal):     icalConnector,
	}

	// --- Aplicação ---
	service := channelapp.NewService(tx, pgadapter.NewRepository(tx, cipher), connectors, events.Publisher,
		logger, cfg.APIPullInterval, cfg.ICalPullInterval)

	// --- Adaptadores de entrada: eventos ---
	// Sem middleware de inbox: os envios aos canais são idempotentes (mandam
	// valores absolutos), então reprocessar um evento é inofensivo.
	router := messaging.NewRouter()
	router.On(contracts.TypeAvailabilityChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.AvailabilityChanged) error {
			return service.OnAvailabilityChanged(ctx, e)
		}))
	router.On(contracts.TypeRateChanged, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.RateChanged) error {
			return service.OnRateChanged(ctx, e)
		}))
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group:       serviceName,
		Topics:      []string{contracts.TopicInventory},
		MaxAttempts: 5,
		Backoff:     time.Second, // a API de um parceiro fora do ar pede mais paciência
	}, router, logger)
	if err != nil {
		return err
	}

	// --- Adaptadores de entrada: HTTP ---
	mux := http.NewServeMux()
	httpx.NewHealth().Add("postgres", pool.Ping).Add("kafka", events.Producer.Ping).Register(mux)
	guard := auth.NewGuard(auth.NewVerifier(cfg.Auth.JWKSURL, cfg.Auth.Issuer))
	rest.NewHandler(service, strings.TrimRight(cfg.PublicAPIURL, "/")).Register(mux, guard)
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "outbox-relay", Component: events.Relay},
		app.Named{Name: "event-consumer", Component: eventConsumer},
		app.Named{Name: "busca-de-reservas", Component: app.Every(20*time.Second, logger, "busca-de-reservas", service.PullDue)},
	)
}
