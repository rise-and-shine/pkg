package outbox

import "time"

const (
	outboxTableName = "_outbox"
	offsetTableName = "_outbox_offset"
)

type WorkerConfig struct {
	ServiceName    string        `yaml:"service_name"    validate:"required"`
	Brokers        string        `yaml:"brokers"         validate:"required"`
	PollInterval   time.Duration `yaml:"poll_interval"                       default:"500ms"`
	RetryInterval  time.Duration `yaml:"retry_interval"                      default:"1s"`
	ResendInterval time.Duration `yaml:"resend_interval"                     default:"1s"`
	BatchSize      int           `yaml:"batch_size"                          default:"100"`
}
