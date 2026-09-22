package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformctx "train/internal/platform/context"
)

func TestHTTPLoggingMiddleware_LogsCompletionAndStatusByLevel(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  string
	}{
		{"success", http.StatusOK, "INFO"},
		{"redirect", http.StatusFound, "INFO"},
		{"client error", http.StatusBadRequest, "WARN"},
		{"server error", http.StatusInternalServerError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			middleware := HTTPLoggingMiddleware(logger)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte("OK"))
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
			rec := httptest.NewRecorder()
			middleware(next).ServeHTTP(rec, req)

			if rec.Code != tt.statusCode {
				t.Fatalf("expected status %d, got %d", tt.statusCode, rec.Code)
			}

			var entry map[string]any
			lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{'\n'})
			if len(lines) == 0 {
				t.Fatal("expected at least one log line")
			}
			if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
				t.Fatalf("failed to parse completion log: %v", err)
			}

			if entry["msg"] != "http request completed" {
				t.Errorf("expected completion message, got %v", entry["msg"])
			}
			if entry["level"] != tt.wantLevel {
				t.Errorf("expected level %s, got %v", tt.wantLevel, entry["level"])
			}
			if int(entry["status"].(float64)) != tt.statusCode {
				t.Errorf("expected status field %d, got %v", tt.statusCode, entry["status"])
			}
			if entry["bytes"].(float64) != 2 {
				t.Errorf("expected bytes field 2, got %v", entry["bytes"])
			}
			if entry["method"] != http.MethodGet {
				t.Errorf("expected method field GET, got %v", entry["method"])
			}
			if entry["path"] != "/api/v1/orders" {
				t.Errorf("expected path field, got %v", entry["path"])
			}
		})
	}
}

func TestHTTPLoggingMiddleware_UsesRequestIDFromPlatformContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := HTTPLoggingMiddleware(logger)

	innerLogged := make(chan string, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Re-retrieve the logger from the context to verify it was injected correctly
		FromContext(r.Context()).Info("inside handler logic")
		innerLogged <- buf.String()
		w.WriteHeader(http.StatusOK)
	})

	handler := platformctx.Middleware(middleware(next))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trains", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	reqID := rec.Header().Get(platformctx.HeaderRequestID)
	if reqID == "" {
		t.Fatal("expected request_id header set by platform context middleware")
	}

	raw := <-innerLogged
	if !strings.Contains(raw, reqID) {
		t.Errorf("expected injected scoped logger to carry request_id %q, got: %s", reqID, raw)
	}

	completion := buf.String()
	if !strings.Contains(completion, reqID) {
		t.Errorf("expected completion log to carry request_id %q, got: %s", reqID, completion)
	}
}

func TestHTTPLoggingMiddleware_TracksWriteWithoutExplicitHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := HTTPLoggingMiddleware(logger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit-200"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", nil)
	rec := httptest.NewRecorder()
	middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected implicit status 200, got %d", rec.Code)
	}

	var entry map[string]any
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{'\n'})
	if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
		t.Fatalf("failed to parse completion log: %v", err)
	}
	if int(entry["status"].(float64)) != http.StatusOK {
		t.Errorf("expected status field 200, got %v", entry["status"])
	}
	if int(entry["bytes"].(float64)) != len("implicit-200") {
		t.Errorf("expected bytes field %d, got %v", len("implicit-200"), entry["bytes"])
	}
}

func TestHTTPLoggingMiddleware_SuppressesProbeSuccessWhenOptedIn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := HTTPLoggingMiddleware(logger, WithProbeSuppression("/livez", "/readyz"))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, path := range []string{"/livez", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		middleware(next).ServeHTTP(httptest.NewRecorder(), req)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected successful probes to be suppressed, got:\n%s", buf.String())
	}
}

func TestHTTPLoggingMiddleware_ReportsProbeFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := HTTPLoggingMiddleware(logger, WithProbeSuppression("/readyz"))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected failing probe to be logged at ERROR even when suppressed, got:\n%s", out)
	}
}

func TestHTTPLoggingMiddleware_UsesRequestIDHeaderFallback(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trains", nil)
	req.Header.Set(platformctx.HeaderRequestID, "req-from-header-42")

	middleware := HTTPLoggingMiddleware(logger)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), "req-from-header-42") {
		t.Fatalf("expected request_id from X-Request-ID header in completion log, got:\n%s", buf.String())
	}
}

func TestHTTPLoggingMiddleware_IncludesClientIPFromPlatformContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := HTTPLoggingMiddleware(logger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.RemoteAddr = "203.0.113.7:4321"
	handler := platformctx.Middleware(middleware(next))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "203.0.113.7:4321") {
		t.Fatalf("expected client_ip in completion log, got:\n%s", out)
	}
}
