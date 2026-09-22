package examples_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"train/.agents/skills/go-errors/examples"
	platformerr "train/internal/platform/errors"
)

func TestExampleValidation(t *testing.T) {
	dto := examples.RegisterDTO{
		Email:    "bad-email",
		Password: "short",
		Age:      16,
	}

	err := examples.ValidateRegisterDTO(dto)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	if !platformerr.Is(err, platformerr.CodeInvalid) {
		t.Errorf("expected INVALID code, got: %s", platformerr.ErrorCode(err))
	}

	var valErr *platformerr.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	if len(valErr.Violations) != 3 {
		t.Errorf("expected 3 violations, got %d", len(valErr.Violations))
	}
}

func TestExampleHTTPHandler(t *testing.T) {
	h := examples.NewUserHandler(nil)

	// 1. Request without ID
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	h.GetUser(w, r)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Result().StatusCode)
	}

	// 2. Request for a non-existent user
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/users?id=999", nil)
	h.GetUser(w, r)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Result().StatusCode)
	}
}

func TestExampleSafeCopyFile(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	dst := filepath.Join(tempDir, "dst.txt")

	if err := os.WriteFile(src, []byte("hello go errors"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := examples.SafeCopyFile(dst, src); err != nil {
		t.Fatalf("SafeCopyFile failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != "hello go errors" {
		t.Errorf("expected 'hello go errors', got %q", string(data))
	}
}

func TestExampleConcurrentProcessing(t *testing.T) {
	ctx := context.Background()
	items := []string{"item1", "invalid", "item2"}

	err := examples.ProcessItemsConcurrently(ctx, items)
	if err == nil {
		t.Fatal("expected error from invalid item, got nil")
	}
}
