// Package server implements the production HTTP server runtime and lifecycle
// for the service, following the go-server-lifecycle skill.
//
// Architectural position:
//   - Lives in the infrastructure layer (internal/infrastructure/server).
//   - Depends ONLY on platform packages (config, errors, context) and sibling
//     infrastructure packages (health, worker). It never imports application,
//     usecase, or domain logic — handlers are injected as plain http.Handler.
//
// Responsibilities:
//
//   - Constructing http.Server instances with strict, non-zero timeouts
//     (Read, ReadHeader, Write, Idle) to resist slowloris and resource
//     exhaustion attacks (Phase 3).
//
//   - Synchronous listener pre-binding via net.Listen in the main goroutine so
//     port conflicts fail fast BEFORE databases or workers are claimed (Phase 2).
//
//   - The eight-phase operational lifecycle (Run):
//
//     Phase 1: Initialization  (done upstream by the composition root)
//     Phase 2: Pre-Binding     (Bind)
//     Phase 3: Configuration   (New sets strict timeouts)
//     Phase 4: Startup         (Serve over pre-bound listener, mark ready)
//     Phase 5: Serving         (answer /livez, /readyz until signal or failure)
//     Phase 6: Drain           (mark not-ready, wait configured drain period)
//     Phase 7: Teardown        (http.Server.Shutdown with bounded timeout)
//     Phase 8: Cleanup         (stop workers, close resources in reverse order)
//
// HTTP request logging itself is owned by the platform logger package
// (internal/platform/logger.HTTPLoggingMiddleware): the composition root wraps
// the platform context middleware and the router with the logger middleware,
// so this package stays free of any HTTP-logging logic.
package server
