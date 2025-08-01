// Package logger provides a structured logging interface for applications.
package logger

import (
	"github.com/code19m/errx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	messageKey = "msg"
	levelKey   = "level"
	nameKey    = "logger"
	callerKey  = "file"
	timeKey    = "time"

	EncodingConsole = "console"
	EncodingJSON    = "json"
)

// Config defines configuration options for the logger.
type Config struct {
	// Level specifies the minimum log level to emit.
	// Valid values are: "debug", "info", "warn", "error"
	// Default is "debug".
	Level string `yaml:"level" validate:"oneof=debug info warn error" default:"debug"`

	// Encoding specifies the log format.
	// Valid values are: "json", "console"
	// Default is "json".
	//
	// When set to "console", the logger will use a development-friendly format:
	// - Colored output based on log levels
	// - Pretty-printed JSON for complex objects and nested structures
	//
	// When set to "json", the logger will produce compact JSON logs suitable
	// for production environments and log processing systems.
	Encoding string `yaml:"encoding" validate:"oneof=json console" default:"json"`
}

// getZapConfig converts the logger Config to a zap.Config.
func (c Config) getZapConfig(cfg Config) (*zap.Config, error) {
	zapLevel := zap.NewAtomicLevel()

	err := zapLevel.UnmarshalText([]byte(c.Level))
	if err != nil {
		return nil, errx.Wrap(err)
	}

	encodeLevel := zapcore.CapitalLevelEncoder
	if c.Encoding != EncodingConsole {
		encodeLevel = zapcore.CapitalColorLevelEncoder
	}

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     messageKey,
		LevelKey:       levelKey,
		NameKey:        nameKey,
		CallerKey:      callerKey,
		TimeKey:        timeKey,
		EncodeLevel:    encodeLevel,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	zapConfig := zap.Config{
		Level:            zapLevel,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		Encoding:         c.Encoding,
		EncoderConfig:    encoderConfig,
	}

	return &zapConfig, nil
}
