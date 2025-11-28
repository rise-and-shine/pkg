package alert

import "time"

// Config defines configuration options for the alert package.
// It specifies how to connect to the Sentinel service and other operational parameters.
type Config struct {
	// Disable, if true, completely disables alert sending. No alerts will be sent.
	Disable bool `yaml:"disable" default:"false"`

	// SentinelHost is the hostname or IP address of the Sentinel service.
	SentinelHost string `yaml:"sentinel_host" validate:"required"`

	// SentinelPort is the port number of the Sentinel service.
	SentinelPort int `yaml:"sentinel_port" validate:"required"`

	// SendTimeout is the timeout duration for sending an alert to Sentinel.
	SendTimeout time.Duration `yaml:"send_timeout" default:"3s"`
}
