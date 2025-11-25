// Package logger provides a structured logging interface for applications.
package logger

import (
	"context"
	"errors"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/meta"
	"go.uber.org/zap"
)

// Logger defines the standard logging interface used across applications.
// It provides methods for different log levels and formatting options.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(args ...any)
	// Info logs a message at info level.
	Info(args ...any)
	// Warn logs a message at warn level.
	Warn(args ...any)
	// Error logs a message at error level.
	Error(args ...any)
	// Fatal logs a message at fatal level and then calls os.Exit(1).
	Fatal(args ...any)

	// Debugf logs a formatted message at debug level.
	Debugf(format string, args ...any)
	// Infof logs a formatted message at info level.
	Infof(format string, args ...any)
	// Warnf logs a formatted message at warn level.
	Warnf(format string, args ...any)
	// Errorf logs a formatted message at error level.
	Errorf(format string, args ...any)
	// Fatalf logs a formatted message at fatal level and then calls os.Exit(1).
	Fatalf(format string, args ...any)

	// Warnx is a special method for easy logging errx.ErrorX instances at warn level.
	Warnx(err error)
	// Errorx is a special method for easy logging errx.ErrorX instances at error level.
	Errorx(err error)
	// Fatalx is a special method for easy logging errx.ErrorX instances at fatal level and then calls os.Exit(1).
	Fatalx(err error)

	// With creates a new logger with the given key-value pairs.
	// The returned logger inherits the properties of the original logger
	// and includes the provided key-value pairs in all subsequent log entries.
	With(keysAndValues ...any) Logger
	// WithContext creates a logger with context information,
	// enriching the log entries with metadata from the context.
	WithContext(ctx context.Context) Logger

	// Named adds a sub-scope to the logger's name.
	Named(name string) Logger

	// Sync flushes any buffered log entries.
	// Intended for use on application shutdown to ensure all logs are written.
	Sync() error
}

// logger implements the Logger interface using zap's SugaredLogger.
type logger struct {
	*zap.SugaredLogger
}

// newWithCallerSkip creates a new Logger with the specified caller skip level.
// The callerSkip parameter indicates how many additional stack frames to skip
// when determining the caller location. Use callerSkip=1 for global logger wrappers.
func newWithCallerSkip(cfg Config, callerSkip int) (Logger, error) {
	if cfg.Disable {
		return &logger{zap.NewNop().Sugar()}, nil
	}

	zapConfig, err := cfg.getZapConfig()
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// Use custom development encoder for pretty mode
	if cfg.Encoding == encPretty {
		return &logger{newPrettyLoggerWithSkip(zapConfig, callerSkip).Sugar()}, nil
	}

	// Build the zap logger for JSON mode
	jsonLogger, err := zapConfig.Build(zap.AddCallerSkip(callerSkip))
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &logger{jsonLogger.Sugar()}, nil
}

// New creates a new Logger instance with the provided configuration.
func New(cfg Config) (Logger, error) {
	return newWithCallerSkip(cfg, 0)
}

func (l *logger) Warnx(err error) {
	var e errx.ErrorX
	if errors.As(err, &e) {
		l.With(
			"error_code", e.Code(),
			"error_type", e.Type().String(),
			"error_trace", e.Trace(),
			"error_fields", e.Fields(),
			"error_details", e.Details(),
		).Warn(err.Error())
		return
	}
	l.Warn(err.Error())
}

func (l *logger) Errorx(err error) {
	var e errx.ErrorX
	if errors.As(err, &e) {
		l.With(
			"error_code", e.Code(),
			"error_type", e.Type().String(),
			"error_trace", e.Trace(),
			"error_fields", e.Fields(),
			"error_details", e.Details(),
		).Error(err.Error())
		return
	}
	l.Error(err.Error())
}

func (l *logger) Fatalx(err error) {
	var e errx.ErrorX
	if errors.As(err, &e) {
		l.With(
			"error_code", e.Code(),
			"error_type", e.Type().String(),
			"error_trace", e.Trace(),
			"error_fields", e.Fields(),
			"error_details", e.Details(),
		).Fatal(err.Error())
		return
	}
	l.Fatal(err.Error())
}

func (l *logger) With(keysAndValues ...any) Logger {
	return &logger{
		SugaredLogger: l.SugaredLogger.With(keysAndValues...),
	}
}

func (l *logger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	var withFields []any
	metaData := meta.ExtractMetaFromContext(ctx)
	for k, v := range metaData {
		if v != "" {
			// Convert ContextKey to string to avoid the "non-string keys" error
			withFields = append(withFields, string(k), v)
		}
	}

	if len(withFields) > 0 {
		return l.With(withFields...)
	}

	return l
}

func (l *logger) Named(name string) Logger {
	return &logger{
		SugaredLogger: l.SugaredLogger.Named(name),
	}
}
