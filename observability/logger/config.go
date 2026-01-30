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
	timeKey    = "time"

	encPretty  = "pretty"
	levelDebug = "debug"
)

// Config defines configuration options for the logger.
type Config struct {
	// Level specifies the minimum log level to emit.
	// Valid values are: "debug", "info", "warn", "error"
	// Default is "debug".
	Level string `yaml:"level" validate:"oneof=debug info warn error" default:"debug"`

	// Encoding specifies the log format.
	// Valid values are: "json", "pretty"
	// Default is "pretty".
	//
	// When set to "pretty", the logger will use a development-friendly format:
	// - Colored output based on log levels
	// - Pretty-printed JSON for complex objects and nested structures
	//
	// When set to "json", the logger will produce compact JSON logs suitable
	// for production environments and log processing systems.
	Encoding string `yaml:"encoding" validate:"oneof=json pretty" default:"pretty"`

	// Disable creates no-op logger . Useful in testing environments. Default is false.
	Disable bool `yaml:"disable" default:"false"`
}

// getZapConfig converts the logger Config to a zap.Config.
func (c Config) getZapConfig() (*zap.Config, error) {
	zapLevel := zap.NewAtomicLevel()

	err := zapLevel.UnmarshalText([]byte(c.Level))
	if err != nil {
		return nil, errx.Wrap(err)
	}

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     messageKey,
		LevelKey:       levelKey,
		NameKey:        nameKey,
		TimeKey:        timeKey,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
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
