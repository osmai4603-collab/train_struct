package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestWithContext_FromContext_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	scoped := slog.New(slog.NewJSONHandler(&buf, nil)).With(slog.String("component", "test"))

	ctx := context.Background()
	if got := FromContext(ctx); got == nil {
		t.Fatal("FromContext(background) returned nil")
	}

	ctx = WithContext(ctx, scoped)
	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext returned nil after WithContext")
	}

	got.Info("scoped event")
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry["component"] != "test" {
		t.Errorf("expected component 'test' propagated via context, got %v", entry["component"])
	}
}

func TestContext_NilSafety(t *testing.T) {
	if got := FromContext(nil); got == nil {
		t.Fatal("FromContext(nil) returned nil, expected slog.Default fallback")
	}

	ctx := WithContext(nil, slog.Default())
	if ctx != nil {
		t.Errorf("WithContext(nil, ...) = %v, want nil", ctx)
	}
}
