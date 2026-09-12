package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// Transactor abstracts transaction management for testability.
type Transactor interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// PgxTransactor implements Transactor using pgxpool.
type PgxTransactor struct {
	pool *pgxpool.Pool
}

// NewTransactor creates a Transactor backed by pgxpool.
func NewTransactor(pool *pgxpool.Pool) Transactor {
	return &PgxTransactor{pool: pool}
}

// RunInTx executes fn within a database transaction.
// If fn returns an error, the transaction is rolled back.
// If a transaction already exists in context, fn runs within it (nested call).
func (t *PgxTransactor) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// If already in a transaction, just execute the function.
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, ctxKey{}, tx)

	if err := fn(txCtx); err != nil {
		// El rollback NO puede usar ctx: si el cliente corto la conexion o el job
		// se cancelo, pgx devuelve "context already done" y el ROLLBACK nunca sale
		// al servidor. La conexion vuelve al pool en 'idle in transaction
		// (aborted)' y se queda ahi ocupando un slot con locks tomados.
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// TxFromContext extracts the active transaction from context.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(ctxKey{}).(pgx.Tx)
	return tx, ok
}

// DBTX is the interface that SQLC generates for its Queries constructor.
// Both pgxpool.Pool and pgx.Tx satisfy this interface.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Querier returns the appropriate database handle from context.
// If a transaction is active, returns the tx. Otherwise, returns the pool.
func Querier(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return pool
}
