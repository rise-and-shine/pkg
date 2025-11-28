package hooks

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/uptrace/bun"
)

// Verify that DebugHook implements bun.QueryHook interface at compile time.
var _ bun.QueryHook = (*DebugHook)(nil)

// DebugHook is a custom Bun query hook that integrates with the rise-and-shine logger.
// It logs database queries with configurable verbosity and slow query detection.
type DebugHook struct {
	enabled            bool
	verbose            bool
	slowQueryThreshold time.Duration
}

// DebugHookOption is a function that configures a QueryHook.
type DebugHookOption func(*DebugHook)

// NewDebugHook creates a new query hook with the provided options.
// By default, the hook is enabled, verbose mode is on, and slow query threshold is 100ms.
func NewDebugHook(opts ...DebugHookOption) *DebugHook {
	hook := &DebugHook{
		enabled:            true,
		verbose:            true,
		slowQueryThreshold: 100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(hook)
	}

	return hook
}

// WithEnabled sets whether the query hook is enabled.
// When disabled, no queries will be logged.
func WithEnabled(enabled bool) DebugHookOption {
	return func(h *DebugHook) {
		h.enabled = enabled
	}
}

// WithVerbose sets whether to log all queries or only failures and warnings.
// When verbose=true: logs all queries at debug level, errors at error level, warnings at warn level.
// When verbose=false: logs only errors and warnings (failed queries, no rows, slow queries).
func WithVerbose(verbose bool) DebugHookOption {
	return func(h *DebugHook) {
		h.verbose = verbose
	}
}

// WithSlowQueryThreshold sets the duration threshold for logging slow queries at warn level.
// Set to 0 to disable slow query detection.
func WithSlowQueryThreshold(threshold time.Duration) DebugHookOption {
	return func(h *DebugHook) {
		h.slowQueryThreshold = threshold
	}
}

// BeforeQuery implements bun.QueryHook interface.
// It is called before query execution and simply returns the context unchanged.
func (h *DebugHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery implements bun.QueryHook interface.
// It is called after query execution and logs the query with appropriate detail level.
func (h *DebugHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	// Early return if hook is disabled
	if !h.enabled {
		return
	}

	// Calculate query duration
	duration := time.Since(event.StartTime)

	// Determine error type
	isNoRowsError := errors.Is(event.Err, sql.ErrNoRows)

	// Treat transaction done errors as no error
	isTxDoneError := errors.Is(event.Err, sql.ErrTxDone)

	hasError := event.Err != nil && !isNoRowsError && !isTxDoneError

	// Check if query is slow
	isSlow := h.slowQueryThreshold > 0 && duration >= h.slowQueryThreshold

	// Skip logging successful queries if not in verbose mode
	if !h.verbose && !hasError && !isNoRowsError && !isSlow {
		return
	}

	// Build logger with structured fields
	logEntry := logger.Named("bun_debug_hook").
		WithContext(ctx).
		With("query", formatQuery(event.Query)).
		With("duration", duration.Round(time.Microsecond))

	// Add query args if present
	if len(event.QueryArgs) > 0 {
		logEntry = logEntry.With("args", event.QueryArgs)
	}

	// Log at appropriate level
	switch {
	case hasError:
		logEntry.With("error", event.Err).Error("[bun-debug] - " + event.Operation())
	case isNoRowsError:
		logEntry.With("error", event.Err).Warn("[bun-debug] - " + event.Operation())
	case isSlow:
		logEntry.Warn("[bun-debug] - " + event.Operation())
	case h.verbose:
		logEntry.Debug("[bun-debug] - " + event.Operation())
	}
}

// formatQuery cleans \" symbols from the query string.
func formatQuery(query string) string {
	return strings.ReplaceAll(query, "\"", "")
}
