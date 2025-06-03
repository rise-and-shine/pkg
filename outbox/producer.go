package outbox

import (
	"context"
	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/code19m/pkg/kafka/otelsarama"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/proto"
)

type OutboxProducer interface {
	ProduceProtoMessages(_ context.Context, idb bun.IDB, topic, key string, message proto.Message)
}

func NewProducer() *outboxProducer {
	return &outboxProducer{
		tableName: outboxTableName,
	}
}

type outboxProducer struct {
	tableName string
}

func (op *outboxProducer) ProduceProtoMessages(
	ctx context.Context,
	idb bun.IDB,
	topic, key string,
	message proto.Message,
) error {

	if _, ok := idb.(bun.Tx); !ok {
		return errx.New("idb must be bun.Tx instance")
	}

	msgBytes, err := proto.Marshal(message)
	if err != nil {
		return errx.Wrap(err)
	}

	envelope := &messageEnvelope{
		DestinationTopic: topic,
		UUID:             uuid.NewString(),
		Payload:          msgBytes,
		Metadata: map[string]string{
			"partition_key": key,
		},
	}

	// inject tracing headers into message envelope
	injectTracingHeaders(ctx, key, envelope)

	outBoxData := outboxMsg{
		UUID:     envelope.UUID,
		Payload:  envelope.Payload,
		Metadata: map[string]string{}, // outbox metadata is not used, instead we use envelope metadata
	}

	_, err = idb.NewInsert().
		ModelTableExpr(op.tableName).
		Model(&outBoxData).
		Value("transaction_id", "pg_current_xact_id()"). // Current transaction ID
		Exec(ctx)

	return errx.Wrap(err)
}

func injectTracingHeaders(ctx context.Context, key string, envelope *messageEnvelope) {
	tempMsg := &sarama.ProducerMessage{
		Topic: envelope.DestinationTopic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(envelope.Payload),
	}

	// inject OpenTelemetry tracing context
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, otelsarama.NewProducerMessageCarrier(tempMsg))

	// copy tracing headers from kafka message to envelope metadata
	for _, header := range tempMsg.Headers {
		envelope.Metadata[string(header.Key)] = string(header.Value)
	}
}

// messageEnvelope wraps Watermill message and contains destination topic.
type messageEnvelope struct {
	DestinationTopic string            `json:"destination_topic" bun:"destination_topic"`
	UUID             string            `json:"uuid"              bun:"uuid"`
	Payload          []byte            `json:"payload"           bun:"payload"`
	Metadata         map[string]string `json:"metadata"          bun:"metadata"`
}

// outboxMsg is a struct that represents the single outbox message to be stored in the database.
type outboxMsg struct {
	UUID     string            `bun:"uuid"`
	Payload  any               `bun:"payload"`
	Metadata map[string]string `bun:"metadata"`
}
