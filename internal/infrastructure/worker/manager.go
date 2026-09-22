// Package worker provides coordinated background goroutine supervision for the
// server runtime. Workers are started only after dependencies are initialized
// and are guaranteed to stop before resources (e.g. connection pools) are closed.
package worker

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

// Manager supervises background worker goroutines using a cancellable context
// and a sync.WaitGroup. StopAll() cancels the shared context and blocks until
// every worker has returned, preventing orphaned goroutines during teardown.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger
}

// NewManager creates a Manager with a fresh cancellable lifecycle context.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Start launches a named worker on the shared lifecycle context and tracks it
// in the WaitGroup. A worker that panics is recovered and logged so that one
// failed worker cannot crash the whole process.
func (m *Manager) Start(name string, fn func(ctx context.Context)) {
	if fn == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("background worker panicked",
					slog.String("name", name),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()
		m.logger.Info("background worker started", slog.String("name", name))
		fn(m.ctx)
		m.logger.Info("background worker stopped", slog.String("name", name))
	}()
}

// StopAll cancels the shared worker context and blocks until every worker
// goroutine has exited cleanly.
func (m *Manager) StopAll() {
	m.logger.Info("stopping background workers")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("all background workers stopped")
}
