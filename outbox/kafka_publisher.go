package outbox

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/code19m/errx"
)

// KafkaPublisher implements Publisher interface using Watermill
type KafkaPublisher struct {
	publisher message.Publisher
}

// NewKafkaPublisher creates a new Kafka publisher using Watermill
func NewKafkaPublisher(publisher message.Publisher) Publisher {
	return &KafkaPublisher{
		publisher: publisher,
	}
}

// Publish implements Publisher interface
func (k *KafkaPublisher) Publish(ctx context.Context, msg *Message) error {
	// Create Watermill message
	watermillMsg := message.NewMessage(msg.ID, msg.Value)

	// Set message metadata
	if msg.Headers != nil {
		for key, value := range msg.Headers {
			watermillMsg.Metadata.Set(key, value)
		}
	}

	if msg.Key != "" {
		watermillMsg.Metadata.Set("partition_key", msg.Key)
	}

	// Publish the message
	if err := k.publisher.Publish(msg.Topic, watermillMsg); err != nil {
		return errx.Wrap(err)
	}

	return nil
}

// Close closes the publisher
func (k *KafkaPublisher) Close() error {
	return k.publisher.Close()
}
