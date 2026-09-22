package examples

import (
	"context"
	"time"
)

// DetachTask creates a detached context for background tasks while keeping the tracing attributes and enforcing the deadline
func DetachTask(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	// 1. Isolate the cancellation signal from the parent request context while keeping all the Values
	detached := context.WithoutCancel(parent)

	// 2. Enforce an independent timeout to protect resources from hanging
	if timeout > 0 {
		return context.WithTimeout(detached, timeout)
	}

	return context.WithCancel(detached)
}

// ExecuteAsyncAuditExample demonstrates how to run a background audit operation completely safely
func ExecuteAsyncAuditExample(reqCtx context.Context, auditFn func(ctx context.Context) error) {
	bgCtx, cancel := DetachTask(reqCtx, 10*time.Second)

	go func() {
		defer cancel()
		_ = auditFn(bgCtx)
	}()
}
