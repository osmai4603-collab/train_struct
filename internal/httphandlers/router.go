package httphandlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"train/internal/infrastructure/health"
	platformctx "train/internal/platform/context"
)

// NewRouter builds the public HTTP route table. It wires the health probes
// (/livez, /readyz), a minimal service ping endpoint, and a structured 404
// fallback for unknown routes.
//
// The composition root is expected to wrap the returned handler with the
// platform context middleware (request_id correlation) and the server logging
// middleware (status-routed levels + probe suppression).
func NewRouter(logger *slog.Logger, hc *health.HealthChecker) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()

	// Operational probes — isolated from business traffic and absent 404ed.
	mux.HandleFunc("GET /livez", hc.HandleLiveness)
	mux.HandleFunc("GET /readyz", hc.HandleReadiness)

	// Minimal public API surface.
	mux.HandleFunc("GET /api/v1/ping", handlePing(logger))

	// Structured 404 for unknown routes (operational endpoints are not exposed).
	mux.HandleFunc("/", handleNotFound)

	return mux
}

func handlePing(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "train",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	reqID, _ := platformctx.RequestIDFromContext(r.Context())
	writeJSON(w, http.StatusNotFound, map[string]any{
		"error":  "not_found",
		"detail": "route not found",
		"path":   r.URL.Path,
		"trace":  reqID,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
