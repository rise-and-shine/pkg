package tracing

import "time"

const (
	reconnectionPeriod = 30 * time.Second
	clientTimeout      = 30 * time.Second
	maxQueueSize       = 10000
	batchTimeout       = 30 * time.Second
	maxExportBatchSize = 1024
)

// Config holds the configuration for the tracing system.
// It is typically loaded from a configuration file (e.g., YAML).
type Config struct {
	// Disable, if true, completely disables tracing. No spans will be collected or exported.
	Disable bool `yaml:"disable" default:"false"`

	// SampleRate determines the sampling rate for traces.
	// It should be a value between 0.0 (no traces) and 1.0 (all traces).
	// For example, a value of 0.1 means 10% of traces will be sampled.
	SampleRate float64 `yaml:"sample_rate" default:"1"`

	// ExporterHost is the hostname or IP address of the OTLP exporter (e.g., Jaeger agent or collector).
	ExporterHost string `yaml:"exporter_host" validate:"required"`

	// ExporterPort is the port number of the OTLP exporter.
	ExporterPort int `yaml:"exporter_port" validate:"required"`

	// Tags is a map of custom key-value pairs to be added as resource attributes to all spans.
	// These can be used to add environment-specific information or other metadata.
	Tags map[string]string `yaml:"tags"`
}
