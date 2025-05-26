// Package command defines interfaces and types for CQRS command handling.
//
// It provides abstractions for command execution, input/output typing, and middleware wrapping.
// Commands represent operations that change state and may return results or errors.
package command

import "context"

// EmptyResult is a placeholder type for commands that do not return a result.
type (
	EmptyResult = struct{}
)

type (
	// Input represents the input type for a command.
	Input any

	// Result represents the result type for a command.
	Result any
)

// Command defines a handler for a CQRS command.
//
// Execute runs the command with the given input and context, returning a result or error.
type Command[I Input, R Result] interface {
	// Execute processes the command input and returns a result or error.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines.
	//   - input: The command input.
	//
	// Returns the command result and error, if any.
	Execute(context.Context, I) (R, error)
}

// WrapFunc defines a middleware function for wrapping command handlers.
//
// It takes a Command and returns a wrapped Command, enabling cross-cutting concerns.
type WrapFunc[I Input, R Result] func(Command[I, R]) Command[I, R]
