package kafka

import (
	"context"
	"strings"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/kafka/otelsarama"
	"go.opentelemetry.io/otel"
)

// Producer represents a Kafka producer.
type Producer struct {
	cfg          ProducerConfig
	serviceName  string
	saramaCfg    *sarama.Config
	syncProducer sarama.SyncProducer
}

// NewProducer creates a new Kafka producer.
func NewProducer(
	cfg ProducerConfig,
	serviceName string,
) (*Producer, error) {
	saramaCfg, err := cfg.getSaramaConfig(serviceName)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// Create a new sync producer
	producer, err := sarama.NewSyncProducer(strings.Split(cfg.Brokers, ","), saramaCfg)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// Wrap producer with OpenTelemetry instrumentation
	wrappedProducer := otelsarama.WrapSyncProducer(saramaCfg, producer)

	return &Producer{
		cfg:          cfg,
		serviceName:  serviceName,
		saramaCfg:    saramaCfg,
		syncProducer: wrappedProducer,
	}, nil
}

// SendMessage sends a message to the configured Kafka topic.
func (p *Producer) SendMessage(ctx context.Context, key []byte, value []byte, headers map[string]string) error {
	// Start timing for logging
	// Create message and set headers
	msg := &sarama.ProducerMessage{
		Topic: p.cfg.Topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}

	// Add any custom headers
	for k, v := range headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	// Add tracing information
	p.injectTracing(ctx, msg)

	// Produce message with error handling and logging
	partition, offset, err := p.syncProducer.SendMessage(msg)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(errx.D{
			"topic":     msg.Topic,
			"partition": partition,
			"offset":    offset,
			"key":       string(key),
			"headers":   msg.Headers,
		}))
	}

	return nil
}

// injectTracing adds OpenTelemetry tracing to the context and message.
func (p *Producer) injectTracing(ctx context.Context, msg *sarama.ProducerMessage) {
	carrier := otelsarama.NewProducerMessageCarrier(msg)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// Close closes the producer.
func (p *Producer) Close() error {
	return errx.Wrap(p.syncProducer.Close())
}
