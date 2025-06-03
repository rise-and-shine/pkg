package wrgrpc

import (
	"fmt"
	"time"
)

// Default message size limits for gRPC
const (
	MaxSendMessageLength    = 2147483647 // 2GB
	MaxReceiveMessageLength = 63554432   // 60MB
)

// Сonfig is the configuration for the gRPC server
type Config struct {
	// Host is the server's bind address (default: 0.0.0.0 or "localhost")
	Host string `yaml:"host" validate:"required"`

	// Port is the server's bind port (default: 8080)
	Port string `yaml:"port" validate:"required"`

	// Reflection enables gRPC reflection, which provides information about the gRPC server
	Reflection bool `yaml:"reflection"`

	// UnaryTimeout is the timeout for unary RPCs
	UnaryTimeout time.Duration `yaml:"unary_timeout" validate:"required" default:"30"`
}

func (c Config) Address() string {
	// Return the server's address in the format "host:port"
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}
