// Package migrations embute os arquivos SQL de migração no binário do serviço.
package migrations

import "embed"

// FS contém as migrações (veja pkg/platform/postgres.Migrate).
//
//go:embed *.sql
var FS embed.FS
