package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Repository interface {
	StoreWithDeduplication(ctx context.Context, tx bun.Tx, event Event, topic string, options ...StoreOption) error

	GetAvailableEvents(ctx context.Context, limit int) ([]*OutboxEntry, error)

	MarkAsPublished(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error

	DeleteOldEvents(ctx context.Context, olderThan time.Duration) error

	GetOffset(ctx context.Context, serviceName string) (*OutboxOffset, error)
	UpdateOffset(ctx context.Context, serviceName string, lastEventID uuid.UUID) error
}

type repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) Repository {
	return &repository{db: db}
}

type StoreOption func(*storeOptions)

type storeOptions struct {
	partitionKey  *string
	userID        *string
	traceID       *string
	dedupKey      *string
	headers       map[string]interface{}
	aggregateType string
	availableAt   *time.Time
}

func WithPartitionKey(key string) StoreOption {
	return func(opts *storeOptions) {
		opts.partitionKey = &key
	}
}

func WithUserID(userID string) StoreOption {
	return func(opts *storeOptions) {
		opts.userID = &userID
	}
}

func WithTraceID(traceID string) StoreOption {
	return func(opts *storeOptions) {
		opts.traceID = &traceID
	}
}

func WithDedupKey(key string) StoreOption {
	return func(opts *storeOptions) {
		opts.dedupKey = &key
	}
}

func WithHeaders(headers map[string]interface{}) StoreOption {
	return func(opts *storeOptions) {
		opts.headers = headers
	}
}

func WithAggregateType(aggregateType string) StoreOption {
	return func(opts *storeOptions) {
		opts.aggregateType = aggregateType
	}
}

func WithDelayedProcessing(availableAt time.Time) StoreOption {
	return func(opts *storeOptions) {
		opts.availableAt = &availableAt
	}
}

func (r *repository) StoreWithDeduplication(ctx context.Context, tx bun.Tx, event Event, topic string, options ...StoreOption) error {
	opts := &storeOptions{
		headers: make(map[string]interface{}),
	}
	for _, option := range options {
		option(opts)
	}

	if opts.dedupKey != nil && *opts.dedupKey != "" {
		exists, err := tx.NewSelect().
			Model((*OutboxEntry)(nil)).
			Where("dedup_key = ?", *opts.dedupKey).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}
		if exists {
			return nil 
		}
	}

	eventDataBytes, err := json.Marshal(event.EventData())
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(eventDataBytes, &eventData); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	aggregateType := opts.aggregateType
	if aggregateType == "" {
		aggregateType = event.EventType() 
	}

	entry := &OutboxEntry{
		AggregateType: aggregateType,
		AggregateID:   event.AggregateID(),
		EventType:     event.EventType(),
		EventVersion:  event.EventVersion(),
		EventData:     eventData,
		Headers:       opts.headers,
		Topic:         topic,
		PartitionKey:  opts.partitionKey,
		DedupKey:      opts.dedupKey,
		UserID:        opts.userID,
		TraceID:       opts.traceID,
		Status:        EventStatusPending,
		Attempts:      0,
	}

	if opts.availableAt != nil {
		entry.AvailableAt = *opts.availableAt
	} else {
		entry.AvailableAt = time.Now()
	}

	if entry.PartitionKey == nil {
		key := event.AggregateID()
		entry.PartitionKey = &key
	}

	if _, err := tx.NewInsert().Model(entry).Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert outbox entry: %w", err)
	}

	return nil
}

func (r *repository) GetAvailableEvents(ctx context.Context, limit int) ([]*OutboxEntry, error) {
	var entries []*OutboxEntry
	err := r.db.NewSelect().
		Model(&entries).
		Where("status = ? AND available_at <= ?", EventStatusPending, time.Now()).
		Order("created_at ASC").
		Limit(limit).
		Scan(ctx)
	return entries, err
}

func (r *repository) MarkAsPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*OutboxEntry)(nil)).
		Set("status = ?", EventStatusSent).
		Set("published_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *repository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	var entry OutboxEntry
	err := r.db.NewSelect().
		Model(&entry).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return err
	}

	entry.Attempts++
	entry.SetRetryDelay()
	entry.Status = EventStatusFailed
	entry.ErrorMessage = &errorMsg

	_, err = r.db.NewUpdate().
		Model((*OutboxEntry)(nil)).
		Set("status = ?", entry.Status).
		Set("attempts = ?", entry.Attempts).
		Set("available_at = ?", entry.AvailableAt).
		Set("error_message = ?", errorMsg).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *repository) DeleteOldEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	_, err := r.db.NewDelete().
		Model((*OutboxEntry)(nil)).
		Where("status = ? AND published_at < ?", EventStatusSent, cutoff).
		Exec(ctx)
	return err
}

func (r *repository) GetOffset(ctx context.Context, serviceName string) (*OutboxOffset, error) {
	offset := &OutboxOffset{}
	err := r.db.NewSelect().
		Model(offset).
		Where("service_name = ?", serviceName).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return offset, err
}

func (r *repository) UpdateOffset(ctx context.Context, serviceName string, lastEventID uuid.UUID) error {
	offset := &OutboxOffset{
		ServiceName: serviceName,
		LastEventID: lastEventID,
	}

	_, err := r.db.NewInsert().
		Model(offset).
		On("CONFLICT (service_name) DO UPDATE").
		Set("last_event_id = EXCLUDED.last_event_id").
		Set("last_processed = EXCLUDED.last_processed").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}
