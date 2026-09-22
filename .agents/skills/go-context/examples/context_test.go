package examples_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"train/.agents/skills/go-context/examples"
)

func TestMetadataBundle(t *testing.T) {
	ctx := t.Context()

	meta := &examples.RequestMetadata{
		RequestID:     "req-12345",
		CorrelationID: "corr-67890",
		TenantID:      "tenant-alpha",
		UserID:        "user-999",
	}

	ctxWithMeta := examples.WithRequestMetadata(ctx, meta)

	retrieved, ok := examples.MetadataFromContext(ctxWithMeta)
	if !ok {
		t.Fatalf("expected metadata in context")
	}

	if retrieved.RequestID != "req-12345" {
		t.Errorf("got %s, want req-12345", retrieved.RequestID)
	}

	reqID, ok := examples.RequestIDFromContext(ctxWithMeta)
	if !ok || reqID != "req-12345" {
		t.Errorf("RequestIDFromContext failed: got %s", reqID)
	}
}

func TestDetachTask(t *testing.T) {
	parent, cancelParent := context.WithCancel(t.Context())

	meta := &examples.RequestMetadata{RequestID: "req-detach"}
	parentWithMeta := examples.WithRequestMetadata(parent, meta)

	// Detach the context with a timeout
	detached, cancelDetached := examples.DetachTask(parentWithMeta, 1*time.Second)
	defer cancelDetached()

	// Cancel the parent context
	cancelParent()

	// Verify that the detached context was not canceled even though the parent was canceled
	select {
	case <-detached.Done():
		t.Fatalf("detached context should not be canceled when parent is canceled")
	default:
		// Still good!
	}

	// Verify that the context data survives
	reqID, ok := examples.RequestIDFromContext(detached)
	if !ok || reqID != "req-detach" {
		t.Errorf("expected metadata to survive detachment, got: %s", reqID)
	}
}

func TestRequireMinimumBudget(t *testing.T) {
	// Context with a 50 millisecond timeout
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	// Requires 200 milliseconds -> should fail
	err := examples.RequireMinimumBudget(ctx, 200*time.Millisecond)
	if !errors.Is(err, examples.ErrInsufficientBudget) {
		t.Errorf("expected ErrInsufficientBudget, got: %v", err)
	}

	// Requires 10 milliseconds -> should succeed
	err = examples.RequireMinimumBudget(ctx, 10*time.Millisecond)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}
