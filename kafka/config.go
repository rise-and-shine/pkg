package kafka

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
)

const (
	newestOffset = "newest"
	oldestOffset = "oldest"
)

type ConsumerConfig struct {
	ServiceName    string `yaml:"service_name"    validate:"required"`
	ServiceVersion string `yaml:"service_version" validate:"required"`
	Brokers        string `yaml:"brokers"         validate:"required"`
	Topic          string `yaml:"topic"           validate:"required"`

	// if not set default to the service name
	GroupID string `yaml:"group_id"`

	KafkeVersion   string        `yaml:"kafke_version"   default:"3.6.0"`
	InitialOffset  string        `yaml:"initial_offset"  default:"newest" validate:"oneof=newest oldest"`
	HandlerTimeout time.Duration `yaml:"handler_timeout" default:"30s"`
	RetryDisabled  bool          `yaml:"retry_disabled"  default:"false"`
	RetryCount     uint8         `yaml:"retry_count"     default:"3"`
	RetryDelay     time.Duration `yaml:"retry_delay"     default:"100ms"`
}

func (c *ConsumerConfig) getSaramaConfig() (*sarama.Config, error) {
	if c.GroupID == "" {
		c.GroupID = c.ServiceName
	}

	saramaConf := sarama.NewConfig()
	saramaConf.ClientID = c.ServiceName

	version, err := sarama.ParseKafkaVersion(c.KafkeVersion)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	saramaConf.Version = version

	switch c.InitialOffset {
	case newestOffset:
		saramaConf.Consumer.Offsets.Initial = sarama.OffsetNewest
	case oldestOffset:
		saramaConf.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		return nil, errx.New(fmt.Sprintf("unknown initial offset %s", c.InitialOffset))
	}

	return saramaConf, nil
}
