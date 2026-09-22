package errors

import (
	"bytes"
	"fmt"
	"log/slog"
)

// Error is the central unified structure for representing errors in the project.
// It implements Ben Johnson's architectural pattern serving the three consumers:
//  1. The application (Code)
//  2. The end user (Message)
//  3. The engineer/operator (Op, Err, RequestID)
type Error struct {
	// Code is the fixed machine-readable code for the application and the HTTP handlers
	Code string

	// Message is the message addressed to the end user, safe and free of technical details
	Message string

	// Op represents the name of the operation or logical function where the failure occurred (Logical Stack Trace)
	Op string

	// Err is the original or internal wrapped error
	Err error

	// RequestID is the request trace ID extracted from the context to correlate the error with the log
	RequestID string
}

// Error prints the full logical path for engineers and logging systems
func (e *Error) Error() string {
	var b bytes.Buffer

	if e.Op != "" {
		fmt.Fprintf(&b, "%s: ", e.Op)
	}

	if e.Code != "" {
		fmt.Fprintf(&b, "<%s> ", e.Code)
	}

	if e.RequestID != "" {
		fmt.Fprintf(&b, "[req_id=%s] ", e.RequestID)
	}

	if e.Message != "" {
		fmt.Fprintf(&b, "%s", e.Message)
		if e.Err != nil {
			b.WriteString(": ")
		}
	}

	if e.Err != nil {
		b.WriteString(e.Err.Error())
	}

	return b.String()
}

// Unwrap provides compatibility with the standard errors.Is and errors.As inspection functions
func (e *Error) Unwrap() error {
	return e.Err
}

// LogValue implements the log/slog.LogValuer interface for high-performance structured logging
func (e *Error) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, 5)

	if e.Code != "" {
		attrs = append(attrs, slog.String("code", e.Code))
	}
	if e.Op != "" {
		attrs = append(attrs, slog.String("op", e.Op))
	}
	if e.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", e.RequestID))
	}
	if e.Message != "" {
		attrs = append(attrs, slog.String("message", e.Message))
	}
	if e.Err != nil {
		attrs = append(attrs, slog.String("cause", e.Err.Error()))
	}

	return slog.GroupValue(attrs...)
}

// WithRequestID sets the request ID on the error and returns the same object for convenient chaining
func (e *Error) WithRequestID(requestID string) *Error {
	e.RequestID = requestID
	return e
}
