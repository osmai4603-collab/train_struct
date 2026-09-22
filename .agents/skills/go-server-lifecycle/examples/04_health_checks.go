package lifecycle

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// =============================================================================
// Phase 4: Pure Lifecycle Health Probes
// =============================================================================
// Probes:
// - Liveness  → /livez  → Is the process running and responsive? (No external deps)
// - Readiness → /readyz → Is the service ready to receive traffic? (Check critical deps)
// =============================================================================

// Pingable is an interface for critical dependencies (e.g. database pool)
type Pingable interface {
	PingContext(ctx context.Context) error
}

type HealthChecker struct {
	ready atomic.Bool
	alive atomic.Bool
	db    Pingable
}

func NewHealthChecker(db Pingable) *HealthChecker {
	hc := &HealthChecker{db: db}
	hc.alive.Store(true)
	hc.ready.Store(false)
	return hc
}

func (hc *HealthChecker) MarkReady()    { hc.ready.Store(true) }
func (hc *HealthChecker) MarkNotReady() { hc.ready.Store(false) }

func (hc *HealthChecker) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /livez", hc.handleLiveness)
	mux.HandleFunc("GET /readyz", hc.handleReadiness)
}

func (hc *HealthChecker) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if !hc.alive.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func (hc *HealthChecker) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !hc.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
		return
	}

	// Verify critical downstream dependencies before claiming traffic readiness
	if hc.db != nil {
		if err := hc.db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "degraded", "error": err.Error()})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
