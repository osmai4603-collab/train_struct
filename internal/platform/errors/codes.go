package errors

// Standard error codes defined at the system and application level
const (
	// CodeInternal represents unexpected internal server or infrastructure errors (HTTP 500)
	CodeInternal = "INTERNAL"

	// CodeNotFound represents a failure to find the requested resource (HTTP 404)
	CodeNotFound = "NOT_FOUND"

	// CodeConflict represents a state conflict such as duplicate keys or a concurrency conflict (HTTP 409)
	CodeConflict = "CONFLICT"

	// CodeInvalid represents input validation failure or invalid parameters (HTTP 400)
	CodeInvalid = "INVALID"

	// CodeUnauthorized represents missing or invalid authentication data (HTTP 401)
	CodeUnauthorized = "UNAUTHORIZED"

	// CodeForbidden represents the user not having permission to perform the operation (HTTP 403)
	CodeForbidden = "FORBIDDEN"

	// CodeRateLimited represents exceeding the allowed request rate limit (HTTP 429)
	CodeRateLimited = "RATE_LIMITED"

	// CodeTimeout represents an operation timing out or context cancellation (HTTP 504)
	CodeTimeout = "TIMEOUT"

	// CodeBadGateway represents a failure to reach an external dependency service or third-party provider (HTTP 502)
	CodeBadGateway = "BAD_GATEWAY"

	// CodeUnavailable represents the service being temporarily unavailable due to maintenance or overload (HTTP 503)
	CodeUnavailable = "UNAVAILABLE"
)
