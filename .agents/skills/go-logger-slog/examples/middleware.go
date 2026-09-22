package examples

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type loggerContextKeyType struct{}

var loggerContextKey = loggerContextKeyType{}

// responseWriterInterceptor intercepts the response status code and the amount of data written
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func (w *responseWriterInterceptor) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriterInterceptor) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}

// generateRequestID generates a unique random ID without external dependencies
func generateRequestID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b)
}

// HTTPLoggingMiddleware creates middleware for request logging and injecting the logger into the context
func HTTPLoggingMiddleware(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = generateRequestID()
			}

			// Build a scoped logger carrying the request context
			reqLogger := baseLogger.With(
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)

			// Add the logger and request ID to the request context
			ctx := context.WithValue(r.Context(), loggerContextKey, reqLogger)
			w.Header().Set("X-Request-ID", reqID)

			interceptor := &responseWriterInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(interceptor, r.WithContext(ctx))

			duration := time.Since(start)

			// Log request completion with execution time and the amount of data written
			level := slog.LevelInfo
			if interceptor.statusCode >= 500 {
				level = slog.LevelError
			} else if interceptor.statusCode >= 400 {
				level = slog.LevelWarn
			}

			reqLogger.LogAttrs(r.Context(), level, "http request completed",
				slog.Int("status", interceptor.statusCode),
				slog.Duration("duration", duration),
				slog.Int("bytes", interceptor.written),
			)
		})
	}
}

// LoggerFromContext retrieves the logger from the context, or safely returns the default logger
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
