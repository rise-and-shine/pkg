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
	slowQueryThreshold time.Duration
}

// DebugHookOption is a function that configures a QueryHook.
type DebugHookOption func(*DebugHook)

// NewDebugHook creates a new query hook with the provided options.
// By default, the hook is enabled, verbose mode is on, and slow query threshold is 100ms.
func NewDebugHook(opts ...DebugHookOption) *DebugHook {
	hook := &DebugHook{
		slowQueryThreshold: 100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(hook)
	}

	return hook
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
	// Calculate query duration
	duration := time.Since(event.StartTime)

	// Determine error type
	isNoRowsError := errors.Is(event.Err, sql.ErrNoRows)

	// Treat transaction done errors as no error
	isTxDoneError := errors.Is(event.Err, sql.ErrTxDone)

	hasError := event.Err != nil && !isNoRowsError && !isTxDoneError

	// Check if query is slow
	isSlow := h.slowQueryThreshold > 0 && duration >= h.slowQueryThreshold

	// Build logger with structured fields
	logEntry := logger.Named("bun_debug_hook").
		WithContext(ctx).
		With("query", formatQuery(event.Query)).
		With("duration", duration.Round(time.Microsecond))

	// Add query args if present
	if len(event.QueryArgs) > 0 {
		logEntry = logEntry.With("args", event.QueryArgs)
	}

	// Determine actual operation from query
	operation := getActualOperation(event)

	// Log at appropriate level
	switch {
	case hasError:
		logEntry.With("error", event.Err).Error("[bun-debug] - " + operation)
	case isNoRowsError:
		logEntry.With("error", event.Err).Warn("[bun-debug] - " + operation)
	case isSlow:
		logEntry.Warn("[bun-debug] - " + operation)
	default:
		logEntry.Debug("[bun-debug] - " + operation)
	}
}

// getActualOperation determines the actual SQL operation from the query.
// This is needed because bun's event.Operation() returns "SELECT" for INSERT...RETURNING queries
// when using .Scan(), since Scan is typically associated with SELECT operations.
func getActualOperation(event *bun.QueryEvent) string {
	query := strings.TrimSpace(strings.ToUpper(event.Query))

	// Check for common SQL operations in order
	switch {
	case strings.HasPrefix(query, "INSERT"):
		return "INSERT"
	case strings.HasPrefix(query, "UPDATE"):
		return "UPDATE"
	case strings.HasPrefix(query, "DELETE"):
		return "DELETE"
	case strings.HasPrefix(query, "SELECT"):
		return "SELECT"
	case strings.HasPrefix(query, "WITH"):
		// For CTEs, look for the main operation after the CTE
		if strings.Contains(query, "INSERT") {
			return "INSERT"
		}
		if strings.Contains(query, "UPDATE") {
			return "UPDATE"
		}
		if strings.Contains(query, "DELETE") {
			return "DELETE"
		}
		return "SELECT" // Default for CTEs
	default:
		// Fall back to bun's operation detection
		return event.Operation()
	}
}

// formatQuery cleans up the query string for logging by removing formatting characters.
// It removes newlines, tabs, double quotes, and collapses multiple spaces into single spaces.
func formatQuery(query string) string {
	// Remove double quotes
	query = strings.ReplaceAll(query, "\"", "")

	// Remove newlines
	query = strings.ReplaceAll(query, "\n", " ")

	// Remove tabs
	query = strings.ReplaceAll(query, "\t", " ")

	// Collapse multiple spaces into single space
	for strings.Contains(query, "  ") {
		query = strings.ReplaceAll(query, "  ", " ")
	}

	// Trim leading and trailing whitespace
	return strings.TrimSpace(query)
}
