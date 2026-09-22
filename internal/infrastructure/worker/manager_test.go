package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"train/internal/platform/logger"
)

func TestManager_StartStopAll(t *testing.T) {
	m := NewManager(logger.NewTestLogger(t))

	started := make(chan struct{})
	cancelled := make(chan struct{})

	m.Start("test-worker", func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(cancelled)
	})

	waitClosed(t, started, "worker did not start")

	m.StopAll()

	waitClosed(t, cancelled, "worker did not observe context cancellation")
}

func TestManager_StartPanicRecovery(t *testing.T) {
	m := NewManager(logger.NewTestLogger(t))

	m.Start("panicking-worker", func(ctx context.Context) {
		panic("boom")
	})

	healthyCancelled := make(chan struct{})
	m.Start("healthy-worker", func(ctx context.Context) {
		<-ctx.Done()
		close(healthyCancelled)
	})

	// Give the panicking goroutine time to run and be recovered.
	time.Sleep(50 * time.Millisecond)

	// StopAll must not hang even though one worker panicked early.
	m.StopAll()

	waitClosed(t, healthyCancelled, "healthy worker did not observe cancellation")
}

func TestManager_ContextIsShared(t *testing.T) {
	m := NewManager(logger.NewTestLogger(t))

	var cancelledAt atomic.Int64
	m.Start("observer", func(ctx context.Context) {
		<-ctx.Done()
		cancelledAt.Store(time.Now().UnixNano())
	})

	before := time.Now().UnixNano()
	m.StopAll()
	after := time.Now().UnixNano()

	observed := cancelledAt.Load()
	if observed < before || observed > after {
		t.Fatalf("worker cancellation time out of range: observed=%d before=%d after=%d", observed, before, after)
	}
}

func waitClosed(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal(msg)
	}
}
