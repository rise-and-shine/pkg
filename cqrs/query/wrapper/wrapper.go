// Package wrapper provides middleware wrappers for CQRS query handlers.
//
// This package enables cross-cutting concerns such as tracing, logging, and metrics
// to be applied to query handlers in a composable way. Wrappers can be used to
// instrument, monitor, or modify the behavior of query execution without changing
// the core business logic.
package wrapper
