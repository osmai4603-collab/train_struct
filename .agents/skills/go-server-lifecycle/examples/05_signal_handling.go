package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// =============================================================================
// Phase 5: Signal Handling
// =============================================================================
// Listen for OS termination signals to trigger graceful shutdown.
//
// Required signals:
//   - SIGINT  (os.Interrupt) → User presses Ctrl+C
//   - SIGTERM                → Kubernetes, systemd, Docker stop
//
// Official source: https://pkg.go.dev/os/signal#NotifyContext
// =============================================================================

// WaitForShutdownSignal blocks until an OS termination signal is received.
// Returns a context that is canceled when the signal arrives.
//
// Modern approach (Go 1.16+): signal.NotifyContext
// This is cleaner than manually creating a channel with signal.Notify.
func WaitForShutdownSignal(logger *slog.Logger) (context.Context, context.CancelFunc) {
	// signal.NotifyContext returns a context that is canceled when one of
	// the specified signals is received.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,    // SIGINT: Ctrl+C
		syscall.SIGTERM, // SIGTERM: Kubernetes, systemd
	)

	logger.Info("signal handler registered",
		"signals", []string{"SIGINT", "SIGTERM"},
	)

	return ctx, stop
}

// WaitForShutdownSignalChannel is the classic approach using channels.
// Use this if you need to support Go versions before 1.16, or if you
// need to distinguish which specific signal was received.
func WaitForShutdownSignalChannel(logger *slog.Logger) <-chan os.Signal {
	quit := make(chan os.Signal, 1)

	// signal.Notify relays incoming signals to the channel.
	// Use a buffered channel (size 1) to avoid missing the signal
	// if nobody is reading from the channel at the exact moment.
	signal.Notify(quit,
		os.Interrupt,    // SIGINT: Ctrl+C
		syscall.SIGTERM, // SIGTERM: Kubernetes, systemd
	)

	logger.Info("signal handler registered (channel mode)",
		"signals", []string{"SIGINT", "SIGTERM"},
	)

	return quit
}
