package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	platformerr "train/internal/platform/errors"

	"github.com/golang-migrate/migrate/v4"
	pgx_migrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// MigrationOptions configures schema migration execution.
type MigrationOptions struct {
	// TableName overrides the default migration metadata table ("schema_migrations").
	TableName string
}

// RunMigrations executes all pending database migrations forward (Up)
// using an embedded filesystem (e.g. go:embed migrations/*.sql).
//
// If no new migrations are found (migrate.ErrNoChange), nil is returned.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS, migrationDir string, logger *slog.Logger) error {
	return RunMigrationsWithOptions(ctx, pool, migrationFS, migrationDir, MigrationOptions{}, logger)
}

// RunMigrationsWithOptions executes migrations with custom options like table name.
func RunMigrationsWithOptions(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS, migrationDir string, opts MigrationOptions, logger *slog.Logger) error {
	const op = "postgres.RunMigrations"

	if logger == nil {
		logger = slog.Default()
	}

	if pool == nil {
		return platformerr.Internal(op, "database connection pool cannot be nil", nil)
	}

	sourceDriver, err := iofs.New(migrationFS, migrationDir)
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to initialize iofs migration driver: %v", err), err)
	}

	db := stdlib.OpenDBFromPool(pool)
	driverConfig := &pgx_migrate.Config{}
	if opts.TableName != "" {
		driverConfig.MigrationsTable = opts.TableName
	}

	driver, err := pgx_migrate.WithInstance(db, driverConfig)
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to create postgres migration driver: %v", err), err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to create migrate instance: %v", err), err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Warn("failed to close migration source driver", slog.String("error", srcErr.Error()))
		}
		if dbErr != nil {
			logger.Warn("failed to close migration db driver", slog.String("error", dbErr.Error()))
		}
	}()

	logger.InfoContext(ctx, "running postgres schema migrations")

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.InfoContext(ctx, "postgres schema migrations are up to date (no change)")
			return nil
		}
		return platformerr.Internal(op, fmt.Sprintf("failed executing schema migrations: %v", err), err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		logger.WarnContext(ctx, "unable to query current migration version", slog.String("error", err.Error()))
	} else {
		logger.InfoContext(ctx, "postgres schema migrations completed successfully",
			slog.Uint64("version", uint64(version)),
			slog.Bool("dirty", dirty),
		)
	}

	return nil
}

// RollbackMigrations rolls back the schema by the specified number of steps.
func RollbackMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS, migrationDir string, steps int, logger *slog.Logger) error {
	const op = "postgres.RollbackMigrations"

	if logger == nil {
		logger = slog.Default()
	}

	if pool == nil {
		return platformerr.Internal(op, "database connection pool cannot be nil", nil)
	}
	if steps <= 0 {
		return platformerr.Invalid(op, "rollback steps must be greater than zero", nil)
	}

	sourceDriver, err := iofs.New(migrationFS, migrationDir)
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to initialize iofs migration driver: %v", err), err)
	}

	db := stdlib.OpenDBFromPool(pool)
	driver, err := pgx_migrate.WithInstance(db, &pgx_migrate.Config{})
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to create postgres migration driver: %v", err), err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return platformerr.Internal(op, fmt.Sprintf("failed to create migrate instance: %v", err), err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	logger.WarnContext(ctx, "rolling back postgres schema migrations", slog.Int("steps", steps))

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.InfoContext(ctx, "no migrations to rollback (already at base)")
			return nil
		}
		return platformerr.Internal(op, fmt.Sprintf("failed executing schema rollback: %v", err), err)
	}

	logger.InfoContext(ctx, "postgres schema rollback completed")
	return nil
}
