---
name: go-server-lifecycle
description: "Production-ready lifecycle management for Go HTTP services. Covers synchronous pre-binding (net.Listen), fail-fast initialization, dual-server architecture (public vs management isolation), strict server timeouts, health probe semantics (/livez and /readyz), probe log suppression, coordinated non-blocking startup, POSIX signal handling, two-stage traffic drain, graceful teardown (http.Server.Shutdown), background worker supervision, and reverse-order cleanup."
---

# Go Server Lifecycle Management Skill

This skill defines a production-ready lifecycle for Go HTTP services. It is intentionally
framework-agnostic and abstracted from internal business logic and deep telemetry, reflecting modern
cloud-native architectures with a composition root, HTTP adapter layer, infrastructure integration,
and a runtime control layer for health, workers, isolated management, and graceful shutdown.

The guidance below is designed for services that must survive startup failures,
traffic spikes, deployment rollouts, and OS termination signals without leaving requests hanging,
dropping ingress connections, or orphaning background workers.

## Production Principles

A correctly managed Go server is not just a server that boots. It is a service that:

- fails fast when required configuration or dependencies are invalid
- **binds listeners synchronously before claiming readiness or launching workers**
- **enforces security isolation between public business routes and internal management endpoints**
- exposes correct, isolated liveness (`/livez`) and readiness (`/readyz`) semantics
- starts background work only after the app and dependencies are ready
- handles OS termination signals reliably using POSIX-compliant patterns (`kill -TERM`, `signal.NotifyContext`)
- drains traffic cleanly before shutting down (two-stage drain)
- coordinates teardown across multiple servers and workers concurrently
- closes resources in a strict reverse order
- routes HTTP process log levels dynamically by status code (5xx -> ERROR, 4xx -> WARN) with high-contrast terminal styling
- persists structured JSON logs to rotating files while providing human-friendly colored terminal output
- suppresses 200 OK logs on health probes to eliminate log noise and disk inflation

## Official Sources & References

- [`net/http.Server`](https://pkg.go.dev/net/http#Server)
- [`net.Listen`](https://pkg.go.dev/net#Listen)
- [`Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown)
- [`os/signal`](https://pkg.go.dev/os/signal)
- [`context`](https://pkg.go.dev/context)
- [`sync.WaitGroup`](https://pkg.go.dev/sync#WaitGroup)
- [`log/slog`](https://pkg.go.dev/log/slog)

---

## Architectural Context: Dual-Server Topology & Security Isolation

In high-reliability Go services, public API traffic and operational management
must be physically isolated on distinct ports and listeners:

```text
┌──────────────────────────────────────────────────────────────────────────┐
│                         Composition / Bootstrap                          │
│  config, logger, synchronous pre-binding, signal handling, coordination  │
├────────────────────────────────────┬─────────────────────────────────────┤
│  - Public Server (e.g. :8070)      │    Management Server (e.g. :8066)   │
│  - Business APIs (/api/v1/*)       │  - Management Endpoints             │
│  - Public Probes (/livez, /readyz) │  - Diagnostic Endpoints             │
│  - 404 for Operational Endpoints   │  - Bearer Token Auth on 0.0.0.0     │
│  - User authentication & limits    │                                     │
├────────────────────────────────────┴─────────────────────────────────────┤
│                       Application / Use Case Layer                       │
├──────────────────────────────────────────────────────────────────────────┤
│                               Domain Layer                               │
├──────────────────────────────────────────────────────────────────────────┤
│                           Infrastructure Layer                           │
│  PostgreSQL pool, background workers, queues, email, external adapters   │
└──────────────────────────────────────────────────────────────────────────┘
```

The key invariants are:

- **Security Isolation**: Operational and diagnostic endpoints are blocked on the public port (return 404).
- **Synchronous Pre-Binding**: Ports are reserved synchronously in the main thread before starting background work.
- **Unified Coordination**: Startup, drain, and shutdown coordinate both servers together cleanly.

---

## Eight-Phase Operational Lifecycle

The server lifecycle should be treated as an ordered operational flow:

```text
Phase 1: Initialization  → validate config, initialize logger, connect infrastructure (fail fast)
Phase 2: Pre-Binding     → synchronously bind TCP listeners (net.Listen); fail fast on port conflict
Phase 3: Configuration   → configure server timeouts, isolate public and management routers
Phase 4: Startup         → start both servers on pre-bound listeners, launch background workers, set ready
Phase 5: Serving         → accept traffic, answer probes (/livez, /readyz), process requests
Phase 6: Drain           → mark not-ready, stop receiving new ingress traffic, wait drain period
Phase 7: Teardown        → call http.Server.Shutdown on both servers concurrently, wait for in-flight work
Phase 8: Cleanup         → stop background workers, close database pools and storage, flush logs
```

---

## Step 1: Fail-Fast Initialization & Multi-Target Logging

The process should be built in a strict order, starting with an environment-aware structured logger supporting multi-target output (terminal + files).

### 1. Multi-Target Structured Logging (Terminal + File Persistence)

A production service needs structured `JSON` logs persisted to disk for auditing and post-mortems, while developers need human-friendly colored logs in terminal sessions:

- **Terminal Output**: When running in a terminal (or when forced via `<APP_PREFIX>_LOG_COLOR=true`, e.g. `TRAIN_LOG_COLOR=true` or generic `LOG_COLOR=true`), use a custom `prettyHandler` with high-contrast ANSI colors.
- **Project-Specific Prefix Customization**: All environment flags (such as `<APP_PREFIX>_LOG_COLOR`, `<APP_PREFIX>_LOG_TO_FILE`, and `<APP_PREFIX>_LOG_DIR`) must adapt to the specific project name (e.g. `TRAIN_` for this service) with fallback to generic variables (`LOG_COLOR`, `LOG_TO_FILE`, `LOG_DIR`).
- **File Persistence (`logs/app.log`)**: Persist all events formatted as newline-delimited JSON.
- **Error Log File (`logs/error.log`)**: Filter and duplicate warnings and errors (`Level >= WARN`) to a dedicated file for rapid troubleshooting.
- **Lifecycle Flush**: Log files must be flushed (`Sync()`) and closed during Phase 8 (Cleanup).

```go
func initLogger() (*slog.Logger, io.Closer) {
    out := os.Stderr
    var writers []io.Writer
    var closers []io.Closer

    // Check terminal status and environment flag (adapt prefix to project, e.g. TRAIN_LOG_COLOR or LOG_COLOR)
    isTerminal := isatty.IsTerminal(out.Fd()) || 
        os.Getenv("TRAIN_LOG_COLOR") == "true" || 
        os.Getenv("LOG_COLOR") == "true"

    logToFile := os.Getenv("TRAIN_LOG_TO_FILE") != "false" && os.Getenv("LOG_TO_FILE") != "false"
    if logToFile || !isTerminal {
        logDir := os.Getenv("TRAIN_LOG_DIR")
        if logDir == "" {
            logDir = os.Getenv("LOG_DIR")
        }
        if logDir == "" {
            logDir = "logs"
        }
        _ = os.MkdirAll(logDir, 0755)
        appLog, err := os.OpenFile(filepath.Join(logDir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
        if err == nil {
            writers = append(writers, appLog)
            closers = append(closers, appLog)
        }
    }
    // Combine terminal pretty printing with structured file logging
    ...
}
```

### 2. HTTP Status-Aware Dynamic Log Level Routing

Never log all HTTP requests at `INFO`. Errors become invisible in high-throughput environments when drowned in thousands of 200 OK entries:

- **Server Errors (`Status >= 500`)**: Log explicitly as **`logger.Error("http request server error", ...)`**.
- **Client Errors (`Status 400..499`)**: Log explicitly as **`logger.Warn("http request client error", ...)`**.
- **Success & Redirects (`Status < 400`)**: Log as **`logger.Info("http request", ...)`**.

### 3. High-Contrast ANSI Terminal Status Highlighting

In terminal sessions, give status codes immediate visual distinction:

- `5xx`: Bold White text on Red Background (`\033[1;37;41mstatus=500\033[0m`).
- `4xx`: Bold Yellow (`\033[1;33mstatus=404\033[0m`) with bold white request path.
- `3xx`: Cyan (`\033[36mstatus=301\033[0m`).
- `2xx`: Green (`\033[32mstatus=200\033[0m`).

### 4. Health Probe Log Suppression

Continuous liveness and readiness probe polling (`/livez`, `/readyz`) generated by Kubernetes or load balancers must **not** pollute terminal output or inflate disk logs:

- **Suppress** probe logs when `status < 400`.
- **Emit immediately** as `WARN` or `ERROR` if a probe fails (`status >= 400`), indicating database or service degradation.

---

## Step 2: Synchronous Listener Pre-Binding (Fail-Fast)

> [!CAUTION]
> **Never call `srv.ListenAndServe()` inside an asynchronous goroutine.**
> If the port is already occupied (`bind: address already in use`), an asynchronous server
> will fail late while the app has already initialized databases, started workers, or claimed readiness!

Bind all network listeners synchronously in the main thread first:

```go
// 1. Bind public listener synchronously
pubLn, err := net.Listen("tcp", cfg.PublicAddress())
if err != nil {
    return fmt.Errorf("bind public %s: %w", cfg.PublicAddress(), err)
}

// 2. Bind management listener synchronously
mgmtLn, err := net.Listen("tcp", cfg.ManagementAddress())
if err != nil {
    pubLn.Close() // Rollback immediately
    return fmt.Errorf("bind management %s: %w", cfg.ManagementAddress(), err)
}
```

---

## Step 3: Configure HTTP Servers with Strict Timeouts

Zero-value `http.Server` configurations are vulnerable to slowloris attacks and resource exhaustion.
Configure strict timeouts on both public and management servers:

| Field | Recommended Value | Purpose |
| :--- | :---: | :--- |
| `ReadTimeout` | 5s–15s | Maximum time to read the full request body |
| `ReadHeaderTimeout` | 2s | Protect against stalled headers and slowloris attacks |
| `WriteTimeout` | 10s–30s | Maximum time to write the response |
| `IdleTimeout` | 120s | Idle keep-alive duration before closing connection |
| `MaxHeaderBytes` | 1 MB | Prevent oversized header memory flooding |

---

## Step 4: Security Isolation & Endpoint Separation

Isolate operational management endpoints from public business traffic:

### 1. Public Router (`:8070`)

- Serves business APIs under `/api/v1/*` behind authentication and rate-limiting.
- Exposes basic Kubernetes probes (`/livez` and `/readyz`).
- **Strict 404 Isolation**: Requests to operational and diagnostic routes on the public port **must return 404**.

### 2. Management Router (`:8066`)

- Exposes internal operational endpoints.
- Diagnostic suites (e.g. pprof, enabled conditionally).
- **Authentication**: When binding to `0.0.0.0`, require Bearer token authentication.

---

## Step 5: Coordinated Non-Blocking Startup

Launch both pre-bound servers using a `sync.WaitGroup` and monitor errors on an error channel:

```go
var wg sync.WaitGroup
errCh := make(chan error, 2)

// Start Public Server
wg.Add(1)
go func() {
    defer wg.Done()
    if err := publicServer.Serve(pubLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
        errCh <- fmt.Errorf("public server error: %w", err)
    }
}()

// Start Management Server
wg.Add(1)
go func() {
    defer wg.Done()
    if err := mgmtServer.Serve(mgmtLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
        errCh <- fmt.Errorf("management server error: %w", err)
    }
}()
```

---

## Step 6: Signal Handling, Two-Stage Drain & POSIX Compliance

Handle termination signals cleanly without dropping active requests:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

When a shutdown signal arrives:

1. **Mark Not Ready**: Flip readiness probe to false (`readyz` returns 503).
2. **Drain Wait**: Sleep for a configured drain duration (e.g., 5s) allowing Ingress / Load Balancers to re-route incoming traffic.

### POSIX-Compliant Termination in Scripts
>
> [!IMPORTANT]
> When orchestrating servers via `Makefile` or shell scripts running under `/bin/sh`, **never use `kill -SIGTERM`**. Many minimal Linux environments (like Dash or Alpine `/bin/sh`) do not recognize the `SIG` prefix and will fail with `Illegal option -S`.
> Always use standard POSIX format: **`kill -TERM $$pid`** or **`kill -15 $$pid`**.

---

## Step 7: Coordinated Graceful Teardown

Shut down both servers concurrently with a bounded context timeout:

```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
defer cancel()

var shutdownWg sync.WaitGroup
shutdownWg.Add(2)

go func() {
    defer shutdownWg.Done()
    _ = publicServer.Shutdown(shutdownCtx)
}()

go func() {
    defer shutdownWg.Done()
    _ = mgmtServer.Shutdown(shutdownCtx)
}()

shutdownWg.Wait() // Wait for active requests to drain
wg.Wait()         // Wait for Serve() goroutines to exit cleanly
```

---

## Step 8: Worker Lifecycle Management

Background workers must be strictly coordinated with the application lifecycle:

- **Start Condition**: Launch workers only after databases are connected and readiness is confirmed.
- **Stop Condition**: Stop background workers before closing database pools.
- **Context Cancellation**: Pass a cancellable context to all worker loops and wait on a `sync.WaitGroup`.

---

## Step 9: Cleanup in Reverse Order

Teardown must execute deterministically in exact reverse order of initialization:

1. Stop HTTP servers (`publicServer` and `mgmtServer`).
2. Stop background worker loops and wait for completion.
3. Close database connection pools (`dbPool.Close()`).
4. Close external messaging, cache, and audit clients.
5. **Flush and Sync Logs**: Sync file buffers to disk (`logFile.Sync()`), close file descriptors (`logFile.Close()`), and exit cleanly.

---

## Cross-Skill Dependencies & Integrations

This lifecycle skill acts as the composition root of the service: it orchestrates every other Go platform skill. The dependency graph below shows how configuration, context, errors, logging, and PostgreSQL infrastructure feed into startup, serving, drain, and teardown:

```text
go-service-configuration ─► go-server-lifecycle (timeouts, addresses, drain/shutdown durations, fail-fast validation)
go-context                ─► go-server-lifecycle (signal.NotifyContext, bounded shutdown ctx, RunDetached workers)
go-errors                 ─► go-server-lifecycle (HTTP status mapping, safe responses, log-or-return)
go-logger-slog            ─► go-server-lifecycle (multi-target logging, status-aware routing, probe suppression)
go-postgres               ─► go-server-lifecycle (pool Close in reverse-order cleanup, readiness checks)
```

### Skills This Skill Depends On (Downstream Dependencies)

| Skill | Dependency Reason | Key Integration Points |
| :--- | :--- | :--- |
| `go-service-configuration` | Sources every runtime value: listeners, timeouts, drain/shutdown durations, pool settings | `cfg.PublicAddress()`, `cfg.ManagementAddress()`, `cfg.ShutdownTimeout`, `cfg.DrainDuration`, fail-fast Phase 1 validation |
| `go-context` | POSIX signal handling and bounded shutdown rely on context primitives; detached background work needs leak-proof contexts | `signal.NotifyContext`, `context.WithTimeout`, `platformctx.RunDetached` / `WithoutCancel` with panic recovery |
| `go-logger-slog` | Multi-target structured logging (terminal + rotating JSON files), HTTP status-aware routing, probe log suppression, final `Sync()`/`Close()` flush | `initLogger()` outputs, status-routed `logger.Error/Warn/Info`, `/livez` `/readyz` suppression, logfile flush in Phase 8 |
| `go-errors` | Consistent HTTP error mapping and safe client responses; strict log-or-return discipline at boundaries | `platformerr.WriteHTTPError`, `platformerr.NotFound/Conflict/...`, structured error fields in logs |
| `go-postgres` | Database pool lifecycle must coordinate with server readiness and reverse-order teardown | `dbPool.Close()` after workers stop, readiness probe backed by pool health check, worker drain before pool close |

### Skills That Integrate With This Skill (Upstream Integrations)

Because this is the composition root, integration is one-directional: every component above is wired together here in `main` bootstrap. There are no higher-level skills that consume lifecycle directly; `httphandlers`/`usecases` simply use the context, logger, and error infrastructure created during startup.

---

## Anti-Patterns to Avoid

| Anti-Pattern | Why It's Wrong | Correct Approach |
| :--- | :--- | :--- |
| `go run` in Production | Signal handling fails; wraps binary and swallows signals | Build binary (`go build`), then execute |
| Blind `srv.ListenAndServe()` | If port is busy, app fails after initializing DB and workers | Synchronous `net.Listen` pre-binding in main thread |
| Exposing management publicly | Leaks internal system endpoints and operational control | Isolate to management port; return 404 publicly |
| Mingling probes with business traffic | Probes create massive log noise | Suppress 200 OK logs on `/livez` and `/readyz` |
| Unconditional `logger.Info` for all HTTP | 4xx client errors and 5xx crashes blend invisibly into logs | Route 5xx to `ERROR`, 4xx to `WARN`, with ANSI contrast |
| Using `kill -SIGTERM` in POSIX scripts | Fails with `Illegal option -S` under `/bin/sh` or Dash | Use POSIX-compliant `kill -TERM $$pid` or `kill -15` |
| Calling `Close()` on shutdown | Terminates in-flight requests abruptly | Use `srv.Shutdown(ctx)` with a drain period |

---

## Production Verification Checklist

```text
[ ] Configuration & environment variables fail fast on missing required keys
[ ] Listeners are pre-bound synchronously via net.Listen before any goroutine starts
[ ] Server timeouts (Read, ReadHeader, Write, Idle, MaxHeaderBytes) are explicit and non-zero
[ ] Dual-server isolation: Public (8070) returns 404 for operational management routes
[ ] Management server (8066) enforces Bearer token auth when bound to 0.0.0.0
[ ] Multi-target logging: colored ANSI for terminal, structured JSON for rotating files
[ ] HTTP log routing emits 5xx as ERROR and 4xx as WARN; probes suppressed on 200 OK
[ ] Liveness (/livez) and Readiness (/readyz) semantics are isolated and correct
[ ] Signal.NotifyContext handles SIGINT and SIGTERM gracefully using POSIX kill -TERM
[ ] Readiness probe marks false before beginning http.Server.Shutdown (drain phase)
[ ] Both servers shut down concurrently and cleanly without hanging goroutines
[ ] Background workers stop before closing database connections
[ ] Resource cleanup follows exact reverse order of construction (including logfile Sync)
[ ] All tests pass with race detector: go test -v -race ./...
```
