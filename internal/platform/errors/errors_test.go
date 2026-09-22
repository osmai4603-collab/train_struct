package errors_test

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformctx "train/internal/platform/context"
	platformerr "train/internal/platform/errors"
)

func TestError_FormatAndUnwrap(t *testing.T) {
	cause := errors.New("connection reset by peer")
	appErr := &platformerr.Error{
		Op:        "user.Find",
		Code:      platformerr.CodeNotFound,
		Message:   "user does not exist",
		Err:       cause,
		RequestID: "req-12345",
	}

	// 1. Check the composed error text
	errStr := appErr.Error()
	expectedSubstrings := []string{
		"user.Find:",
		"<NOT_FOUND>",
		"[req_id=req-12345]",
		"user does not exist",
		"connection reset by peer",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(errStr, sub) {
			t.Errorf("expected %q to contain %q", errStr, sub)
		}
	}

	// 2. Check unwrap (Unwrap) via errors.Is
	if !errors.Is(appErr, cause) {
		t.Errorf("expected errors.Is(appErr, cause) to be true")
	}

	// 3. Check type extraction via errors.As
	var extracted *platformerr.Error
	if !errors.As(appErr, &extracted) {
		t.Fatalf("expected errors.As to succeed")
	}
	if extracted.Code != platformerr.CodeNotFound {
		t.Errorf("expected code %s, got %s", platformerr.CodeNotFound, extracted.Code)
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *platformerr.Error
		wantCode   string
		wantStatus int
	}{
		{
			name:       "NotFound",
			err:        platformerr.NotFound("op.Get", "resource missing", nil),
			wantCode:   platformerr.CodeNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Invalid",
			err:        platformerr.Invalid("op.Create", "bad input", nil),
			wantCode:   platformerr.CodeInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Conflict",
			err:        platformerr.Conflict("op.Save", "already exists", nil),
			wantCode:   platformerr.CodeConflict,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "Unauthorized",
			err:        platformerr.Unauthorized("op.Auth", "missing token", nil),
			wantCode:   platformerr.CodeUnauthorized,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Forbidden",
			err:        platformerr.Forbidden("op.Check", "access denied", nil),
			wantCode:   platformerr.CodeForbidden,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "RateLimited",
			err:        platformerr.RateLimited("op.Call", "too many calls", nil),
			wantCode:   platformerr.CodeRateLimited,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "Timeout",
			err:        platformerr.Timeout("op.Wait", "operation timed out", nil),
			wantCode:   platformerr.CodeTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "BadGateway",
			err:        platformerr.BadGateway("op.Proxy", "upstream failure", nil),
			wantCode:   platformerr.CodeBadGateway,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "Unavailable",
			err:        platformerr.Unavailable("op.Ping", "maintenance mode", nil),
			wantCode:   platformerr.CodeUnavailable,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "Internal",
			err:        platformerr.Internal("op.Calc", "unexpected bug", nil),
			wantCode:   platformerr.CodeInternal,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := platformerr.ErrorCode(tt.err); code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, code)
			}
			if status := platformerr.HTTPStatusCode(tt.err); status != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, status)
			}
			if !platformerr.Is(tt.err, tt.wantCode) {
				t.Errorf("expected platformerr.Is to match %s", tt.wantCode)
			}
		})
	}
}

func TestFlexibleConstructorE(t *testing.T) {
	origErr := errors.New("db timeout")
	err := platformerr.E("repo.Find", platformerr.CodeTimeout, "could not fetch record", origErr)

	if err.Op != "repo.Find" {
		t.Errorf("unexpected op: %s", err.Op)
	}
	if err.Code != platformerr.CodeTimeout {
		t.Errorf("unexpected code: %s", err.Code)
	}
	if err.Message != "could not fetch record" {
		t.Errorf("unexpected message: %s", err.Message)
	}
	if !errors.Is(err, origErr) {
		t.Errorf("expected err to wrap origErr")
	}
}

func TestValidationError(t *testing.T) {
	valErr := platformerr.NewValidationError("UserRegistration")
	if valErr.HasViolations() {
		t.Errorf("expected HasViolations to be false initially")
	}

	valErr.AddViolation("email", "invalid format")
	valErr.AddViolation("password", "must be at least 8 characters")

	if !valErr.HasViolations() {
		t.Errorf("expected HasViolations to be true")
	}

	appErr := valErr.AsError("user.Register")
	if appErr.Code != platformerr.CodeInvalid {
		t.Errorf("expected code %s, got %s", platformerr.CodeInvalid, appErr.Code)
	}

	if !errors.Is(appErr, valErr) {
		t.Errorf("expected appErr to wrap valErr")
	}
}

func TestFromContext(t *testing.T) {
	ctx := platformctx.WithRequestID(stdctx.Background(), "trace-xyz-999")
	err := errors.New("something broke")

	enriched := platformerr.FromContext(ctx, err)
	reqID := platformerr.ErrorRequestID(enriched)
	if reqID != "trace-xyz-999" {
		t.Errorf("expected request_id trace-xyz-999, got %s", reqID)
	}
}

func TestWriteHTTPError(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/orders/123", nil)
	r = r.WithContext(platformctx.WithRequestID(r.Context(), "req-http-test"))

	valErr := platformerr.NewValidationError("Order")
	valErr.AddViolation("quantity", "must be greater than 0")
	appErr := valErr.AsError("order.Create")

	platformerr.WriteHTTPError(w, r, appErr, logger)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var jsonResp platformerr.HTTPErrorResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v", err)
	}

	if jsonResp.Code != platformerr.CodeInvalid {
		t.Errorf("expected code INVALID, got %s", jsonResp.Code)
	}
	if jsonResp.RequestID != "req-http-test" {
		t.Errorf("expected req_id req-http-test, got %s", jsonResp.RequestID)
	}
	if len(jsonResp.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(jsonResp.Violations))
	}
	if jsonResp.Violations[0].Field != "quantity" {
		t.Errorf("expected field quantity, got %s", jsonResp.Violations[0].Field)
	}

	// Verify the log entry
	if !strings.Contains(buf.String(), "http client rejection") {
		t.Errorf("expected log buffer to contain warning log")
	}
}

func TestNilSafety(t *testing.T) {
	if platformerr.ErrorCode(nil) != "" {
		t.Errorf("expected empty string for nil error code")
	}
	if platformerr.ErrorMessage(nil) != "" {
		t.Errorf("expected empty string for nil error message")
	}
	if platformerr.ErrorOp(nil) != "" {
		t.Errorf("expected empty string for nil error op")
	}
	if platformerr.ErrorRequestID(nil) != "" {
		t.Errorf("expected empty string for nil error requestID")
	}
	if platformerr.HTTPStatusCode(nil) != http.StatusOK {
		t.Errorf("expected StatusOK for nil error")
	}
	if platformerr.FromContext(nil, nil) != nil {
		t.Errorf("expected nil when both ctx and err are nil")
	}
}
