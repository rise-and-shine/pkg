package outbox

import (
	"github.com/ThreeDotsLabs/watermill"
	"github.com/rise-and-shine/pkg/logger"
	"go.uber.org/zap"
)

var _ watermill.LoggerAdapter = (*loggerAdapter)(nil)

// loggerAdapter is to adapt zap sugared logger to watermill Logger.
type loggerAdapter struct {
	base logger.Logger
}

func newLoggerAdapter(logger logger.Logger) *loggerAdapter {
	return &loggerAdapter{
		base: logger,
	}
}

func (l *loggerAdapter) Error(msg string, err error, fields watermill.LogFields) {
	logger := l.withZapFields(fields)
	logger.Error(msg, err)
}

func (l *loggerAdapter) Info(msg string, fields watermill.LogFields) {
	logger := l.withZapFields(fields)
	logger.Info(msg)
}

func (l *loggerAdapter) Debug(msg string, fields watermill.LogFields) {
	logger := l.withZapFields(fields)
	logger.Debug(msg)
}

func (l *loggerAdapter) Trace(msg string, fields watermill.LogFields) {
	logger := l.withZapFields(fields)
	logger.Debug(msg)
}

func (l *loggerAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	logger := l.withZapFields(fields)
	return &loggerAdapter{
		base: logger,
	}
}

func (l *loggerAdapter) withZapFields(fields watermill.LogFields) logger.Logger {
	logger := l.base
	for k, v := range fields {
		logger = logger.With(zap.Any(k, v))
	}
	return logger
}
