package server

import (
	"fmt"
	"time"
)

// Config defines configuration options for the HTTP server.
type Config struct {
	// Debug enables debug mode for verbose responses on error cases.
	Debug bool `yaml:"debug"`

	// Host address to bind the server to (required).
	Host string `yaml:"host" validate:"required"`

	// Port number to listen on (required).
	Port int `yaml:"port" validate:"required"`

	// ReadTimeout is a maximum duration for reading the entire request. Default is 5 seconds.
	ReadTimeout time.Duration `yaml:"read_timeout" validate:"required" default:"5s"`

	// WriteTimeout is a maximum duration before timing out writes of the response. Default is 5 seconds.
	WriteTimeout time.Duration `yaml:"write_timeout" validate:"required" default:"5s"`

	// IdleTimeout is a maximum amount of time to wait for the next request. Default is 120 seconds.
	IdleTimeout time.Duration `yaml:"idle_timeout" validate:"required" default:"120s"`

	// HandleTimeout is a maximum duration for handling a single request. Default is 10 seconds.
	HandleTimeout time.Duration `yaml:"request_timeout" validate:"required" default:"10s"`
}

// Address returns the server's listen address in the form "host:port".
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
