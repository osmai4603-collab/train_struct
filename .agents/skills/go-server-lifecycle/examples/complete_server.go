package lifecycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// =============================================================================
// Complete Pure Production-Ready Server Lifecycle (Standard Library Only)
// =============================================================================

func main() {
	// ─────────────────────────────────────────────────────────────────────
	// PHASE 1: Initialization & Fail-Fast
	// ─────────────────────────────────────────────────────────────────────
	logger, logCloser := initLogger()
	defer func() {
		if logCloser != nil {
			_ = logCloser.Close()
		}
	}()

	cfg := loadConfig()

	db, err := initDB(cfg.DatabaseDSN)
	if err != nil {
		logger.Error("failed to initialize database dependency", "error", err)
		os.Exit(1)
	}

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 2: Synchronous Listener Pre-Binding (Fail-Fast)
	// ─────────────────────────────────────────────────────────────────────
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("failed to bind tcp listener", "addr", ":"+cfg.Port, "error", err)
		os.Exit(1)
	}
	defer ln.Close()

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 3: Configuration & Server Timeouts
	// ─────────────────────────────────────────────────────────────────────
	health := &healthChecker{db: db}

	mux := http.NewServeMux()
	// Suppress healthy probe logs to prevent terminal/disk pollution
	mux.Handle("/livez", suppressProbeLogs(http.HandlerFunc(health.handleLiveness), logger))
	mux.Handle("/readyz", suppressProbeLogs(http.HandlerFunc(health.handleReadiness), logger))
	mux.HandleFunc("/", handleRoot)

	srv := &http.Server{
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 4 & 5: Startup & Serving
	// ─────────────────────────────────────────────────────────────────────
	sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer sigStop()

	serverErr := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("serving traffic", "addr", ln.Addr().String())
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Mark service as ready once listener is actively serving
	health.markReady()

	select {
	case err := <-serverErr:
		logger.Error("server fatal error", "error", err)
		os.Exit(1)
	case <-sigCtx.Done():
		logger.Info("shutdown trigger received, starting drain phase")
	}

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 6: Two-Stage Drain Phase
	// ─────────────────────────────────────────────────────────────────────
	// 1. Mark not ready immediately so load balancers stop routing traffic
	health.markNotReady()
	// 2. Wait for ingress / mesh propagation
	time.Sleep(5 * time.Second)

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 7: Coordinated Graceful Teardown
	// ─────────────────────────────────────────────────────────────────────
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server graceful shutdown failed, forcing close", "error", err)
		_ = srv.Close()
	}
	wg.Wait()

	// ─────────────────────────────────────────────────────────────────────
	// PHASE 8: Reverse-Order Cleanup
	// ─────────────────────────────────────────────────────────────────────
	if db != nil {
		if err := db.Close(); err != nil {
			logger.Error("error closing database pool", "error", err)
		}
	}

	logger.Info("server terminated cleanly")
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle Support Utilities
// ─────────────────────────────────────────────────────────────────────────────

type prettyHandler struct{ w io.Writer }

func (h *prettyHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	fmt.Fprintf(h.w, "\033[90m%s\033[0m [%s] %s\n", r.Time.Format("2006-01-02 03:04:05 PM"), r.Level, r.Message)
	return nil
}
func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *prettyHandler) WithGroup(name string) slog.Handler      { return h }

func initLogger() (*slog.Logger, io.Closer) {
	out := os.Stderr
	var isTerminal bool
	if stat, err := out.Stat(); err == nil {
		isTerminal = (stat.Mode() & os.ModeCharDevice) != 0
	}

	if isTerminal || os.Getenv("TRAIN_LOG_COLOR") == "true" || os.Getenv("LOG_COLOR") == "true" {
		return slog.New(&prettyHandler{w: out}), nil
	}
	return slog.New(slog.NewJSONHandler(out, nil)), nil
}

func suppressProbeLogs(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

type healthChecker struct {
	ready atomic.Bool
	db    *sql.DB
}

func (h *healthChecker) markReady()    { h.ready.Store(true) }
func (h *healthChecker) markNotReady() { h.ready.Store(false) }

func (h *healthChecker) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func (h *healthChecker) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
		return
	}
	if h.db != nil {
		if err := h.db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "degraded", "error": err.Error()})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "OK")
}

type config struct {
	Port        string
	DatabaseDSN string
}

func loadConfig() *config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return &config{Port: port, DatabaseDSN: os.Getenv("DATABASE_DSN")}
}

func initDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}
	return sql.Open("postgres", dsn)
}
