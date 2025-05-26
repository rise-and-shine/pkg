// Package logger provides a structured logging interface for applications.
package logger

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// devEncoder is a custom encoder for development mode that outputs colored,
// human-readable logs with indented JSON for complex objects.
type devEncoder struct {
	zapcore.Encoder
	consoleEncoder zapcore.Encoder
	jsonEncoder    zapcore.Encoder
	pool           buffer.Pool
}

// newDevEncoder creates a new development encoder with color support and JSON indentation.
func newDevEncoder(encoderConfig zapcore.EncoderConfig) zapcore.Encoder {
	consoleEnc := zapcore.NewConsoleEncoder(encoderConfig)
	return &devEncoder{
		Encoder:        consoleEnc, // Embed the console encoder to implement the Encoder interface
		consoleEncoder: consoleEnc,
		jsonEncoder:    zapcore.NewJSONEncoder(encoderConfig),
		pool:           buffer.NewPool(),
	}
}

// EncodeEntry formats a log entry with advanced formatting:
// - Uses colors for log levels
// - Indents any JSON objects in the log entry.
func (e *devEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Use the console encoder for the basic structure
	consoleBuf, err := e.consoleEncoder.EncodeEntry(entry, nil)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline for now
	consoleOutput := strings.TrimRight(consoleBuf.String(), "\n")

	// Colorize the log level
	colorized := e.colorizeLevel(consoleOutput, entry.Level)

	// Process fields with pretty-printing
	if len(fields) > 0 {
		// Use the JSON encoder for fields to get proper JSON formatting
		fieldBuf, encErr := e.jsonEncoder.EncodeEntry(entry, fields)
		if encErr != nil {
			return nil, encErr
		}

		var fieldsMap map[string]any
		err = json.Unmarshal(fieldBuf.Bytes(), &fieldsMap)
		if err != nil {
			// If we can't parse as JSON, just use the original fields
			colorized += " " + fieldBuf.String()
		} else {
			colorized = e.processFieldsMap(colorized, fieldsMap, fieldBuf)
		}
	}

	// Get a buffer from the pool
	buf := e.pool.Get()
	buf.AppendString(colorized)
	buf.AppendString("\n")

	return buf, nil
}

// ProcessFieldsMap handles the formatting of field maps for log entries.
func (e *devEncoder) processFieldsMap(colorized string, fieldsMap map[string]any, fieldBuf *buffer.Buffer) string {
	// Remove common fields that are already displayed in the log prefix
	delete(fieldsMap, "msg")
	delete(fieldsMap, "level")
	delete(fieldsMap, "ts")
	delete(fieldsMap, "caller")
	delete(fieldsMap, "logger")

	// If there are fields remaining, format them nicely
	if len(fieldsMap) > 0 {
		prettyJSON, marshalErr := json.MarshalIndent(fieldsMap, "", "  ")
		if marshalErr != nil {
			colorized += " " + fieldBuf.String()
		} else {
			colorized += "\n" + string(prettyJSON)
		}
	}

	return colorized
}

// colorizeLevel adds color to the log level based on its severity.
func (e *devEncoder) colorizeLevel(line string, level zapcore.Level) string {
	var colorFunc func(a ...any) string

	// Define colors for different log levels
	switch level {
	case zapcore.DebugLevel:
		colorFunc = color.New(color.FgCyan).SprintFunc()
	case zapcore.InfoLevel:
		colorFunc = color.New(color.FgGreen).SprintFunc()
	case zapcore.WarnLevel:
		colorFunc = color.New(color.FgYellow).SprintFunc()
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		colorFunc = color.New(color.FgRed, color.Bold).SprintFunc()
	case zapcore.InvalidLevel:
		colorFunc = color.New(color.FgMagenta).SprintFunc()
	default:
		// Default no color function
		colorFunc = func(a ...any) string {
			if len(a) == 1 {
				if s, ok := a[0].(string); ok {
					return s
				}
			}
			return fmt.Sprint(a...)
		}
	}

	// Map log levels to their string representation
	capLevelStr := level.CapitalString()
	lowLevelStr := level.String()

	// Replace the log level in the line with its colored version
	if strings.Contains(line, capLevelStr) {
		return strings.Replace(line, capLevelStr, colorFunc(capLevelStr), 1)
	} else if strings.Contains(line, lowLevelStr) {
		return strings.Replace(line, lowLevelStr, colorFunc(lowLevelStr), 1)
	}

	return line
}
