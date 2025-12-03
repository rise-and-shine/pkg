// Package ucdef defines use case definitions that is used across the application.
package ucdef

import "context"

// Use case types.
const (
	TypeUserAction      = "user_action"
	TypeEventSubscriber = "event_subscriber"
	TypeAsyncTask       = "async_task"
	TypeManualCommand   = "manual_command"
)

// UserAction represents a synchronous business operation triggered by user interaction.
// It handles user-initiated requests through HTTP, gRPC, or WebSocket protocols and
// returns an immediate response. These operations are typically exposed via API endpoints
// and require request validation, authorization, and synchronous error handling.
//
// Type parameters:
//   - I: Input data type (request payload)
//   - O: Output data type (response, result of the operation)
//
// Examples: CreateOrder, UpdateUserProfile, PlaceInvestment, SubmitTenderBid
//
// Characteristics:
//   - Synchronous execution with immediate response
//   - User is waiting for the result
//   - Requires comprehensive input validation
//   - Should be audited for write operations (commands)
//   - Errors are returned directly to the user as HTTP response
type UserAction[I, O any] interface {
	// OperationID returns a unique identifier for the use case.
	OperationID() string

	// Execute executes the use case.
	Execute(ctx context.Context, input I) (O, error)
}

// EventSubscriber represents an asynchronous business operation triggered by domain events.
// It reacts to events published by other parts of the system without direct coupling to
// the event publisher. Subscribers process events via message brokers (Kafka, Redis, RabbitMQ)
// or any pub/sub systems (like Pg as a PubSub). This interface is used for both pub/sub patterns
// (broadcast to multiple subscribers) and queue patterns (single consumer per message).
//
// Type parameters:
//   - E: Event type (domain event structure)
//
// Examples: SendEmailOnUserRegistered, UpdateInventoryOnOrderPlaced, NotifyOnPaymentReceived
//
// Characteristics:
//   - Asynchronous execution, no immediate response to publisher
//   - Must be idempotent (handle duplicate events gracefully)
//   - Uses dead-letter queues for failures
//   - Errors trigger retries and alerts, not user-facing responses
//   - Should be audited to track state changes
//   - Enables loose coupling between system components
type EventSubscriber[E any] interface {
	// OperationID returns a unique identifier for the use case.
	OperationID() string

	// Handle handles the event.
	Handle(ctx context.Context, event E) error
}

// AsyncTask represents an asynchronous unit of work executed in the background by workers.
// Tasks are enqueued through two mechanisms: scheduled execution (cron) or on-demand
// from use cases. Workers poll the task queue and execute tasks.
//
// Examples: GenerateInvoicePDF, SendNotification, CleanupExpiredSessions, ProcessUpload
//
// Execution Flow:
//  1. Scheduled: Cron manager enqueues task at specified time → worker executes
//  2. On-demand: Use case enqueues task → worker executes
//
// Characteristics:
//   - Always asynchronous - never blocks caller at enqueue time (use case or cron)
//   - Accepts serializable payload (stored in queue)
//   - Should be idempotent (queue may deliver same task multiple times)
//   - Workers inject dependencies (repos, services) before execution
//   - Execution status tracked in platform observability
//   - Failed tasks can be retried based on queue configuration
//
// Payload Guidelines:
//   - Must be JSON-serializable (stored in queue)
//   - Scheduled tasks: nil or empty struct (task fetches data via repos)
//   - Use case tasks: struct with IDs/parameters (e.g., {InvoiceID: "inv_123"})
//   - Keep payloads small - store references (IDs), not full entities for large payloads
type AsyncTask[P any] interface {
	// OperationID returns a unique identifier for the use case.
	OperationID() string

	// Execute performs the task operation with the given payload.
	Execute(ctx context.Context, payload P) error
}

// ManualCommand represents an administrative operation executed manually via CLI.
// These are operational commands run by developers or operators for maintenance,
// data migration, or system administration tasks. Unlike user actions, they don't
// return structured responses - success/failure is indicated via error and logs.
//
// Type parameters:
//   - I: Input parameters (command arguments, flags)
//
// Examples: CreateSuperUser, MigrateData, RegenerateSearchIndex, PurgeOldRecords
//
// Characteristics:
//   - Manually triggered by operators via command-line interface
//   - Takes input parameters but returns no structured output
//   - Success/failure communicated through error and log messages
//   - Should be audited for accountability
//   - May have elevated privileges or bypass certain validations
//   - Temporary scripts should be removed after use
type ManualCommand[I any] interface {
	// OperationID returns a unique identifier for the use case.
	OperationID() string

	// Execute executes the manual command.
	Execute(ctx context.Context, input I) error
}
