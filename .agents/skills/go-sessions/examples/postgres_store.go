package examples

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresSessionStore محرك تخزين الجلسات المتين في PostgreSQL
type PostgresSessionStore struct {
	db *sql.DB
}

func NewPostgresSessionStore(db *sql.DB) *PostgresSessionStore {
	return &PostgresSessionStore{db: db}
}

// Get يسترجع الجلسة ويتأكد من عدم انتهاء صلاحيتها
func (s *PostgresSessionStore) Get(ctx context.Context, tokenHash string) (*SessionData, error) {
	query := `
		SELECT user_id, roles, csrf_token, created_at, last_active_at, absolute_exp, data
		FROM auth_sessions
		WHERE token_hash = $1 AND expires_at > NOW();
	`

	var (
		data       SessionData
		rolesArray []string
		metaJSON   []byte
	)

	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&data.UserID,
		&rolesArray,
		&data.CSRFToken,
		&data.CreatedAt,
		&data.LastActiveAt,
		&data.AbsoluteExp,
		&metaJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("postgres get session failed: %w", err)
	}

	data.Roles = rolesArray
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &data.Metadata)
	}

	return &data, nil
}

// Set يحفظ أو يُحدّث الجلسة عبر عملية دمج ذرية (UPSERT)
func (s *PostgresSessionStore) Set(ctx context.Context, tokenHash string, data *SessionData, ttl time.Duration) error {
	query := `
		INSERT INTO auth_sessions (
			token_hash, user_id, roles, csrf_token, created_at, last_active_at, absolute_exp, expires_at, data
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		ON CONFLICT (token_hash) DO UPDATE SET
			last_active_at = EXCLUDED.last_active_at,
			expires_at = EXCLUDED.expires_at,
			data = EXCLUDED.data;
	`

	metaJSON, err := json.Marshal(data.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	expiresAt := time.Now().UTC().Add(ttl)

	_, err = s.db.ExecContext(ctx, query,
		tokenHash,
		data.UserID,
		data.Roles,
		data.CSRFToken,
		data.CreatedAt,
		data.LastActiveAt,
		data.AbsoluteExp,
		expiresAt,
		metaJSON,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return fmt.Errorf("pg error code %s: %w", pgErr.Code, err)
		}
		return fmt.Errorf("postgres set session failed: %w", err)
	}

	return nil
}

// Delete يحذف جلسة معينة
func (s *PostgresSessionStore) Delete(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM auth_sessions WHERE token_hash = $1;`
	_, err := s.db.ExecContext(ctx, query, tokenHash)
	return err
}

// DeleteByUserID يحذف كافة جلسات المستخدم
func (s *PostgresSessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM auth_sessions WHERE user_id = $1;`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// CleanupExpired يحذف الجلسات المنتهية (يُشغل دورياً عبر Background Worker)
func (s *PostgresSessionStore) CleanupExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM auth_sessions WHERE expires_at <= NOW();`
	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
