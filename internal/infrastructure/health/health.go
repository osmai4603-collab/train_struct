package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// Pinger defines an interface for critical dependencies that must be verified during readiness probes.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthChecker manages the liveness and readiness state of the application service
// using atomic flags for concurrent safety.
type HealthChecker struct {
	alive atomic.Bool
	ready atomic.Bool
	deps  Pinger
}

// NewHealthChecker creates a HealthChecker instance.
// The service starts as alive, but NOT ready (until initialization and startup complete).
func NewHealthChecker(deps Pinger) *HealthChecker {
	hc := &HealthChecker{
		deps: deps,
	}
	hc.alive.Store(true)
	hc.ready.Store(false)
	return hc
}

// MarkReady signals that startup is complete and the service is ready to accept user traffic.
func (hc *HealthChecker) MarkReady() {
	hc.ready.Store(true)
}

// MarkNotReady signals that the service is draining or shutting down and should stop receiving traffic.
func (hc *HealthChecker) MarkNotReady() {
	hc.ready.Store(false)
}

// IsReady returns the current readiness status.
func (hc *HealthChecker) IsReady() bool {
	return hc.ready.Load()
}

// IsAlive returns the current liveness status.
func (hc *HealthChecker) IsAlive() bool {
	return hc.alive.Load()
}

// HandleLiveness probe (/livez):
// Invariant: ONLY inspects local process health. NEVER calls external dependencies.
// Checking databases or remote services here causes cascading container restarts during transient network blips.
func (hc *HealthChecker) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !hc.alive.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "dead",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "alive",
	})
}

// HandleReadiness probe (/readyz):
// Invariant: Verifies that the process is initialized, not draining, and all critical dependencies respond.
// Returning 503 instructs the load balancer / orchestrator to temporarily remove the instance from routing.
func (hc *HealthChecker) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !hc.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"reason": "server is draining or initializing",
		})
		return
	}

	if hc.deps != nil {
		if err := hc.deps.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "not_ready",
				"reason": "critical dependency check failed: " + err.Error(),
			})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}
