package alert

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

//nolint:gochecknoglobals // Global variables are required for the global alert singleton pattern
var (
	global   atomic.Value // stores Provider
	setOnce  sync.Once    // ensures SetGlobal is called once
	initOnce sync.Once    // ensures lazy initialization happens once
)

// SetGlobal sets the global alert provider instance.
// This should be called during application startup to configure the global alert provider.
// This should be called before any alert functions are used.
// Returns an error if:
// - The provider creation fails;
// - SetGlobal is called more than once.
func SetGlobal(cfg Config, serviceName, serviceVersion string) error {
	var err error
	called := false

	setOnce.Do(func() {
		// Prevent lazy initialization from happening after this
		initOnce.Do(func() {})

		provider, providerErr := NewProvider(cfg, serviceName, serviceVersion)
		if providerErr != nil {
			err = fmt.Errorf("[alert]: failed to initialize global alert provider: %w", providerErr)
			return
		}
		global.Store(provider)
		called = true
	})

	if !called {
		return errors.New("[alert]: SetGlobal can only be called once")
	}

	return err
}

// SendError sends an error alert using the global provider.
// If SetGlobal has not been called, it uses a no-op provider that silently does nothing.
func SendError(ctx context.Context, errCode, msg, operation string, details map[string]string) error {
	return getGlobal().SendError(ctx, errCode, msg, operation, details)
}

// initDefault initializes the default no-op provider lazily.
// This is used when SetGlobal is not called, providing silent no-op behavior.
func initDefault() {
	initOnce.Do(func() {
		global.Store(&noOpProvider{})
	})
}

// getGlobal returns the current global alert provider instance.
// If no provider has been set, it initializes a no-op provider lazily.
func getGlobal() Provider {
	if p := global.Load(); p != nil {
		provider, ok := p.(Provider)
		if !ok {
			panic("[alert]: global contains invalid type")
		}
		return provider
	}
	initDefault()
	provider, ok := global.Load().(Provider)
	if !ok {
		panic("[alert]: global contains invalid type after initialization")
	}
	return provider
}
