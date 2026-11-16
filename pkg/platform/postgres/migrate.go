package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" para database/sql
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate aplica as migrações SQL pendentes de um serviço.
//
// Os arquivos .sql ficam embutidos no binário (diretiva //go:embed em cada
// serviço), então a imagem Docker carrega consigo o esquema de que precisa.
// O goose anota em uma tabela quais migrações já rodaram; um "advisory lock"
// do PostgreSQL garante que, com várias réplicas subindo ao mesmo tempo,
// apenas uma aplica as migrações.
func Migrate(ctx context.Context, databaseURL string, migrations fs.FS) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("postgres: abrindo conexão para migração: %w", err)
	}
	defer db.Close()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("postgres: criando lock de migração: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("postgres: preparando migrações: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("postgres: aplicando migrações: %w", err)
	}
	for _, result := range results {
		slog.InfoContext(ctx, "migração aplicada",
			slog.String("arquivo", result.Source.Path),
			slog.Duration("duracao", result.Duration))
	}
	return nil
}
