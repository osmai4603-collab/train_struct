package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"
)

func TestRedactingHandler_RedactsRootKeys(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	handler := NewRedactingHandler(base, "password", "apikey")
	logger := slog.New(handler)

	logger.Info("client credentials",
		slog.String("username", "admin"),
		slog.String("password", "plaintext-secret"),
		slog.String("apikey", "key-12345"),
	)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry["username"] != "admin" {
		t.Errorf("expected username 'admin', got %v", entry["username"])
	}
	if entry["password"] != "***" {
		t.Errorf("expected redacted password '***', got %v", entry["password"])
	}
	if entry["apikey"] != "***" {
		t.Errorf("expected redacted apikey '***', got %v", entry["apikey"])
	}
}

func TestRedactingHandler_RedactsNestedGroupKeys(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	handler := NewRedactingHandler(base, "token")
	logger := slog.New(handler)

	logger.Info("nested secrets",
		slog.Group("credentials",
			slog.String("user", "alice"),
			slog.String("token", "deep-secret-value"),
		),
	)

	raw := buf.String()
	if strings.Contains(raw, "deep-secret-value") {
		t.Errorf("nested secret token leaked in raw log output: %s", raw)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}

	group, ok := entry["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("expected credentials group, got %v", entry["credentials"])
	}
	if group["token"] != "***" {
		t.Errorf("expected nested token '***', got %v", group["token"])
	}
}

func TestRedactingHandler_SlogtestCompliance(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	handler := NewRedactingHandler(base, "secret")

	err := slogtest.TestHandler(handler, func() []map[string]any {
		var results []map[string]any
		for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal(line, &entry); err != nil {
				t.Fatal(err)
			}
			results = append(results, entry)
		}
		return results
	})

	if err != nil {
		t.Fatalf("RedactingHandler failed slogtest compliance: %v", err)
	}
}
