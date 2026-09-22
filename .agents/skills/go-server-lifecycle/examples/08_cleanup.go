package lifecycle

import (
	"context"
	"log/slog"
	"sync"
)

// =============================================================================
// Phase 8: Resource Cleanup
// =============================================================================
// Close resources in REVERSE order of creation.
// Use sync.WaitGroup to wait for background workers to finish.
//
// Creation order:  Config → Logger → DB → Cache → Workers → Server
// Cleanup order:   Server → Workers → Cache → DB → Logger
//
// Official source: https://pkg.go.dev/sync#WaitGroup
// =============================================================================

// WorkerManager manages background goroutines (workers) and ensures they
// all complete before the process exits.
type WorkerManager struct {
	wg     sync.WaitGroup
	cancel context.CancelFunc
	ctx    context.Context
	logger *slog.Logger
}

// NewWorkerManager creates a manager that controls background workers.
// The returned context should be passed to all workers — when cancel()
// is called, all workers should observe ctx.Done() and exit.
func NewWorkerManager(logger *slog.Logger) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerManager{
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Start launches a background worker. The worker function receives a context
// that will be canceled during shutdown. The worker MUST exit when ctx is done.
//
// Usage:
//
//	wm.Start("metrics-reporter", func(ctx context.Context) {
//	    ticker := time.NewTicker(30 * time.Second)
//	    defer ticker.Stop()
//	    for {
//	        select {
//	        case <-ctx.Done():
//	            return
//	        case <-ticker.C:
//	            reportMetrics()
//	        }
//	    }
//	})
func (wm *WorkerManager) Start(name string, fn func(ctx context.Context)) {
	wm.wg.Add(1)
	go func() {
		defer wm.wg.Done()
		wm.logger.Info("worker started", "name", name)
		fn(wm.ctx)
		wm.logger.Info("worker stopped", "name", name)
	}()
}

// StopAll cancels the context (signals all workers to stop) and waits
// for all workers to finish. This blocks until every worker has returned.
func (wm *WorkerManager) StopAll() {
	wm.logger.Info("stopping all workers...")

	// Signal all workers to stop
	wm.cancel()

	// Wait for all workers to finish their current work and exit
	wm.wg.Wait()

	wm.logger.Info("all workers stopped")
}

// ─────────────────────────────────────────────────────────────────────────────
// Full Cleanup Sequence
// ─────────────────────────────────────────────────────────────────────────────
//
// This is the complete cleanup procedure called after Shutdown() returns:
//
//   func cleanup(deps *Dependencies, wm *WorkerManager, logger *slog.Logger) {
//       // 1. Stop background workers (they may write to DB)
//       wm.StopAll()
//
//       // 2. Close dependencies in reverse order
//       deps.Close()
//
//       // 3. Final log message
//       logger.Info("cleanup complete — process exiting")
//   }
//
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Common Mistake: Not waiting for workers
// ─────────────────────────────────────────────────────────────────────────────
//
// BAD (data loss):
//   srv.Shutdown(ctx)
//   os.Exit(0)  // Workers are still running!
//
// GOOD (safe):
//   srv.Shutdown(ctx)
//   wm.StopAll()      // Wait for all workers
//   deps.Close()      // Close DB, cache, etc.
//   os.Exit(0)        // Now it's safe
// ─────────────────────────────────────────────────────────────────────────────
