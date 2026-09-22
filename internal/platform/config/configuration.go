package config

import (
	"fmt"
	"net"
	"time"
)

// =============================================================================
// Complete Configuration Schema
// =============================================================================

// Configuration defines the complete operational parameters for the service.
type Configuration struct {
	Server   ServerSettings   `json:"server"`
	Logger   LoggerSettings   `json:"logger"`
	Database DatabaseSettings `json:"database"`
	Auth     AuthSettings     `json:"auth"`
}

// ServerSettings encapsulates HTTP listener and timeout parameters.
type ServerSettings struct {
	Host              string   `json:"host" env:"HOST" default:"0.0.0.0" desc:"HTTP server binding host interface"`
	Port              string   `json:"port" env:"PORT" default:"8080" desc:"HTTP server TCP listen port"`
	ReadTimeout       Duration `json:"read_timeout" env:"READ_TIMEOUT" default:"5s" desc:"Maximum duration for reading the full request body"`
	ReadHeaderTimeout Duration `json:"read_header_timeout" env:"READ_HEADER_TIMEOUT" default:"2s" desc:"Maximum duration for reading request headers (slowloris protection)"`
	WriteTimeout      Duration `json:"write_timeout" env:"WRITE_TIMEOUT" default:"10s" desc:"Maximum duration for writing HTTP response"`
	IdleTimeout       Duration `json:"idle_timeout" env:"IDLE_TIMEOUT" default:"120s" desc:"Idle keep-alive duration before closing a connection"`
	ShutdownTimeout   Duration `json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" default:"15s" desc:"Maximum duration allowed for graceful server shutdown"`
	DrainDuration     Duration `json:"drain_duration" env:"DRAIN_DURATION" default:"5s" desc:"Initial pause to allow load balancer health check failure propagation"`
	MaxHeaderBytes    ByteSize `json:"max_header_bytes" env:"MAX_HEADER_BYTES" default:"1MB" desc:"Maximum permitted HTTP request header bytes"`
	MaxBodySize       ByteSize `json:"max_body_size" env:"MAX_BODY_SIZE" default:"10MB" desc:"Maximum permitted HTTP request body size"`
}

// LoggerSettings encapsulates structured logging behavior: minimum level,
// terminal color forcing, and optional file persistence targets.
type LoggerSettings struct {
	Level      string `json:"level" env:"LOG_LEVEL" default:"info" desc:"Minimum log level (debug, info, warn, error)"`
	ForceColor bool   `json:"force_color" env:"LOG_COLOR" default:"false" desc:"Force colored terminal output even when not attached to a TTY"`
	ToFile     bool   `json:"to_file" env:"LOG_TO_FILE" default:"true" desc:"Enable file-based JSON log persistence"`
	LogDir     string `json:"log_dir" env:"LOG_DIR" default:"logs" desc:"Directory for log file output (app.log, error.log)"`
}

// DatabaseSettings encapsulates persistence layer connection and pool options.
type DatabaseSettings struct {
	Driver            string   `json:"driver" env:"DB_DRIVER" default:"postgres" desc:"Database driver (supported: 'postgres', 'memory')"`
	Host              string   `json:"host" env:"DB_HOST" default:"localhost" desc:"Database server host or IP"`
	Port              string   `json:"port" env:"DB_PORT" default:"5432" desc:"Database server TCP port"`
	Name              string   `json:"name" env:"DB_NAME" default:"train_db" desc:"Target database name"`
	User              string   `json:"user" env:"DB_USER" default:"train_user" desc:"Database authentication user"`
	Password          string   `json:"password" env:"DB_PASSWORD" default:"" desc:"Database authentication password"`
	DatabaseURL       string   `json:"database_url" env:"DATABASE_URL" default:"" desc:"Direct database connection URI override"`
	MaxOpenConns      int      `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"25" desc:"Maximum open connections pool size"`
	MaxIdleConns      int      `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"25" desc:"Maximum idle connections pool size"`
	MinConns          int      `json:"min_conns" env:"DB_MIN_CONNS" default:"5" desc:"Minimum idle connections kept alive in pool"`
	MaxConnLifetime   Duration `json:"max_conn_lifetime" env:"DB_MAX_CONN_LIFETIME" default:"1h" desc:"Maximum connection lifetime before recycling"`
	MaxConnIdleTime   Duration `json:"max_conn_idle_time" env:"DB_MAX_CONN_IDLE_TIME" default:"5m" desc:"Maximum connection idle duration before pruning"`
	HealthCheckPeriod Duration `json:"health_check_period" env:"DB_HEALTH_CHECK_PERIOD" default:"30s" desc:"Periodic background connection health probe frequency"`
	SSLMode           string   `json:"ssl_mode" env:"DB_SSL_MODE" default:"disable" desc:"PostgreSQL SSL connection mode"`
}

// AuthSettings encapsulates authentication and cryptographic parameters.
type AuthSettings struct {
	JWTSecret string   `json:"jwt_secret" env:"JWT_SECRET" default:"default-development-jwt-secret-key-32-chars-min" desc:"Cryptographic secret key for signing JWT tokens"`
	TokenTTL  Duration `json:"token_ttl" env:"JWT_TOKEN_TTL" default:"24h" desc:"Time-to-live duration for JWT authentication tokens"`
}

// DefaultConfiguration returns a fully populated, production-safe fallback configuration.
func DefaultConfiguration() *Configuration {
	return &Configuration{
		Server: ServerSettings{
			Host:              "0.0.0.0",
			Port:              "8080",
			ReadTimeout:       Duration(5 * time.Second),
			ReadHeaderTimeout: Duration(2 * time.Second),
			WriteTimeout:      Duration(10 * time.Second),
			IdleTimeout:       Duration(120 * time.Second),
			ShutdownTimeout:   Duration(15 * time.Second),
			DrainDuration:     Duration(5 * time.Second),
			MaxHeaderBytes:    1 * Megabyte,
			MaxBodySize:       10 * Megabyte,
		},
		Logger: LoggerSettings{
			Level:      "info",
			ForceColor: false,
			ToFile:     true,
			LogDir:     "logs",
		},
		Database: DatabaseSettings{
			Driver:            "postgres",
			Host:              "localhost",
			Port:              "5432",
			Name:              "train_db",
			User:              "train_user",
			Password:          "",
			DatabaseURL:       "",
			MaxOpenConns:      25,
			MaxIdleConns:      25,
			MinConns:          5,
			MaxConnLifetime:   Duration(1 * time.Hour),
			MaxConnIdleTime:   Duration(5 * time.Minute),
			HealthCheckPeriod: Duration(30 * time.Second),
			SSLMode:           "disable",
		},
		Auth: AuthSettings{
			JWTSecret: "default-development-jwt-secret-key-32-chars-min",
			TokenTTL:  Duration(24 * time.Hour),
		},
	}
}

// Clone creates a deep copy of the configuration structure.
func (c *Configuration) Clone() *Configuration {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// HTTPAddr returns the formatted host:port address for the HTTP server.
func (c *Configuration) HTTPAddr() string {
	return c.Server.Address()
}

// Address returns the host:port listening address using net.JoinHostPort
// so IPv6 literals are bracketed correctly (e.g. "[::1]:8080").
func (s ServerSettings) Address() string {
	host := s.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := s.Port
	if port == "" {
		port = "8080"
	}
	return net.JoinHostPort(host, port)
}

// DSN returns the PostgreSQL connection string. DatabaseURL takes precedence if set.
func (c *Configuration) DSN() string {
	if c.Database.DatabaseURL != "" {
		return c.Database.DatabaseURL
	}
	sslMode := c.Database.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	if c.Database.Password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Name, sslMode)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s",
		c.Database.User, c.Database.Host, c.Database.Port, c.Database.Name, sslMode)
}
