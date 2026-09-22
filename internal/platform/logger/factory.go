package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Environment represents a service operating environment and determines the
// default log format and level.
type Environment string

// Standard operating environments supported across the project.
const (
	// EnvDevelopment uses the human-readable TextHandler format for local output.
	EnvDevelopment Environment = "development"
	// EnvStaging uses the JSONHandler format for shared test platforms.
	EnvStaging Environment = "staging"
	// EnvProduction uses the JSONHandler (NDJSON) format for automated aggregation systems.
	EnvProduction Environment = "production"
)

// ParseEnvironment parses the input string into a supported operating environment,
// returning EnvDevelopment by default for empty or unknown values.
func ParseEnvironment(s string) Environment {
	switch Environment(strings.ToLower(strings.TrimSpace(s))) {
	case EnvProduction:
		return EnvProduction
	case EnvStaging:
		return EnvStaging
	default:
		return EnvDevelopment
	}
}

// ParseLevel parses the input string into one of the log/slog levels.
// It returns (LevelInfo, false) when parsing fails.
func ParseLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

// Config represents the base event logger configuration settings for the rest of the services.
type Config struct {
	// Env determines the log format (JSON for production, Text for development).
	Env Environment
	// Level determines the minimum level passed through to the handler.
	Level slog.Level
	// AddSource includes the source file name and line number in every log record
	// (preferably enabled only in development).
	AddSource bool
	// Output is the destination where log records are written (defaults to os.Stdout).
	Output io.Writer
	// RedactKeys is a list of keys whose values are redacted automatically via
	// RedactingHandler, and is empty when no redacting handler is desired.
	RedactKeys []string
}

// DefaultConfig returns the safe default development configuration.
func DefaultConfig() Config {
	return Config{
		Env:       EnvDevelopment,
		Level:     slog.LevelInfo,
		AddSource: true,
		Output:    os.Stdout,
	}
}

// New creates a *slog.Logger configured according to the given environment and settings.
// It selects JSONHandler for production and Staging environments, and TextHandler
// for development, wrapping the result with RedactingHandler when redaction keys are set.
func New(cfg Config) *slog.Logger {
	return slog.New(NewHandler(cfg))
}

// NewHandler builds the underlying slog.Handler according to the given settings
// without creating a Logger. Useful when composing a Logger from a custom handler
// (for example, pointing at a test bytes.Buffer).
func NewHandler(cfg Config) slog.Handler {
	var opts slog.HandlerOptions

	switch strings.ToLower(string(cfg.Env)) {
	case string(EnvProduction), string(EnvStaging):
		opts.Level = cfg.Level
		opts.AddSource = cfg.AddSource
		handler := slog.NewJSONHandler(resolveOutput(cfg.Output), &opts)
		return wrapRedaction(handler, cfg.RedactKeys)
	default:
		opts.Level = cfg.Level
		opts.AddSource = cfg.AddSource
		handler := slog.NewTextHandler(resolveOutput(cfg.Output), &opts)
		return wrapRedaction(handler, cfg.RedactKeys)
	}
}

// resolveOutput ensures a non-nil default output destination exists.
func resolveOutput(out io.Writer) io.Writer {
	if out == nil {
		return os.Stdout
	}
	return out
}

// wrapRedaction wraps the handler with RedactingHandler when redaction keys exist.
func wrapRedaction(inner slog.Handler, keys []string) slog.Handler {
	if len(keys) == 0 {
		return inner
	}
	return NewRedactingHandler(inner, keys...)
}
