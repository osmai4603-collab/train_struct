// Package context provides the standard infrastructure for managing context (context.Context)
// in the platform layer (Platform / Foundation Layer).
//
// Architectural placement:
// This package lives in the common foundation layer (internal/platform/context) and follows
// the strict dependency rules of Clean Architecture:
//  1. No dependencies on upper layers: the package imports nothing from domain or usecases,
//     or services, storage, or httphandlers.
//  2. Import safety: all upper layers (transport, application, storage) may safely import
//     this package to inject or read request attributes and monitor deadlines without any risk
//     of circular import loops.
package context
