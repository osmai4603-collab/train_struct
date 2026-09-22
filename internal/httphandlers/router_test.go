package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"train/internal/infrastructure/health"
	"train/internal/platform/logger"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

func TestNewRouter_HealthProbes(t *testing.T) {
	hc := health.NewHealthChecker(okPinger{})
	router := NewRouter(logger.NewTestLogger(t), hc)

	t.Run("liveness before ready", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("livez expected 200, got %d", rec.Code)
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["status"] != "alive" {
			t.Fatalf("livez body = %v, want status=alive", body)
		}
	})

	t.Run("readiness not ready initially", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("readyz expected 503 before MarkReady, got %d", rec.Code)
		}
	})

	t.Run("readiness ready after mark", func(t *testing.T) {
		hc.MarkReady()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("readyz expected 200 after MarkReady, got %d", rec.Code)
		}
	})
}

func TestNewRouter_UnknownRoutesAreNotFound(t *testing.T) {
	router := NewRouter(logger.NewTestLogger(t), health.NewHealthChecker(okPinger{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez-typo", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not_found") {
		t.Fatalf("expected structured not_found body, got: %s", rec.Body.String())
	}
}

func TestNewRouter_WrongMethodIsNotFound(t *testing.T) {
	router := NewRouter(logger.NewTestLogger(t), health.NewHealthChecker(okPinger{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/readyz", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /readyz expected 404, got %d", rec.Code)
	}
}
