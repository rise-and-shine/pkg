package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type UoWRepository interface {
	StoreEvent(ctx context.Context, event Event, topic string, options ...StoreOption) error
	StoreSimpleEvent(ctx context.Context, eventType, aggregateID string, eventData []byte, topic string) error

	GetAvailableEvents(ctx context.Context, limit int) ([]*OutboxEntry, error)
	MarkAsPublished(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	DeleteOldEvents(ctx context.Context, olderThan time.Duration) error
}

type uowRepository struct {
	idb bun.IDB
	db  *bun.DB
}

func NewUoWRepository(idb bun.IDB, db *bun.DB) UoWRepository {
	return &uowRepository{
		idb: idb,
		db:  db,
	}
}

func (r *uowRepository) StoreEvent(ctx context.Context, event Event, topic string, options ...StoreOption) error {
	opts := &storeOptions{}
	for _, opt := range options {
		opt(opts)
	}

	eventJSON, err := json.Marshal(event.EventData())
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var eventDataMap map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventDataMap); err != nil {
		return fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	headers := make(map[string]interface{})
	if opts.userID != nil {
		headers["user_id"] = *opts.userID
	}
	if opts.traceID != nil {
		headers["trace_id"] = *opts.traceID
	}

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

	_, err = r.idb.NewInsert().Model(entry).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store outbox event: %w", err)
	}

	return nil
}

func (r *uowRepository) StoreSimpleEvent(
	ctx context.Context,
	eventType string,
	aggregateID string,
	eventData []byte,
	topic string,
) error {
	var eventDataMap map[string]interface{}
	if err := json.Unmarshal(eventData, &eventDataMap); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

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

	_, err := r.idb.NewInsert().Model(entry).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store outbox event: %w", err)
	}

	return nil
}

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
