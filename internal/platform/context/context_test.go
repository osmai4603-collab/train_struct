package context_test

import (
	stdctx "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformctx "train/internal/platform/context"
)

func TestRequestMetadata(t *testing.T) {
	ctx := t.Context()

	// 1. Check the initial context
	meta, ok := platformctx.MetadataFromContext(ctx)
	if ok || meta != nil {
		t.Fatalf("expected empty metadata on fresh context")
	}

	// 2. Inject a unified set of attributes
	initial := &platformctx.RequestMetadata{
		RequestID:     "req-001",
		CorrelationID: "corr-001",
		TenantID:      "tenant-main",
		UserID:        "usr-admin",
		ClientIP:      "192.168.1.1",
	}

	ctx = platformctx.WithRequestMetadata(ctx, initial)

	retrieved, ok := platformctx.MetadataFromContext(ctx)
	if !ok || retrieved == nil {
		t.Fatalf("expected to retrieve metadata")
	}
	if retrieved.RequestID != "req-001" {
		t.Errorf("got requestID %s, want req-001", retrieved.RequestID)
	}

	// 3. Check the individual functions
	reqID, ok := platformctx.RequestIDFromContext(ctx)
	if !ok || reqID != "req-001" {
		t.Errorf("RequestIDFromContext failed: got %s", reqID)
	}

	tenantID, ok := platformctx.TenantIDFromContext(ctx)
	if !ok || tenantID != "tenant-main" {
		t.Errorf("TenantIDFromContext failed: got %s", tenantID)
	}

	userID, ok := platformctx.UserIDFromContext(ctx)
	if !ok || userID != "usr-admin" {
		t.Errorf("UserIDFromContext failed: got %s", userID)
	}

	// 4. Check safe modification without affecting the previous context (Immutability)
	childCtx := platformctx.WithRequestID(ctx, "req-002")
	childReqID, _ := platformctx.RequestIDFromContext(childCtx)
	parentReqID, _ := platformctx.RequestIDFromContext(ctx)

	if childReqID != "req-002" {
		t.Errorf("child should have updated request ID, got: %s", childReqID)
	}
	if parentReqID != "req-001" {
		t.Errorf("parent should remain unchanged, got: %s", parentReqID)
	}
}

func TestDetach(t *testing.T) {
	parent, cancelParent := stdctx.WithCancel(t.Context())

	meta := &platformctx.RequestMetadata{
		RequestID: "req-detached-100",
		TenantID:  "tenant-detached",
	}
	parent = platformctx.WithRequestMetadata(parent, meta)

	// Detach the context with a 500ms timeout
	detachedCtx, cancelDetached := platformctx.Detach(parent, 500*time.Millisecond)
	defer cancelDetached()

	// Cancel the parent context
	cancelParent()

	// Verify the detached context is still alive
	select {
	case <-detachedCtx.Done():
		t.Fatalf("detached context should NOT be canceled when parent is canceled")
	default:
		// Still alive!
	}

	// Verify the tracing attributes survive
	reqID, ok := platformctx.RequestIDFromContext(detachedCtx)
	if !ok || reqID != "req-detached-100" {
		t.Errorf("expected metadata to survive Detach, got: %s", reqID)
	}
}

func TestDeadlineBudget(t *testing.T) {
	ctx, cancel := stdctx.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	// 1. Check the remaining time
	remaining, ok := platformctx.RemainingDeadline(ctx)
	if !ok || remaining <= 0 {
		t.Errorf("expected valid remaining deadline, got: %v", remaining)
	}

	// 2. Check sufficiency
	if platformctx.HasSufficientBudget(ctx, 100*time.Millisecond) {
		t.Errorf("expected HasSufficientBudget to return false for 100ms when remaining is ~50ms")
	}
	if !platformctx.HasSufficientBudget(ctx, 10*time.Millisecond) {
		t.Errorf("expected HasSufficientBudget to return true for 10ms")
	}

	// 3. Check the budget error
	err := platformctx.RequireMinimumBudget(ctx, 200*time.Millisecond)
	if !errors.Is(err, platformctx.ErrInsufficientBudget) {
		t.Errorf("expected ErrInsufficientBudget, got: %v", err)
	}

	err = platformctx.RequireMinimumBudget(ctx, 5*time.Millisecond)
	if err != nil {
		t.Errorf("expected nil error for valid budget, got: %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	var capturedMeta *platformctx.RequestMetadata

	handler := platformctx.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta, ok := platformctx.MetadataFromContext(r.Context())
		if ok {
			capturedMeta = meta
		}
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "req-xyz-789")
	req.Header.Set("X-Tenant-ID", "tenant-42")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if capturedMeta == nil {
		t.Fatalf("expected metadata to be populated by middleware")
	}
	if capturedMeta.RequestID != "req-xyz-789" {
		t.Errorf("got reqID %s, want req-xyz-789", capturedMeta.RequestID)
	}
	if capturedMeta.TenantID != "tenant-42" {
		t.Errorf("got tenantID %s, want tenant-42", capturedMeta.TenantID)
	}
	if rr.Header().Get("X-Request-ID") != "req-xyz-789" {
		t.Errorf("expected response header X-Request-ID to be set")
	}
}

func TestRunDetached(t *testing.T) {
	done := make(chan struct{})
	var capturedErr error

	platformctx.RunDetached(t.Context(), 1*time.Second, func(ctx stdctx.Context) error {
		defer close(done)
		return nil
	}, func(err error) {
		capturedErr = err
	})

	select {
	case <-done:
		// Succeeded
	case <-time.After(2 * time.Second):
		t.Fatalf("RunDetached task timed out")
	}

	if capturedErr != nil {
		t.Errorf("unexpected error in RunDetached: %v", capturedErr)
	}
}
