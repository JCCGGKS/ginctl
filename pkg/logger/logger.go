// Package logger configures the process-wide zap logger from config. The rest
// of the app calls logger.Debug/Info/Warn/Error with zap fields; the level and
// output format are set once at startup via Init.
package logger

import (
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log = defaultLogger(zapcore.DebugLevel, false)

// Init installs the process-wide logger from level and format config.
// level accepts debug|info|warn|error; format accepts "json" or "console".
func Init(level, format string) {
	log = defaultLogger(parseLevel(level), strings.EqualFold(format, "json"))
}

// Sync flushes buffered log output. Call it during graceful shutdown.
func Sync() {
	_ = log.Sync()
}

// Writer returns an io.Writer that routes plain lines (e.g. gin's default
// writer) into the zap logger at info level.
func Writer() io.Writer {
	return &plainWriter{}
}

func Debug(msg string, fields ...zap.Field) { log.Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { log.Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { log.Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { log.Error(msg, fields...) }

func defaultLogger(level zapcore.Level, json bool) *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if json {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

type plainWriter struct{}

func (w *plainWriter) Write(p []byte) (int, error) {
	if msg := strings.TrimSpace(string(p)); msg != "" {
		log.Info(msg)
	}
	return len(p), nil
}
