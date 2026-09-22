package examples

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"train/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterHealthEndpoints registers Kubernetes liveness and readiness probe handlers.
func RegisterHealthEndpoints(mux *http.ServeMux, pool *pgxpool.Pool, logger *slog.Logger) {
	checker := postgres.NewHealthChecker(pool)

	// Readiness probe: verifies active connectivity to PostgreSQL
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := checker.Check(r.Context()); err != nil {
			logger.WarnContext(r.Context(), "readiness check failed", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ready",
			"pool":   checker.Stats(),
		})
	})
}
