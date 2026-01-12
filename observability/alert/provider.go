// Package alert provides functionality for sending alerts and error reports
// to a centralized monitoring system, currently Sentinel.
// It defines a standard interface for alert providers and includes an
// implementation for sending alerts via gRPC to a Sentinel service.
package alert

import (
	"context"

	"github.com/rise-and-shine/pkg/meta"
)

// Provider defines the interface for sending error alerts.
// Implementations of this interface can send alerts to various monitoring systems.
type Provider interface {
	// SendError sends an error alert with the given details.
	// ctx is the context for the operation.
	// errCode is a specific code identifying the error.
	// msg is a human-readable error message.
	// operation describes the operation during which the error occurred.
	// details is a map of additional string key-value pairs providing more context about the error.
	// Returns an error if sending the alert fails.
	SendError(ctx context.Context, errCode, msg, operation string, details map[string]string) error
}

// NewProvider creates a new alert provider.
// If cfg.Disable is true, it returns a no-op provider.
func NewProvider(cfg Config) (Provider, error) {
	if cfg.Disable {
		return &noOpProvider{}, nil
	}
	return newSentinelProvider(cfg, meta.ServiceName(), meta.ServiceVersion())
}

// noOpProvider is a no-operation alert provider that does nothing.
// It is used as the default global provider when SetGlobal is not called.
type noOpProvider struct{}

// SendError implements the Provider interface but does nothing.
// It always returns nil.
func (n *noOpProvider) SendError(
	_ context.Context,
	_, _, _ string,
	_ map[string]string,
) error {
	return nil
}
