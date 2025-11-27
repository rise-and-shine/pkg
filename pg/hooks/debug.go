package hooks

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rise-and-shine/pkg/logger"
	"github.com/uptrace/bun"
)

// Verify that QueryHook implements bun.QueryHook interface at compile time.
var _ bun.QueryHook = (*QueryHook)(nil)

// QueryHook is a custom Bun query hook that integrates with the rise-and-shine logger.
// It logs database queries with configurable verbosity and slow query detection.
type QueryHook struct {
	enabled            bool
	verbose            bool
	slowQueryThreshold time.Duration
}

// QueryHookOption is a function that configures a QueryHook.
type QueryHookOption func(*QueryHook)

// NewQueryHook creates a new query hook with the provided options.
// By default, the hook is enabled, verbose mode is on, and slow query threshold is 100ms.
func NewQueryHook(opts ...QueryHookOption) *QueryHook {
	hook := &QueryHook{
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
func WithEnabled(enabled bool) QueryHookOption {
	return func(h *QueryHook) {
		h.enabled = enabled
	}
}

// WithVerbose sets whether to log all queries or only failures and warnings.
// When verbose=true: logs all queries at debug level, errors at error level, warnings at warn level.
// When verbose=false: logs only errors and warnings (failed queries, no rows, slow queries).
func WithVerbose(verbose bool) QueryHookOption {
	return func(h *QueryHook) {
		h.verbose = verbose
	}
}

// WithSlowQueryThreshold sets the duration threshold for logging slow queries at warn level.
// Set to 0 to disable slow query detection.
func WithSlowQueryThreshold(threshold time.Duration) QueryHookOption {
	return func(h *QueryHook) {
		h.slowQueryThreshold = threshold
	}
}

// BeforeQuery implements bun.QueryHook interface.
// It is called before query execution and simply returns the context unchanged.
func (h *QueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery implements bun.QueryHook interface.
// It is called after query execution and logs the query with appropriate detail level.
func (h *QueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
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
		With("query", event.Query).
		With("operation", event.Operation()).
		With("duration", duration.String()).
		With("duration_ms", duration.Milliseconds())

	// Add query args if present
	if len(event.QueryArgs) > 0 {
		logEntry = logEntry.With("args", event.QueryArgs)
	}

	// Log at appropriate level
	if hasError {
		logEntry.With("error", event.Err).Error("[bun_debug_hook] database query failed")
	} else if isNoRowsError {
		logEntry.With("error", event.Err).Warn("[bun_debug_hook] database query returned no rows")
	} else if isSlow {
		logEntry.Warn("[bun_debug_hook] slow database query detected")
	} else if h.verbose {
		logEntry.Debug("[bun_debug_hook] database query executed")
	}
}
