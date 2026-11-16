// Package postgres concentra o acesso ao PostgreSQL: pool de conexões,
// transações (Unit of Work), classificação de erros e migrações.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool abre o pool de conexões e espera o banco responder.
//
// Um pool reaproveita conexões já abertas: abrir uma conexão PostgreSQL custa
// caro (handshake TLS, autenticação, fork de processo no servidor), então cada
// requisição pega uma emprestada e devolve ao terminar.
func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres: URL de conexão inválida: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	// Cada comando SQL vira um span no trace distribuído. O nome do span traz
	// só o tipo do comando (SELECT, INSERT...), não o SQL inteiro: é o padrão
	// do otelpgx.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: criando pool: %w", err)
	}

	// Em Docker/Kubernetes o serviço pode subir antes do banco estar pronto.
	// Tentamos por alguns segundos antes de desistir.
	deadline := time.Now().Add(45 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			pool.Close()
			return nil, fmt.Errorf("postgres: banco indisponível: %w", err)
		}
		time.Sleep(time.Second)
	}
}

// Códigos SQLSTATE do PostgreSQL que nos interessam.
// Referência: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeExclusionViolation  = "23P01"
	codeCheckViolation      = "23514"
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation: a operação violou uma restrição UNIQUE / chave primária.
func IsUniqueViolation(err error) bool { return pgCode(err) == codeUniqueViolation }

// IsForeignKeyViolation: a operação violou uma chave estrangeira.
func IsForeignKeyViolation(err error) bool { return pgCode(err) == codeForeignKeyViolation }

// IsExclusionViolation: a operação violou uma restrição EXCLUDE (usada para
// impedir que duas reservas ocupem o mesmo quarto em datas sobrepostas).
func IsExclusionViolation(err error) bool { return pgCode(err) == codeExclusionViolation }

// IsCheckViolation: a operação violou uma restrição CHECK.
func IsCheckViolation(err error) bool { return pgCode(err) == codeCheckViolation }

// ConstraintName devolve o nome da restrição violada (ou "" se não houver).
func ConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}
