package logger

import (
	"context"
	"log/slog"
	"strings"
)

// RedactingHandler is a custom handler that wraps an inner handler and automatically
// redacts the values of sensitive keys as *** before passing the record on to the
// inner handler, without affecting the remaining attributes.
// It is used in sensitive environments as an extra line of defense on top of the
// slog.LogValuer interface implementation.
type RedactingHandler struct {
	inner slog.Handler
	keys  map[string]struct{}
}

// redactedBytes is the unified replacement value used when redacting a sensitive attribute.
var redactedBytes = slog.StringValue(redactedValue)

// NewRedactingHandler creates a new redaction handler wrapping inner and redacting
// the given keys (case-insensitive comparison, trimming redundant whitespace from
// both ends of each key).
func NewRedactingHandler(inner slog.Handler, keys ...string) *RedactingHandler {
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	return &RedactingHandler{
		inner: inner,
		keys:  keySet,
	}
}

// Enabled delegates the level check to the inner handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle sanitizes the record attributes by redacting sensitive keys, then sends the record to the inner handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.sanitizeAttr(a))
		return true
	})

	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs derives a new handler carrying the added attributes after sanitizing them.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		sanitized = append(sanitized, h.sanitizeAttr(a))
	}
	return &RedactingHandler{
		inner: h.inner.WithAttrs(sanitized),
		keys:  h.keys,
	}
}

// WithGroup derives a new handler that operates within a named group.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner: h.inner.WithGroup(name),
		keys:  h.keys,
	}
}

// sanitizeAttr redacts the value of a sensitive attribute and also recurses into
// grouped attributes to sanitize them.
func (h *RedactingHandler) sanitizeAttr(a slog.Attr) slog.Attr {
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

	if _, ok := h.keys[strings.ToLower(a.Key)]; ok {
		return slog.Attr{
			Key:   a.Key,
			Value: redactedBytes,
		}
	}

	return a
}
