package kafka

import (
	"context"
	"strings"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/kafka/otelsarama"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
)

// Message represents a Producer Kafka message with key, value, and headers.
type Message struct {
	Key     []byte
	Value   []byte
	Headers map[string]string
}

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
func (p *Producer) SendMessage(ctx context.Context, m *Message) error {
	kafkaMsg := p.buildKafkaProducerMsg(ctx, m)

	// Produce message
	partition, offset, err := p.syncProducer.SendMessage(kafkaMsg)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(errx.D{
			"topic":     kafkaMsg.Topic,
			"partition": partition,
			"offset":    offset,
			"key":       string(m.Key),
			"headers":   kafkaMsg.Headers,
		}))
	}

	return nil
}

// SendMessages sends multiple messages to the configured Kafka topic.
func (p *Producer) SendMessages(ctx context.Context, messages []Message) error {
	kafkaMessages := lo.Map(messages, func(m Message, _ int) *sarama.ProducerMessage {
		return p.buildKafkaProducerMsg(ctx, &m)
	})

	err := p.syncProducer.SendMessages(kafkaMessages)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(errx.D{
			"topic": p.cfg.Topic,
		}))
	}

	return nil
}

func (p *Producer) buildKafkaProducerMsg(ctx context.Context, m *Message) *sarama.ProducerMessage {
	msg := &sarama.ProducerMessage{
		Topic: p.cfg.Topic,
		Key:   sarama.ByteEncoder(m.Key),
		Value: sarama.ByteEncoder(m.Value),
	}

	// Add headers to the message
	for k, v := range m.Headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	// Inject tracing information into the message
	p.injectTracing(ctx, msg)

	return msg
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
