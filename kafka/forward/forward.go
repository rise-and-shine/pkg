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
			Named("kafka.forward.debug_logger").
			WithContext(ctx).
			With("event", mask.StructToOrdMap(event))

		err = uc.Handle(ctx, event)
		if err != nil {
			log.Errorx(err)
			return errx.Wrap(err)
		}

		log.Debug(nil)
		return nil
	}
}

func newEvent[E any]() (E, error) {
	var req E

	reqType := reflect.TypeOf((*E)(nil)).Elem()
	if reqType.Kind() != reflect.Pointer || reqType.Elem().Kind() != reflect.Struct {
		return req, errx.New("input type I must be a pointer")
	}

	reqVal := reflect.New(reqType.Elem()).Interface().(E) //nolint:errcheck // safe type assertion
	return reqVal, nil
}
