package examples

import (
	"context"
)

// userIDKey is a private, unexported type of zero bytes to prevent any collisions between packages
type userIDKey struct{}

// WithUserID injects the user ID in a type-safe way
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserIDFromContext extracts the user ID completely safely without causing a panic
func UserIDFromContext(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(userIDKey{}).(string)
	return val, ok
}
