package examples

import (
	"context"
	"log/slog"
	"os"
	"time"

	"train/internal/infrastructure/postgres"
	"train/internal/platform/config"
)

// ExamplePoolSetup demonstrates how to initialize, tune, and safely manage
// a PostgreSQL connection pool from service configuration.
func ExamplePoolSetup() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 1. Load application configuration
	appConfig := config.DefaultConfiguration()

	// 2. Map database settings to postgres infrastructure config
	dbConfig := postgres.NewConfigFromSettings(appConfig.Database)

	// 3. Initialize connection pool with fail-fast ping verification
	pool, err := postgres.NewPool(ctx, dbConfig, logger)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 4. Ensure graceful closure during application shutdown
	defer postgres.ClosePool(pool, logger)

	logger.Info("database pool ready for application traffic")
}
