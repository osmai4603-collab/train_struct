package examples

import (
	"context"
	"embed"
	"log/slog"

	"train/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

// In a real project, migration SQL files are placed in a 'migrations' folder
// and embedded into the application binary:
//
// //go:embed migrations/*.sql
// var migrationsFS embed.FS

// RunAppMigrations demonstrates applying database schema migrations at startup.
func RunAppMigrations(ctx context.Context, pool *pgxpool.Pool, fs embed.FS, logger *slog.Logger) error {
	// Applies all unapplied migrations from the embedded folder "migrations"
	if err := postgres.RunMigrations(ctx, pool, fs, "migrations", logger); err != nil {
		return err
	}

	logger.Info("schema migrations successfully applied")
	return nil
}
