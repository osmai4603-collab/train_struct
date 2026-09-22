package examples

import (
	"encoding/json"
	"log/slog"
	"net/http"

	platformerr "train/internal/platform/errors"
)

// UserHandler represents an HTTP request handler for users
type UserHandler struct {
	logger *slog.Logger
}

// NewUserHandler creates a new handler
func NewUserHandler(logger *slog.Logger) *UserHandler {
	return &UserHandler{logger: logger}
}

// GetUser handles a fetch-user request and writes the response or the structured error
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		err := platformerr.Invalid("handler.GetUser", "query parameter 'id' is required", nil)
		platformerr.WriteHTTPError(w, r, err, h.logger)
		return
	}

	if id == "999" {
		err := platformerr.NotFound("handler.GetUser", "user not found", nil)
		platformerr.WriteHTTPError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":    id,
		"email": "user@example.com",
	})
}
