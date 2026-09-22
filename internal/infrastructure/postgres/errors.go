package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"

	platformerr "train/internal/platform/errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TranslateError isolates the database failure domain by mapping driver-specific
// PostgreSQL errors (pgx and pgconn.PgError) into typed, domain-safe platform errors.
//
// If err is nil, it returns nil immediately.
// If err is already a *platformerr.Error, it is returned untouched to prevent double-wrapping.
func TranslateError(op string, err error) error {
	if err == nil {
		return nil
	}

	// Prevent re-wrapping already translated platform errors
	var pErr *platformerr.Error
	if errors.As(err, &pErr) {
		return err
	}

	// 1. Check for Context timeouts and cancellations
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return platformerr.Timeout(op, "database operation timed out or canceled", err)
	}

	// 2. Check for pgx.ErrNoRows -> NOT_FOUND (404)
	if errors.Is(err, pgx.ErrNoRows) {
		return platformerr.NotFound(op, "requested database record was not found", err)
	}

	// 3. Inspect PostgreSQL wire protocol errors (SQLSTATE)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation: // 23505 -> CONFLICT (409)
			msg := fmt.Sprintf("database unique constraint violation (constraint: %s)", pgErr.ConstraintName)
			return platformerr.Conflict(op, msg, err)

		case pgerrcode.ForeignKeyViolation: // 23503 -> INVALID (400)
			msg := fmt.Sprintf("database foreign key reference violation (constraint: %s)", pgErr.ConstraintName)
			return platformerr.Invalid(op, msg, err)

		case pgerrcode.NotNullViolation: // 23502 -> INVALID (400)
			msg := fmt.Sprintf("database column cannot be null (column: %s)", pgErr.ColumnName)
			return platformerr.Invalid(op, msg, err)

		case pgerrcode.CheckViolation: // 23514 -> INVALID (400)
			msg := fmt.Sprintf("database check constraint violation (constraint: %s)", pgErr.ConstraintName)
			return platformerr.Invalid(op, msg, err)

		case pgerrcode.NumericValueOutOfRange, // 22003
			pgerrcode.StringDataRightTruncationDataException, // 22001
			pgerrcode.InvalidTextRepresentation:             // 22P02
			return platformerr.Invalid(op, "invalid database data format or value range", err)

		case pgerrcode.SerializationFailure: // 40001 -> UNAVAILABLE (503 retryable)
			return platformerr.Unavailable(op, "transaction serialization failure; safe to retry", err)

		case pgerrcode.DeadlockDetected: // 40P01 -> UNAVAILABLE (503 retryable)
			return platformerr.Unavailable(op, "transaction deadlock detected; safe to retry", err)

		case pgerrcode.AdminShutdown, // 57P01
			pgerrcode.CrashShutdown,    // 57P02
			pgerrcode.CannotConnectNow, // 57P03
			pgerrcode.QueryCanceled:    // 57014
			return platformerr.Unavailable(op, "database server is temporarily unavailable", err)

		case pgerrcode.InsufficientPrivilege: // 42501 -> FORBIDDEN (403)
			return platformerr.Forbidden(op, "database permission denied", err)

		default:
			// Unclassified SQLSTATE errors are classified as internal infrastructure failures
			return platformerr.Internal(op, fmt.Sprintf("database error [sqlstate: %s]: %s", pgErr.Code, pgErr.Message), err)
		}
	}

	// 4. Check for network / socket / connectivity failures
	var netErr net.Error
	if errors.As(err, &netErr) {
		return platformerr.Unavailable(op, "database network connectivity failure", err)
	}

	// 5. Default fallback to Internal (500)
	return platformerr.Internal(op, "unexpected internal database failure", err)
}

// ExtractPgError extracts the underlying *pgconn.PgError if present.
func ExtractPgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// IsTransient reports whether the given error represents a transient condition
// that can be safely retried (e.g. Deadlock 40P01, Serialization Failure 40001,
// or temporary network/connection drops).
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	// Check underlying pgconn.PgError
	if pgErr, ok := ExtractPgError(err); ok {
		switch pgErr.Code {
		case pgerrcode.SerializationFailure, // 40001
			pgerrcode.DeadlockDetected,    // 40P01
			pgerrcode.AdminShutdown,       // 57P01
			pgerrcode.CrashShutdown,       // 57P02
			pgerrcode.CannotConnectNow,    // 57P03
			pgerrcode.ConnectionFailure,   // 08006
			pgerrcode.ConnectionDoesNotExist, // 08003
			pgerrcode.ConnectionException: // 08000
			return true
		}
	}

	// Check if translated platform error is UNAVAILABLE
	if platformerr.ErrorCode(err) == platformerr.CodeUnavailable {
		return true
	}

	// Check network temporary errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return false
}

// IsConstraintViolation checks whether err is a PostgreSQL constraint error matching the constraintName.
func IsConstraintViolation(err error, constraintName string) bool {
	if pgErr, ok := ExtractPgError(err); ok {
		return pgErr.ConstraintName == constraintName
	}
	return false
}

// IsUniqueViolation checks whether err is a PostgreSQL unique constraint violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	if pgErr, ok := ExtractPgError(err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation
	}
	return platformerr.ErrorCode(err) == platformerr.CodeConflict
}

// IsForeignKeyViolation checks whether err is a PostgreSQL foreign key violation (SQLSTATE 23503).
func IsForeignKeyViolation(err error) bool {
	if pgErr, ok := ExtractPgError(err); ok {
		return pgErr.Code == pgerrcode.ForeignKeyViolation
	}
	return false
}

// IsNotNullViolation checks whether err is a PostgreSQL NOT NULL violation (SQLSTATE 23502).
func IsNotNullViolation(err error) bool {
	if pgErr, ok := ExtractPgError(err); ok {
		return pgErr.Code == pgerrcode.NotNullViolation
	}
	return false
}
