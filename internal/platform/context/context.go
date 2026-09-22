package context

import (
	stdctx "context"
	"errors"
	"fmt"
	"time"
)

// ErrInsufficientBudget is returned when the remaining time in the deadline budget is insufficient to complete the operation
var ErrInsufficientBudget = errors.New("insufficient deadline budget for operation")

// Detach creates an independent context for background tasks via WithoutCancel while preserving tracing attributes
// and imposing a new timeout that prevents resources from hanging
func Detach(parent stdctx.Context, timeout time.Duration) (stdctx.Context, stdctx.CancelFunc) {
	if parent == nil {
		parent = stdctx.Background()
	}

	// 1. Detach the cancellation signal while keeping all values and attributes
	detached := stdctx.WithoutCancel(parent)

	// 2. Apply an independent timeout to protect resources
	if timeout > 0 {
		return stdctx.WithTimeout(detached, timeout)
	}

	return stdctx.WithCancel(detached)
}

// RemainingDeadline returns the time remaining before the context's deadline expires
// It returns (0, false) if no deadline is set
func RemainingDeadline(ctx stdctx.Context) (time.Duration, bool) {
	if ctx == nil {
		return 0, false
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0, true
	}
	return remaining, true
}

// HasSufficientBudget checks whether the remaining time is greater than or equal to the minimum required
func HasSufficientBudget(ctx stdctx.Context, minRequired time.Duration) bool {
	remaining, ok := RemainingDeadline(ctx)
	if !ok {
		return true // no restrictive deadline
	}
	return remaining >= minRequired
}

// RequireMinimumBudget proactively checks whether the remaining time is sufficient and otherwise returns a Fail-Fast error
func RequireMinimumBudget(ctx stdctx.Context, minRequired time.Duration) error {
	remaining, ok := RemainingDeadline(ctx)
	if !ok {
		return nil
	}
	if remaining < minRequired {
		return fmt.Errorf("%w: remaining %v < required %v", ErrInsufficientBudget, remaining, minRequired)
	}
	return nil
}

// RunDetached runs an asynchronous task in the background safely while preserving tracing attributes,
// applying an independent timeout, and providing full panic recovery
func RunDetached(parent stdctx.Context, timeout time.Duration, task func(ctx stdctx.Context) error, onErr func(err error)) {
	detachedCtx, cancel := Detach(parent, timeout)
	go func() {
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				if onErr != nil {
					onErr(fmt.Errorf("detached task panicked: %v", r))
				}
			}
		}()

		if err := task(detachedCtx); err != nil {
			if onErr != nil {
				onErr(err)
			}
		}
	}()
}
