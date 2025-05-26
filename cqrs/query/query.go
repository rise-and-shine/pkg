// Package query defines interfaces and types for CQRS query handling.
//
// It provides abstractions for query execution, input/output typing, and middleware wrapping.
// Queries represent read-only operations that return results and do not change state.
package query

import "context"

type (
	// Input represents the input type for a query.
	Input any

	// Result represents the result type for a query.
	Result any
)

// Query defines a handler for a CQRS query.
//
// Execute runs the query with the given input and context, returning a result or error.
type Query[I Input, R Result] interface {
	// Execute processes the query input and returns a result or error.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines.
	//   - input: The query input.
	//
	// Returns the query result and error, if any.
	Execute(context.Context, I) (R, error)
}

// WrapFunc defines a middleware function for wrapping query handlers.
//
// It takes a Query and returns a wrapped Query, enabling cross-cutting concerns.
type WrapFunc[I Input, R Result] func(Query[I, R]) Query[I, R]
