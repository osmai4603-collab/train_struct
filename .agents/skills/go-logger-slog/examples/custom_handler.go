package examples

import (
	"context"
	"log/slog"
	"strings"
)

// RedactingHandler is a custom handler that wraps another handler and redacts sensitive fields
type RedactingHandler struct {
	inner      slog.Handler
	redactKeys map[string]bool
}

// NewRedactingHandler creates a new redacting handler for the specified keys
func NewRedactingHandler(inner slog.Handler, keys []string) *RedactingHandler {
	maskMap := make(map[string]bool, len(keys))
	for _, k := range keys {
		maskMap[strings.ToLower(strings.TrimSpace(k))] = true
	}
	return &RedactingHandler{
		inner:      inner,
		redactKeys: maskMap,
	}
}

// Enabled delegates the level check to the inner handler
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle inspects and redacts sensitive attributes, then sends the record to the inner handler
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.sanitizeAttr(a))
		return true
	})

	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs derives a new handler with the given attributes after sanitizing them
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		sanitized = append(sanitized, h.sanitizeAttr(a))
	}
	return &RedactingHandler{
		inner:      h.inner.WithAttrs(sanitized),
		redactKeys: h.redactKeys,
	}
}

// WithGroup derives a new handler within a named group
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner:      h.inner.WithGroup(name),
		redactKeys: h.redactKeys,
	}
}

// sanitizeAttr processes the attribute and redacts its value if it matches a forbidden key
func (h *RedactingHandler) sanitizeAttr(a slog.Attr) slog.Attr {
	// If the attribute is a group, sanitize its elements internally
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		sanitizedGroup := make([]slog.Attr, 0, len(groupAttrs))
		for _, ga := range groupAttrs {
			sanitizedGroup = append(sanitizedGroup, h.sanitizeAttr(ga))
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(sanitizedGroup...),
		}
	}

	if h.redactKeys[strings.ToLower(a.Key)] {
		return slog.Attr{
			Key:   a.Key,
			Value: slog.StringValue("***"),
		}
	}

	return a
}
