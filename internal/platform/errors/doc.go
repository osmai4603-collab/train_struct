// Package errors provides a unified infrastructure for managing and engineering errors in the platform layer (Platform Layer).
//
// This system is based on global best practices and Ben Johnson's architectural pattern (Failure is your Domain),
// which deals with three independent consumers for each error:
//  1. The system and program logic (The Application): needs a fixed machine-readable code (Machine-Readable Code) to make decisions.
//  2. The end user (The End User): needs a clear, safe message free of leaked technical details.
//  3. The operations and monitoring engineer (The Operator): needs a complete logical stack trace (Logical Stack Trace) and structured logs.
//
// # Dependency rules
//
//  1. This package lives in the core platform layer (internal/platform/errors).
//  2. The package does not depend on any upper layers (Domain, UseCases, Services, Storage, Handlers).
//  3. The package integrates with the context package (internal/platform/context) to automatically extract trace IDs.
//  4. The package supports the log/slog.LogValuer interface for high-performance structured logging.
package errors
