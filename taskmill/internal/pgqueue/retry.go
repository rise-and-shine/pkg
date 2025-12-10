package pgqueue

import (
	"math"
	"time"
)

// RetryStrategy defines the interface for retry behavior.
type RetryStrategy interface {
	// ShouldRetry determines if a message should be retried based on attempts.
	ShouldRetry(attempts, maxAttempts int) bool

	// NextRetryDelay calculates the delay before the next retry.
	NextRetryDelay(attempts int) time.Duration
}

// ExponentialBackoffStrategy implements exponential backoff retry strategy.
type ExponentialBackoffStrategy struct {
	// InitialDelay is the base delay for the first retry.
	InitialDelay time.Duration

	// Multiplier is the factor by which the delay increases each attempt.
	Multiplier float64

	// MaxDelay caps the maximum delay between retries.
	MaxDelay time.Duration
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy with defaults.
func NewExponentialBackoffStrategy() *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		Multiplier:   2.0,
		MaxDelay:     5 * time.Minute,
	}
}

// ShouldRetry determines if a message should be retried.
func (s *ExponentialBackoffStrategy) ShouldRetry(attempts, maxAttempts int) bool {
	return attempts < maxAttempts
}

// NextRetryDelay calculates the exponential backoff delay.
func (s *ExponentialBackoffStrategy) NextRetryDelay(attempts int) time.Duration {
	delay := float64(s.InitialDelay) * math.Pow(s.Multiplier, float64(attempts))

	if delay > float64(s.MaxDelay) {
		return s.MaxDelay
	}

	return time.Duration(delay)
}

// FixedDelayStrategy implements a fixed delay retry strategy.
type FixedDelayStrategy struct {
	// Delay is the fixed delay between retries.
	Delay time.Duration
}

// NewFixedDelayStrategy creates a new fixed delay strategy.
func NewFixedDelayStrategy(delay time.Duration) *FixedDelayStrategy {
	return &FixedDelayStrategy{
		Delay: delay,
	}
}

// ShouldRetry determines if a message should be retried.
func (s *FixedDelayStrategy) ShouldRetry(attempts, maxAttempts int) bool {
	return attempts < maxAttempts
}

// NextRetryDelay returns the fixed delay.
func (s *FixedDelayStrategy) NextRetryDelay(_ int) time.Duration {
	return s.Delay
}

// NoRetryStrategy implements a strategy that never retries.
type NoRetryStrategy struct{}

// NewNoRetryStrategy creates a new no-retry strategy.
func NewNoRetryStrategy() *NoRetryStrategy {
	return &NoRetryStrategy{}
}

// ShouldRetry always returns false.
func (s *NoRetryStrategy) ShouldRetry(_, _ int) bool {
	return false
}

// NextRetryDelay returns zero duration.
func (s *NoRetryStrategy) NextRetryDelay(_ int) time.Duration {
	return 0
}
