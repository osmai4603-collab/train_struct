package examples

import (
	"context"
	"fmt"

	"train/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BankingService demonstrates transaction orchestration using postgres.ExecTxWithRetry.
type BankingService struct {
	pool *pgxpool.Pool
}

// NewBankingService constructs a BankingService.
func NewBankingService(pool *pgxpool.Pool) *BankingService {
	return &BankingService{pool: pool}
}

// TransferFunds transfers money atomically between two bank accounts.
// If a transient deadlock (40P01) or serialization conflict (40001) occurs,
// ExecTxWithRetry transparently replays the transaction using Exponential Backoff with Full Jitter.
func (s *BankingService) TransferFunds(ctx context.Context, fromAccountID, toAccountID string, amount int64) error {
	retryCfg := postgres.DefaultRetryConfig()

	return postgres.ExecTxWithRetry(ctx, s.pool, retryCfg, func(txCtx context.Context, tx pgx.Tx) error {
		// Repositories are instantiated with the active transaction handle 'tx'
		// which satisfies postgres.DBTX.
		deductQuery := `UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance >= $1`
		tag, err := tx.Exec(txCtx, deductQuery, amount, fromAccountID)
		if err != nil {
			return err // Triggers automatic rollback
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("insufficient funds or account not found: %s", fromAccountID)
		}

		creditQuery := `UPDATE accounts SET balance = balance + $1 WHERE id = $2`
		tag, err = tx.Exec(txCtx, creditQuery, amount, toAccountID)
		if err != nil {
			return err // Triggers automatic rollback
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("destination account not found: %s", toAccountID)
		}

		// Returning nil automatically calls tx.Commit(txCtx)
		return nil
	})
}
