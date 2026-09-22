package logger

import (
	"bytes"
	"log/slog"
	"testing"
)

// testLogWriter routes log records to the test output (t.Log) instead of
// os.Stdout so they only appear when a test fails or the -v flag is used,
// without cluttering the go test output.
type testLogWriter struct {
	tb testing.TB
}

func (w testLogWriter) Write(p []byte) (int, error) {
	w.tb.Helper()
	w.tb.Log(string(bytes.TrimSpace(p)))
	return len(p), nil
}

// NewTestLogger creates a test logger at Debug level directed at the test output.
// It is injected into services under test to ensure messages appear only when needed.
func NewTestLogger(tb testing.TB) *slog.Logger {
	tb.Helper()
	handler := slog.NewTextHandler(testLogWriter{tb: tb}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return slog.New(handler)
}
