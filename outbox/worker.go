package outbox

import (
	"context"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	wkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill-sql/v3/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/code19m/errx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rise-and-shine/pkg/alert"
	"github.com/rise-and-shine/pkg/logger"
)

type Worker struct {
	forwarder *forwarder.Forwarder
	publisher message.Publisher
}

func NewWorker(
	cfg WorkerConfig,
	pool *pgxpool.Pool,
	logger logger.Logger,
	alertProvider alert.Provider,
) (*Worker, error) {
	// wrappers for watermill compatability
	loggerAdapter := newLoggerAdapter(logger.Named("outbox"))
	db := stdlib.OpenDBFromPool(pool)

	subscriber, err := newSubscriber(cfg, db, loggerAdapter)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	publisher, err := newPublisher(cfg, loggerAdapter)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	forwarder, err := forwarder.NewForwarder(
		subscriber,
		publisher,
		loggerAdapter,
		forwarder.Config{
			ForwarderTopic: outboxTableName,
			Middlewares: []message.HandlerMiddleware{
				newAlertMiddleware(alertProvider, loggerAdapter),
			},
		},
	)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &Worker{
		forwarder: forwarder,
		publisher: publisher,
	}, nil
}

func (w *Worker) Start() error {
	return w.forwarder.Run(context.Background())
}

func (w *Worker) Stop() error {
	// stop forwarder
	err := w.forwarder.Close()
	if err != nil {
		return errx.Wrap(err)
	}

	// stop publisher
	return errx.Wrap(w.publisher.Close())
}

func newSubscriber(cfg WorkerConfig, db sql.Beginner, logger watermill.LoggerAdapter) (*sql.Subscriber, error) {
	subscriberCfg := sql.SubscriberConfig{
		ConsumerGroup:  cfg.ServiceName,
		BackoffManager: sql.NewDefaultBackoffManager(cfg.PollInterval, cfg.RetryInterval),
		AckDeadline:    nil,
		ResendInterval: cfg.ResendInterval,
		SchemaAdapter: sql.DefaultPostgreSQLSchema{
			GenerateMessagesTableName: func(_ string) string {
				return outboxTableName
			},
			SubscribeBatchSize: cfg.BatchSize,
		},
		OffsetsAdapter: sql.DefaultPostgreSQLOffsetsAdapter{
			GenerateMessagesOffsetsTableName: func(_ string) string {
				return offsetTableName
			},
		},
		InitializeSchema: true,
	}

	subscriber, err := sql.NewSubscriber(db, subscriberCfg, logger)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return subscriber, nil
}

func newPublisher(cfg WorkerConfig, logger watermill.LoggerAdapter) (message.Publisher, error) {
	saramaCfg := wkafka.DefaultSaramaSyncPublisherConfig()
	saramaCfg.ClientID = cfg.ServiceName

	marshaler := wkafka.NewWithPartitioningMarshaler(func(_ string, msg *message.Message) (string, error) {
		partitionKey := msg.Metadata.Get("partition_key")
		if partitionKey == "" {
			return "", errx.New("partition key is empty")
		}
		return partitionKey, nil
	})

	publisher, err := wkafka.NewPublisher(strings.Split(cfg.Brokers, ","), marshaler, saramaCfg, logger)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return publisher, nil
}

func newAlertMiddleware(alertProvider alert.Provider, logger watermill.LoggerAdapter) message.HandlerMiddleware {
	return func(next message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			messages, err := next(msg)
			if err == nil {
				return messages, nil
			}

			operation := "outbox_worker"
			details := make(map[string]string)

			sendErr := alertProvider.SendError(context.Background(), "", err.Error(), operation, details)
			if sendErr != nil {
				logger.Error("Failed to send error alert", sendErr, nil)
			}

			return nil, err
		}
	}
}
