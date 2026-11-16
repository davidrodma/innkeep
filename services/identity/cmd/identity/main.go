// Comando identity: serviço de autenticação e gestão de usuários internos.
//
// Este arquivo é a "raiz de composição" (composition root) do serviço: o
// único lugar que conhece todas as peças concretas e as conecta. A injeção de
// dependências em Go é isto — passar valores para construtores, à mão, sem
// framework nem mágica de reflexão.
//
// Uso:
//
//	identity          inicia o serviço
//	identity migrate  aplica as migrações e sai
//	identity keygen   gera uma chave JWT (JWT_PRIVATE_KEY) e sai
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/davidrodma/innkeep/pkg/platform/app"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/postgres"
	"github.com/davidrodma/innkeep/pkg/platform/redisx"
	"github.com/davidrodma/innkeep/services/identity/internal/adapter/in/rest"
	"github.com/davidrodma/innkeep/services/identity/internal/adapter/out/argon2id"
	pgadapter "github.com/davidrodma/innkeep/services/identity/internal/adapter/out/postgres"
	redisadapter "github.com/davidrodma/innkeep/services/identity/internal/adapter/out/redis"
	identityapp "github.com/davidrodma/innkeep/services/identity/internal/app"
	"github.com/davidrodma/innkeep/services/identity/internal/config"
	"github.com/davidrodma/innkeep/services/identity/migrations"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "keygen" {
		keygen()
		return
	}
	app.Main(run)
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
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

	privateKey, err := loadPrivateKey(cfg)
	if err != nil {
		return err
	}
	signer := auth.NewSigner(privateKey, cfg.Issuer)

	// --- Adaptadores de saída ---
	users := pgadapter.NewUserRepository(tx)
	sessions := pgadapter.NewSessionRepository(tx)
	hasher := argon2id.New()

	// --- Aplicação (casos de uso) ---
	authService, err := identityapp.NewAuthService(users, sessions, hasher, signer,
		redisadapter.NewLoginThrottle(redisClient), redisx.NewTokenRevocations(redisClient),
		cfg.AccessTTL, cfg.RefreshTTL)
	if err != nil {
		return err
	}
	userService := identityapp.NewUserService(users, sessions, hasher)

	if err := userService.EnsureBootstrapAdmin(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminName, cfg.BootstrapAdminPassword); err != nil {
		return fmt.Errorf("criando administrador inicial: %w", err)
	}

	// --- Adaptadores de entrada ---
	mux := http.NewServeMux()
	httpx.NewHealth().
		Add("postgres", pool.Ping).
		Add("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }).
		Register(mux)

	// O próprio identity valida tokens com a chave pública que ele mesmo emite.
	guard := auth.NewGuard(auth.NewStaticVerifier(cfg.Issuer, privateKey.Public().(ed25519.PublicKey)))
	rest.NewHandler(authService, userService, signer.PublicJWKS()).Register(mux, guard)

	server := httpx.NewServer(cfg.HTTPAddr, httpx.Standard(cfg.ServiceName, logger, mux), logger)

	return app.Run(ctx, logger,
		app.Named{Name: "http", Component: server},
		app.Named{Name: "limpeza-de-sessoes", Component: app.Every(time.Hour, logger, "limpeza-de-sessoes", authService.PurgeExpiredSessions)},
	)
}

// loadPrivateKey lê a chave de assinatura dos JWTs.
//
// Em produção a chave é OBRIGATÓRIA e vem de um cofre de segredos. Em
// desenvolvimento, se não houver chave, criamos uma efêmera: conveniente, mas
// todos os tokens deixam de valer a cada reinício — por isso proibido em
// produção.
func loadPrivateKey(cfg config.Config) (ed25519.PrivateKey, error) {
	if cfg.JWTPrivateKey == "" {
		if cfg.IsProduction() {
			return nil, errors.New("JWT_PRIVATE_KEY é obrigatória em produção (gere com `identity keygen`)")
		}
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("gerando chave efêmera: %w", err)
		}
		fmt.Fprintln(os.Stderr, "AVISO: JWT_PRIVATE_KEY não definida; usando chave efêmera (somente desenvolvimento)")
		return privateKey, nil
	}

	seed, err := base64.StdEncoding.DecodeString(cfg.JWTPrivateKey)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("JWT_PRIVATE_KEY inválida: esperado base64 de 32 bytes (gere com `identity keygen`)")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// keygen imprime uma nova semente Ed25519 em base64, pronta para ser gravada
// no cofre de segredos como JWT_PRIVATE_KEY.
func keygen() {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		fmt.Fprintln(os.Stderr, "erro ao gerar chave:", err)
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(seed))
}
