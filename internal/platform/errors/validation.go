package errors

import (
	"fmt"
	"log/slog"
	"strings"
)

// FieldViolation represents the invalidity details of a specific input field
type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// ValidationError represents a custom, structured error for collecting all data validation violations
type ValidationError struct {
	Entity     string           `json:"entity"`
	Violations []FieldViolation `json:"violations"`
}

// NewValidationError creates an empty validation error object for a specific entity
func NewValidationError(entity string) *ValidationError {
	return &ValidationError{
		Entity:     entity,
		Violations: make([]FieldViolation, 0),
	}
}

// AddViolation adds a new violation for a specific field
func (e *ValidationError) AddViolation(field, description string) {
	e.Violations = append(e.Violations, FieldViolation{
		Field:       field,
		Description: description,
	})
}

// HasViolations checks whether there are any recorded violations
func (e *ValidationError) HasViolations() bool {
	return len(e.Violations) > 0
}

// Error prints a summary of the validation violations
func (e *ValidationError) Error() string {
	if len(e.Violations) == 0 {
		return fmt.Sprintf("validation failed for %s", e.Entity)
	}

	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = fmt.Sprintf("%s: %s", v.Field, v.Description)
	}

	return fmt.Sprintf("validation failed for %s: [%s]", e.Entity, strings.Join(msgs, ", "))
}

// AsError converts the validation error into a unified *Error object with the INVALID code set
func (e *ValidationError) AsError(op string) *Error {
	return &Error{
		Op:      op,
		Code:    CodeInvalid,
		Message: e.Error(),
		Err:     e,
	}
}

// LogValue implements the log/slog.LogValuer interface
func (e *ValidationError) LogValue() slog.Value {
	violationsAttrs := make([]any, len(e.Violations))
	for i, v := range e.Violations {
		violationsAttrs[i] = slog.Group("violation",
			slog.String("field", v.Field),
			slog.String("description", v.Description),
		)
	}

	return slog.GroupValue(
		slog.String("entity", e.Entity),
		slog.Int("count", len(e.Violations)),
		slog.Any("violations", violationsAttrs),
	)
}
