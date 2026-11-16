// Package config carrega a configuração dos serviços a partir de variáveis de
// ambiente — o fator III do "Twelve-Factor App" (https://12factor.net/pt_br/config).
//
// Não existe arquivo de configuração por ambiente dentro da imagem Docker: a
// MESMA imagem roda no seu notebook e em produção; só as variáveis mudam.
// Segredos (senhas, chaves) entram pelo mesmo caminho e nunca vão para o Git.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Base reúne as opções comuns a todos os serviços. Cada serviço embute esta
// struct na sua própria Config.
type Base struct {
	// ServiceName identifica o serviço em logs, traces e métricas.
	ServiceName string `env:"SERVICE_NAME"`
	// Environment: "development", "staging" ou "production".
	Environment string `env:"APP_ENV" envDefault:"development"`
	// Version é a versão do build (normalmente o SHA do commit).
	Version   string `env:"APP_VERSION" envDefault:"dev"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
	HTTPAddr  string `env:"HTTP_ADDR" envDefault:":8080"`
	// OTLPEndpoint é o endereço do OpenTelemetry Collector. Vazio = telemetria
	// desligada (o serviço funciona normalmente, só não exporta nada).
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// IsProduction informa se o serviço está rodando em produção. Em produção
// alguns atalhos de desenvolvimento são proibidos (ex.: gerar chaves efêmeras).
func (b Base) IsProduction() bool { return b.Environment == "production" }

// Postgres configura o acesso ao banco relacional do serviço.
type Postgres struct {
	URL      string `env:"DATABASE_URL,required"`
	MaxConns int32  `env:"DATABASE_MAX_CONNS" envDefault:"10"`
	// AutoMigrate aplica as migrações pendentes ao iniciar. Prático em
	// desenvolvimento; em produção as migrações rodam em um initContainer.
	AutoMigrate bool `env:"DATABASE_AUTO_MIGRATE" envDefault:"true"`
}

// Kafka configura o acesso ao broker de eventos.
type Kafka struct {
	Brokers []string `env:"KAFKA_BROKERS,required" envSeparator:","`
	// TLS e SASL são usados em produção (Amazon MSK); em desenvolvimento o
	// broker local aceita conexões sem autenticação.
	TLS           bool   `env:"KAFKA_TLS" envDefault:"false"`
	SASLMechanism string `env:"KAFKA_SASL_MECHANISM"` // "", PLAIN, SCRAM-SHA-256 ou SCRAM-SHA-512
	Username      string `env:"KAFKA_USERNAME"`
	Password      string `env:"KAFKA_PASSWORD"`
	// EnsureTopics cria os tópicos que faltarem ao iniciar.
	EnsureTopics     bool  `env:"KAFKA_ENSURE_TOPICS" envDefault:"true"`
	TopicPartitions  int32 `env:"KAFKA_TOPIC_PARTITIONS" envDefault:"3"`
	TopicReplication int16 `env:"KAFKA_TOPIC_REPLICATION" envDefault:"1"`
}

// Redis configura o acesso ao Redis.
type Redis struct {
	URL string `env:"REDIS_URL,required"`
}

// Auth configura a verificação dos tokens emitidos pelo serviço identity.
type Auth struct {
	// JWKSURL é onde o serviço busca as chaves PÚBLICAS para validar os JWTs.
	JWKSURL string `env:"AUTH_JWKS_URL,required"`
	Issuer  string `env:"AUTH_ISSUER" envDefault:"innkeep-identity"`
}

// Load preenche uma struct de configuração a partir do ambiente.
//
//	cfg, err := config.Load[Config]()
func Load[T any]() (T, error) {
	cfg, err := env.ParseAs[T]()
	if err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
