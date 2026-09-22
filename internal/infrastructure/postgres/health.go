package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker provides health verification and telemetry for the database pool.
type HealthChecker struct {
	pool *pgxpool.Pool
}

// NewHealthChecker constructs a HealthChecker wrapping the given pgxpool.
func NewHealthChecker(pool *pgxpool.Pool) *HealthChecker {
	return &HealthChecker{pool: pool}
}

// Check probes database liveness by executing a ping against the connection pool.
func (h *HealthChecker) Check(ctx context.Context) error {
	const op = "postgres.HealthChecker.Check"

	if h.pool == nil {
		return TranslateError(op, errors.New("database pool is uninitialized"))
	}

	if err := h.pool.Ping(ctx); err != nil {
		return TranslateError(op, err)
	}

	return nil
}

// Ping satisfies the health.Pinger contract (internal/infrastructure/health)
// by delegating to Check. This lets the HealthChecker be plugged directly into
// readiness probes without an adapter.
func (h *HealthChecker) Ping(ctx context.Context) error {
	return h.Check(ctx)
}

// PoolStats provides a clean snapshot of connection pool telemetry.
type PoolStats struct {
	TotalConns        int32         `json:"total_conns"`
	AcquiredConns     int32         `json:"acquired_conns"`
	IdleConns         int32         `json:"idle_conns"`
	MaxConns          int32         `json:"max_conns"`
	ConstructingConns int32         `json:"constructing_conns"`
	EmptyAcquireCount int64         `json:"empty_acquire_count"`
	AcquireCount      int64         `json:"acquire_count"`
	AcquireDuration   time.Duration `json:"acquire_duration"`
}

// LogValue implements slog.LogValuer to allow safe and structured logging of pool metrics.
func (s PoolStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("total_conns", int(s.TotalConns)),
		slog.Int("acquired_conns", int(s.AcquiredConns)),
		slog.Int("idle_conns", int(s.IdleConns)),
		slog.Int("max_conns", int(s.MaxConns)),
		slog.Int64("empty_acquires", s.EmptyAcquireCount),
		slog.Int64("acquire_count", s.AcquireCount),
		slog.Duration("acquire_duration", s.AcquireDuration),
	)
}

// Stats returns a snapshot of current connection pool metrics.
func (h *HealthChecker) Stats() PoolStats {
	if h.pool == nil {
		return PoolStats{}
	}

	stat := h.pool.Stat()
	return PoolStats{
		TotalConns:        stat.TotalConns(),
		AcquiredConns:     stat.AcquiredConns(),
		IdleConns:         stat.IdleConns(),
		MaxConns:          stat.MaxConns(),
		ConstructingConns: stat.ConstructingConns(),
		EmptyAcquireCount: stat.EmptyAcquireCount(),
		AcquireCount:      stat.AcquireCount(),
		AcquireDuration:   stat.AcquireDuration(),
	}
}
