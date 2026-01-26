package forward

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/kafka"
	"github.com/rise-and-shine/pkg/mask"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/ucdef"
)

// ToEventSubscriber forwards a Kafka message to an event subscriber use case.
// It handles event decoding from JSON and logging.
// E is the event type.
func ToEventSubscriber[E any](uc ucdef.EventSubscriber[E]) kafka.HandleFunc {
	return func(ctx context.Context, cm *sarama.ConsumerMessage) error {
		event, err := newEvent[E]()
		if err != nil {
			return errx.Wrap(err)
		}

		err = json.Unmarshal(cm.Value, event)
		if err != nil {
			return errx.Wrap(err)
		}

		log := logger.
			Named("kafka.handler").
			WithContext(ctx).
			With(
				"operation_id", uc.OperationID(),
				"event", mask.StructToOrdMap(event),
			)

		err = uc.Handle(ctx, event)
		if err != nil {
			log.Errorx(err)
			return errx.Wrap(err)
		}

		log.Debug("")
		return nil
	}
}

func newEvent[E any]() (E, error) {
	var req E

	reqType := reflect.TypeOf((*E)(nil)).Elem()
	if reqType.Kind() != reflect.Pointer || reqType.Elem().Kind() != reflect.Struct {
		return req, errx.New("event type I must be a pointer")
	}

	reqVal := reflect.New(reqType.Elem()).Interface().(E) //nolint:errcheck // safe type assertion
	return reqVal, nil
}
