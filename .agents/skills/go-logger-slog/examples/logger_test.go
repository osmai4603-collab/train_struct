package examples

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/slogtest"
)

func TestLoggerFactory(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Env:       EnvProduction,
		Level:     slog.LevelInfo,
		AddSource: false,
		Output:    &buf,
	}

	logger := NewLoggerFactory(cfg)
	logger.Info("service startup", slog.String("version", "v1.0.0"))

	output := buf.String()
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Fatalf("expected valid JSON output from production logger, got error: %v, raw: %s", err, output)
	}

	if data["msg"] != "service startup" {
		t.Errorf("expected msg 'service startup', got: %v", data["msg"])
	}
	if data["version"] != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got: %v", data["version"])
	}
}

func TestSensitiveTypes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	secret := SecretString("super-confidential-password")
	sensitiveToken := NewSensitive("jwt-bearer-secret-token")

	logger.Info("user auth",
		slog.Any("password", secret),
		slog.Any("token", sensitiveToken),
	)

	raw := buf.String()
	if strings.Contains(raw, "super-confidential-password") {
		t.Errorf("secret password leaked in raw log output: %s", raw)
	}
	if strings.Contains(raw, "jwt-bearer-secret-token") {
		t.Errorf("sensitive token leaked in raw log output: %s", raw)
	}

	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal JSON log: %v", err)
	}

	if data["password"] != "***" {
		t.Errorf("expected password to be '***', got: %v", data["password"])
	}
	if data["token"] != "***" {
		t.Errorf("expected token to be '***', got: %v", data["token"])
	}
}

func TestRedactingHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password", "apikey"})
	logger := slog.New(handler)

	logger.Info("client credentials",
		slog.String("username", "admin"),
		slog.String("password", "plaintext-secret"),
		slog.String("apikey", "key-12345"),
	)

	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if data["username"] != "admin" {
		t.Errorf("expected username 'admin', got: %v", data["username"])
	}
	if data["password"] != "***" {
		t.Errorf("expected password '***', got: %v", data["password"])
	}
	if data["apikey"] != "***" {
		t.Errorf("expected apikey '***', got: %v", data["apikey"])
	}
}

func TestRedactingHandler_SlogtestCompliance(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	handler := NewRedactingHandler(base, []string{"secret"})

	err := slogtest.TestHandler(handler, func() []map[string]any {
		var results []map[string]any
		lines := bytes.Split(buf.Bytes(), []byte{'\n'})
		for _, line := range lines {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(line, &m); err == nil {
				results = append(results, m)
			}
		}
		return results
	})

	if err != nil {
		t.Fatalf("RedactingHandler failed slogtest verification: %v", err)
	}
}

func TestHTTPLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := HTTPLoggingMiddleware(logger)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqLogger := LoggerFromContext(r.Context())
		reqLogger.Info("inside handler logic")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()

	middleware(testHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got: %d", rec.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "/api/v1/orders") {
		t.Errorf("expected log to contain path, got: %s", output)
	}
	if !strings.Contains(output, "inside handler logic") {
		t.Errorf("expected log to contain inner message, got: %s", output)
	}
}
