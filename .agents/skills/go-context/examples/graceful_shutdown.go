package examples

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunServerWithGracefulShutdown demonstrates graceful server shutdown via signal.NotifyContext
func RunServerWithGracefulShutdown(handler http.Handler, port string) error {
	// 1. Listen for interrupt and termination signals from the operating system
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}

	serverErrors := make(chan error, 1)

	go func() {
		slog.Info("server starting", "addr", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 2. Wait for a termination signal to arrive or for a runtime error to occur
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server startup failed: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received, commencing draining")
	}

	// 3. A bounded deadline for closing open connections and completing in-flight requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("forced server shutdown: %w", err)
	}

	slog.Info("server exited cleanly")
	return nil
}
