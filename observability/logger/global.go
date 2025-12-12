package logger

import (
	"context"
	"sync"
	"sync/atomic"
)

//nolint:gochecknoglobals // Global variables are required for the global logger singleton pattern
var (
	global   atomic.Value // stores Logger
	setOnce  sync.Once    // ensures SetGlobal is called once
	initOnce sync.Once    // ensures lazy initialization happens once
)

// SetGlobal sets the global logger instance.
// This should be called during application startup to configure the global logger.
// This should be called before any logging functions are used.
func SetGlobal(cfg Config) {
	called := false
	setOnce.Do(func() {
		// Prevent lazy initialization from happening after this
		initOnce.Do(func() {})

		logger, err := newLogger(cfg)
		if err != nil {
			panic("[logger]: failed to initialize global logger: " + err.Error())
		}
		global.Store(logger)
		called = true
	})
	if !called {
		panic("[logger]: SetGlobal can only be called once")
	}
}

// Debug logs a message at debug level using the global logger.
func Debug(msg any) {
	getGlobal().Debug(msg)
}

// Info logs a message at info level using the global logger.
func Info(msg any) {
	getGlobal().Info(msg)
}

// Warn logs a message at warn level using the global logger.
func Warn(msg any) {
	getGlobal().Warn(msg)
}

// Error logs a message at error level using the global logger.
func Error(msg any) {
	getGlobal().Error(msg)
}

// Fatal logs a message at fatal level using the global logger and then calls os.Exit(1).
func Fatal(msg any) {
	getGlobal().Fatal(msg)
}

// Debugf logs a formatted message at debug level using the global logger.
func Debugf(format string, args ...any) {
	getGlobal().Debugf(format, args...)
}

// Infof logs a formatted message at info level using the global logger.
func Infof(format string, args ...any) {
	getGlobal().Infof(format, args...)
}

// Warnf logs a formatted message at warn level using the global logger.
func Warnf(format string, args ...any) {
	getGlobal().Warnf(format, args...)
}

// Errorf logs a formatted message at error level using the global logger.
func Errorf(format string, args ...any) {
	getGlobal().Errorf(format, args...)
}

// Fatalf logs a formatted message at fatal level using the global logger and then calls os.Exit(1).
func Fatalf(format string, args ...any) {
	getGlobal().Fatalf(format, args...)
}

// Warnx logs an errx.ErrorX instance at warn level using the global logger.
func Warnx(err error) {
	getGlobal().Warnx(err)
}

// Errorx logs an errx.ErrorX instance at error level using the global logger.
func Errorx(err error) {
	getGlobal().Errorx(err)
}

// Fatalx logs an errx.ErrorX instance at fatal level using the global logger and then calls os.Exit(1).
func Fatalx(err error) {
	getGlobal().Fatalx(err)
}

// With creates a new logger with the given key-value pairs using the global logger.
// The returned logger inherits the properties of the global logger
// and includes the provided key-value pairs in all subsequent log entries.
func With(keysAndValues ...any) Logger {
	return getGlobal().With(keysAndValues...)
}

// WithContext creates a logger with context information using the global logger,
// enriching the log entries with metadata from the context.
func WithContext(ctx context.Context) Logger {
	return getGlobal().WithContext(ctx)
}

// Named adds a sub-scope to the logger's name using the global logger.
func Named(name string) Logger {
	return getGlobal().Named(name)
}

// Sync flushes any buffered log entries from the global logger.
// Intended for use on application shutdown to ensure all logs are written.
func Sync() error {
	return getGlobal().Sync()
}

// initDefault initializes the default logger lazily.
func initDefault() {
	initOnce.Do(func() {
		defaultLogger, err := newLogger(Config{
			Level:    levelDebug,
			Encoding: encPretty,
		})
		if err != nil {
			panic("[logger]: failed to initialize default logger: " + err.Error())
		}
		global.Store(defaultLogger)
	})
}

// getGlobal returns the current global logger instance.
// If no logger has been set, it initializes a default logger lazily.
func getGlobal() Logger {
	if l := global.Load(); l != nil {
		logger, ok := l.(Logger)
		if !ok {
			panic("[logger]: global contains invalid type")
		}
		return logger
	}
	initDefault()
	logger, ok := global.Load().(Logger)
	if !ok {
		panic("[logger]: global contains invalid type after initialization")
	}
	return logger
}
