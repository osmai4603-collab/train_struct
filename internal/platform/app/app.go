// Package app is the application bootstrapping and composition root. It wires
// together the platform and infrastructure packages into a single production
// lifecycle, mirroring the two-layer pattern used across the cashflow backend:
//
//	internal/platform/app           -> bootstrap (this package)
//	internal/infrastructure/server  -> runtime (Serve/Drain/Shutdown)
//
// The App exposes Bootstrap(args) for initialization and Run() for the serving
// phase. The eight-phase lifecycle is split as follows:
//
//	Phase 1 (Initialization): config engine, multi-target logger, postgres pool,
//	                          schema migrations (fail fast).
//	Phase 2 (Pre-Binding):    delegated to server.Run via synchronous net.Listen.
//	Phase 3 (Configuration):  timeouts, middleware chain, routers built here.
//	Phase 4-8 (Startup..Cleanup): executed inside server.Run.
package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	httphandlers "train/internal/httphandlers"
	"train/internal/infrastructure/health"
	"train/internal/infrastructure/postgres"
	"train/internal/infrastructure/server"
	"train/internal/infrastructure/worker"
	"train/internal/platform/config"
	platformctx "train/internal/platform/context"
	platformlogger "train/internal/platform/logger"
	"train/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultConfigPath is the JSON config store resolved in layer 2 of the
// pipeline (Defaults < File < Env < CLI < Overrides).
const defaultConfigPath = "config/config.json"

// App is the composition root coordinating initialization and cleanup order.
type App struct {
	cfg       *config.Configuration
	logger    *slog.Logger
	logCloser io.Closer
	health    *health.HealthChecker
	workerMgr *worker.Manager
	srv       *server.Server
	pool      *pgxpool.Pool
	resource  io.Closer
}

// New returns an uninitialized App; call Bootstrap then Run.
func New() *App {
	return &App{}
}

// Bootstrap performs Phase 1 (Initialization) and Phase 3 (Configuration):
// resolve configuration, build the logger, connect infrastructure, migrate the
// schema, wire the middleware chain, and construct the runtime server.
//
// Resources are intentionally bounded: if initialization fails at any step,
// the error is returned (fail-fast) and previously opened resources are closed.
func (a *App) Bootstrap(args []string) (err error) {
	// ─────────────────────────────────────────────────────────────────────
	// PHASE 1a: Configuration
	// ─────────────────────────────────────────────────────────────────────
	engine := config.NewEngine(
		config.WithConfigFile(defaultConfigPath),
		config.WithCLIArgs(args),
	)
	cfg, err := engine.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	a.cfg = cfg

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 1b: Multi-target structured logger
	// ─────────────────────────────────────────────────────────────────────
	log, logCloser := newLogger(cfg.Logger)
	a.logger = log
	a.logCloser = logCloser
	defer func() {
		if err != nil {
			_ = a.logCloser.Close()
		}
	}()

	a.logger.Info("configuration loaded",
		slog.String("interface", cfg.Server.Host),
		slog.String("port", cfg.Server.Port),
		slog.String("log_level", cfg.Logger.Level),
		slog.Bool("log_to_file", cfg.Logger.ToFile),
	)

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 1c: Infrastructure (fail fast)
	// ─────────────────────────────────────────────────────────────────────
	switch cfg.Database.Driver {
	case "postgres":
		if err := a.connectPostgres(); err != nil {
			return err
		}
	case "memory":
		a.logger.Info("using in-memory development database driver")
		a.health = health.NewHealthChecker(noopPinger{})
	default:
		return fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 3: Configuration — workers, router, middleware chain, server
	// ─────────────────────────────────────────────────────────────────────
	a.workerMgr = worker.NewManager(a.logger)

	if a.pool != nil {
		a.workerMgr.Start("pool-monitor", newPoolMonitor(a.pool, a.logger))
	}

	router := httphandlers.NewRouter(a.logger, a.health)

	// The platform context middleware MUST run outermost so that request_id and
	// client_ip are injected into the context before the request logging
	// middleware reads them.
	handler := platformctx.Middleware(
		platformlogger.HTTPLoggingMiddleware(
			a.logger,
			platformlogger.WithProbeSuppression("/livez", "/readyz"),
		)(router),
	)

	var resources []io.Closer
	if a.resource != nil {
		resources = append(resources, a.resource)
	}
	a.srv = server.New(cfg.Server, handler, a.health, a.workerMgr, a.logger, resources...)

	a.logger.Info("bootstrap complete",
		slog.String("addr", cfg.Server.Address()),
		slog.String("driver", cfg.Database.Driver),
	)
	return nil
}

// connectPostgres establishes the connection pool and applies schema migrations.
func (a *App) connectPostgres() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, postgres.NewConfigFromSettings(a.cfg.Database), a.logger)
	if err != nil {
		return fmt.Errorf("initialize postgres: %w", err)
	}
	a.pool = pool
	a.resource = poolCloser{pool: pool}

	if err := postgres.RunMigrations(ctx, pool, migrations.FS, ".", a.logger); err != nil {
		return fmt.Errorf("run postgres migrations: %w", err)
	}

	a.health = health.NewHealthChecker(postgres.NewHealthChecker(pool))
	return nil
}

// Run executes the serving lifecycle (Phases 2, 4-8 inside server.Run) until an
// OS termination signal arrives or a fatal server error occurs, then flushes
// and closes the log files.
func (a *App) Run() error {
	if a.srv == nil {
		return fmt.Errorf("app has not been bootstrapped")
	}

	sigCtx, sigStop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer sigStop()

	a.logger.Info("starting service", slog.String("addr", a.cfg.Server.Address()))

	if err := a.srv.Run(sigCtx); err != nil {
		a.logger.Error("server runtime failure", slog.Any("error", err))
		return fmt.Errorf("server runtime failure: %w", err)
	}

	a.logger.Info("service terminated successfully")

	if a.logCloser != nil {
		if err := a.logCloser.Close(); err != nil {
			a.logger.Warn("failed to close log files", slog.Any("error", err))
		}
	}
	return nil
}

// Close flushes and closes application-level resources (log files).
func (a *App) Close() error {
	if a.logCloser != nil {
		return a.logCloser.Close()
	}
	return nil
}

type noopPinger struct{}

func (noopPinger) Ping(context.Context) error { return nil }

type poolCloser struct {
	pool *pgxpool.Pool
}

func (c poolCloser) Close() error {
	c.pool.Close()
	return nil
}

// newPoolMonitor demonstrates supervised background work: it periodically
// emits database pool telemetry at DEBUG level and always stops cleanly on
// context cancellation during teardown.
func newPoolMonitor(pool *pgxpool.Pool, logger *slog.Logger) func(ctx context.Context) {
	hc := postgres.NewHealthChecker(pool)
	interval := 30 * time.Second

	return func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !logger.Enabled(ctx, slog.LevelDebug) {
					continue
				}
				stats := hc.Stats()
				logger.LogAttrs(ctx, slog.LevelDebug, "postgres pool telemetry",
					slog.Int("total_conns", int(stats.TotalConns)),
					slog.Int("idle_conns", int(stats.IdleConns)),
					slog.Int("acquired_conns", int(stats.AcquiredConns)),
					slog.Int64("acquire_count", stats.AcquireCount),
				)
			}
		}
	}
}
