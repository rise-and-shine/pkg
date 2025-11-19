package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var (
	ErrEventNotFound = errors.New("event not found")
)

type Event interface {
	EventType() string
	EventData() interface{}
	AggregateID() string
	EventVersion() string
}

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

type EventStatus string

const (
	EventStatusPending EventStatus = "pending"
	EventStatusSent    EventStatus = "sent"
	EventStatusFailed  EventStatus = "failed"
)

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

func (e *OutboxEntry) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}
		e.CreatedAt = time.Now()
		e.UpdatedAt = time.Now()
		if e.AvailableAt.IsZero() {
			e.AvailableAt = time.Now()
		}
	case *bun.UpdateQuery:
		e.UpdatedAt = time.Now()
	}
	return nil
}

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

func (e *OutboxEntry) SetRetryDelay() {
	delaySeconds := 1 << e.Attempts
	if delaySeconds > 3600 {
		delaySeconds = 3600
	}
	e.AvailableAt = time.Now().Add(time.Duration(delaySeconds) * time.Second)
}

type OutboxOffset struct {
	bun.BaseModel `bun:"table:outbox_offset,alias:oo"`

	ServiceName   string    `bun:",pk" json:"service_name"`
	LastEventID   uuid.UUID `bun:"last_event_id" json:"last_event_id"`
	LastProcessed time.Time `bun:"last_processed,notnull,default:current_timestamp" json:"last_processed"`
	UpdatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

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

type OutboxStatus = EventStatus

const (
	StatusPending   OutboxStatus = EventStatusPending
	StatusPublished OutboxStatus = EventStatusSent
	StatusFailed    OutboxStatus = EventStatusFailed
)

type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Key       string            `json:"key,omitempty"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

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

type Publisher interface {
	Publish(ctx context.Context, message *Message) error
	Close() error
}

type Logger interface {
	Info(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Debug(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
}

func CalculateRetryDelay(retryCount int, baseDelay time.Duration) time.Duration {
	if retryCount <= 0 {
		return baseDelay
	}

	multiplier := 1 << uint(retryCount)
	delay := baseDelay * time.Duration(multiplier)

	maxDelay := time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

type NoOpPublisher struct{}

func NewNoOpPublisher() Publisher {
	return &NoOpPublisher{}
}

func (n *NoOpPublisher) Publish(ctx context.Context, message *Message) error {
	return nil
}

func (n *NoOpPublisher) Close() error {
	return nil
}
