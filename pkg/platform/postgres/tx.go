package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX é o subconjunto de operações comum a um pool e a uma transação. Os
// repositórios dependem desta interface e, por isso, funcionam tanto dentro
// quanto fora de uma transação sem saber a diferença.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// txState é o que viaja dentro do contexto durante uma transação.
type txState struct {
	tx          pgx.Tx
	afterCommit []func()
}

func stateFrom(ctx context.Context) (*txState, bool) {
	state, ok := ctx.Value(txKey{}).(*txState)
	return state, ok
}

// TxManager implementa o padrão Unit of Work: várias operações de repositórios
// diferentes são confirmadas (COMMIT) ou desfeitas (ROLLBACK) como uma unidade.
//
// A transação viaja DENTRO do context.Context. Assim a camada de aplicação
// consegue dizer "isto tudo é atômico" sem conhecer pgx nem SQL:
//
//	err := tx.WithinTx(ctx, func(ctx context.Context) error {
//	    if err := repo.Save(ctx, reserva); err != nil { return err }
//	    return publisher.Publish(ctx, evento) // grava na outbox, MESMA transação
//	})
//
// É esse "mesma transação" que sustenta o padrão Transactional Outbox.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager cria o gerenciador de transações sobre um pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

// Pool expõe o pool para os poucos casos que precisam de uma conexão dedicada.
func (m *TxManager) Pool() *pgxpool.Pool { return m.pool }

// DB devolve a transação em andamento no contexto ou, se não houver, o pool.
func (m *TxManager) DB(ctx context.Context) DBTX {
	if state, ok := stateFrom(ctx); ok {
		return state.tx
	}
	return m.pool
}

// InTx informa se o contexto já carrega uma transação.
func (m *TxManager) InTx(ctx context.Context) bool {
	_, ok := stateFrom(ctx)
	return ok
}

// WithinTx executa fn dentro de uma transação. Se fn devolver erro (ou entrar
// em pânico), tudo é desfeito. Se já existir uma transação no contexto, fn
// simplesmente participa dela (não há transações aninhadas reais).
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if m.InTx(ctx) {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: iniciando transação: %w", err)
	}
	state := &txState{tx: tx}

	defer func() {
		if p := recover(); p != nil {
			// WithoutCancel: o rollback precisa acontecer mesmo que o contexto
			// da requisição já tenha sido cancelado.
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, state)); err != nil {
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("postgres: rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	for _, hook := range state.afterCommit {
		hook()
	}
	return nil
}

// AfterCommit agenda fn para rodar DEPOIS que a transação do contexto for
// confirmada com sucesso. Fora de uma transação, fn roda imediatamente.
//
// Serve para efeitos colaterais que não podem ser desfeitos e que só fazem
// sentido se o dado realmente foi gravado — o exemplo clássico é invalidar um
// cache. Invalidar ANTES do commit abre uma janela em que outra requisição lê
// o valor antigo do banco e o recoloca no cache.
func (m *TxManager) AfterCommit(ctx context.Context, fn func()) {
	if state, ok := stateFrom(ctx); ok {
		state.afterCommit = append(state.afterCommit, fn)
		return
	}
	fn()
}

// WithinSavepoint executa fn protegido por um SAVEPOINT dentro da transação
// atual. No PostgreSQL, qualquer erro "envenena" a transação inteira (os
// comandos seguintes falham com "current transaction is aborted"). Com um
// savepoint, um erro esperado — por exemplo "este quarto já está ocupado" —
// desfaz apenas o trecho de fn, e a transação externa segue utilizável para
// tentar outra coisa.
func (m *TxManager) WithinSavepoint(ctx context.Context, fn func(ctx context.Context) error) error {
	outer, ok := stateFrom(ctx)
	if !ok {
		return m.WithinTx(ctx, fn)
	}

	// No pgx, Begin sobre uma transação cria um SAVEPOINT.
	savepoint, err := outer.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: criando savepoint: %w", err)
	}
	inner := &txState{tx: savepoint}
	if err := fn(context.WithValue(ctx, txKey{}, inner)); err != nil {
		if rbErr := savepoint.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			return errors.Join(err, fmt.Errorf("postgres: rollback do savepoint: %w", rbErr))
		}
		return err
	}
	if err := savepoint.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: liberando savepoint: %w", err)
	}
	// Os ganchos do trecho protegido passam para a transação externa: só
	// rodam se ela também for confirmada.
	outer.afterCommit = append(outer.afterCommit, inner.afterCommit...)
	return nil
}
