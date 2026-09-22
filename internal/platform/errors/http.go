package errors

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	platformctx "train/internal/platform/context"
)

// HTTPStatusCode converts the logical error code to the appropriate HTTP status code
func HTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	code := ErrorCode(err)
	switch code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeInvalid:
		return http.StatusBadRequest
	case CodeConflict:
		return http.StatusConflict
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeBadGateway:
		return http.StatusBadGateway
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// HTTPErrorResponse represents the safe error response structure for the end user
type HTTPErrorResponse struct {
	Code       string           `json:"code"`
	Message    string           `json:"message"`
	RequestID  string           `json:"request_id,omitempty"`
	Violations []FieldViolation `json:"violations,omitempty"`
}

// WriteHTTPError translates the error, writes it as a safe JSON response, and logs it via log/slog
func WriteHTTPError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	if err == nil {
		return
	}

	status := HTTPStatusCode(err)
	code := ErrorCode(err)
	userMessage := ErrorMessage(err)

	reqID := ErrorRequestID(err)
	if reqID == "" && r != nil {
		if id, ok := platformctx.RequestIDFromContext(r.Context()); ok {
			reqID = id
		}
	}

	// Extract validation details if present
	var violations []FieldViolation
	var valErr *ValidationError
	if errors.As(err, &valErr) && len(valErr.Violations) > 0 {
		violations = valErr.Violations
	}

	// Log the error with context
	if logger != nil && r != nil {
		ctx := r.Context()
		if status >= http.StatusInternalServerError {
			logger.ErrorContext(ctx, "http server error",
				slog.String("code", code),
				slog.Int("status", status),
				slog.String("request_id", reqID),
				slog.String("path", r.URL.Path),
				slog.Any("error", err),
			)
		} else {
			logger.WarnContext(ctx, "http client rejection",
				slog.String("code", code),
				slog.Int("status", status),
				slog.String("request_id", reqID),
				slog.String("path", r.URL.Path),
				slog.Any("error", err),
			)
		}
	}

	resp := HTTPErrorResponse{
		Code:       code,
		Message:    userMessage,
		RequestID:  reqID,
		Violations: violations,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
