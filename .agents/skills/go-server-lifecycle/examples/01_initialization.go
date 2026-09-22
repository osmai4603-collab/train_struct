package lifecycle

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// =============================================================================
// Phase 1: Initialization
// =============================================================================
// Initialize all dependencies BEFORE starting the server.
// If any critical dependency fails, exit immediately (Fail-Fast).
// Order: config → logger → database → cache → services
// =============================================================================

// Config holds the application configuration.
type Config struct {
	Port            string
	DatabaseDSN     string
	ShutdownTimeout int
	DrainTimeout    int
}

// LoadConfig reads configuration.
func LoadConfig() (*Config, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN is required")
	}

	return &Config{
		Port:            "8080",
		DatabaseDSN:     dsn,
		ShutdownTimeout: 15,
		DrainTimeout:    10,
	}, nil
}

// initPrettyHandler is a simplified custom slog.Handler for colored terminal output.
type initPrettyHandler struct {
	w io.Writer
}

func (h *initPrettyHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *initPrettyHandler) Handle(_ context.Context, r slog.Record) error {
	timeStr := r.Time.Format("2006-01-02 03:04:05 PM")
	fmt.Fprintf(h.w, "\033[90m%s\033[0m [%s] %s\n", timeStr, r.Level, r.Message)
	return nil
}
func (h *initPrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *initPrettyHandler) WithGroup(name string) slog.Handler      { return h }

// initPhaseLogger creates an environment-aware logger.
// Best Practice: Colored Text for TTY, Structured JSON for Production.
func initPhaseLogger() *slog.Logger {
	out := os.Stderr
	isTerminal := false
	if stat, err := out.Stat(); err == nil {
		isTerminal = (stat.Mode() & os.ModeCharDevice) != 0
	}

	if isTerminal {
		return slog.New(&initPrettyHandler{w: out})
	}

	return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02 03:04:05 PM"))
			}
			return a
		},
	}))
}

// Dependencies holds all initialized dependencies.
type Dependencies struct {
	Logger *slog.Logger
	DB     *sql.DB
}

// InitDependencies creates all dependencies in the correct order.
func InitDependencies(cfg *Config) (*Dependencies, error) {
	// 1. Logger (Dual-Mode)
	logger := initPhaseLogger()

	// 2. Database connection
	db, err := sql.Open("postgres", cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("all dependencies initialized successfully")

	return &Dependencies{
		Logger: logger,
		DB:     db,
	}, nil
}

// Close releases all dependencies in REVERSE order of creation.
func (d *Dependencies) Close() {
	if d.DB != nil {
		_ = d.DB.Close()
	}
	d.Logger.Info("all dependencies closed")
}
