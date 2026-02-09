// Package alert provides functionality for sending alerts and error reports
// to Discord or Telegram with intelligent cooldown management.
// It stores errors in PostgreSQL and prevents alert fatigue by enforcing
// a minimum interval between notifications for the same service+operation.
package alert

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

const (
	providerDiscord  = "discord"
	providerTelegram = "telegram"
	provideNoop      = "noop"
)

// Config defines configuration options for the alert package.
type Config struct {
	// Provider specifies the notification provider to use ("discord" or "telegram").
	Provider string `yaml:"provider" validate:"oneof=discord telegram noop" default:"noop"`

	// CooldownMinutes is the minimum interval (in minutes) between alerts
	// for the same service+operation combination.
	CooldownMinutes int `yaml:"cooldown_minutes" default:"5"`

	// SendTimeout is the timeout duration for sending a notification.
	SendTimeout time.Duration `yaml:"send_timeout" default:"3s"`

	// Schema is the PostgreSQL schema for the errors table.
	Schema string `yaml:"schema" default:"sentinel"`

	// TelegramBotToken is the Telegram bot token (required when Provider is "telegram").
	TelegramBotToken string `yaml:"telegram_bot_token" mask:"true"`

	// TelegramChatIDs is the list of Telegram chat IDs to send alerts to.
	TelegramChatIDs []int64 `yaml:"telegram_chat_ids"`

	// DiscordBotToken is the Discord bot token (required when Provider is "discord").
	DiscordBotToken string `yaml:"discord_bot_token" mask:"true"`

	// DiscordChannelIDs is the list of Discord channel IDs to send alerts to.
	DiscordChannelIDs []string `yaml:"discord_channel_ids"`
}

// Provider defines the interface for sending error alerts.
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
// If cfg.Provider is noop, it returns a no-op provider.
func NewProvider(cfg Config, db *bun.DB) (Provider, error) {
	if cfg.Provider == provideNoop {
		return &noOpProvider{}, nil
	}

	s, err := newStore(db, cfg.Schema)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	n, err := newNotifier(cfg)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &alertProvider{
		cfg:      cfg,
		store:    s,
		notifier: n,
	}, nil
}

// noOpProvider is a no-operation alert provider that does nothing.
type noOpProvider struct{}

func (n *noOpProvider) SendError(
	_ context.Context,
	_, _, _ string,
	_ map[string]string,
) error {
	return nil
}

// --- Global singleton ---

//nolint:gochecknoglobals // Global variables are required for the global alert singleton pattern
var (
	global   atomic.Value // stores Provider
	setOnce  sync.Once    // ensures SetGlobal is called once
	initOnce sync.Once    // ensures lazy initialization happens once
)

// SetGlobal sets the global alert provider instance.
// This should be called during application startup to configure the global alert provider.
// This should be called before any alert functions are used.
// The db parameter is the bun.DB connection used for storing error records.
// When cfg.Disable is true, db can be nil.
// Returns an error if:
// - The provider creation fails;
// - SetGlobal is called more than once.
func SetGlobal(cfg Config, db *bun.DB) error {
	var err error
	called := false

	setOnce.Do(func() {
		// Prevent lazy initialization from happening after this
		initOnce.Do(func() {})

		provider, providerErr := NewProvider(cfg, db)
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

func initDefault() {
	initOnce.Do(func() {
		global.Store(&noOpProvider{})
	})
}

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
