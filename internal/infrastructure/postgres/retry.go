package postgres

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryConfig configures the exponential backoff algorithm with full jitter.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts after the initial failure.
	MaxRetries int

	// InitialInterval is the base wait duration before the first retry.
	InitialInterval time.Duration

	// MaxInterval is the upper bound cap on wait duration.
	MaxInterval time.Duration

	// Multiplier is the backoff factor applied on subsequent attempts.
	Multiplier float64
}

// DefaultRetryConfig returns production-safe fallback retry parameters.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
	}
}

// RetryableFn is a function signature that accepts context and can be retried.
type RetryableFn func(ctx context.Context) error

// WithRetry executes the supplied operation and retries only if the returned error
// is transient (e.g. SerializationFailure, Deadlock, temporary connection drops).
//
// Full Jitter Formula:
//
//	temp = min(MaxInterval, InitialInterval * (Multiplier ^ attempt))
//	sleep = rand(0, temp)
func WithRetry(ctx context.Context, cfg RetryConfig, fn RetryableFn) error {
	const op = "postgres.WithRetry"

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialInterval <= 0 {
		cfg.InitialInterval = 50 * time.Millisecond
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 2 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	var lastErr error
	backoff := float64(cfg.InitialInterval)

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Respect context cancellation before executing
		if err := ctx.Err(); err != nil {
			return TranslateError(op, err)
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Only retry if error is transient and attempts remain
		if !IsTransient(lastErr) || attempt == cfg.MaxRetries {
			return lastErr
		}

		// Calculate sleep duration with Full Jitter
		capInterval := float64(cfg.MaxInterval)
		currentMax := backoff
		if currentMax > capInterval {
			currentMax = capInterval
		}

		// Jitter between 0 and currentMax
		sleepDuration := time.Duration(rand.Float64() * currentMax)

		// Update backoff for next iteration
		backoff *= cfg.Multiplier

		// Wait for sleep duration or context cancellation
		select {
		case <-ctx.Done():
			return TranslateError(op, ctx.Err())
		case <-time.After(sleepDuration):
		}
	}

	return lastErr
}
