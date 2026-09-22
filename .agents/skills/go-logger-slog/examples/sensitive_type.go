package examples

import (
	"fmt"
	"log/slog"
)

// SecretString is a custom type for sensitive string values that prevents them from appearing in logs and normal printing
type SecretString string

// LogValue implements the slog.LogValuer interface to redact the value in structured logs
func (s SecretString) LogValue() slog.Value {
	return slog.StringValue("***")
}

// String implements the fmt.Stringer interface to prevent leakage when printed normally
func (s SecretString) String() string {
	return "***"
}

// Expose retrieves the real value only when business logic requires it
func (s SecretString) Expose() string {
	return string(s)
}

// Sensitive is a generic container that wraps any data type and prevents its leakage
type Sensitive[T any] struct {
	value T
}

// NewSensitive creates a new secure container
func NewSensitive[T any](v T) Sensitive[T] {
	return Sensitive[T]{value: v}
}

// LogValue implements the slog.LogValuer interface to fully redact the value
func (s Sensitive[T]) LogValue() slog.Value {
	return slog.StringValue("***")
}

// String implements the fmt.Stringer interface to prevent leakage via printing and formatting packages
func (s Sensitive[T]) String() string {
	return "***"
}

// GoString implements the fmt.GoStringer interface to prevent leakage via the %#v format
func (s Sensitive[T]) GoString() string {
	return "Sensitive(***)"
}

// Value retrieves the original protected value
func (s Sensitive[T]) Value() T {
	return s.value
}

// Format prevents the value from leaking when custom formatting functions are used
func (s Sensitive[T]) Format(f fmt.State, c rune) {
	_, _ = f.Write([]byte("***"))
}
