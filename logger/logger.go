// Package logger provides a structured logging interface for applications.
//
// It wraps the zap logging library to provide a simpler API while maintaining
// high performance. The package supports different log levels, formatting options,
// and context-aware logging.
package logger

import (
	"context"
	"os"

	"github.com/code19m/errx"
	"github.com/code19m/pkg/meta"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	// Debugw logs a message with key-value pairs at debug level.
	Debugw(msg string, keysAndValues ...any)
	// Infow logs a message with key-value pairs at info level.
	Infow(msg string, keysAndValues ...any)
	// Warnw logs a message with key-value pairs at warn level.
	Warnw(msg string, keysAndValues ...any)
	// Errorw logs a message with key-value pairs at error level.
	Errorw(msg string, keysAndValues ...any)
	// Fatalw logs a message with key-value pairs at fatal level and then calls os.Exit(1).
	Fatalw(msg string, keysAndValues ...any)

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

// New creates a new Logger instance with the provided configuration.
func New(cfg Config) (Logger, error) {
	zapConfig, err := cfg.getZapConfig()
	if err != nil {
		return nil, errx.Wrap(err)
	}

	var zapLogger *zap.Logger

	// Use custom development encoder for console mode
	if cfg.Encoding == "console" {
		// Initialize custom encoder config
		encoderConfig := zapConfig.EncoderConfig

		// Create the custom development encoder
		enc := newDevEncoder(encoderConfig)

		// Build a core with our custom encoder
		core := zapcore.NewCore(
			enc,
			zapcore.AddSync(os.Stdout),
			zapConfig.Level,
		)

		// Build the logger with the custom core
		zapLogger = zap.New(core,
			zap.AddCaller(),
			zap.AddCallerSkip(1),
		)
	} else {
		// For regular JSON mode, use the standard build method
		zapLogger, err = zapConfig.Build()
		if err != nil {
			return nil, errx.Wrap(err)
		}
	}

	return &logger{
		SugaredLogger: zapLogger.Sugar(),
	}, nil
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
