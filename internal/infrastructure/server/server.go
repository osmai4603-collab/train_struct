package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"train/internal/infrastructure/health"
	"train/internal/infrastructure/worker"
	"train/internal/platform/config"
)

// Server coordinates the complete HTTP server lifecycle across the operational
// phases on a single public listener.
//
// Phase 2: Pre-Binding   — the TCP listener is claimed synchronously on the
//
//	calling goroutine before readiness is announced (fail-fast on port conflict).
//
// Phase 4: Startup       — Serve runs on the pre-bound listener, then readiness
//
//	is marked and background workers may be launched.
//
// Phase 6: Drain         — on signal or fatal error, readiness is flipped to
//
//	false and the configured drain period lets load balancers re-route traffic.
//
// Phase 7: Teardown      — http.Server.Shutdown runs under a bounded context.
//
// Phase 8: Cleanup       — background workers stop, then resources close in the
//
//	exact reverse order of construction.
type Server struct {
	httpServer *http.Server
	cfg        config.ServerSettings
	health     *health.HealthChecker
	workerMgr  *worker.Manager
	resources  []io.Closer
	logger     *slog.Logger
	listener   atomic.Pointer[net.Listener]
}

// New constructs a Server with explicit, non-zero network timeouts to resist
// slowloris attacks and resource exhaustion. ReadHeaderTimeout and IdleTimeout
// default to the skill-recommended 2s and 120s via configuration defaults.
func New(
	cfg config.ServerSettings,
	handler http.Handler,
	healthChecker *health.HealthChecker,
	workerMgr *worker.Manager,
	logger *slog.Logger,
	resources ...io.Closer,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	srv := &http.Server{
		Addr:              cfg.Address(),
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout.Duration(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout.Duration(),
		WriteTimeout:      cfg.WriteTimeout.Duration(),
		IdleTimeout:       cfg.IdleTimeout.Duration(),
		MaxHeaderBytes:    int(cfg.MaxHeaderBytes.Bytes()),
	}

	return &Server{
		httpServer: srv,
		cfg:        cfg,
		health:     healthChecker,
		workerMgr:  workerMgr,
		resources:  resources,
		logger:     logger,
	}
}

// SetListener injects an externally bound listener (used by tests and dynamic
// port assignment). Calling Run without SetListener binds the configured address.
func (srv *Server) SetListener(ln net.Listener) {
	srv.listener.Store(&ln)
	srv.httpServer.Addr = ln.Addr().String()
}

// Addr returns the effective bound address, falling back to the configured one
// when Run has not yet applied a listener.
func (srv *Server) Addr() string {
	if lnPtr := srv.listener.Load(); lnPtr != nil {
		return (*lnPtr).Addr().String()
	}
	return srv.httpServer.Addr
}

// Run executes the server through its complete lifecycle until ctx is canceled
// or Serve returns a fatal error. A fatal serving error still drains and shuts
// down cleanly; the process never leaves the listener half-closed.
func (srv *Server) Run(ctx context.Context) error {
	startedAt := time.Now()

	// Phase 2 & 4a: synchronous pre-binding.
	// The listener is reserved here, in the caller's goroutine, before any
	// background work begins or readiness is advertised.
	var publicLn net.Listener
	if lnPtr := srv.listener.Load(); lnPtr != nil {
		publicLn = *lnPtr
	} else {
		bound, err := Bind(srv.cfg.Address())
		if err != nil {
			return err
		}
		publicLn = bound
		srv.listener.Store(&bound)
	}
	srv.httpServer.Addr = publicLn.Addr().String()

	// Phase 4b: serve over the pre-bound listener, monitoring fatal errors.
	serverErr := make(chan error, 1)
	var serveWG sync.WaitGroup
	serveWG.Add(1)
	go func() {
		defer serveWG.Done()
		srv.logger.Info("http server serving", slog.String("addr", publicLn.Addr().String()))
		if err := srv.httpServer.Serve(publicLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srv.pushError(serverErr, err)
		}
	}()

	if srv.health != nil {
		srv.health.MarkReady()
	}
	srv.logger.Info("server is ready to accept traffic")

	// Phase 5: serving — wait for a termination signal or a fatal server error.
	shutdownReason := "signal"
	select {
	case err := <-serverErr:
		srv.logger.Error("http server fatal error", slog.Any("error", err))
		shutdownReason = "server_failure"
	case <-ctx.Done():
		srv.logger.Info("shutdown signal received")
	}

	// Phase 6: drain — stop advertising readiness so the load balancer re-routes,
	// then wait out the configured period before closing connections.
	if srv.health != nil {
		srv.health.MarkNotReady()
	}
	srv.logger.Info("drain phase started",
		slog.Duration("drain_duration", srv.cfg.DrainDuration.Duration()),
		slog.String("shutdown_reason", shutdownReason),
	)
	if srv.cfg.DrainDuration.Duration() > 0 {
		time.Sleep(srv.cfg.DrainDuration.Duration())
	}
	srv.logger.Info("drain phase complete")

	// Phase 7: teardown — bounded graceful shutdown, forcing close on timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), srv.cfg.ShutdownTimeout.Duration())
	shutdownStarted := time.Now()
	if err := srv.httpServer.Shutdown(shutdownCtx); err != nil {
		srv.logger.Error("graceful shutdown failed — forcing server close", slog.Any("error", err))
		_ = srv.httpServer.Close()
	}
	shutdownCancel()
	srv.logger.Info("graceful shutdown completed",
		slog.Duration("duration", time.Since(shutdownStarted).Round(time.Millisecond)),
	)

	// Wait for the Serve goroutine to exit so no goroutines are leaked.
	serveWG.Wait()
	srv.logger.Info("http server stopped")

	// Phase 8: cleanup in strict reverse order of construction:
	// workers first, then external resources (DB pools, storage, etc.).
	if srv.workerMgr != nil {
		srv.workerMgr.StopAll()
	}
	for i := len(srv.resources) - 1; i >= 0; i-- {
		if srv.resources[i] == nil {
			continue
		}
		if err := srv.resources[i].Close(); err != nil {
			srv.logger.Error("failed to close resource",
				slog.Int("index", i),
				slog.Any("error", err),
			)
		}
	}

	srv.logger.Info("server exited cleanly",
		slog.String("shutdown_reason", shutdownReason),
		slog.Duration("uptime", time.Since(startedAt).Round(time.Millisecond)),
	)
	return nil
}

// pushError forwards a fatal serving error to the lifecycle channel without
// blocking, since only the first error is actionable.
func (srv *Server) pushError(serverErr chan<- error, err error) {
	select {
	case serverErr <- err:
	default:
	}
}
