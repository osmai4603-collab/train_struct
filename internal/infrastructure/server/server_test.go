package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"train/internal/infrastructure/health"
	"train/internal/infrastructure/worker"
	"train/internal/platform/config"
	"train/internal/platform/logger"
)

type noopPinger struct{}

func (noopPinger) Ping(ctx context.Context) error { return nil }

type closeOrder struct {
	mu    sync.Mutex
	names []string
}

func (o *closeOrder) closer(name string) io.Closer {
	return closerFunc(func() error {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.names = append(o.names, name)
		return nil
	})
}

func (o *closeOrder) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]string, len(o.names))
	copy(out, o.names)
	return out
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func testServerSettings() config.ServerSettings {
	return config.ServerSettings{
		Host:              "127.0.0.1",
		Port:              "0",
		ReadTimeout:       config.Duration(5 * time.Second),
		ReadHeaderTimeout: config.Duration(2 * time.Second),
		WriteTimeout:      config.Duration(10 * time.Second),
		IdleTimeout:       config.Duration(2 * time.Second),
		ShutdownTimeout:   config.Duration(3 * time.Second),
		DrainDuration:     config.Duration(20 * time.Millisecond),
		MaxHeaderBytes:    config.Megabyte,
	}
}

func TestNew_SetsStrictTimeouts(t *testing.T) {
	s := New(config.DefaultConfiguration().Server, http.NewServeMux(), health.NewHealthChecker(noopPinger{}), nil, logger.NewTestLogger(t))

	if s.httpServer.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout = %v, want 5s", s.httpServer.ReadTimeout)
	}
	if s.httpServer.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 2s", s.httpServer.ReadHeaderTimeout)
	}
	if s.httpServer.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %v, want 10s", s.httpServer.WriteTimeout)
	}
	if s.httpServer.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %v, want 120s", s.httpServer.IdleTimeout)
	}
	if s.httpServer.MaxHeaderBytes != int(config.Megabyte.Bytes()) {
		t.Fatalf("MaxHeaderBytes = %d, want %d", s.httpServer.MaxHeaderBytes, int(config.Megabyte.Bytes()))
	}
}

func TestBind_ConflictFailsFast(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	if _, err := Bind(ln.Addr().String()); err == nil {
		t.Fatal("expected bind error on an already-occupied port")
	}
}

func TestServer_RunLifecycle(t *testing.T) {
	cfg := testServerSettings()
	hc := health.NewHealthChecker(noopPinger{})
	wm := worker.NewManager(logger.NewTestLogger(t))

	var order closeOrder
	closerDB := order.closer("db")
	closerCache := order.closer("cache")

	routes := http.NewServeMux()
	routes.HandleFunc("/livez", hc.HandleLiveness)
	routes.HandleFunc("/readyz", hc.HandleReadiness)
	routes.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	s := New(cfg, routes, hc, wm, logger.NewTestLogger(t), closerDB, closerCache, nil)
	s.SetListener(ln)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- s.Run(ctx) }()

	base := "http://" + ln.Addr().String()
	waitForReady(t, base)

	resp, err := http.Get(base + "/ping")
	if err != nil {
		t.Fatalf("request during serving: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 from live handler, got %d", resp.StatusCode)
	}

	// Trigger the shutdown path: not-ready -> drain -> shutdown -> cleanup.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("clean shutdown expected, got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}

	if hc.IsReady() {
		t.Fatal("expected readiness to be false after drain phase")
	}

	got := order.snapshot()
	want := []string{"cache", "db"} // reverse order of construction
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("resource close order = %v, want %v", got, want)
	}
}

func waitForReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not become ready in time")
}
