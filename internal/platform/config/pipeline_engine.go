package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// Pipeline Engine: 5-Layer Resolution Order
// =============================================================================

// PipelineEngine executes the layered resolution pipeline:
// Defaults < File < Environment < CLI < Overrides
type PipelineEngine struct {
	configFile string
	cliArgs    []string
	disableEnv bool
	envLookup  func(key string) (string, bool)
	overrides  []func(*Configuration)
}

// NewPipelineEngine creates a new pipeline engine with standard environment lookup.
func NewPipelineEngine() *PipelineEngine {
	return &PipelineEngine{
		envLookup: os.LookupEnv,
		overrides: make([]func(*Configuration), 0),
	}
}

// Resolve executes all layers in strict deterministic order and produces the raw merged Configuration.
func (p *PipelineEngine) Resolve() (*Configuration, error) {
	// -------------------------------------------------------------------------
	// Layer 1: Hardcoded Defaults
	// -------------------------------------------------------------------------
	cfg := DefaultConfiguration()

	// -------------------------------------------------------------------------
	// Layer 2: File Store (JSON) - Sparse unmarshaling
	// -------------------------------------------------------------------------
	if p.configFile != "" {
		if err := p.loadFile(p.configFile, cfg); err != nil {
			return nil, fmt.Errorf("layer 2 (file %s): %w", p.configFile, err)
		}
	}

	// -------------------------------------------------------------------------
	// Layer 3: Environment Variables with Canonical Keys & Aliases
	// -------------------------------------------------------------------------
	if !p.disableEnv {
		if err := p.loadEnvironment(cfg); err != nil {
			return nil, fmt.Errorf("layer 3 (environment): %w", err)
		}
	}

	// -------------------------------------------------------------------------
	// Layer 4: CLI Flags
	// -------------------------------------------------------------------------
	if len(p.cliArgs) > 0 {
		if err := p.loadCLI(p.cliArgs, cfg); err != nil {
			return nil, fmt.Errorf("layer 4 (cli): %w", err)
		}
	}

	// -------------------------------------------------------------------------
	// Layer 5: Runtime Programmatic Overrides
	// -------------------------------------------------------------------------
	for _, override := range p.overrides {
		if override != nil {
			override(cfg)
		}
	}

	return cfg, nil
}

func (p *PipelineEngine) loadFile(path string, target *Configuration) error {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Optional file not present on disk is allowed
			return nil
		}
		return err
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	return nil
}

func (p *PipelineEngine) lookupEnv(keys ...string) (string, bool) {
	lookup := p.envLookup
	if lookup == nil {
		lookup = os.LookupEnv
	}

	for _, key := range keys {
		if val, exists := lookup(key); exists {
			return val, true
		}
	}
	return "", false
}

// lookupSecret resolves credentials by prioritizing direct environment keys,
// followed by file-based secret paths (*_FILE) for Kubernetes/Docker container security.
func (p *PipelineEngine) lookupSecret(envKey, fileEnvKey string, aliases ...string) (string, bool) {
	allKeys := append([]string{envKey}, aliases...)
	if val, ok := p.lookupEnv(allKeys...); ok && val != "" {
		return val, true
	}

	if filePath, ok := p.lookupEnv(fileEnvKey); ok && filePath != "" {
		if data, err := os.ReadFile(filepath.Clean(filePath)); err == nil {
			return strings.TrimSpace(string(data)), true
		}
	}

	return p.lookupEnv(allKeys...)
}

func (p *PipelineEngine) loadEnvironment(cfg *Configuration) error {
	// Server settings
	if val, ok := p.lookupEnv("PORT", "HTTP_PORT", "TRAIN_PORT"); ok {
		cfg.Server.Port = val
	}
	if val, ok := p.lookupEnv("HOST", "HTTP_INTERFACE", "BIND_ADDRESS"); ok {
		cfg.Server.Host = val
	}
	if val, ok := p.lookupEnv("READ_TIMEOUT", "HTTP_READ_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.ReadTimeout = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("READ_HEADER_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "TRAIN_READ_HEADER_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.ReadHeaderTimeout = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("WRITE_TIMEOUT", "HTTP_WRITE_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.WriteTimeout = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("IDLE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "TRAIN_IDLE_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.IdleTimeout = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("SHUTDOWN_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.ShutdownTimeout = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("DRAIN_DURATION"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Server.DrainDuration = Duration(d)
		}
	}
	if val, ok := p.lookupEnv("MAX_HEADER_BYTES"); ok {
		if sz, err := ParseByteSize(val); err == nil {
			cfg.Server.MaxHeaderBytes = sz
		}
	}
	if val, ok := p.lookupEnv("MAX_BODY_SIZE"); ok {
		if sz, err := ParseByteSize(val); err == nil {
			cfg.Server.MaxBodySize = sz
		}
	}

	// Logger settings
	if val, ok := p.lookupEnv("LOG_LEVEL", "TRAIN_LOG_LEVEL"); ok {
		cfg.Logger.Level = val
	}
	if val, ok := p.lookupEnv("LOG_COLOR", "TRAIN_LOG_COLOR"); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			cfg.Logger.ForceColor = b
		}
	}
	if val, ok := p.lookupEnv("LOG_TO_FILE", "TRAIN_LOG_TO_FILE"); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			cfg.Logger.ToFile = b
		}
	}
	if val, ok := p.lookupEnv("LOG_DIR", "TRAIN_LOG_DIR"); ok {
		cfg.Logger.LogDir = val
	}

	// Database settings
	if val, ok := p.lookupEnv("DB_DRIVER"); ok {
		cfg.Database.Driver = val
	}
	if val, ok := p.lookupEnv("DB_HOST", "PGHOST", "POSTGRES_HOST"); ok {
		cfg.Database.Host = val
	}
	if val, ok := p.lookupEnv("DB_PORT", "PGPORT", "POSTGRES_PORT"); ok {
		cfg.Database.Port = val
	}
	if val, ok := p.lookupEnv("DB_NAME", "PGDATABASE", "POSTGRES_DB"); ok {
		cfg.Database.Name = val
	}
	if val, ok := p.lookupEnv("DB_USER", "PGUSER", "POSTGRES_USER"); ok {
		cfg.Database.User = val
	}
	if val, ok := p.lookupSecret("DB_PASSWORD", "DB_PASSWORD_FILE", "PGPASSWORD", "POSTGRES_PASSWORD"); ok {
		cfg.Database.Password = val
	}
	if val, ok := p.lookupSecret("DATABASE_URL", "DATABASE_URL_FILE", "DB_URL", "POSTGRES_URL"); ok {
		cfg.Database.DatabaseURL = val
	}
	if val, ok := p.lookupEnv("DB_MAX_OPEN_CONNS"); ok {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.Database.MaxOpenConns = n
		}
	}
	if val, ok := p.lookupEnv("DB_MAX_IDLE_CONNS"); ok {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.Database.MaxIdleConns = n
		}
	}

	// Auth settings
	if val, ok := p.lookupSecret("JWT_SECRET", "JWT_SECRET_FILE"); ok {
		cfg.Auth.JWTSecret = val
	}
	if val, ok := p.lookupEnv("JWT_TOKEN_TTL", "TOKEN_TTL"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Auth.TokenTTL = Duration(d)
		}
	}

	return nil
}

func (p *PipelineEngine) loadCLI(args []string, cfg *Configuration) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Mute usage on error to avoid noisy stdout

	var (
		port              = fs.String("port", "", "HTTP server port")
		host              = fs.String("host", "", "HTTP server host")
		readTimeout       = fs.String("read-timeout", "", "HTTP read timeout")
		readHeaderTimeout = fs.String("read-header-timeout", "", "HTTP read header timeout")
		writeTimeout      = fs.String("write-timeout", "", "HTTP write timeout")
		shutdownTimeout   = fs.String("shutdown-timeout", "", "Server graceful shutdown timeout")
		drainDuration     = fs.String("drain-duration", "", "Server drain duration")
		maxHeaderBytes    = fs.String("max-header-bytes", "", "Maximum permitted HTTP header bytes")
		maxBodySize       = fs.String("max-body-size", "", "Maximum permitted HTTP body size")

		logLevel    = fs.String("log-level", "", "Minimum log level (debug, info, warn, error)")
		logColor    = fs.Bool("log-color", false, "Force colored terminal output")
		logToFile   = fs.Bool("log-to-file", false, "Enable file-based log persistence")
		logDir      = fs.String("log-dir", "", "Directory for log file output")
		idleTimeout = fs.String("idle-timeout", "", "HTTP idle keep-alive timeout")

		dbDriver       = fs.String("db-driver", "", "Database driver")
		dbHost         = fs.String("db-host", "", "Database host")
		dbPort         = fs.String("db-port", "", "Database port")
		dbName         = fs.String("db-name", "", "Database name")
		dbUser         = fs.String("db-user", "", "Database user")
		dbPassword     = fs.String("db-password", "", "Database password")
		databaseURL    = fs.String("database-url", "", "Database URL")
		dbMaxOpenConns = fs.Int("db-max-open-conns", 0, "Max open connections")
		dbMaxIdleConns = fs.Int("db-max-idle-conns", 0, "Max idle connections")

		jwtSecret = fs.String("jwt-secret", "", "Auth JWT secret")
		tokenTTL  = fs.String("jwt-token-ttl", "", "JWT token TTL")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Only override fields if explicitly passed
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "port":
			cfg.Server.Port = *port
		case "host":
			cfg.Server.Host = *host
		case "read-timeout":
			if d, err := time.ParseDuration(*readTimeout); err == nil {
				cfg.Server.ReadTimeout = Duration(d)
			}
		case "read-header-timeout":
			if d, err := time.ParseDuration(*readHeaderTimeout); err == nil {
				cfg.Server.ReadHeaderTimeout = Duration(d)
			}
		case "write-timeout":
			if d, err := time.ParseDuration(*writeTimeout); err == nil {
				cfg.Server.WriteTimeout = Duration(d)
			}
		case "shutdown-timeout":
			if d, err := time.ParseDuration(*shutdownTimeout); err == nil {
				cfg.Server.ShutdownTimeout = Duration(d)
			}
		case "drain-duration":
			if d, err := time.ParseDuration(*drainDuration); err == nil {
				cfg.Server.DrainDuration = Duration(d)
			}
		case "max-header-bytes":
			if sz, err := ParseByteSize(*maxHeaderBytes); err == nil {
				cfg.Server.MaxHeaderBytes = sz
			}
		case "max-body-size":
			if sz, err := ParseByteSize(*maxBodySize); err == nil {
				cfg.Server.MaxBodySize = sz
			}
		case "log-level":
			cfg.Logger.Level = *logLevel
		case "log-color":
			cfg.Logger.ForceColor = *logColor
		case "log-to-file":
			cfg.Logger.ToFile = *logToFile
		case "log-dir":
			cfg.Logger.LogDir = *logDir
		case "idle-timeout":
			if d, err := time.ParseDuration(*idleTimeout); err == nil {
				cfg.Server.IdleTimeout = Duration(d)
			}
		case "db-driver":
			cfg.Database.Driver = *dbDriver
		case "db-host":
			cfg.Database.Host = *dbHost
		case "db-port":
			cfg.Database.Port = *dbPort
		case "db-name":
			cfg.Database.Name = *dbName
		case "db-user":
			cfg.Database.User = *dbUser
		case "db-password":
			cfg.Database.Password = *dbPassword
		case "database-url":
			cfg.Database.DatabaseURL = *databaseURL
		case "db-max-open-conns":
			cfg.Database.MaxOpenConns = *dbMaxOpenConns
		case "db-max-idle-conns":
			cfg.Database.MaxIdleConns = *dbMaxIdleConns
		case "jwt-secret":
			cfg.Auth.JWTSecret = *jwtSecret
		case "jwt-token-ttl":
			if d, err := time.ParseDuration(*tokenTTL); err == nil {
				cfg.Auth.TokenTTL = Duration(d)
			}
		}
	})

	return nil
}
