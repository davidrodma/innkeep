// Comando notification: envia e-mails transacionais aos hóspedes.
//
// É o exemplo mais puro de serviço orientado a eventos do InnKeep: não tem
// API de negócio, não tem banco próprio e ninguém o chama. Ele só observa o
// tópico de reservas e reage. Se estiver fora do ar, nada mais para — os
// eventos ficam guardados no Kafka e os e-mails saem quando ele voltar.
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/app"
	"github.com/davidrodma/innkeep/pkg/platform/config"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/pkg/platform/redisx"
	"github.com/davidrodma/innkeep/services/notification/internal/mailer"
	"github.com/davidrodma/innkeep/services/notification/internal/notifier"
)

const serviceName = "notification"

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Kafka config.Kafka
	Redis config.Redis

	SMTPHost     string `env:"SMTP_HOST,required"`
	SMTPPort     int    `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername string `env:"SMTP_USERNAME"`
	SMTPPassword string `env:"SMTP_PASSWORD"`
	// SMTPTLS: "starttls" (padrão em produção), "tls" (implícito) ou "none"
	// (somente para o servidor de testes local).
	SMTPTLS  string `env:"SMTP_TLS" envDefault:"starttls"`
	MailFrom string `env:"MAIL_FROM" envDefault:"InnKeep <reservas@innkeep.local>"`
	// PublicURL é o endereço do site público, usado nos links dos e-mails.
	PublicURL string `env:"PUBLIC_WEB_URL" envDefault:"http://localhost:3000"`
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

	sender := mailer.NewSMTP(mailer.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
		TLS: cfg.SMTPTLS, From: cfg.MailFrom,
	})
	service, err := notifier.New(sender, redisClient, cfg.PublicURL, logger)
	if err != nil {
		return err
	}

	router := messaging.NewRouter()
	service.Register(router)
	eventConsumer, err := messaging.NewConsumer(ctx, cfg.Kafka, messaging.ConsumerConfig{
		Group:  serviceName,
		Topics: []string{contracts.TopicProperty, contracts.TopicReservation},
		// E-mail é lento e o servidor pode estar momentaneamente fora: mais
		// tentativas e mais espaçadas do que o padrão.
		MaxAttempts: 6,
		Backoff:     time.Second,
	}, router, logger)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	httpx.NewHealth().
		Add("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }).
		Register(mux)
	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(serviceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "event-consumer", Component: eventConsumer},
	)
}
