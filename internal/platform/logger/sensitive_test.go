package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretString_RedactedInLogsAndFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	secret := SecretString("super-confidential-password")
	logger.Info("user auth", slog.Any("password", secret))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry["password"] != "***" {
		t.Errorf("expected password '***', got %v", entry["password"])
	}

	for _, out := range []string{fmt.Sprintf("%s", secret), fmt.Sprintf("%v", secret), fmt.Sprintf("%#v", secret)} {
		if strings.Contains(out, "super-confidential-password") {
			t.Errorf("secret leaked via fmt formatting: %q", out)
		}
	}
	if secret.Expose() != "super-confidential-password" {
		t.Errorf("Expose() = %q, want the original value", secret.Expose())
	}
}

func TestSensitive_RedactedInLogsAndFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	token := NewSensitive("jwt-bearer-secret-token")
	logger.Info("client authenticated", slog.Any("token", token))

	raw := buf.String()
	if strings.Contains(raw, "jwt-bearer-secret-token") {
		t.Errorf("sensitive token leaked in raw log output: %s", raw)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry["token"] != "***" {
		t.Errorf("expected token '***', got %v", entry["token"])
	}

	for _, out := range []string{fmt.Sprintf("%s", token), fmt.Sprintf("%v", token), fmt.Sprintf("%#v", token)} {
		if strings.Contains(out, "jwt-bearer-secret-token") {
			t.Errorf("token leaked via fmt formatting: %q", out)
		}
	}
	if token.Value() != "jwt-bearer-secret-token" {
		t.Errorf("Value() = %q, want the original value", token.Value())
	}
}

func TestSensitive_GenericIntType(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	bankAccount := NewSensitive(1234567890)
	logger.Info("transfer initiated", slog.Any("account", bankAccount))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry["account"] != "***" {
		t.Errorf("expected account '***', got %v", entry["account"])
	}
}
