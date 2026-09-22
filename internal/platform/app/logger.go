package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"train/internal/platform/config"
	platformlogger "train/internal/platform/logger"
)

// newLogger builds a multi-target structured logger for the process:
//
//   - Terminal output: human-friendly, high-contrast colored pretty handler when
//     attached to a TTY (or forced via LOG_COLOR); otherwise compact JSON.
//   - File persistence: newline-delimited JSON for every event (log_dir/app.log).
//   - Error file: WARN+ events duplicated to log_dir/error.log for rapid triage.
//
// The returned io.Closer flushes (Sync) and closes all opened log files during
// the cleanup phase. Terminal detection uses os.File.Stat() (ModeCharDevice) so
// no external isatty dependency is required.
func newLogger(settings config.LoggerSettings) (*slog.Logger, io.Closer) {
	out := os.Stderr

	level := slog.LevelInfo
	if parsed, ok := platformlogger.ParseLevel(settings.Level); ok {
		level = parsed
	}

	var isTerminal bool
	if stat, err := out.Stat(); err == nil {
		isTerminal = (stat.Mode() & os.ModeCharDevice) != 0
	}

	jsonOpts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, attr.Value.Time().Format(time.RFC3339))
			}
			return attr
		},
	}

	var terminalHandler slog.Handler
	if isTerminal || settings.ForceColor {
		terminalHandler = &prettyHandler{w: out}
	} else {
		terminalHandler = slog.NewJSONHandler(out, jsonOpts)
	}

	handlers := []slog.Handler{terminalHandler}
	var files []*os.File

	if settings.ToFile {
		logDir := settings.LogDir
		if logDir == "" {
			logDir = "logs"
		}
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			// 1. All events -> app.log in JSON format.
			appLogPath := filepath.Join(logDir, "app.log")
			if appFile, err := os.OpenFile(appLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				files = append(files, appFile)
				handlers = append(handlers, slog.NewJSONHandler(appFile, jsonOpts))
			}

			// 2. WARN+ only -> error.log in JSON format.
			errLogPath := filepath.Join(logDir, "error.log")
			if errFile, err := os.OpenFile(errLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				files = append(files, errFile)
				handlers = append(handlers, &minLevelHandler{
					handler:  slog.NewJSONHandler(errFile, jsonOpts),
					minLevel: slog.LevelWarn,
				})
			}
		}
	}

	closer := &logCloser{files: files}
	if len(handlers) == 1 {
		return slog.New(terminalHandler), closer
	}
	return slog.New(&multiHandler{handlers: handlers}), closer
}

// prettyHandler renders records with high-contrast ANSI colors and a compact,
// single-line layout optimized for developer terminals.
type prettyHandler struct {
	w io.Writer
}

func (h *prettyHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var levelColor string
	switch {
	case r.Level >= slog.LevelError:
		levelColor = "\033[31m" // Red
	case r.Level >= slog.LevelWarn:
		levelColor = "\033[33m" // Yellow
	case r.Level >= slog.LevelInfo:
		levelColor = "\033[32m" // Green
	case r.Level >= slog.LevelDebug:
		levelColor = "\033[36m" // Cyan
	}

	timeStr := fmt.Sprintf("\033[90m%s\033[0m", r.Time.Format("2006-01-02 03:04:05 PM"))
	levelStr := fmt.Sprintf("%s%-5s\033[0m", levelColor, r.Level.String())

	var methodStr string
	var otherAttrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "method" {
			val := strings.ToUpper(fmt.Sprint(a.Value.Any()))
			var mColor string
			switch val {
			case "GET":
				mColor = "\033[34m" // Blue
			case "POST":
				mColor = "\033[32m" // Green
			case "PUT", "PATCH":
				mColor = "\033[33m" // Yellow
			case "DELETE":
				mColor = "\033[31m" // Red
			default:
				mColor = "\033[35m" // Magenta
			}
			methodStr = fmt.Sprintf(" \033[1m%s%-4s\033[0m", mColor, val)
		} else {
			otherAttrs = append(otherAttrs, a)
		}
		return true
	})

	fmt.Fprintf(h.w, "%s %s%s %-30s", timeStr, levelStr, methodStr, r.Message)

	for _, a := range otherAttrs {
		if a.Key == "status" {
			var sCode int
			switch v := a.Value.Any().(type) {
			case int:
				sCode = v
			case int64:
				sCode = int(v)
			}
			if sCode > 0 {
				var sColor string
				switch {
				case sCode >= 500:
					sColor = "\033[1;37;41m" // Bold White on Red Background
				case sCode >= 400:
					sColor = "\033[1;33m" // Bold Yellow
				case sCode >= 300:
					sColor = "\033[36m" // Cyan
				default:
					sColor = "\033[32m" // Green
				}
				fmt.Fprintf(h.w, " %sstatus=%d\033[0m", sColor, sCode)
				continue
			}
		}
		if a.Key == "path" && r.Level >= slog.LevelWarn {
			fmt.Fprintf(h.w, " \033[1;37mpath=%v\033[0m", a.Value.Any())
			continue
		}
		fmt.Fprintf(h.w, " \033[90m%s=\033[0m%v", a.Key, a.Value.Any())
	}

	fmt.Fprint(h.w, "\n")
	return nil
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *prettyHandler) WithGroup(name string) slog.Handler       { return h }

// multiHandler fans each record out to every enabled sub-handler.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// minLevelHandler filters records to only forward those at or above minLevel.
type minLevelHandler struct {
	handler  slog.Handler
	minLevel slog.Level
}

func (h *minLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel && h.handler.Enabled(ctx, level)
}

func (h *minLevelHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < h.minLevel {
		return nil
	}
	return h.handler.Handle(ctx, r)
}

func (h *minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &minLevelHandler{handler: h.handler.WithAttrs(attrs), minLevel: h.minLevel}
}

func (h *minLevelHandler) WithGroup(name string) slog.Handler {
	return &minLevelHandler{handler: h.handler.WithGroup(name), minLevel: h.minLevel}
}

// logCloser flushes and closes all opened log files in the cleanup phase.
type logCloser struct {
	files []*os.File
}

func (lc *logCloser) Close() error {
	var firstErr error
	for _, f := range lc.files {
		if f == nil {
			continue
		}
		_ = f.Sync()
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
