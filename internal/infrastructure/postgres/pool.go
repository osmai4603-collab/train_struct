package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"train/internal/platform/config"
	platformerr "train/internal/platform/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config encapsulates configuration parameters for establishing and tuning
// the pgx connection pool.
type Config struct {
	DatabaseURL       string
	Host              string
	Port              string
	Name              string
	User              string
	Password          string
	SSLMode           string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// NewConfigFromSettings extracts a typed Config from platform DatabaseSettings.
func NewConfigFromSettings(s config.DatabaseSettings) Config {
	maxConns := int32(25)
	if s.MaxOpenConns > 0 {
		maxConns = int32(s.MaxOpenConns)
	}

	minConns := int32(5)
	if s.MinConns >= 0 {
		minConns = int32(s.MinConns)
	}

	maxLifetime := 1 * time.Hour
	if s.MaxConnLifetime.Duration() > 0 {
		maxLifetime = s.MaxConnLifetime.Duration()
	}

	maxIdleTime := 5 * time.Minute
	if s.MaxConnIdleTime.Duration() > 0 {
		maxIdleTime = s.MaxConnIdleTime.Duration()
	}

	healthPeriod := 30 * time.Second
	if s.HealthCheckPeriod.Duration() > 0 {
		healthPeriod = s.HealthCheckPeriod.Duration()
	}

	sslMode := s.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return Config{
		DatabaseURL:       s.DatabaseURL,
		Host:              s.Host,
		Port:              s.Port,
		Name:              s.Name,
		User:              s.User,
		Password:          s.Password,
		SSLMode:           sslMode,
		MaxConns:          maxConns,
		MinConns:          minConns,
		MaxConnLifetime:   maxLifetime,
		MaxConnIdleTime:   maxIdleTime,
		HealthCheckPeriod: healthPeriod,
	}
}

// DSN generates a PostgreSQL connection string. If DatabaseURL is provided,
// it is returned directly; otherwise, a structured DSN is constructed.
func (c Config) DSN() string {
	if strings.TrimSpace(c.DatabaseURL) != "" {
		return c.DatabaseURL
	}

	host := c.Host
	if host == "" {
		host = "localhost"
	}
	port := c.Port
	if port == "" {
		port = "5432"
	}
	name := c.Name
	if name == "" {
		name = "postgres"
	}
	user := c.User
	if user == "" {
		user = "postgres"
	}
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	var userInfo *url.Userinfo
	if c.Password != "" {
		userInfo = url.UserPassword(user, c.Password)
	} else {
		userInfo = url.User(user)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     userInfo,
		Host:     fmt.Sprintf("%s:%s", host, port),
		Path:     name,
		RawQuery: fmt.Sprintf("sslmode=%s", url.QueryEscape(sslMode)),
	}

	return u.String()
}

// RedactedDSN returns the connection URI with the credentials safely masked for logging.
func (c Config) RedactedDSN() string {
	dsn := c.DSN()
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "postgres://[malformed-dsn]"
	}
	return parsed.Redacted()
}

// NewPool initializes, tunes, and verifies a pgxpool.Pool instance.
// It verifies connectivity via Ping before returning to guarantee fail-fast behavior.
func NewPool(ctx context.Context, cfg Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	const op = "postgres.NewPool"

	if logger == nil {
		logger = slog.Default()
	}

	connStr := cfg.DSN()
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, platformerr.E(op, platformerr.CodeInvalid,
			fmt.Sprintf("failed to parse connection string: %v", err), err)
	}

	// Apply connection pool tuning invariants
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns >= 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	logger.InfoContext(ctx, "initializing postgres connection pool",
		slog.String("op", op),
		slog.String("target", cfg.RedactedDSN()),
		slog.Int("max_conns", int(poolConfig.MaxConns)),
		slog.Int("min_conns", int(poolConfig.MinConns)),
		slog.Duration("max_conn_lifetime", poolConfig.MaxConnLifetime),
		slog.Duration("max_conn_idle_time", poolConfig.MaxConnIdleTime),
		slog.Duration("health_check_period", poolConfig.HealthCheckPeriod),
	)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, platformerr.E(op, platformerr.CodeUnavailable,
			fmt.Sprintf("failed to create connection pool: %v", err), err)
	}

	// Fail-fast ping verification
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, platformerr.E(op, platformerr.CodeUnavailable,
			fmt.Sprintf("failed to ping database: %v", err), err)
	}

	logger.InfoContext(ctx, "postgres connection pool established successfully",
		slog.String("op", op),
	)

	return pool, nil
}

// ClosePool closes the pool gracefully and logs the shutdown event.
func ClosePool(pool *pgxpool.Pool, logger *slog.Logger) {
	if pool == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("closing postgres connection pool")
	pool.Close()
	logger.Info("postgres connection pool closed")
}
