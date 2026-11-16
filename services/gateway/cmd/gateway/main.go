// Comando gateway: a porta de entrada única da API do InnKeep.
//
// Padrão API Gateway: o front-end (e qualquer integração externa) conhece um
// só endereço. O gateway cuida do que é comum a todas as chamadas —
// autenticação, limite de requisições, CORS, cabeçalhos de segurança,
// identificação da requisição — e encaminha cada rota ao serviço responsável.
// Os serviços ficam inacessíveis de fora da rede interna.
//
// O gateway NÃO contém regra de negócio nem guarda dados: é um proxy reverso
// com políticas. Em Kubernetes ele fica atrás do Ingress (ALB).
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/platform/app"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/redisx"
	"github.com/davidrodma/innkeep/services/gateway/internal/proxy"
)

const serviceName = "gateway"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Redis config.Redis
	Auth  config.Auth

	// Endereços internos dos serviços.
	IdentityURL    string `env:"IDENTITY_URL,required"`
	PropertyURL    string `env:"PROPERTY_URL,required"`
	InventoryURL   string `env:"INVENTORY_URL,required"`
	ReservationURL string `env:"RESERVATION_URL,required"`
	GuestURL       string `env:"GUEST_URL,required"`
	BillingURL     string `env:"BILLING_URL,required"`
	ChannelURL     string `env:"CHANNEL_URL,required"`

	// Origens de navegador autorizadas a chamar a API diretamente (CORS).
	AllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`

	// Limites de requisições por minuto.
	RateLimitPublic        int `env:"RATE_LIMIT_PUBLIC_PER_MINUTE" envDefault:"60"`         // por IP, rotas públicas
	RateLimitAuth          int `env:"RATE_LIMIT_AUTH_PER_MINUTE" envDefault:"20"`           // por IP, login/refresh
	RateLimitAuthenticated int `env:"RATE_LIMIT_AUTHENTICATED_PER_MINUTE" envDefault:"600"` // por usuário
	// TrustedProxies: quantos proxies confiáveis existem na frente do gateway
	// (ex.: 1 para o ALB). Define qual IP do X-Forwarded-For é o do cliente.
	TrustedProxies int `env:"TRUSTED_PROXIES" envDefault:"0"`
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

	redisClient, err := redisx.NewClient(ctx, cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	gateway, err := proxy.New(proxy.Options{
		Routes: []proxy.Route{
			// A ordem importa: o primeiro prefixo que casar vence.
			{Prefix: "/v1/public/properties", Upstream: cfg.PropertyURL, Public: true},
			{Prefix: "/v1/public/availability", Upstream: cfg.InventoryURL, Public: true},
			{Prefix: "/v1/public/reservations", Upstream: cfg.ReservationURL, Public: true},
			{Prefix: "/v1/public/channels", Upstream: cfg.ChannelURL, Public: true},
			{Prefix: "/v1/auth/login", Upstream: cfg.IdentityURL, Public: true, Sensitive: true},
			{Prefix: "/v1/auth/refresh", Upstream: cfg.IdentityURL, Public: true, Sensitive: true},
			{Prefix: "/v1/auth/logout", Upstream: cfg.IdentityURL, Public: true},
			{Prefix: "/v1/auth", Upstream: cfg.IdentityURL},
			{Prefix: "/v1/users", Upstream: cfg.IdentityURL},
			{Prefix: "/v1/properties", Upstream: cfg.PropertyURL},
			{Prefix: "/v1/room-types", Upstream: cfg.PropertyURL},
			{Prefix: "/v1/rooms", Upstream: cfg.PropertyURL},
			{Prefix: "/v1/availability", Upstream: cfg.InventoryURL},
			{Prefix: "/v1/free-rooms", Upstream: cfg.InventoryURL},
			{Prefix: "/v1/rates", Upstream: cfg.InventoryURL},
			{Prefix: "/v1/blocks", Upstream: cfg.InventoryURL},
			{Prefix: "/v1/reservations", Upstream: cfg.ReservationURL},
			{Prefix: "/v1/guests", Upstream: cfg.GuestURL},
			{Prefix: "/v1/registrations", Upstream: cfg.GuestURL},
			{Prefix: "/v1/registration-options", Upstream: cfg.GuestURL},
			{Prefix: "/v1/folios", Upstream: cfg.BillingURL},
			{Prefix: "/v1/payment-methods", Upstream: cfg.BillingURL},
			{Prefix: "/v1/expenses", Upstream: cfg.BillingURL},
			{Prefix: "/v1/cash", Upstream: cfg.BillingURL},
			{Prefix: "/v1/fee-rules", Upstream: cfg.BillingURL},
			{Prefix: "/v1/reports", Upstream: cfg.BillingURL},
			{Prefix: "/v1/channels", Upstream: cfg.ChannelURL},
		},
		Verifier:    auth.NewVerifier(cfg.Auth.JWKSURL, cfg.Auth.Issuer),
		Revocations: redisx.NewTokenRevocations(redisClient),
		Limiter:     redisx.NewRateLimiter(redisClient, "gateway:ratelimit"),
		Limits: proxy.Limits{
			Public: cfg.RateLimitPublic, Sensitive: cfg.RateLimitAuth,
			Authenticated: cfg.RateLimitAuthenticated, Window: time.Minute,
		},
		TrustedProxies: cfg.TrustedProxies,
		Logger:         logger,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	httpx.NewHealth().
		Add("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }).
		Register(mux)
	mux.Handle("/", gateway)

	handler := httpx.Chain(mux,
		httpx.Recover(logger),
		httpx.SecurityHeaders,
		httpx.CORS(cfg.AllowedOrigins),
		httpx.RequestID,
		httpx.Tracing(serviceName),
		httpx.AccessLog(logger),
	)
	server := httpx.NewServer(cfg.HTTPAddr, handler, logger)

	return app.Run(ctx, logger, app.Named{Name: "http", Component: server})
}
