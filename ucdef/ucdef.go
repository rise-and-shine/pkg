// Package ucdef defines use case definitions that is used across the application.
package ucdef

import "context"

// Use case types.
const (
	TypeUserAction      = "user_action"
	TypeEventSubscriber = "event_subscriber"
	TypeScheduledJob    = "scheduled_job"
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
	Execute(ctx context.Context, in I) (O, error)
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
	Handle(ctx context.Context, e E) error
}

// ScheduledJob represents a time-triggered business operation executed periodically or at
// specific times. Jobs are self-contained operations configured via cron expressions and
// run without external input. They fetch required data from repositories and services.
//
// Examples: SendDailyReport, CleanupExpiredSessions, RecalculateMetrics, BackupDatabase
//
// Characteristics:
//   - Triggered by time (cron schedule), not user action or event
//   - No input parameters - configuration injected via constructor
//   - Self-contained - fetches its own data from dependencies
//   - Should be idempotent (safe to run multiple times)
//   - Failures logged and monitored, may trigger alerts
//   - Consider distributed locking for multi-instance deployments
type ScheduledJob interface {
	// OperationID returns a unique identifier for the use case.
	OperationID() string

	// Execute executes the scheduled job.
	Execute(ctx context.Context) error
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
	Execute(ctx context.Context, in I) error
}
