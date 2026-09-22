package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// =============================================================================
// Complete 5-Layer Configuration Resolution Pipeline
// =============================================================================

type Configuration struct {
	Server   ServerSettings   `json:"server"`
	Database DatabaseSettings `json:"database"`
	Auth     AuthSettings     `json:"auth"`
}

type ServerSettings struct {
	Port            string   `json:"port" env:"PORT" default:"8080"`
	ReadTimeout     Duration `json:"read_timeout" env:"READ_TIMEOUT" default:"5s"`
	WriteTimeout    Duration `json:"write_timeout" env:"WRITE_TIMEOUT" default:"10s"`
	ShutdownTimeout Duration `json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" default:"15s"`
	DrainDuration   Duration `json:"drain_duration" env:"DRAIN_DURATION" default:"5s"`
}

type DatabaseSettings struct {
	Driver   string `json:"driver" env:"DB_DRIVER" default:"postgres"`
	Host     string `json:"host" env:"DB_HOST" default:"localhost"`
	Port     string `json:"port" env:"DB_PORT" default:"5432"`
	Name     string `json:"name" env:"DB_NAME" default:"train_db"`
	User     string `json:"user" env:"DB_USER" default:"train_user"`
	Password string `json:"password" env:"DB_PASSWORD"`
}

type AuthSettings struct {
	JWTSecret string `json:"jwt_secret" env:"JWT_SECRET"`
}

// Defaults returns the hardened fallback configuration.
func Defaults() *Configuration {
	return &Configuration{
		Server: ServerSettings{
			Port:            "8080",
			ReadTimeout:     Duration(5 * time.Second),
			WriteTimeout:    Duration(10 * time.Second),
			ShutdownTimeout: Duration(15 * time.Second),
			DrainDuration:   Duration(5 * time.Second),
		},
		Database: DatabaseSettings{
			Driver: "postgres",
			Host:   "localhost",
			Port:   "5432",
			Name:   "train_db",
			User:   "train_user",
		},
		Auth: AuthSettings{
			JWTSecret: "change-this-default-secret-in-production-now",
		},
	}
}

// Loader executes the complete 5-layer resolution pipeline.
type Loader struct {
	filePath  string
	cliArgs   []string
	overrides []func(*Configuration)
}

func NewLoader(filePath string) *Loader {
	return &Loader{
		filePath:  filePath,
		cliArgs:   os.Args[1:],
		overrides: make([]func(*Configuration), 0),
	}
}

func (l *Loader) WithCLI(args []string) *Loader {
	l.cliArgs = args
	return l
}

func (l *Loader) WithOverride(fn func(*Configuration)) *Loader {
	if fn != nil {
		l.overrides = append(l.overrides, fn)
	}
	return l
}

// Load compiles the configuration in strict 5-layer priority order.
func (l *Loader) Load() (*Configuration, error) {
	// Layer 1: Start with hardcoded defaults
	cfg := Defaults()

	// Layer 2: Sparse JSON config file on disk (if present)
	if l.filePath != "" {
		if data, err := os.ReadFile(filepath.Clean(l.filePath)); err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config file %s: %w", l.filePath, err)
			}
		}
	}

	// Layer 3: Environment Variable overrides (canonical + aliases + file-based secrets)
	if val := os.Getenv("PORT"); val != "" {
		cfg.Server.Port = val
	}
	if val := os.Getenv("DB_HOST"); val != "" {
		cfg.Database.Host = val
	}
	if val := os.Getenv("DB_PASSWORD"); val != "" {
		cfg.Database.Password = val
	} else if file := os.Getenv("DB_PASSWORD_FILE"); file != "" {
		if b, err := os.ReadFile(filepath.Clean(file)); err == nil {
			cfg.Database.Password = string(b)
		}
	}
	if val := os.Getenv("JWT_SECRET"); val != "" {
		cfg.Auth.JWTSecret = val
	} else if file := os.Getenv("JWT_SECRET_FILE"); file != "" {
		if b, err := os.ReadFile(filepath.Clean(file)); err == nil {
			cfg.Auth.JWTSecret = string(b)
		}
	}

	// Layer 4: Command-Line Flags (CLI)
	for i := 0; i < len(l.cliArgs); i++ {
		arg := l.cliArgs[i]
		if arg == "--port" && i+1 < len(l.cliArgs) {
			cfg.Server.Port = l.cliArgs[i+1]
			i++
		} else if arg == "--db-host" && i+1 < len(l.cliArgs) {
			cfg.Database.Host = l.cliArgs[i+1]
			i++
		}
	}

	// Layer 5: Runtime Programmatic Overrides
	for _, fn := range l.overrides {
		if fn != nil {
			fn(cfg)
		}
	}

	// Post-resolution: Fail-Fast Multi-Error Validation
	validator := NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed:\n%w", err)
	}

	return cfg, nil
}
