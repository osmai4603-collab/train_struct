package examples

import (
	"context"
	"database/sql"
	"errors"

	platformerr "train/internal/platform/errors"
)

// User represents the user entity in the domain
type User struct {
	ID    string
	Email string
}

// MockUserRepo is a repository that demonstrates how to translate SQL errors into domain errors
type MockUserRepo struct {
	db *sql.DB
}

// FindUser looks up the user and translates storage errors while preventing SQL details from leaking
func (r *MockUserRepo) FindUser(ctx context.Context, id string) (*User, error) {
	const op = "repository.MockUserRepo.FindUser"

	var u User
	err := r.db.QueryRowContext(ctx, "SELECT id, email FROM users WHERE id = $1", id).Scan(&u.ID, &u.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// ✅ Translate into NOT_FOUND while hiding the SQL query details from the client
			return nil, platformerr.NotFound(op, "user not found", err)
		}
		// ✅ Wrap as an internal server error
		return nil, platformerr.Internal(op, "failed to query user database", err)
	}

	return &u, nil
}
