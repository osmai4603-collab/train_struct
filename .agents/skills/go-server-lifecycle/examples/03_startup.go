package lifecycle

import (
	"errors"
	"log/slog"
	"net/http"
)

// =============================================================================
// Phase 3: Startup (Non-blocking)
// =============================================================================
// CRITICAL: Run ListenAndServe in a SEPARATE goroutine.
// main() must remain free to listen for OS signals.
//
// Official source: https://pkg.go.dev/net/http#Server.ListenAndServe
// =============================================================================

// StartServer launches the server in a non-blocking goroutine.
// It returns a channel that receives an error if the server fails to start
// (excluding the expected ErrServerClosed).
//
// Why non-blocking?
//   - ListenAndServe blocks until the server is closed
//   - If we call it directly in main(), we can never handle OS signals
//   - The goroutine lets main() proceed to signal handling
func StartServer(srv *http.Server, logger *slog.Logger) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		logger.Info("server starting", "addr", srv.Addr)

		err := srv.ListenAndServe()

		// IMPORTANT: When Shutdown() is called, ListenAndServe immediately
		// returns http.ErrServerClosed. This is EXPECTED, not an error.
		// Use errors.Is() — NOT == — for reliable comparison.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed to start", "error", err)
			errCh <- err
			return
		}

		// ErrServerClosed means Shutdown() was called — this is normal.
		logger.Info("server stopped accepting new connections")
	}()

	return errCh
}

// StartServerTLS is the TLS variant for HTTPS servers.
// Same non-blocking pattern, same ErrServerClosed handling.
func StartServerTLS(srv *http.Server, certFile, keyFile string, logger *slog.Logger) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		logger.Info("server starting (TLS)", "addr", srv.Addr)

		err := srv.ListenAndServeTLS(certFile, keyFile)

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("TLS server failed to start", "error", err)
			errCh <- err
			return
		}

		logger.Info("TLS server stopped accepting new connections")
	}()

	return errCh
}
