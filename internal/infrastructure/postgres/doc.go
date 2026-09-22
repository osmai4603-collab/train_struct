// Package postgres provides production-grade, centralized infrastructure for PostgreSQL
// using pgx/v5 (pgxpool) in accordance with Go and enterprise architecture best practices.
//
// Architecture & Layer Positioning:
//
//	Layer: Infrastructure (internal/infrastructure/postgres)
//	Position: Lowest architectural layer alongside other platform adapters.
//
// Dependency Rules:
//   - Allowed imports: internal/platform/errors, internal/platform/config, internal/platform/context,
//     standard library (context, time, log/slog, etc.), and pgx/v5 ecosystem.
//   - Strictly forbidden imports: domain, usecases, services, httphandlers.
//
// Key Responsibilities:
//   - pgxpool Connection Lifecycle & Tuning (MaxConns, MinConns, IdleTime, HealthCheckPeriod)
//   - Failure Domain Isolation & Error Translation (pgconn.PgError / SQLSTATE -> platform errors)
//   - Transaction Orchestration (ExecTx, ExecTxWithRetry, DBTX interface)
//   - Transient Error Handling & Resilient Retries (Exponential Backoff with Full Jitter)
//   - Health Checks & Real-Time Pool Telemetry
//   - Database Schema Migrations via golang-migrate and embed.FS
package postgres
