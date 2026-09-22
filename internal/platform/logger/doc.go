// Package logger provides the standard structured logging infrastructure for the
// platform (Platform / Foundation Layer), built on the official log/slog package
// available since Go 1.21.
//
// This package follows the globally adopted architecture of separating the
// interface from the implementation (Frontend/Backend):
//   - Frontend: application code only calls *slog.Logger through context-aware
//     functions such as InfoContext, ErrorContext, and LogAttrs.
//   - Backend: slog.Handler is responsible for formatting and output (JSONHandler
//     for production and TextHandler for local development).
//
// Architectural placement:
// This package lives in the shared foundation layer (internal/platform/logger) and
// follows the strict dependency rules of Clean Architecture:
//  1. No dependencies on upper layers: the package does not import any component
//     from the domain, usecases, services, storage, or httphandlers layers.
//  2. Import safety: all upper layers (transport, application, storage) may safely
//     import this package to create scoped loggers and tie them to the request
//     context without any risk of circular import cycles.
//
// # Features
//
//   - Environment-based unified configuration via New, with NDJSON support for production.
//   - Explicit dependency injection by passing *slog.Logger to constructor functions
//     and scoping with With.
//   - Tying log records to the request context and scoping per component via
//     WithContext and FromContext.
//   - Strict protection of sensitive data via Sensitive[T], SecretString, and
//     RedactingHandler.
//   - Zero-allocation hot paths via slog.LogAttrs.
//   - HTTP middleware for request logging linking the request_id from the context package.
//   - Conformance testing of custom handlers against the official testing/slogtest
//     standard.
package logger
