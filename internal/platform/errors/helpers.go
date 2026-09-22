package errors

import (
	stdctx "context"
	"errors"

	platformctx "train/internal/platform/context"
)

// ErrorCode walks the error chain looking for the first error matching *Error that carries a non-empty code
// and returns CodeInternal by default when no specific code is found
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var appErr *Error
	if errors.As(err, &appErr) && appErr.Code != "" {
		return appErr.Code
	}

	return CodeInternal
}

// ErrorMessage extracts the safe end-user message from the error chain
// and returns a generic safe message when no custom message is available
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var appErr *Error
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}

	return "An internal error occurred. Please try again later."
}

// ErrorOp extracts the last logical operation name recorded in the error chain
func ErrorOp(err error) string {
	if err == nil {
		return ""
	}

	var appErr *Error
	if errors.As(err, &appErr) && appErr.Op != "" {
		return appErr.Op
	}

	return ""
}

// ErrorRequestID extracts the request ID associated with the error, if any
func ErrorRequestID(err error) string {
	if err == nil {
		return ""
	}

	var appErr *Error
	if errors.As(err, &appErr) && appErr.RequestID != "" {
		return appErr.RequestID
	}

	return ""
}

// Is checks whether the error code matches the target code
func Is(err error, code string) bool {
	return ErrorCode(err) == code
}

// FromContext attaches the request ID extracted from the context to the error to unify log tracing
func FromContext(ctx stdctx.Context, err error) error {
	if err == nil || ctx == nil {
		return err
	}

	reqID, ok := platformctx.RequestIDFromContext(ctx)
	if !ok || reqID == "" {
		return err
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		if appErr.RequestID == "" {
			appErr.RequestID = reqID
		}
		return err
	}

	return &Error{
		Code:      ErrorCode(err),
		Err:       err,
		RequestID: reqID,
	}
}
