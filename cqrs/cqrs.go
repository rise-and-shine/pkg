// Package cqrs provides Command Query Responsibility Segregation (CQRS) pattern implementation.
//
// This package defines abstractions for commands and queries, enabling separation of read and write operations.
// It includes generic interfaces for command and query handlers, as well as middleware wrappers for cross-cutting concerns
// such as tracing, logging, and metrics. The CQRS package is designed to help structure application logic for scalability,
// testability, and maintainability.
package cqrs
