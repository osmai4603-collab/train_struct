package postgres

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"train/internal/platform/config"
	platformerr "train/internal/platform/errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestConfig_DSN(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectedDSN string
	}{
		{
			name: "direct database_url takes precedence",
			cfg: Config{
				DatabaseURL: "postgres://custom:secret@db.internal:5433/prod_db?sslmode=require",
			},
			expectedDSN: "postgres://custom:secret@db.internal:5433/prod_db?sslmode=require",
		},
		{
			name: "constructed dsn with all fields",
			cfg: Config{
				Host:     "127.0.0.1",
				Port:     "5432",
				Name:     "app_db",
				User:     "app_user",
				Password: "secret_password",
				SSLMode:  "verify-full",
			},
			expectedDSN: "postgres://app_user:secret_password@127.0.0.1:5432/app_db?sslmode=verify-full",
		},
		{
			name: "constructed dsn with defaults for empty fields",
			cfg:  Config{},
			expectedDSN: "postgres://postgres@localhost:5432/postgres?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.DSN()
			if got != tt.expectedDSN {
				t.Fatalf("expected DSN %q, got %q", tt.expectedDSN, got)
			}
		})
	}
}

func TestConfig_RedactedDSN(t *testing.T) {
	cfg := Config{
		Host:     "localhost",
		Port:     "5432",
		Name:     "testdb",
		User:     "testuser",
		Password: "supersecretpassword",
		SSLMode:  "disable",
	}

	redacted := cfg.RedactedDSN()
	if redacted == "" {
		t.Fatal("redacted DSN should not be empty")
	}
	if stringContains(redacted, "supersecretpassword") {
		t.Fatalf("redacted DSN leaked plaintext password: %s", redacted)
	}
	if !stringContains(redacted, "xxxxx") && !stringContains(redacted, "%2A%2A%2A") && !stringContains(redacted, "testuser") {
		t.Fatalf("redacted DSN should contain masked credentials: %s", redacted)
	}
}

func TestNewConfigFromSettings(t *testing.T) {
	s := config.DatabaseSettings{
		Host:              "db.example.com",
		Port:              "5432",
		Name:              "orders_db",
		User:              "orders_svc",
		Password:          "pwd123",
		MaxOpenConns:      50,
		MinConns:          10,
		MaxConnLifetime:   config.Duration(2 * time.Hour),
		MaxConnIdleTime:   config.Duration(15 * time.Minute),
		HealthCheckPeriod: config.Duration(45 * time.Second),
		SSLMode:           "require",
	}

	cfg := NewConfigFromSettings(s)
	if cfg.Host != "db.example.com" {
		t.Errorf("expected host db.example.com, got %s", cfg.Host)
	}
	if cfg.MaxConns != 50 {
		t.Errorf("expected MaxConns 50, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 10 {
		t.Errorf("expected MinConns 10, got %d", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != 2*time.Hour {
		t.Errorf("expected MaxConnLifetime 2h, got %v", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 15*time.Minute {
		t.Errorf("expected MaxConnIdleTime 15m, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != 45*time.Second {
		t.Errorf("expected HealthCheckPeriod 45s, got %v", cfg.HealthCheckPeriod)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("expected SSLMode require, got %s", cfg.SSLMode)
	}
}

func TestTranslateError(t *testing.T) {
	const op = "repository.SaveUser"

	tests := []struct {
		name         string
		err          error
		expectedCode string
	}{
		{
			name:         "nil returns nil",
			err:          nil,
			expectedCode: "",
		},
		{
			name:         "already platform error is preserved",
			err:          platformerr.NotFound("op", "custom not found", nil),
			expectedCode: platformerr.CodeNotFound,
		},
		{
			name:         "pgx.ErrNoRows translates to NOT_FOUND",
			err:          pgx.ErrNoRows,
			expectedCode: platformerr.CodeNotFound,
		},
		{
			name:         "context.DeadlineExceeded translates to TIMEOUT",
			err:          context.DeadlineExceeded,
			expectedCode: platformerr.CodeTimeout,
		},
		{
			name:         "context.Canceled translates to TIMEOUT",
			err:          context.Canceled,
			expectedCode: platformerr.CodeTimeout,
		},
		{
			name: "unique constraint violation translates to CONFLICT",
			err: &pgconn.PgError{
				Code:           pgerrcode.UniqueViolation,
				ConstraintName: "users_email_key",
				Message:        "duplicate key value violates unique constraint",
			},
			expectedCode: platformerr.CodeConflict,
		},
		{
			name: "foreign key violation translates to INVALID",
			err: &pgconn.PgError{
				Code:           pgerrcode.ForeignKeyViolation,
				ConstraintName: "orders_user_id_fkey",
			},
			expectedCode: platformerr.CodeInvalid,
		},
		{
			name: "not null violation translates to INVALID",
			err: &pgconn.PgError{
				Code:       pgerrcode.NotNullViolation,
				ColumnName: "username",
			},
			expectedCode: platformerr.CodeInvalid,
		},
		{
			name: "check violation translates to INVALID",
			err: &pgconn.PgError{
				Code:           pgerrcode.CheckViolation,
				ConstraintName: "positive_balance",
			},
			expectedCode: platformerr.CodeInvalid,
		},
		{
			name: "serialization failure translates to UNAVAILABLE (retryable)",
			err: &pgconn.PgError{
				Code: pgerrcode.SerializationFailure,
			},
			expectedCode: platformerr.CodeUnavailable,
		},
		{
			name: "deadlock detected translates to UNAVAILABLE (retryable)",
			err: &pgconn.PgError{
				Code: pgerrcode.DeadlockDetected,
			},
			expectedCode: platformerr.CodeUnavailable,
		},
		{
			name: "insufficient privilege translates to FORBIDDEN",
			err: &pgconn.PgError{
				Code: pgerrcode.InsufficientPrivilege,
			},
			expectedCode: platformerr.CodeForbidden,
		},
		{
			name: "unclassified pgerror translates to INTERNAL",
			err: &pgconn.PgError{
				Code:    "XX000",
				Message: "internal server crash",
			},
			expectedCode: platformerr.CodeInternal,
		},
		{
			name:         "generic error translates to INTERNAL",
			err:          errors.New("something bad happened"),
			expectedCode: platformerr.CodeInternal,
		},
		{
			name:         "network error translates to UNAVAILABLE",
			err:          &net.DNSError{Err: "no such host"},
			expectedCode: platformerr.CodeUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateError(op, tt.err)
			if tt.err == nil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected error, got nil")
			}

			gotCode := platformerr.ErrorCode(result)
			if gotCode != tt.expectedCode {
				t.Fatalf("expected code %q, got %q (err: %v)", tt.expectedCode, gotCode, result)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	if IsTransient(nil) {
		t.Error("nil should not be transient")
	}

	deadlock := &pgconn.PgError{Code: pgerrcode.DeadlockDetected}
	if !IsTransient(deadlock) {
		t.Error("deadlock should be transient")
	}

	serialization := &pgconn.PgError{Code: pgerrcode.SerializationFailure}
	if !IsTransient(serialization) {
		t.Error("serialization failure should be transient")
	}

	uniqueViolation := &pgconn.PgError{Code: pgerrcode.UniqueViolation}
	if IsTransient(uniqueViolation) {
		t.Error("unique constraint violation should NOT be transient")
	}

	unavailableErr := platformerr.Unavailable("op", "down", nil)
	if !IsTransient(unavailableErr) {
		t.Error("CodeUnavailable should be transient")
	}
}

func TestConstraintChecks(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:           pgerrcode.UniqueViolation,
		ConstraintName: "users_email_idx",
	}

	if !IsConstraintViolation(pgErr, "users_email_idx") {
		t.Error("expected constraint match for users_email_idx")
	}
	if IsConstraintViolation(pgErr, "other_idx") {
		t.Error("expected false for other_idx")
	}
	if !IsUniqueViolation(pgErr) {
		t.Error("expected unique violation to be true")
	}

	fkErr := &pgconn.PgError{
		Code: pgerrcode.ForeignKeyViolation,
	}
	if !IsForeignKeyViolation(fkErr) {
		t.Error("expected foreign key violation to be true")
	}

	nnErr := &pgconn.PgError{
		Code: pgerrcode.NotNullViolation,
	}
	if !IsNotNullViolation(nnErr) {
		t.Error("expected not null violation to be true")
	}
}

func TestWithRetry(t *testing.T) {
	t.Run("succeeds first attempt without retrying", func(t *testing.T) {
		calls := 0
		err := WithRetry(context.Background(), DefaultRetryConfig(), func(ctx context.Context) error {
			calls++
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got %d", calls)
		}
	})

	t.Run("retries transient error and succeeds", func(t *testing.T) {
		calls := 0
		cfg := RetryConfig{
			MaxRetries:      3,
			InitialInterval: 2 * time.Millisecond,
			MaxInterval:     10 * time.Millisecond,
			Multiplier:      2.0,
		}

		err := WithRetry(context.Background(), cfg, func(ctx context.Context) error {
			calls++
			if calls < 3 {
				return &pgconn.PgError{Code: pgerrcode.SerializationFailure}
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("fails immediately on non-transient error", func(t *testing.T) {
		calls := 0
		cfg := RetryConfig{
			MaxRetries:      3,
			InitialInterval: 2 * time.Millisecond,
		}

		err := WithRetry(context.Background(), cfg, func(ctx context.Context) error {
			calls++
			return &pgconn.PgError{Code: pgerrcode.UniqueViolation}
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 1 {
			t.Fatalf("expected exactly 1 call for non-transient error, got %d", calls)
		}
	})

	t.Run("exhausts max retries on continuous transient error", func(t *testing.T) {
		calls := 0
		cfg := RetryConfig{
			MaxRetries:      2,
			InitialInterval: 2 * time.Millisecond,
			MaxInterval:     5 * time.Millisecond,
			Multiplier:      1.5,
		}

		err := WithRetry(context.Background(), cfg, func(ctx context.Context) error {
			calls++
			return &pgconn.PgError{Code: pgerrcode.DeadlockDetected}
		})

		if err == nil {
			t.Fatal("expected error after max retries")
		}
		if calls != 3 { // 1 initial + 2 retries = 3 calls
			t.Fatalf("expected 3 total calls, got %d", calls)
		}
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately canceled

		err := WithRetry(ctx, DefaultRetryConfig(), func(ctx context.Context) error {
			return nil
		})

		if err == nil {
			t.Fatal("expected error due to canceled context")
		}
		if platformerr.ErrorCode(err) != platformerr.CodeTimeout {
			t.Fatalf("expected timeout code for canceled context, got %s", platformerr.ErrorCode(err))
		}
	})
}

func TestHealthChecker_NilPool(t *testing.T) {
	checker := NewHealthChecker(nil)
	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected error when pool is nil")
	}

	stats := checker.Stats()
	if stats.TotalConns != 0 {
		t.Errorf("expected 0 total conns for nil pool, got %d", stats.TotalConns)
	}

	// Test LogValue does not panic
	val := stats.LogValue()
	if val.Kind() != slog.KindGroup {
		t.Errorf("expected slog GroupValue, got %v", val.Kind())
	}
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
