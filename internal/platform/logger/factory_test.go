package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestParseEnvironment(t *testing.T) {
	tests := []struct {
		in   string
		want Environment
	}{
		{"production", EnvProduction},
		{"PRODUCTION", EnvProduction},
		{"staging", EnvStaging},
		{"development", EnvDevelopment},
		{"", EnvDevelopment},
		{"unknown-env", EnvDevelopment},
	}

	for _, tt := range tests {
		if got := ParseEnvironment(tt.in); got != tt.want {
			t.Errorf("ParseEnvironment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	if lvl, ok := ParseLevel("debug"); !ok || lvl != slog.LevelDebug {
		t.Errorf("ParseLevel(debug) = %v,%v, want %v,true", lvl, ok, slog.LevelDebug)
	}
	if lvl, ok := ParseLevel("ERROR"); !ok || lvl != slog.LevelError {
		t.Errorf("ParseLevel(ERROR) = %v,%v, want %v,true", lvl, ok, slog.LevelError)
	}
	if _, ok := ParseLevel("verbose"); ok {
		t.Errorf("ParseLevel(verbose) unexpectedly succeeded")
	}
}

func TestNew_ProductionEmitsNDJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Env:    EnvProduction,
		Level:  slog.LevelInfo,
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("service startup", slog.String("version", "v1.0.0"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON from production logger, got error: %v; raw: %s", err, buf.String())
	}
	if entry["msg"] != "service startup" {
		t.Errorf("expected msg 'service startup', got %v", entry["msg"])
	}
	if entry["version"] != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %v", entry["version"])
	}
}

func TestNew_DevelopmentUsesText(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Env:    EnvDevelopment,
		Level:  slog.LevelInfo,
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("local development event", slog.String("key", "value"))

	raw := buf.String()
	var entry map[string]any
	_ = json.Unmarshal([]byte(raw), &entry)

	if _, isJSON := entry["msg"]; isJSON {
		t.Errorf("expected human-readable TextHandler output, got JSON: %s", raw)
	}
	if !bytes.Contains(buf.Bytes(), []byte("local development event")) {
		t.Errorf("expected message in text output, got: %s", raw)
	}
}

func TestNew_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Env:    EnvProduction,
		Level:  slog.LevelWarn,
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("should be suppressed")
	logger.Error("should be emitted")

	raw := buf.String()
	if bytes.Contains([]byte(raw), []byte("should be suppressed")) {
		t.Errorf("INFO record leaked despite LevelWarn threshold: %s", raw)
	}
	if !bytes.Contains([]byte(raw), []byte("should be emitted")) {
		t.Errorf("ERROR record missing from output: %s", raw)
	}
}

func TestNew_RedactKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Env:    EnvProduction,
		Level:  slog.LevelInfo,
		Output: &buf,
		RedactKeys: []string{
			"password",
			"api_key",
		},
	}

	logger := New(cfg)
	logger.Info("client credentials",
		slog.String("username", "admin"),
		slog.String("password", "plaintext-secret"),
		slog.String("api_key", "key-12345"),
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
	if entry["api_key"] != "***" {
		t.Errorf("expected redacted api_key '***', got %v", entry["api_key"])
	}
}
