package logger

import (
	"fmt"
	"log/slog"
)

// redactedValue is the unified safe value shown in place of any sensitive data.
const redactedValue = "***"

// SecretString is a custom type that fully redacts the sensitive string from
// structured logs and any other textual output, while the real value remains
// available only via Expose for internal use.
type SecretString string

// LogValue implements the slog.LogValuer interface to redact the value in structured logs.
func (s SecretString) LogValue() slog.Value {
	return slog.StringValue(redactedValue)
}

// String implements the fmt.Stringer interface to prevent leakage via plain printing and formatting.
func (s SecretString) String() string {
	return redactedValue
}

// GoString implements the fmt.GoStringer interface to prevent leakage via the %#v format.
func (s SecretString) GoString() string {
	return "SecretString(***)"
}

// Format prevents the value from leaking through arbitrary formatting functions.
func (s SecretString) Format(f fmt.State, c rune) {
	_, _ = f.Write([]byte(redactedValue))
}

// Expose grants access to the real value exclusively to authorized business logic.
func (s SecretString) Expose() string {
	return string(s)
}

// Sensitive is a generic container that wraps any value and automatically
// prevents its leakage through slog and the printing packages, ensuring secrets
// are protected at the type level rather than relying on developer attention.
type Sensitive[T any] struct {
	value T
}

// NewSensitive creates a new safe container wrapping the given value.
func NewSensitive[T any](v T) Sensitive[T] {
	return Sensitive[T]{value: v}
}

// LogValue implements the slog.LogValuer interface to fully redact the value in structured logs.
func (s Sensitive[T]) LogValue() slog.Value {
	return slog.StringValue(redactedValue)
}

// String implements the fmt.Stringer interface to prevent leakage via printing and formatting packages.
func (s Sensitive[T]) String() string {
	return redactedValue
}

// GoString implements the fmt.GoStringer interface to prevent leakage via the %#v format.
func (s Sensitive[T]) GoString() string {
	return "Sensitive(***)"
}

// Format prevents the value from leaking when custom formatting functions are used.
func (s Sensitive[T]) Format(f fmt.State, c rune) {
	_, _ = f.Write([]byte(redactedValue))
}

// Value retrieves the original protected value for authorized internal use.
func (s Sensitive[T]) Value() T {
	return s.value
}
