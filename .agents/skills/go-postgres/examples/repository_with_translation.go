package examples

import (
	"context"
	"time"

	"train/internal/infrastructure/postgres"
	platformerr "train/internal/platform/errors"
)

// User represents a domain model entity.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
}

// UserRepository demonstrates production repository implementation using postgres.DBTX
// and comprehensive failure domain isolation via postgres.TranslateError.
type UserRepository struct {
	db postgres.DBTX
}

// NewUserRepository constructs a repository bound to a DBTX executor (Pool or Tx).
func NewUserRepository(db postgres.DBTX) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user record into PostgreSQL and translates driver errors.
func (r *UserRepository) Create(ctx context.Context, u *User) error {
	const op = "UserRepository.Create"

	query := `
		INSERT INTO users (id, email, full_name, created_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Exec(ctx, query, u.ID, u.Email, u.FullName, u.CreatedAt)
	if err != nil {
		// Custom handling for specific constraints before default translation
		if postgres.IsConstraintViolation(err, "users_email_key") {
			return platformerr.Conflict(op, "email address is already registered", err)
		}
		return postgres.TranslateError(op, err)
	}

	return nil
}

// GetByID retrieves a single user by primary key, mapping pgx.ErrNoRows to CodeNotFound.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	const op = "UserRepository.GetByID"

	query := `
		SELECT id, email, full_name, created_at
		FROM users
		WHERE id = $1
	`

	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.FullName, &u.CreatedAt)
	if err != nil {
		// Automatically converts pgx.ErrNoRows to platformerr.CodeNotFound (404)
		return nil, postgres.TranslateError(op, err)
	}

	return &u, nil
}
