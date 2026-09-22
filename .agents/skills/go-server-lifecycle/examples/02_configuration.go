package lifecycle

import (
	"net/http"
	"time"
)

// =============================================================================
// Phase 2: Configuration
// =============================================================================
// CRITICAL: Never use a zero-value http.Server.
// Always set all four timeout fields to prevent resource exhaustion.
//
// Official source: https://pkg.go.dev/net/http#Server
// =============================================================================

// NewServer creates a properly configured http.Server with all required
// timeouts set according to official Go documentation.
//
// Timeout Diagram:
//
//	Client -> [Accept] -> [Read Headers] -> [Read Body] -> [Handler] -> [Write] -> [Idle]
//	          |<-------- ReadTimeout -------->|                          |          |
//	          |<- ReadHeaderTimeout ->|                                  |          |
//	                                                     |<-- WriteTimeout ------->|
//	                                                                    |<- IdleTimeout ->|
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		// Addr is the TCP address to listen on.
		Addr: addr,

		// Handler is the http.Handler to invoke. nil means http.DefaultServeMux.
		Handler: handler,

		// ReadTimeout is the maximum duration for reading the entire request,
		// including the body. It starts counting after the connection is accepted.
		// A zero value means no timeout — DANGEROUS in production.
		ReadTimeout: 5 * time.Second,

		// ReadHeaderTimeout is the amount of time allowed to read request headers.
		// This is critical protection against Slowloris attacks where a client
		// sends headers very slowly to hold connections open.
		ReadHeaderTimeout: 2 * time.Second,

		// WriteTimeout is the maximum duration before timing out writes of the
		// response. It is reset after the request header is read.
		// For HTTPS, this includes the TLS handshake time.
		WriteTimeout: 10 * time.Second,

		// IdleTimeout is the maximum time to wait for the next request when
		// keep-alives are enabled. If zero, the value of ReadTimeout is used.
		// If both are zero, there is no timeout — DANGEROUS in production.
		IdleTimeout: 120 * time.Second,

		// MaxHeaderBytes controls the maximum number of bytes the server will
		// read parsing the request header's keys and values.
		// Default is 1 MB (1 << 20).
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}

// IMPORTANT: These timeouts control NETWORK I/O only.
// They do NOT cancel handler logic (e.g., slow DB queries).
//
// For handler-level timeouts, use one of these approaches:
//
// Approach 1: http.TimeoutHandler (middleware)
//
//	handler := http.TimeoutHandler(yourHandler, 30*time.Second, "request timeout")
//
// Approach 2: context.WithTimeout inside the handler
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
//	    defer cancel()
//	    // use ctx for DB calls, HTTP calls, etc.
//	}
