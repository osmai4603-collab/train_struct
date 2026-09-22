package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is a unified database executor interface satisfied by *pgxpool.Pool,
// *pgx.Conn, and pgx.Tx. It allows repositories to remain completely agnostic
// of whether they are executing inside a transaction or against the connection pool.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxFn defines the closure signature executed within an atomic database transaction.
type TxFn func(ctx context.Context, tx pgx.Tx) error

// ExecTx executes the provided closure within an atomic database transaction with
// default transaction options.
//
// Safety Guarantees:
//   - Defers Rollback immediately after Begin; if Commit is reached, Rollback is a safe no-op.
//   - Recovers from panics, issues Rollback, and re-panics to preserve stack traces.
//   - Translates any returned error through TranslateError.
func ExecTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	return ExecTxWithOptions(ctx, pool, pgx.TxOptions{}, fn)
}

// ExecTxWithOptions executes the closure within a transaction configured with custom
// pgx.TxOptions (e.g. pgx.Serializable, pgx.ReadOnly).
func ExecTxWithOptions(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn TxFn) (err error) {
	const op = "postgres.ExecTx"

	if pool == nil {
		return TranslateError(op, errors.New("database connection pool is nil"))
	}

	tx, beginErr := pool.BeginTx(ctx, opts)
	if beginErr != nil {
		return TranslateError(op, fmt.Errorf("failed to begin transaction: %w", beginErr))
	}

	// Panic safety: ensure rollback runs on unhandled panic before re-throwing
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	// Defer rollback: if tx is already committed, pgx.ErrTxClosed is safely ignored
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Execute user domain logic inside transaction
	if err := fn(ctx, tx); err != nil {
		return TranslateError(op, err)
	}

	// Commit transaction
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return TranslateError(op, fmt.Errorf("failed to commit transaction: %w", commitErr))
	}

	return nil
}

// ExecTxWithRetry executes a transaction with automatic retry on transient failures
// (such as serialization failures 40001 or deadlocks 40P01) using exponential backoff.
//
// The supplied TxFn MUST be idempotent because it may be executed multiple times.
func ExecTxWithRetry(ctx context.Context, pool *pgxpool.Pool, cfg RetryConfig, fn TxFn) error {
	return WithRetry(ctx, cfg, func(attemptCtx context.Context) error {
		return ExecTx(attemptCtx, pool, fn)
	})
}
