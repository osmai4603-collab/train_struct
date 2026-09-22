package app

import (
	"strings"
	"testing"
)

// Bootstrap uses a local (non-TTY) writer and never persists to disk in tests,
// so each invocation passes --log-to-file=false to keep the repo clean.
// Equals-form flag syntax is mandatory for bool flags (Go flag stops parsing at
// the first non-flag positional argument).
var testArgs = []string{
	"--log-to-file=false",
	"--log-color=false",
}

func TestBootstrap_MemoryDriver(t *testing.T) {
	a := New()
	args := append(append([]string{}, testArgs...), "--db-driver", "memory")

	if err := a.Bootstrap(args); err != nil {
		t.Fatalf("Bootstrap(memory) failed: %v", err)
	}

	if a.cfg == nil {
		t.Fatal("expected configuration to be loaded")
	}
	if a.logger == nil {
		t.Fatal("expected logger to be initialized")
	}
	if a.health == nil {
		t.Fatal("expected health checker to be initialized")
	}
	if a.workerMgr == nil {
		t.Fatal("expected worker manager to be initialized")
	}
	if a.srv == nil {
		t.Fatal("expected server to be constructed")
	}
	if a.pool != nil {
		t.Fatal("expected no database pool for memory driver")
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestBootstrap_UnsupportedDriverFailsFast(t *testing.T) {
	a := New()
	args := append(append([]string{}, testArgs...), "--db-driver", "cassandra")

	err := a.Bootstrap(args)
	if err == nil {
		t.Fatal("expected bootstrap to fail for unsupported driver")
	}
	if !strings.Contains(err.Error(), "cassandra") {
		t.Fatalf("expected error to mention driver, got: %v", err)
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Close after failed bootstrap failed: %v", err)
	}
}

func TestRun_WithoutBootstrapFails(t *testing.T) {
	a := New()
	if err := a.Run(); err == nil {
		t.Fatal("expected Run before Bootstrap to return an error")
	}
}
