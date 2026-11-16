// Package config define a configuração do serviço identity.
package config

import (
	"time"

	"github.com/davidrodma/innkeep/pkg/platform/config"
)

// Config é lida das variáveis de ambiente (veja .env.example na raiz).
type Config struct {
	config.Base
	Postgres config.Postgres
	Redis    config.Redis

	// JWTPrivateKey é a semente Ed25519 (32 bytes) em base64. Gere uma com
	// `identity keygen`. Em desenvolvimento pode ficar vazia: uma chave
	// efêmera é criada a cada inicialização.
	JWTPrivateKey string        `env:"JWT_PRIVATE_KEY"`
	Issuer        string        `env:"AUTH_ISSUER" envDefault:"innkeep-identity"`
	AccessTTL     time.Duration `env:"AUTH_ACCESS_TTL" envDefault:"15m"`
	RefreshTTL    time.Duration `env:"AUTH_REFRESH_TTL" envDefault:"168h"`

	// Administrador criado na primeira inicialização, quando ainda não existe
	// nenhum usuário.
	BootstrapAdminEmail    string `env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string `env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string `env:"BOOTSTRAP_ADMIN_NAME" envDefault:"Administrador"`
}

// Load lê a configuração do ambiente.
func Load() (Config, error) {
	cfg, err := config.Load[Config]()
	if err != nil {
		return cfg, err
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "identity"
	}
	return cfg, nil
}
