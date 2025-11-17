package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// UoWRepository provides outbox functionality compatible with Unit of Work pattern
// This repository accepts the transaction via constructor instead of method parameters
type UoWRepository interface {
	// Write operations (use injected transaction)
	StoreEvent(ctx context.Context, event Event, topic string, options ...StoreOption) error
	StoreSimpleEvent(ctx context.Context, eventType, aggregateID string, eventData []byte, topic string) error

	// Background operations (use main DB connection)
	GetAvailableEvents(ctx context.Context, limit int) ([]*OutboxEntry, error)
	MarkAsPublished(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	DeleteOldEvents(ctx context.Context, olderThan time.Duration) error
}

// uowRepository implements UoWRepository
type uowRepository struct {
	idb bun.IDB // Transaction for writes (injected by UoW)
	db  *bun.DB // Main database for background operations
}

// NewUoWRepository creates a new UoW-compatible outbox repository
// idb: Transaction for write operations (StoreEvent, StoreSimpleEvent)
// db: Main database for background operations (GetAvailableEvents, Mark*, Delete*)
func NewUoWRepository(idb bun.IDB, db *bun.DB) UoWRepository {
	return &uowRepository{
		idb: idb,
		db:  db,
	}
}

// StoreEvent stores an event using the injected transaction
func (r *uowRepository) StoreEvent(ctx context.Context, event Event, topic string, options ...StoreOption) error {
	opts := &storeOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Convert event data to map
	eventJSON, err := json.Marshal(event.EventData())
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var eventDataMap map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventDataMap); err != nil {
		return fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	// Build headers
	headers := make(map[string]interface{})
	if opts.userID != nil {
		headers["user_id"] = *opts.userID
	}
	if opts.traceID != nil {
		headers["trace_id"] = *opts.traceID
	}

	// Create outbox entry
	entry := &OutboxEntry{
		AggregateID:  event.AggregateID(),
		EventType:    event.EventType(),
		EventVersion: event.EventVersion(),
		EventData:    eventDataMap,
		Topic:        topic,
		Headers:      headers,
		Status:       EventStatusPending,
		PartitionKey: opts.partitionKey,
		DedupKey:     opts.dedupKey,
		AvailableAt:  time.Now(),
	}

	if opts.availableAt != nil {
		entry.AvailableAt = *opts.availableAt
	}

	// Insert using injected transaction
	_, err = r.idb.NewInsert().Model(entry).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store outbox event: %w", err)
	}

	return nil
}

// StoreSimpleEvent stores a simple event (for compatibility)
func (r *uowRepository) StoreSimpleEvent(
	ctx context.Context,
	eventType string,
	aggregateID string,
	eventData []byte,
	topic string,
) error {
	// Convert to map
	var eventDataMap map[string]interface{}
	if err := json.Unmarshal(eventData, &eventDataMap); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Create outbox entry
	entry := &OutboxEntry{
		AggregateID:  aggregateID,
		EventType:    eventType,
		EventVersion: "v1",
		EventData:    eventDataMap,
		Topic:        topic,
		Status:       EventStatusPending,
		Headers:      make(map[string]interface{}),
		AvailableAt:  time.Now(),
	}

	// Insert using injected transaction
	_, err := r.idb.NewInsert().Model(entry).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store outbox event: %w", err)
	}

	return nil
}

// GetAvailableEvents returns events available for processing (uses main DB, not transaction)
func (r *uowRepository) GetAvailableEvents(ctx context.Context, limit int) ([]*OutboxEntry, error) {
	var events []*OutboxEntry

	err := r.db.NewSelect().
		Model(&events).
		Where("status = ?", EventStatusPending).
		Where("available_at <= ?", time.Now()).
		Order("created_at ASC").
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get available events: %w", err)
	}

	return events, nil
}

// MarkAsPublished marks an event as successfully published (uses main DB)
func (r *uowRepository) MarkAsPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*OutboxEntry)(nil)).
		Set("status = ?", EventStatusSent).
		Set("published_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to mark event as published: %w", err)
	}

	return nil
}

// MarkAsFailed marks an event as failed (uses main DB)
func (r *uowRepository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*OutboxEntry)(nil)).
		Set("status = ?", EventStatusFailed).
		Set("error_message = ?", errorMsg).
		Set("retry_count = retry_count + 1").
		Set("last_retry_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	return nil
}

// DeleteOldEvents deletes old published events (uses main DB)
func (r *uowRepository) DeleteOldEvents(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	_, err := r.db.NewDelete().
		Model((*OutboxEntry)(nil)).
		Where("status = ?", EventStatusSent).
		Where("published_at < ?", cutoffTime).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete old events: %w", err)
	}

	return nil
}
