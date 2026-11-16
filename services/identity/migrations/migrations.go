// Package migrations embute os arquivos SQL de migração no binário do serviço.
package migrations

import "embed"

// FS contém as migrações. A diretiva go:embed copia os arquivos .sql para
// dentro do executável em tempo de compilação: a imagem Docker não precisa
// carregar arquivos soltos e a versão do esquema anda junto com a do código.
//
//go:embed *.sql
var FS embed.FS
