// Package logger provides a structured logging interface for applications.
package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	orderedmap "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiFaint = "\033[2m"

	callerColor   = "\033[38;2;148;163;184m"
	metaKeyColor  = "\033[38;2;94;234;212m"
	metaValColor  = "\033[38;2;226;232;240m"
	textColor     = "\033[38;2;226;232;240m"
	warnKeyColor  = "\033[38;2;251;191;36m"
	warnValColor  = "\033[38;2;253;230;138m"
	errorKeyColor = "\033[38;2;248;113;113m"
	errorValColor = "\033[38;2;254;202;202m"
)

//nolint:gochecknoglobals // palette is a static lookup shared across encoder instances.
var levelPalette = map[zapcore.Level]string{
	zapcore.DebugLevel:   "\033[38;2;129;140;248m",
	zapcore.InfoLevel:    "\033[38;2;16;185;129m",
	zapcore.WarnLevel:    "\033[38;2;245;158;11m",
	zapcore.ErrorLevel:   "\033[38;2;248;113;113m",
	zapcore.DPanicLevel:  "\033[38;2;244;63;94m",
	zapcore.PanicLevel:   "\033[38;2;244;63;94m",
	zapcore.FatalLevel:   "\033[38;2;217;70;239m",
	zapcore.InvalidLevel: textColor,
}

//nolint:gochecknoglobals // emoji mapping is static and reused for all encoders.
var levelEmoji = map[zapcore.Level]string{
	zapcore.DebugLevel:   "🧪",
	zapcore.InfoLevel:    "ℹ️ ", // added space for alignment
	zapcore.WarnLevel:    "⚠️ ", // added space for alignment
	zapcore.ErrorLevel:   "🚨",
	zapcore.DPanicLevel:  "🚨",
	zapcore.PanicLevel:   "🚨",
	zapcore.FatalLevel:   "💥",
	zapcore.InvalidLevel: "",
}

// prettyLogger wraps zap's JSON encoder to produce colorized, indented output suited for terminals.
type prettyLogger struct {
	zapcore.Encoder
}

// Clone ensures derived loggers keep the pretty encoder wrapper.
func (e *prettyLogger) Clone() zapcore.Encoder {
	return &prettyLogger{Encoder: e.Encoder.Clone()}
}

// newPrettyLogger creates a pretty logger without caller tracking.
func newPrettyLogger(cfg *zap.Config) *zap.Logger {
	enc := &prettyLogger{Encoder: zapcore.NewJSONEncoder(cfg.EncoderConfig)}
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), cfg.Level)
	opts := buildPrettyOptions(cfg)
	return zap.New(core, opts...)
}

func buildPrettyOptions(cfg *zap.Config) []zap.Option {
	opts := []zap.Option{zap.ErrorOutput(zapcore.AddSync(os.Stderr))}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}
	// Caller tracking removed - use Named() loggers for component identification
	if len(cfg.InitialFields) > 0 {
		keys := make([]string, 0, len(cfg.InitialFields))
		for k := range cfg.InitialFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fields := make([]zap.Field, 0, len(keys))
		for _, k := range keys {
			fields = append(fields, zap.Any(k, cfg.InitialFields[k]))
		}
		opts = append(opts, zap.Fields(fields...))
	}
	return opts
}

// unmarshalOrdered unmarshals JSON into ordered maps recursively.
func unmarshalOrdered(data []byte) (*orderedmap.OrderedMap[string, any], error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, err
	}

	return decodeObject(decoder)
}

func decodeObject(decoder *json.Decoder) (*orderedmap.OrderedMap[string, any], error) {
	om := orderedmap.New[string, any]()

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyToken.(string)

		value, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}

		om.Set(key, value)
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return om, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if delim, ok := token.(json.Delim); ok {
		switch delim {
		case '{':
			return decodeObject(decoder)
		case '[':
			return decodeArray(decoder)
		}
	}

	return token, nil
}

func decodeArray(decoder *json.Decoder) ([]any, error) {
	var arr []any
	for decoder.More() {
		value, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}
		arr = append(arr, value)
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return arr, nil
}

// EncodeEntry formats a log entry with pretty printing and colorization.
func (e *prettyLogger) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	jsonBuf, err := e.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return nil, err
	}

	raw := append([]byte(nil), jsonBuf.Bytes()...)
	jsonBuf.Reset()

	trimmed := bytes.TrimSpace(raw)
	payload, unmarshalErr := unmarshalOrdered(trimmed)
	if unmarshalErr != nil {
		if writeErr := writeBytes(jsonBuf, raw); writeErr != nil {
			return nil, writeErr
		}
		if len(raw) == 0 || raw[len(raw)-1] != '\n' {
			if newlineErr := writeByte(jsonBuf, '\n'); newlineErr != nil {
				return nil, newlineErr
			}
		}
		return jsonBuf, nil
	}

	if headerErr := writeString(jsonBuf, buildHeader(entry, payload)); headerErr != nil {
		return nil, headerErr
	}
	meta := filterReserved(payload)
	if metadataErr := writeMetadata(jsonBuf, meta, entry.Level); metadataErr != nil {
		return nil, metadataErr
	}

	return jsonBuf, nil
}

func buildHeader(entry zapcore.Entry, payload *orderedmap.OrderedMap[string, any]) string {
	timestamp := headerTimestamp(entry)
	level := headerLevel(entry, payload)
	message := headerMessage(entry, payload)
	emoji := levelEmoji[entry.Level]

	var b strings.Builder
	b.WriteString(styleTime("[" + timestamp + "]"))
	b.WriteByte(' ')
	if emoji != "" {
		b.WriteString(emoji)
		b.WriteByte(' ')
	}
	b.WriteString(styleLevel(level, entry.Level))
	if message != "" {
		b.WriteByte(' ')
		b.WriteString(styleMessage(entry.Level, message))
	}
	b.WriteByte('\n')
	return b.String()
}

func headerTimestamp(entry zapcore.Entry) string {
	timestamp := entry.Time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	value := timestamp.Format(time.DateTime)
	return value
}

func headerLevel(entry zapcore.Entry, payload *orderedmap.OrderedMap[string, any]) string {
	value := strings.ToUpper(entry.Level.String())
	if lvlVal, hasLevel := payload.Get(levelKey); hasLevel {
		if lvlText, okString := lvlVal.(string); okString && lvlText != "" {
			value = strings.ToUpper(lvlText)
		}
	}
	return value
}

func headerMessage(entry zapcore.Entry, payload *orderedmap.OrderedMap[string, any]) string {
	value := entry.Message
	if msgVal, hasMessage := payload.Get(messageKey); hasMessage {
		if msgText, okString := msgVal.(string); okString {
			value = msgText
		}
	}
	return value
}

func filterReserved(payload *orderedmap.OrderedMap[string, any]) *orderedmap.OrderedMap[string, any] {
	meta := orderedmap.New[string, any]()
	for pair := payload.Oldest(); pair != nil; pair = pair.Next() {
		switch pair.Key {
		case timeKey, levelKey, messageKey:
			continue
		default:
			meta.Set(pair.Key, pair.Value)
		}
	}
	if _, ok := meta.Get(nameKey); !ok {
		if nameVal, has := payload.Get(nameKey); has {
			meta.Set(nameKey, nameVal)
		}
	}
	return meta
}

func marshalOrderedIndent(value any, prefix, indent string) ([]byte, error) {
	if om, isOrderedMap := value.(*orderedmap.OrderedMap[string, any]); isOrderedMap {
		return marshalOrderedMap(om, prefix, indent)
	}

	if arr, isSlice := value.([]any); isSlice {
		return marshalArray(arr, prefix, indent)
	}

	return json.Marshal(value)
}

func marshalOrderedMap(om *orderedmap.OrderedMap[string, any], prefix, indent string) ([]byte, error) {
	if om.Len() == 0 {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("{\n")

	i := 0
	for pair := om.Oldest(); pair != nil; pair = pair.Next() {
		if i > 0 {
			buf.WriteString(",\n")
		}

		buf.WriteString(prefix + indent)

		keyBytes, err := json.Marshal(pair.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteString(": ")

		valueBytes, err := marshalOrderedIndent(pair.Value, prefix+indent, indent)
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)

		i++
	}

	buf.WriteString("\n" + prefix + "}")
	return buf.Bytes(), nil
}

func marshalArray(arr []any, prefix, indent string) ([]byte, error) {
	if len(arr) == 0 {
		return []byte("[]"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("[\n")

	for i, elem := range arr {
		if i > 0 {
			buf.WriteString(",\n")
		}

		buf.WriteString(prefix + indent)

		elemBytes, err := marshalOrderedIndent(elem, prefix+indent, indent)
		if err != nil {
			return nil, err
		}
		buf.Write(elemBytes)
	}

	buf.WriteString("\n" + prefix + "]")
	return buf.Bytes(), nil
}

func writeMetadata(buf *buffer.Buffer, meta *orderedmap.OrderedMap[string, any], level zapcore.Level) error {
	if meta.Len() == 0 {
		return nil
	}

	keyColor, valColor := metaColours(level)
	pretty, err := marshalOrderedIndent(meta, "", "  ")
	if err != nil {
		if fallbackWriteErr := writeString(buf, ansiFaint+valColor+metaFallback(meta)+ansiReset); fallbackWriteErr != nil {
			return fallbackWriteErr
		}
		return writeByte(buf, '\n')
	}

	lines := bytes.Split(pretty, []byte("\n"))
	written := false
	for i, line := range lines {
		formatted := styleMetaLine(line, keyColor, valColor)
		if formatted == "" {
			continue
		}
		if lineWriteErr := writeString(buf, formatted); lineWriteErr != nil {
			return lineWriteErr
		}
		written = true
		if i < len(lines)-1 {
			if newlineErr := writeByte(buf, '\n'); newlineErr != nil {
				return newlineErr
			}
		}
	}
	if !written {
		return nil
	}
	return writeByte(buf, '\n')
}

func styleLevel(level string, lvl zapcore.Level) string {
	color := levelPalette[lvl]
	if color == "" {
		color = levelPalette[zapcore.InfoLevel]
	}
	return ansiBold + color + level + ansiReset
}

func styleTime(v string) string {
	return ansiFaint + callerColor + v + ansiReset
}

func styleMessage(level zapcore.Level, v string) string {
	if v == "" {
		return ""
	}
	colour := messageColour(level)
	return colour + v + ansiReset
}

func messageColour(level zapcore.Level) string {
	switch level {
	case zapcore.WarnLevel:
		return warnValColor
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return errorValColor
	case zapcore.DebugLevel, zapcore.InfoLevel, zapcore.InvalidLevel:
		return textColor
	default:
		return textColor
	}
}

func metaFallback(meta *orderedmap.OrderedMap[string, any]) string {
	raw, err := json.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func metaColours(level zapcore.Level) (string, string) {
	switch level {
	case zapcore.WarnLevel:
		return warnKeyColor, warnValColor
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return errorKeyColor, errorValColor
	case zapcore.DebugLevel, zapcore.InfoLevel, zapcore.InvalidLevel:
		return metaKeyColor, metaValColor
	default:
		return metaKeyColor, metaValColor
	}
}

func styleMetaLine(line []byte, keyColor, valColor string) string {
	if len(line) == 0 {
		return ""
	}
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return ""
	}
	indentLen := len(line) - len(bytes.TrimLeft(line, " "))
	indent := string(line[:indentLen])
	colonIdx := bytes.IndexByte(trimmed, ':')
	if colonIdx == -1 {
		return indent + ansiFaint + valColor + string(trimmed) + ansiReset
	}
	key := string(trimmed[:colonIdx])
	rest := string(trimmed[colonIdx+1:])
	return indent + keyColor + key + ansiReset + ":" + ansiFaint + valColor + rest + ansiReset
}

func writeBytes(buf *buffer.Buffer, data []byte) error {
	_, err := buf.Write(data)
	return err
}

func writeString(buf *buffer.Buffer, value string) error {
	_, err := buf.WriteString(value)
	return err
}

func writeByte(buf *buffer.Buffer, b byte) error { //nolint:unparam // byte value varies as we add more formatting
	return buf.WriteByte(b)
}
