package kafka

import (
	"time"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/meta"
)

const (
	newestOffset = "newest"
	oldestOffset = "oldest"
)

// ConsumerConfig holds configuration for a single kafka consumer.
type ConsumerConfig struct {
	// Topic is the Kafka topic to consume messages from.
	Topic string `yaml:"topic" validate:"required"`

	// If not set defaults to the service name.
	GroupID string `yaml:"group_id"`

	// InitialOffset can be "newest" or "oldest". Defaults to "newest".
	// "newest" - Start consuming from the end of the topic.
	// "oldest" - Start consuming from the beginning of the topic.
	InitialOffset string `yaml:"initial_offset" default:"newest" validate:"oneof=newest oldest"`

	// HandlerTimeout is the maximum time a handler can take to process a message.
	HandlerTimeout time.Duration `yaml:"handler_timeout" default:"30s"`
}

func (c *ConsumerConfig) getSaramaConfig(brokerConfig BrokerConfig) (*sarama.Config, error) {
	if c.GroupID == "" {
		c.GroupID = meta.ServiceName()
	}
	saramaConf := sarama.NewConfig()
	saramaConf.ClientID = c.GroupID
	version, err := sarama.ParseKafkaVersion(brokerConfig.KafkaVersion)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	saramaConf.Version = version

	// Currently support only SASL_PLAINTEXT authentication.
	if brokerConfig.SaslUsername != "" && brokerConfig.SaslPassword != "" {
		saramaConf.Net.SASL.Enable = true
		saramaConf.Net.SASL.User = brokerConfig.SaslUsername
		saramaConf.Net.SASL.Password = brokerConfig.SaslPassword
		saramaConf.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}

	switch c.InitialOffset {
	case newestOffset:
		saramaConf.Consumer.Offsets.Initial = sarama.OffsetNewest
	case oldestOffset:
		saramaConf.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		return nil, errx.New("[kafka] unknown initial offset", errx.WithDetails(errx.D{
			"initial_offset": c.InitialOffset,
		}))
	}

	return saramaConf, nil
}

// BrokerConfig holds configuration for a Kafka producer.
type BrokerConfig struct {
	Brokers      string `yaml:"brokers"       validate:"required"`
	SaslUsername string `yaml:"sasl_username"`
	SaslPassword string `yaml:"sasl_password"                     mask:"true"`

	KafkaVersion string `yaml:"kafka_version" default:"3.6.0"`
}

func (c *BrokerConfig) getSaramaProducerConfig() (*sarama.Config, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.ClientID = meta.ServiceName()
	version, err := sarama.ParseKafkaVersion(c.KafkaVersion)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	saramaCfg.Version = version

	// Currently support only SASL_PLAINTEXT authentication.
	if c.SaslUsername != "" && c.SaslPassword != "" {
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.User = c.SaslUsername
		saramaCfg.Net.SASL.Password = c.SaslPassword
		saramaCfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}

	// Set Return.Successes and Return.Errors to true,
	// since we are using sync producer.
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true

	return saramaCfg, nil
}
