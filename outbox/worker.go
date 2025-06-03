package outbox

import (
	"context"
	"github.com/ThreeDotsLabs/watermill"
	wkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill-sql/v3/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/code19m/errx"
	"github.com/code19m/pkg/alert"
	log "github.com/code19m/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"strings"
)

type outboxWorker struct {
	forwarder *forwarder.Forwarder
	publisher message.Publisher
}

func NewOutboxWorker(cfg WorkerConfig, pool *pgxpool.Pool, logger log.Logger, alertProvider alert.Provider) (*outboxWorker, error) {

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

	return &outboxWorker{
		forwarder: forwarder,
		publisher: publisher,
	}, nil
}

func (w *outboxWorker) Start() error {
	return w.forwarder.Run(context.Background())
}

func (w *outboxWorker) Stop() error {
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
			GenerateMessagesTableName: func(topic string) string {
				return outboxTableName
			},
			SubscribeBatchSize: int(cfg.BatchSize),
		},
		OffsetsAdapter: sql.DefaultPostgreSQLOffsetsAdapter{
			GenerateMessagesOffsetsTableName: func(topic string) string {
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

	marshaler := wkafka.NewWithPartitioningMarshaler(func(topic string, msg *message.Message) (string, error) {
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
			messagesm, err := next(msg)
			if err == nil {
				return messagesm, nil
			}

			operation := "outbox_worker"
			details := make(map[string]string)

			sendErr := alertProvider.SendError(context.Background(), "", err.Error(), operation, details)
			if err != nil {
				logger.Error("Failed to send error alert", sendErr, nil)
			}

			return nil, err
		}
	}
}
