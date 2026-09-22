package lifecycle

import (
	"log/slog"
	"time"
)

// =============================================================================
// Phase 6: Drain Phase
// =============================================================================
// When running behind a load balancer (Kubernetes, AWS ALB, etc.),
// you must give the LB time to stop routing traffic to this server
// BEFORE shutting it down.
//
// Without drain phase:
//   Signal → Shutdown → Requests drop (LB still sends traffic)
//
// With drain phase:
//   Signal → Mark not-ready → Wait → Shutdown → No drops
// =============================================================================

// DrainAndPrepareShutdown executes the drain phase:
// 1. Mark the server as not ready (returns 503 on /readyz)
// 2. Wait for the load balancer to remove this server from rotation
// 3. Return — caller should then call Shutdown()
//
// drainTimeout is how long to wait for the LB to update.
// Typical values: 5-15 seconds depending on your LB health check interval.
func DrainAndPrepareShutdown(hc *HealthChecker, drainTimeout time.Duration, logger *slog.Logger) {
	// Step 1: Mark as not ready.
	// The readiness endpoint (/readyz) will now return 503.
	// The load balancer's next health check will see this and stop
	// sending new traffic to this server.
	hc.MarkNotReady()
	logger.Info("server marked as not ready — drain phase started",
		"drain_timeout", drainTimeout.String(),
	)

	// Step 2: Wait for the load balancer to notice and update.
	// This sleep duration should be >= your LB's health check interval.
	//
	// Example intervals:
	//   - Kubernetes: default 10s
	//   - AWS ALB:    default 30s
	//   - Nginx:      configurable
	//
	// During this wait:
	//   - /readyz returns 503 → LB stops routing NEW requests
	//   - In-flight requests continue normally
	//   - /livez still returns 200 → pod is NOT restarted
	time.Sleep(drainTimeout)

	logger.Info("drain phase completed — proceeding to shutdown")
}

// Sequence in a production deployment:
//
// ┌──────────────────────────────────────────────────────────┐
// │  Time 0s    : SIGTERM received                          │
// │  Time 0s    : /readyz → 503 (MarkNotReady)              │
// │  Time 0-5s  : LB health check detects 503               │
// │  Time 5s    : LB stops routing new traffic here          │
// │  Time 5s    : server.Shutdown(ctx) called                │
// │  Time 5-15s : In-flight requests finish                  │
// │  Time 15s   : All resources cleaned up                   │
// │  Time 15s   : Process exits cleanly (exit code 0)        │
// └──────────────────────────────────────────────────────────┘
