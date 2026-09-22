package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestHealthChecker_Liveness(t *testing.T) {
	hc := NewHealthChecker(nil)

	// Liveness starts as alive
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	hc.HandleLiveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// When marked dead (for testing failure case)
	hc.alive.Store(false)
	rec = httptest.NewRecorder()
	hc.HandleLiveness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}

func TestHealthChecker_Readiness(t *testing.T) {
	mock := &mockPinger{}
	hc := NewHealthChecker(mock)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	// 1. Initial state: alive but not ready
	rec := httptest.NewRecorder()
	hc.HandleReadiness(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before MarkReady, got %d", rec.Code)
	}

	// 2. Mark ready, ping succeeds
	hc.MarkReady()
	rec = httptest.NewRecorder()
	hc.HandleReadiness(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when ready and ping ok, got %d", rec.Code)
	}

	// 3. Mark ready, but dependency ping fails
	mock.err = errors.New("db connection timeout")
	rec = httptest.NewRecorder()
	hc.HandleReadiness(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when ping fails, got %d", rec.Code)
	}

	// 4. Mark not ready (draining)
	mock.err = nil
	hc.MarkNotReady()
	rec = httptest.NewRecorder()
	hc.HandleReadiness(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after MarkNotReady, got %d", rec.Code)
	}
}
