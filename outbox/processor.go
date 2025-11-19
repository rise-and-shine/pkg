package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/logger"
)

type ProcessorConfig struct {
	ServiceName     string        `yaml:"service_name" env:"OUTBOX_SERVICE_NAME"`
	Interval        time.Duration `yaml:"interval" env:"OUTBOX_INTERVAL" envDefault:"5s"`
	BatchSize       int           `yaml:"batch_size" env:"OUTBOX_BATCH_SIZE" envDefault:"100"`
	Enabled         bool          `yaml:"enabled" env:"OUTBOX_ENABLED" envDefault:"true"`
	RetryAttempts   int           `yaml:"retry_attempts" env:"OUTBOX_RETRY_ATTEMPTS" envDefault:"3"`
	RetryDelay      time.Duration `yaml:"retry_delay" env:"OUTBOX_RETRY_DELAY" envDefault:"10s"`
	CleanupEnabled  bool          `yaml:"cleanup_enabled" env:"OUTBOX_CLEANUP_ENABLED" envDefault:"true"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" env:"OUTBOX_CLEANUP_INTERVAL" envDefault:"24h"`
	CleanupAfter    int           `yaml:"cleanup_after_days" env:"OUTBOX_CLEANUP_AFTER_DAYS" envDefault:"7"`
}

func DefaultProcessorConfig(serviceName string) ProcessorConfig {
	return ProcessorConfig{
		ServiceName:     serviceName,
		Interval:        5 * time.Second,
		BatchSize:       100,
		Enabled:         true,
		RetryAttempts:   3,
		RetryDelay:      10 * time.Second,
		CleanupEnabled:  true,
		CleanupInterval: 24 * time.Hour,
		CleanupAfter:    7,
	}
}

type Processor struct {
	config    ProcessorConfig
	service   Service
	logger    logger.Logger
	publisher Publisher

	stopCh  chan struct{}
	stopped chan struct{}
}

func NewProcessor(config ProcessorConfig, service Service, publisher Publisher, logger logger.Logger) *Processor {
	return &Processor{
		config:    config,
		service:   service,
		publisher: publisher,
		logger:    logger.Named("outbox.processor"),
		stopCh:    make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

func (p *Processor) Start(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	log := p.logger.WithContext(ctx)
	log.
		With("service_name", p.config.ServiceName).
		With("batch_size", p.config.BatchSize).
		With("interval", p.config.Interval).
		Info("starting enhanced outbox processor")

	if p.canUseBuiltInProcessor() {
		return p.startWithBuiltInProcessor(ctx)
	}

	return p.startWithCustomLoop(ctx)
}

func (p *Processor) canUseBuiltInProcessor() bool {
	return p.publisher != nil && p.config.Interval == 5*time.Second
}

func (p *Processor) startWithBuiltInProcessor(ctx context.Context) error {
	enhancedConfig := EnhancedProcessorConfig{
		BatchSize:    p.config.BatchSize,
		PollInterval: p.config.Interval,
		MaxRetries:   p.config.RetryAttempts,
		RetryBackoff: p.config.RetryDelay,
	}

	eventProcessor := func(events []*OutboxEntry) error {
		successfulEventIDs := make([]uuid.UUID, 0, len(events))

		for _, event := range events {
			if err := p.publishEvent(ctx, event); err != nil {
				p.logger.With("event_id", event.ID).
					With("error", err).
					Error("failed to publish event in built-in processor")

				if markErr := p.service.MarkAsFailed(ctx, event.ID, err.Error()); markErr != nil {
					p.logger.With("event_id", event.ID).
						With("error", markErr).
						Error("failed to mark event as failed")
				}
				continue
			}
			successfulEventIDs = append(successfulEventIDs, event.ID)
		}

		if len(successfulEventIDs) > 0 {
			if err := p.service.MarkAsPublished(ctx, successfulEventIDs); err != nil {
				return fmt.Errorf("failed to mark events as published: %w", err)
			}

			p.logger.With("published_count", len(successfulEventIDs)).
				Info("processed events with built-in processor")
		}
		return nil
	}

	return p.service.StartEventProcessor(ctx, enhancedConfig, eventProcessor)
}

func (p *Processor) startWithCustomLoop(ctx context.Context) error {
	go p.processLoop(ctx)

	if p.config.CleanupEnabled {
		go p.cleanupLoop(ctx)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.stopCh:
		return nil
	}
}

func (p *Processor) Stop() error {
	p.logger.Info("stopping outbox processor")
	close(p.stopCh)

	select {
	case <-p.stopped:
		p.logger.Info("outbox processor stopped")
		return nil
	case <-time.After(30 * time.Second):
		return errx.New("timeout waiting for outbox processor to stop")
	}
}

func (p *Processor) processLoop(ctx context.Context) {
	defer close(p.stopped)

	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("processor context cancelled")
			return
		case <-p.stopCh:
			p.logger.Info("processor stop requested")
			return
		case <-ticker.C:
			p.processEvents(ctx)
		}
	}
}

func (p *Processor) processEvents(ctx context.Context) {
	log := p.logger.WithContext(ctx)

	successfulEventIDs := make([]uuid.UUID, 0)

	eventProcessor := func(events []*OutboxEntry) error {
		for _, event := range events {
			if err := p.publishEvent(ctx, event); err != nil {
				log.With("event_id", event.ID).
					With("error", err).
					Error("failed to publish event")

				if markErr := p.service.MarkAsFailed(ctx, event.ID, err.Error()); markErr != nil {
					log.With("event_id", event.ID).
						With("error", markErr).
						Error("failed to mark event as failed")
				}
				continue
			}
			successfulEventIDs = append(successfulEventIDs, event.ID)
		}

		if len(successfulEventIDs) > 0 {
			if err := p.service.MarkAsPublished(ctx, successfulEventIDs); err != nil {
				log.With("error", err).Error("failed to mark events as published")
				return err
			}
		}
		return nil
	}

	err := p.service.ProcessAvailableEvents(ctx, p.config.BatchSize, eventProcessor)
	if err != nil {
		log.With("error", err).Error("failed to process outbox events")
		return
	}

	successfulRetryIDs := make([]uuid.UUID, 0)

	retryProcessor := func(events []*OutboxEntry) error {
		for _, event := range events {
			if err := p.publishEvent(ctx, event); err != nil {
				log.With("event_id", event.ID).
					With("error", err).
					Error("failed to retry event")

				if markErr := p.service.MarkAsFailed(ctx, event.ID, err.Error()); markErr != nil {
					log.With("event_id", event.ID).
						With("error", markErr).
						Error("failed to mark retry event as failed")
				}
				continue
			}
			successfulRetryIDs = append(successfulRetryIDs, event.ID)
		}

		if len(successfulRetryIDs) > 0 {
			if err := p.service.MarkAsPublished(ctx, successfulRetryIDs); err != nil {
				log.With("error", err).Error("failed to mark retried events as published")
				return err
			}
		}
		return nil
	}

	if err := p.service.RetryFailedEvents(ctx, p.config.BatchSize, retryProcessor); err != nil {
		log.With("error", err).Error("failed to retry failed outbox events")
	}

	totalProcessed := len(successfulEventIDs) + len(successfulRetryIDs)
	if totalProcessed > 0 {
		log.With("published_count", len(successfulEventIDs)).
			With("retried_count", len(successfulRetryIDs)).
			With("total_processed", totalProcessed).
			Info("processed outbox events")
	}
}

func (p *Processor) publishEvent(ctx context.Context, entry *OutboxEntry) error {
	if p.publisher == nil {
		return fmt.Errorf("no publisher configured")
	}

	eventDataBytes, err := json.Marshal(entry.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	message := &Message{
		ID:        entry.ID.String(),
		Topic:     entry.Topic,
		Key:       entry.AggregateID,
		Value:     eventDataBytes,
		Headers:   p.buildHeaders(entry),
		Timestamp: entry.CreatedAt,
	}

	return p.publisher.Publish(ctx, message)
}

func (p *Processor) buildHeaders(entry *OutboxEntry) map[string]string {
	headers := map[string]string{
		"event-id":     entry.ID.String(),
		"event-type":   entry.EventType,
		"aggregate-id": entry.AggregateID,
		"created-at":   entry.CreatedAt.Format(time.RFC3339),
	}

	if entry.DedupKey != nil {
		headers["dedup-key"] = *entry.DedupKey
	}

	if entry.TraceID != nil {
		headers["trace-id"] = *entry.TraceID
	}

	if entry.UserID != nil {
		headers["user-id"] = *entry.UserID
	}

	return headers
}

func (p *Processor) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	p.runCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.runCleanup(ctx)
		}
	}
}

func (p *Processor) runCleanup(ctx context.Context) {
	log := p.logger.WithContext(ctx)

	deleted, err := p.service.CleanupOldEvents(ctx, time.Duration(p.config.CleanupAfter)*24*time.Hour)
	if err != nil {
		log.
			With("error", err).
			Error("failed to cleanup old outbox events")
		return
	}

	if deleted > 0 {
		log.
			With("deleted_count", deleted).
			With("older_than_days", p.config.CleanupAfter).
			Info("cleaned up old outbox events")
	}
}

func (p *Processor) Healthcheck() error {
	select {
	case <-p.stopped:
		return errx.New("outbox processor is stopped")
	default:
		return nil
	}
}

func (p *Processor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"service_name":     p.config.ServiceName,
		"interval":         p.config.Interval.String(),
		"batch_size":       p.config.BatchSize,
		"enabled":          p.config.Enabled,
		"cleanup_enabled":  p.config.CleanupEnabled,
		"cleanup_interval": p.config.CleanupInterval.String(),
		"cleanup_after":    fmt.Sprintf("%d days", p.config.CleanupAfter),
		"retry_attempts":   p.config.RetryAttempts,
		"retry_delay":      p.config.RetryDelay.String(),
		"processor_type":   p.getProcessorType(),
		"has_publisher":    p.publisher != nil,
		"uses_enhanced":    true,
	}
}

func (p *Processor) getProcessorType() string {
	if p.canUseBuiltInProcessor() {
		return "enhanced_built_in"
	}
	return "enhanced_custom_loop"
}
