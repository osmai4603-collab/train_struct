package errors

// E is a flexible constructor that creates an Error and sets its attributes based on the types of the passed arguments
// It supports: string (message or code), error (the original error), and known error codes
func E(op string, args ...any) *Error {
	e := &Error{Op: op}

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			switch v {
			case CodeInternal, CodeNotFound, CodeConflict, CodeInvalid,
				CodeUnauthorized, CodeForbidden, CodeRateLimited,
				CodeTimeout, CodeBadGateway, CodeUnavailable:
				e.Code = v
			default:
				if e.Message == "" {
					e.Message = v
				}
			}
		case *Error:
			e.Err = v
		case error:
			e.Err = v
		}
	}

	if e.Code == "" {
		if e.Err != nil {
			e.Code = ErrorCode(e.Err)
		} else {
			e.Code = CodeInternal
		}
	}

	return e
}

// NotFound creates a resource-not-found error (NOT_FOUND / 404)
func NotFound(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeNotFound, Message: msg, Err: err}
}

// Invalid creates a validation failure or invalid input error (INVALID / 400)
func Invalid(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeInvalid, Message: msg, Err: err}
}

// Conflict creates a state conflict error (CONFLICT / 409)
func Conflict(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeConflict, Message: msg, Err: err}
}

// Unauthorized creates an authentication failure error (UNAUTHORIZED / 401)
func Unauthorized(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeUnauthorized, Message: msg, Err: err}
}

// Forbidden creates a missing-permission error (FORBIDDEN / 403)
func Forbidden(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeForbidden, Message: msg, Err: err}
}

// RateLimited creates a request-rate-limit-exceeded error (RATE_LIMITED / 429)
func RateLimited(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeRateLimited, Message: msg, Err: err}
}

// Timeout creates an operation timeout error (TIMEOUT / 504)
func Timeout(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeTimeout, Message: msg, Err: err}
}

// Internal creates an internal server or infrastructure error (INTERNAL / 500)
func Internal(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeInternal, Message: msg, Err: err}
}

// BadGateway creates an external dependency failure error (BAD_GATEWAY / 502)
func BadGateway(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeBadGateway, Message: msg, Err: err}
}

// Unavailable creates a temporarily unavailable service error (UNAVAILABLE / 503)
func Unavailable(op, msg string, err error) *Error {
	return &Error{Op: op, Code: CodeUnavailable, Message: msg, Err: err}
}
