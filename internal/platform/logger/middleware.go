package logger

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	platformctx "train/internal/platform/context"
)

// middlewareOptions holds the configurable behavior of the HTTP logging
// middleware.
type middlewareOptions struct {
	// suppressProbePaths contains exact URL paths whose successful (< 400)
	// completion logs are suppressed. Failing probes (>= 400) are never
	// suppressed so outages always surface.
	suppressProbePaths map[string]struct{}
}

// Option configures HTTPLoggingMiddleware behavior.
type Option func(*middlewareOptions)

// WithProbeSuppression silences completion logs for successful requests to the
// given paths. It is intended for health-check endpoints such as /livez and
// /readyz that are polled continuously; probe failures (>= 400) are always
// logged at WARN/ERROR so problems remain visible.
func WithProbeSuppression(paths ...string) Option {
	return func(o *middlewareOptions) {
		for _, p := range paths {
			if p = strings.TrimSpace(p); p != "" {
				o.suppressProbePaths[p] = struct{}{}
			}
		}
	}
}

// responseRecorder intercepts the response status code and the number of bytes
// written so they can be recorded in the request completion log after handling.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader captures the status code and prevents double writes.
func (w *responseRecorder) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes sent to the client.
func (w *responseRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// HTTPLoggingMiddleware creates an HTTP middleware that:
//
//  1. Extracts request_id and client_ip from the platform package context
//     (internal/platform/context), with the X-Request-ID header as a final fallback.
//  2. Builds a scoped logger carrying the request identifier and destination, and
//     injects it into the request context via WithContext.
//  3. Routes the log level according to the status code (5xx -> ERROR, 4xx -> WARN,
//     otherwise INFO), measures the duration, and records "http request completed"
//     via LogAttrs (a zero-allocation path for hot-path data).
//
// When WithProbeSuppression is passed, completion logs for the specified health
// check successes are ignored to avoid log bloat, while their failures always
// remain visible.
//
// This middleware must be mounted strictly after the platform package's context
// Middleware to guarantee request_id is available in the context before reaching
// this point.
func HTTPLoggingMiddleware(base *slog.Logger, opts ...Option) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}

	options := middlewareOptions{suppressProbePaths: make(map[string]struct{})}
	for _, opt := range opts {
		opt(&options)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Build a scoped logger carrying the current request attributes
			reqLogger := base.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)

			if meta, ok := platformctx.MetadataFromContext(r.Context()); ok {
				if meta.RequestID != "" {
					reqLogger = reqLogger.With(slog.String("request_id", meta.RequestID))
				}
				if meta.ClientIP != "" {
					reqLogger = reqLogger.With(slog.String("client_ip", meta.ClientIP))
				}
			} else if reqID := r.Header.Get(platformctx.HeaderRequestID); reqID != "" {
				reqLogger = reqLogger.With(slog.String("request_id", reqID))
			}

			ctx := WithContext(r.Context(), reqLogger)
			rec := &responseRecorder{ResponseWriter: w}

			// Execute the handler chain
			next.ServeHTTP(rec, r.WithContext(ctx))

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}

			// Suppress completion logs for the specified healthy probes when requested
			if _, suppress := options.suppressProbePaths[r.URL.Path]; suppress && status < http.StatusBadRequest {
				return
			}

			// Select the log level based on the status code
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			// Log request completion after it finishes, using LogAttrs to avoid memory allocations
			reqLogger.LogAttrs(ctx, level, "http request completed",
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)),
				slog.Int("bytes", rec.bytes),
			)
		})
	}
}
