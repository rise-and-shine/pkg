package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Common errors
var (
	ErrEventNotFound = errors.New("event not found")
)

// Event represents a domain event that needs to be published
type Event interface {
	// EventType returns the type/name of the event
	EventType() string
	// EventData returns the payload data for the event
	EventData() interface{}
	// AggregateID returns the ID of the aggregate that generated this event
	AggregateID() string
	// EventVersion returns the version of the event schema (for evolution)
	EventVersion() string
}

// BaseEvent provides a default implementation of Event interface
type BaseEvent struct {
	Type        string      `json:"type"`
	Data        interface{} `json:"data"`
	AggregateId string      `json:"aggregate_id"`
	Version     string      `json:"version"`
	Timestamp   time.Time   `json:"timestamp"`
	TraceID     string      `json:"trace_id,omitempty"`
	UserID      string      `json:"user_id,omitempty"`
}

func (e BaseEvent) EventType() string {
	return e.Type
}

func (e BaseEvent) EventData() interface{} {
	return e.Data
}

func (e BaseEvent) AggregateID() string {
	return e.AggregateId
}

func (e BaseEvent) EventVersion() string {
	if e.Version == "" {
		return "v1"
	}
	return e.Version
}

// EventBuilder helps create events with proper metadata
type EventBuilder struct {
	eventType   string
	aggregateID string
	version     string
	traceID     string
	userID      string
	data        interface{}
}

func NewEventBuilder(eventType, aggregateID string) *EventBuilder {
	return &EventBuilder{
		eventType:   eventType,
		aggregateID: aggregateID,
		version:     "v1",
	}
}

func (b *EventBuilder) WithVersion(version string) *EventBuilder {
	b.version = version
	return b
}

func (b *EventBuilder) WithTraceID(traceID string) *EventBuilder {
	b.traceID = traceID
	return b
}

func (b *EventBuilder) WithUserID(userID string) *EventBuilder {
	b.userID = userID
	return b
}

func (b *EventBuilder) WithData(data interface{}) *EventBuilder {
	b.data = data
	return b
}

func (b *EventBuilder) Build() BaseEvent {
	return BaseEvent{
		Type:        b.eventType,
		Data:        b.data,
		AggregateId: b.aggregateID,
		Version:     b.version,
		Timestamp:   time.Now(),
		TraceID:     b.traceID,
		UserID:      b.userID,
	}
}

// EventStatus represents the status of an outbox event
type EventStatus string

const (
	EventStatusPending EventStatus = "pending"
	EventStatusSent    EventStatus = "sent"
	EventStatusFailed  EventStatus = "failed"
)

// OutboxEntry represents a stored event in the outbox table
type OutboxEntry struct {
	bun.BaseModel `bun:"table:outbox_events,alias:oe"`

	ID            uuid.UUID              `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AggregateType string                 `bun:"aggregate_type,notnull" json:"aggregate_type"`
	AggregateID   string                 `bun:"aggregate_id,notnull" json:"aggregate_id"`
	EventType     string                 `bun:"event_type,notnull" json:"event_type"`
	EventVersion  string                 `bun:"event_version,notnull" json:"event_version"`
	EventData     map[string]interface{} `bun:"event_data,type:jsonb,notnull" json:"event_data"`
	Headers       map[string]interface{} `bun:"headers,type:jsonb,notnull,default:'{}'" json:"headers"`
	Topic         string                 `bun:"topic,notnull" json:"topic"`
	PartitionKey  *string                `bun:"partition_key" json:"partition_key,omitempty"`
	DedupKey      *string                `bun:"dedup_key" json:"dedup_key,omitempty"`
	UserID        *string                `bun:"user_id" json:"user_id,omitempty"`
	TraceID       *string                `bun:"trace_id" json:"trace_id,omitempty"`
	Status        OutboxStatus           `bun:"status,notnull,default:'pending'" json:"status"`
	Attempts      int                    `bun:"attempts,notnull,default:0" json:"attempts"`
	RetryCount    int                    `bun:"retry_count,notnull,default:0" json:"retry_count"`
	NextRetryAt   *time.Time             `bun:"next_retry_at,nullzero" json:"next_retry_at,omitempty"`
	CreatedAt     time.Time              `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	AvailableAt   time.Time              `bun:"available_at,nullzero,notnull,default:current_timestamp" json:"available_at"`
	PublishedAt   *time.Time             `bun:"published_at,nullzero" json:"published_at,omitempty"`
	ErrorMessage  *string                `bun:"error_message" json:"error_message,omitempty"`
	UpdatedAt     time.Time              `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	Seq           *int64                 `bun:"seq" json:"seq,omitempty"`
}

// BeforeAppendModel implements bun.BeforeAppendModelHook
func (e *OutboxEntry) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}
		e.CreatedAt = time.Now()
		e.UpdatedAt = time.Now()
		// Set AvailableAt to now if not set
		if e.AvailableAt.IsZero() {
			e.AvailableAt = time.Now()
		}
	case *bun.UpdateQuery:
		e.UpdatedAt = time.Now()
	}
	return nil
}

// ToJSON converts the outbox entry to JSON for publishing
func (e *OutboxEntry) ToJSON() ([]byte, error) {
	eventData := map[string]interface{}{
		"id":           e.ID,
		"type":         e.EventType,
		"aggregate_id": e.AggregateID,
		"version":      e.EventVersion,
		"data":         e.EventData,
		"headers":      e.Headers,
		"timestamp":    e.CreatedAt,
		"trace_id":     e.TraceID,
		"user_id":      e.UserID,
	}
	return json.Marshal(eventData)
}

// SetRetryDelay calculates and sets the next available time based on exponential backoff
func (e *OutboxEntry) SetRetryDelay() {
	// Exponential backoff: 2^attempts seconds
	delaySeconds := 1 << e.Attempts // 1, 2, 4, 8, 16, 32, 64...
	if delaySeconds > 3600 {        // Max 1 hour
		delaySeconds = 3600
	}
	e.AvailableAt = time.Now().Add(time.Duration(delaySeconds) * time.Second)
}

// OutboxOffset tracks the last processed event for each service
type OutboxOffset struct {
	bun.BaseModel `bun:"table:outbox_offset,alias:oo"`

	ServiceName   string    `bun:",pk" json:"service_name"`
	LastEventID   uuid.UUID `bun:"last_event_id" json:"last_event_id"`
	LastProcessed time.Time `bun:"last_processed,notnull,default:current_timestamp" json:"last_processed"`
	UpdatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// BeforeAppendModel implements bun.BeforeAppendModelHook
func (o *OutboxOffset) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.UpdateQuery:
		o.UpdatedAt = time.Now()
		o.LastProcessed = time.Now()
	case *bun.InsertQuery:
		o.UpdatedAt = time.Now()
		o.LastProcessed = time.Now()
	}
	return nil
}

// OutboxStatus represents the status of an outbox event (alias for EventStatus)
type OutboxStatus = EventStatus

// Additional status constants for consistency
const (
	StatusPending   OutboxStatus = EventStatusPending
	StatusPublished OutboxStatus = EventStatusSent
	StatusFailed    OutboxStatus = EventStatusFailed
)

// Message represents a message to be published
type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Key       string            `json:"key,omitempty"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// PublishEvent represents an event to be published
type PublishEvent struct {
	ID           string                 `json:"id"`
	EventType    string                 `json:"event_type"`
	AggregateID  string                 `json:"aggregate_id"`
	EventVersion string                 `json:"event_version"`
	EventData    map[string]interface{} `json:"event_data"`
	Topic        string                 `json:"topic"`
	PartitionKey string                 `json:"partition_key,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	TraceID      string                 `json:"trace_id,omitempty"`
	Headers      map[string]interface{} `json:"headers,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// Publisher defines the interface for publishing events to external systems
type Publisher interface {
	// Publish publishes a message to the configured destination
	Publish(ctx context.Context, message *Message) error
	// Close closes the publisher and releases resources
	Close() error
}

// Logger defines the interface for logging operations
type Logger interface {
	// Info logs an info message with optional fields
	Info(msg string, fields map[string]interface{})
	// Error logs an error message with optional fields
	Error(msg string, fields map[string]interface{})
	// Debug logs a debug message with optional fields
	Debug(msg string, fields map[string]interface{})
	// Warn logs a warning message with optional fields
	Warn(msg string, fields map[string]interface{})
}

// CalculateRetryDelay calculates exponential backoff delay
func CalculateRetryDelay(retryCount int, baseDelay time.Duration) time.Duration {
	if retryCount <= 0 {
		return baseDelay
	}

	// Exponential backoff: baseDelay * 2^retryCount
	multiplier := 1 << uint(retryCount) // 2^retryCount
	delay := baseDelay * time.Duration(multiplier)

	// Cap at 1 hour
	maxDelay := time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// NoOpPublisher is a publisher that does nothing (for testing)
type NoOpPublisher struct{}

// NewNoOpPublisher creates a new no-op publisher
func NewNoOpPublisher() Publisher {
	return &NoOpPublisher{}
}

// Publish does nothing and always returns nil
func (n *NoOpPublisher) Publish(ctx context.Context, message *Message) error {
	return nil
}

// Close does nothing
func (n *NoOpPublisher) Close() error {
	return nil
}
