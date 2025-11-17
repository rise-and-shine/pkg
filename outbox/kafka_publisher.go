package outbox

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/code19m/errx"
)

type KafkaPublisher struct {
	publisher message.Publisher
}

func NewKafkaPublisher(publisher message.Publisher) Publisher {
	return &KafkaPublisher{
		publisher: publisher,
	}
}

func (k *KafkaPublisher) Publish(ctx context.Context, msg *Message) error {
	watermillMsg := message.NewMessage(msg.ID, msg.Value)

	if msg.Headers != nil {
		for key, value := range msg.Headers {
			watermillMsg.Metadata.Set(key, value)
		}
	}

	if msg.Key != "" {
		watermillMsg.Metadata.Set("partition_key", msg.Key)
	}

	if err := k.publisher.Publish(msg.Topic, watermillMsg); err != nil {
		return errx.Wrap(err)
	}

	return nil
}

func (k *KafkaPublisher) Close() error {
	return k.publisher.Close()
}
