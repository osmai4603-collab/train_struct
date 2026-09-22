package lifecycle

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// =============================================================================
// Phase 7: Graceful Shutdown
// =============================================================================
// Use server.Shutdown(ctx) — NEVER server.Close().
//
// Shutdown() does three things in order:
//   1. Closes all open listeners (stops accepting new connections)
//   2. Closes all idle connections
//   3. Waits for active connections to become idle, then closes them
//
// Official source: https://pkg.go.dev/net/http#Server.Shutdown
// =============================================================================

// GracefulShutdown performs a graceful server shutdown with a timeout.
//
// Parameters:
//   - srv: the HTTP server to shut down
//   - timeout: maximum time to wait for in-flight requests (5-30 seconds recommended)
//   - logger: structured logger for shutdown events
//
// Returns an error if shutdown does not complete within the timeout.
func GracefulShutdown(srv *http.Server, timeout time.Duration, logger *slog.Logger) error {
	// Create a context with a deadline.
	// If in-flight requests don't complete within this timeout,
	// Shutdown will return the context's error (context.DeadlineExceeded).
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger.Info("graceful shutdown started",
		"timeout", timeout.String(),
	)

	// Shutdown gracefully shuts down the server:
	//
	// 1. Closes all open listeners → no new connections accepted
	// 2. Closes idle connections → frees resources immediately
	// 3. Waits for active requests to complete → no dropped requests
	//
	// If the context deadline is reached before all requests complete,
	// Shutdown returns ctx.Err() (usually context.DeadlineExceeded).
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed — forcing close",
			"error", err,
		)
		// As a last resort, force-close everything.
		// This WILL drop in-flight requests.
		srv.Close()
		return err
	}

	logger.Info("graceful shutdown completed — all requests finished")
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// IMPORTANT: Hijacked Connections (WebSockets)
// ─────────────────────────────────────────────────────────────────────────────
//
// From the official docs:
//   "Shutdown does not attempt to close nor wait for hijacked connections
//    such as WebSockets. The caller of Shutdown should separately notify
//    such long-lived connections of shutdown and wait for them to close,
//    if desired."
//
// If your server uses WebSockets or any hijacked connections, you must:
//   1. Keep a registry of active hijacked connections
//   2. Notify them to close (e.g., send a close frame)
//   3. Wait for them to close before exiting
//
// Example:
//
//   type HijackedConnRegistry struct {
//       mu    sync.Mutex
//       conns map[string]net.Conn
//   }
//
//   func (r *HijackedConnRegistry) CloseAll() {
//       r.mu.Lock()
//       defer r.mu.Unlock()
//       for id, conn := range r.conns {
//           conn.Close()
//           delete(r.conns, id)
//       }
//   }
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Shutdown vs Close — Key Differences
// ─────────────────────────────────────────────────────────────────────────────
//
// | Method     | In-flight requests | Idle connections | Listeners |
// |:-----------|:-------------------|:-----------------|:----------|
// | Shutdown() | Waits for them     | Closes           | Closes    |
// | Close()    | DROPS them         | Closes           | Closes    |
//
// ALWAYS use Shutdown() in production.
// Use Close() only as a last resort after Shutdown() times out.
// ─────────────────────────────────────────────────────────────────────────────
