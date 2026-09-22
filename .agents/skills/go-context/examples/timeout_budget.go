package examples

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInsufficientBudget = errors.New("timeout budget exhausted before operation start")

// RemainingDeadline returns the time remaining before the deadline expires
func RemainingDeadline(ctx context.Context) (time.Duration, bool) {
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

// RequireMinimumBudget proactively checks whether the remaining time is sufficient to run the operation
func RequireMinimumBudget(ctx context.Context, minRequired time.Duration) error {
	remaining, ok := RemainingDeadline(ctx)
	if !ok {
		return nil // no deadline is set
	}
	if remaining < minRequired {
		return fmt.Errorf("%w: remaining %v < required %v", ErrInsufficientBudget, remaining, minRequired)
	}
	return nil
}
