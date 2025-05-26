// Package wrapper provides middleware wrappers for CQRS command handlers.
//
// This package enables cross-cutting concerns such as tracing, logging, and metrics
// to be applied to command handlers in a composable way. Wrappers can be used to
// instrument, monitor, or modify the behavior of command execution without changing
// the core business logic.
package wrapper
