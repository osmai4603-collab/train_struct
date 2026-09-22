# Go Server Lifecycle — Detailed Reference

This document provides in-depth technical explanations of server lifecycle phases, synchronous pre-binding, professional process logging, signal handling, and clean shutdown.

---

## 1. Professional Structured Logging

In production, logs must be machine-readable (JSON) for aggregators like ELK, Loki, or Datadog. For local terminal development, humans need readable, high-contrast colored output.

### TTY Detection & Multi-Target Strategy

Use `github.com/mattn/go-isatty` to detect if the standard output/error is an interactive terminal:

- **TTY Mode**: Use a custom `slog.Handler` that formats output with ANSI colors and human-readable timestamps (`2006-01-02 03:04:05 PM`).
- **File Persistence (`logs/app.log`)**: Format as newline-delimited JSON for complete auditability.
- **Error Log File (`logs/error.log`)**: Duplicate warnings and errors (`Level >= WARN`) to a dedicated file.
- **Lifecycle Flush**: Flush buffers (`file.Sync()`) and close file descriptors during the cleanup phase.

---

## 2. Synchronous Pre-Binding (Fail-Fast)

> [!CAUTION]
> **Never call `srv.ListenAndServe()` directly in a background goroutine.**
> If the port is already bound (`bind: address already in use`), an asynchronous server fails late after database connections have already been established and background workers started.

Always bind the network listener synchronously in the main thread:

```go
ln, err := net.Listen("tcp", addr)
if err != nil {
    return fmt.Errorf("bind listener %s: %w", addr, err)
}
defer ln.Close()
```

When using a dual-server architecture (e.g. Public `:8070` and Management `:8066`), pre-bind both listeners sequentially before starting either server. If the second binding fails, immediately close the first listener and fail fast.

---

## 3. Log Suppression for Probes

Orchestrators (Kubernetes, AWS ECS) poll liveness (`/livez`) and readiness (`/readyz`) every few seconds. Logging every 200 OK probe poll creates massive log bloat and drowns actual application events.

**Best Practice**:

- Do NOT log healthy (`Status < 400`) requests to probe endpoints.
- DO log errors (`Status >= 400`) immediately as `WARN` or `ERROR` to ensure degraded dependencies or broken instances are promptly identified.

---

## 4. The 8-Phase Operational Flow

| Phase | Action | Purpose |
| :--- | :--- | :--- |
| **1. Init** | Validate config, init logger, connect deps | Fail-Fast if environment or deps are broken |
| **2. Pre-Bind** | `net.Listen("tcp", addr)` synchronously | Reserve ports before launching goroutines |
| **3. Config** | Router setup, explicit timeouts | Prevent slowloris and stalled connections |
| **4. Startup** | Non-blocking `srv.Serve(ln)`, start workers | Enter active execution state |
| **5. Serving** | Handle requests, serve `/livez` & `/readyz` | Main steady-state serving |
| **6. Drain** | Mark ready=false, sleep drain duration | Allow load balancers to detach cleanly |
| **7. Shutdown** | `srv.Shutdown(ctx)` with bounded timeout | Drain in-flight requests without dropping connections |
| **8. Cleanup** | Stop workers, close DB pools, flush logs | Reverse-order teardown with zero leaks |

---

## 5. POSIX Signal Handling & Shell Traps

### POSIX Signal Compatibility

When stopping servers via Makefiles or shell scripts running under minimal environments (e.g. Dash, Alpine `/bin/sh`), **never use `kill -SIGTERM`**. The `SIG` prefix is non-standard in POSIX shells and causes `Illegal option -S`.
Always use:

```sh
kill -TERM $$pid
# or
kill -15 $$pid
```

### The "go run" Signal Trap

Never use `go run` in production or Makefiles. `go run` compiles and executes the binary as a child process. Termination signals sent to `go run` are often swallowed or cause the parent process to exit abruptly, leaving the child process orphaned and occupying network ports (`address already in use`). Always build the binary (`go build -o bin/server`) and run it directly.
