package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Service interface {
	StoreEvent(ctx context.Context, tx bun.Tx, event Event, topic string, options ...StoreOption) error

	ProcessAvailableEvents(ctx context.Context, batchSize int, processor func([]*OutboxEntry) error) error

	RetryFailedEvents(ctx context.Context, batchSize int, processor func([]*OutboxEntry) error) error

	MarkAsPublished(ctx context.Context, eventIDs []uuid.UUID) error

	MarkAsFailed(ctx context.Context, eventID uuid.UUID, errorMsg string) error

	CleanupOldEvents(ctx context.Context, olderThan time.Duration) (int, error)

	StartEventProcessor(ctx context.Context, config EnhancedProcessorConfig, processor EventProcessor) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) StoreEvent(ctx context.Context, tx bun.Tx, event Event, topic string, options ...StoreOption) error {
	return s.repo.StoreWithDeduplication(ctx, tx, event, topic, options...)
}

func (s *service) ProcessAvailableEvents(ctx context.Context, batchSize int, processor func([]*OutboxEntry) error) error {
	events, err := s.repo.GetAvailableEvents(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get available events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	if err := processor(events); err != nil {
		for _, event := range events {
			_ = s.repo.MarkAsFailed(ctx, event.ID, err.Error())
		}
		return fmt.Errorf("failed to process events: %w", err)
	}

	var eventIDs []uuid.UUID
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}

	if err := s.MarkAsPublished(ctx, eventIDs); err != nil {
		return fmt.Errorf("failed to mark events as published: %w", err)
	}

	return nil
}

func (s *service) RetryFailedEvents(ctx context.Context, batchSize int, processor func([]*OutboxEntry) error) error {
	events, err := s.repo.GetAvailableEvents(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get available events for retry: %w", err)
	}

	var failedEvents []*OutboxEntry
	for _, event := range events {
		if event.Status == EventStatusFailed {
			failedEvents = append(failedEvents, event)
		}
	}

	if len(failedEvents) == 0 {
		return nil
	}

	if err := processor(failedEvents); err != nil {
		for _, event := range failedEvents {
			_ = s.repo.MarkAsFailed(ctx, event.ID, err.Error())
		}
		return fmt.Errorf("failed to retry events: %w", err)
	}

	var eventIDs []uuid.UUID
	for _, event := range failedEvents {
		eventIDs = append(eventIDs, event.ID)
	}

	if err := s.MarkAsPublished(ctx, eventIDs); err != nil {
		return fmt.Errorf("failed to mark retried events as published: %w", err)
	}

	return nil
}

func (s *service) MarkAsPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	for _, eventID := range eventIDs {
		if err := s.repo.MarkAsPublished(ctx, eventID); err != nil {
			return fmt.Errorf("failed to mark event %s as published: %w", eventID, err)
		}
	}
	return nil
}

func (s *service) MarkAsFailed(ctx context.Context, eventID uuid.UUID, errorMsg string) error {
	return s.repo.MarkAsFailed(ctx, eventID, errorMsg)
}

func (s *service) CleanupOldEvents(ctx context.Context, olderThan time.Duration) (int, error) {
	if err := s.repo.DeleteOldEvents(ctx, olderThan); err != nil {
		return 0, fmt.Errorf("failed to cleanup old events: %w", err)
	}
	return 0, nil
}

type EventProcessor func([]*OutboxEntry) error

type EnhancedProcessorConfig struct {
	BatchSize    int
	PollInterval time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

func DefaultEnhancedProcessorConfig() EnhancedProcessorConfig {
	return EnhancedProcessorConfig{
		BatchSize:    10,
		PollInterval: 5 * time.Second,
		MaxRetries:   5,
		RetryBackoff: 30 * time.Second,
	}
}

func (s *service) StartEventProcessor(ctx context.Context, config EnhancedProcessorConfig, processor EventProcessor) error {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.ProcessAvailableEvents(ctx, config.BatchSize, processor); err != nil {
				fmt.Printf("Error processing events: %v\n", err)
			}

			if err := s.RetryFailedEvents(ctx, config.BatchSize, processor); err != nil {
				fmt.Printf("Error retrying failed events: %v\n", err)
			}
		}
	}
}
