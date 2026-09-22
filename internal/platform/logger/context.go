package logger

import (
	stdctx "context"
	"log/slog"
)

// loggerContextKey is an unexported zero-sized type that prevents any collisions
// in context keys.
type loggerContextKey struct{}

// WithContext injects a scoped logger into the request context so it can be
// retrieved from any downstream layer without being passed around manually.
func WithContext(ctx stdctx.Context, l *slog.Logger) stdctx.Context {
	if ctx == nil || l == nil {
		return ctx
	}
	return stdctx.WithValue(ctx, loggerContextKey{}, l)
}

// FromContext retrieves the scoped logger from the request context.
// If no logger has been injected, it safely returns slog.Default() as a fallback
// so request-handling layers never fail when given a bare context.
func FromContext(ctx stdctx.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
