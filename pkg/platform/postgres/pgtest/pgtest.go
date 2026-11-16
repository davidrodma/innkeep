// Package pgtest ajuda a escrever testes de INTEGRAÇÃO contra um PostgreSQL de
// verdade.
//
// Testes de integração ficam junto do adaptador que testam, com a build tag
// "integration", e só rodam quando pedidos explicitamente:
//
//	INTEGRATION_DATABASE_URL=postgres://postgres:postgres_dev@localhost:5432/postgres?sslmode=disable \
//	  go test -tags=integration ./...
//
// Cada teste ganha um banco NOVO e descartável, com as migrações reais do
// serviço aplicadas, apagado ao final: os testes não interferem entre si nem
// com os dados de desenvolvimento.
package pgtest

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/davidrodma/innkeep/pkg/platform/postgres"
)

var counter atomic.Int64

// FreshDatabase cria um banco vazio, aplica as migrações e devolve o
// TxManager conectado a ele. Sem INTEGRATION_DATABASE_URL, o teste é pulado.
func FreshDatabase(t *testing.T, migrations fs.FS) *postgres.TxManager {
	t.Helper()
	adminURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if adminURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL não definida: teste de integração pulado")
	}
	ctx := context.Background()

	name := fmt.Sprintf("innkeep_it_%d_%d", time.Now().UnixNano(), counter.Add(1))
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("pgtest: conectando ao PostgreSQL: %v", err)
	}
	// O nome é gerado aqui (não vem de entrada externa), por isso pode ser
	// concatenado no SQL — CREATE DATABASE não aceita parâmetros ($1).
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("pgtest: criando banco de teste: %v", err)
	}

	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("pgtest: URL inválida: %v", err)
	}
	parsed.Path = "/" + name
	if err := postgres.Migrate(ctx, parsed.String(), migrations); err != nil {
		t.Fatalf("pgtest: migrações: %v", err)
	}
	pool, err := postgres.NewPool(ctx, parsed.String(), 25)
	if err != nil {
		t.Fatalf("pgtest: pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	return postgres.NewTxManager(pool)
}
