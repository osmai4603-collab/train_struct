package examples

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Environment represents the service's operating environment
type Environment string

const (
	EnvProduction  Environment = "production"
	EnvStaging     Environment = "staging"
	EnvDevelopment Environment = "development"
)

// Config represents the logger's configuration settings
type Config struct {
	Env       Environment
	Level     slog.Level
	AddSource bool
	Output    io.Writer
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Env:       EnvDevelopment,
		Level:     slog.LevelInfo,
		AddSource: false,
		Output:    os.Stdout,
	}
}

// NewLoggerFactory creates a *slog.Logger configured according to the given environment
func NewLoggerFactory(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	switch strings.ToLower(string(cfg.Env)) {
	case string(EnvProduction), string(EnvStaging):
		// JSONHandler for production and automated CI systems
		handler = slog.NewJSONHandler(out, opts)
	default:
		// TextHandler for the local development environment with easy readability
		handler = slog.NewTextHandler(out, opts)
	}

	return slog.New(handler)
}
