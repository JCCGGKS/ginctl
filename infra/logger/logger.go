// Package logger builds one zap logger per application module. All modules
// share a single root directory and roll to a new file at local midnight.
package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"
	"sync"
	"time"

	"gin-scaffold/infra/requestid"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	moduleApp      = "app"
	moduleBusiness = "business"
	moduleGORM     = "gorm"
	moduleAccess   = "access"
)

// Logger wraps a module-scoped zap logger and adds request_id from ctx.
type Logger struct {
	zap *zap.Logger
}

type manager struct {
	app      *Logger
	business *Logger
	gorm     *Logger
	access   *Logger
	writers  []*dailyWriter
}

var (
	currentMu sync.RWMutex
	current   = newFallbackManager()
)

// Init replaces the process loggers with file-backed module loggers. rootDir
// is the only path setting; files are written to
// <rootDir>/<YYYY-MM-DD>/<module>.log. level accepts debug|info|warn|error;
// format accepts json|console and controls the file encoder.
func Init(rootDir, level, format string) error {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return errors.New("log root dir is required")
	}

	modules := []string{moduleApp, moduleBusiness, moduleGORM, moduleAccess}
	loggers := make([]*Logger, 0, len(modules))
	writers := make([]*dailyWriter, 0, len(modules))

	closeWriters := func() {
		for _, writer := range writers {
			_ = writer.Close()
		}
	}

	jsonEncoding := strings.EqualFold(format, "json")
	levelValue := parseLevel(level)
	now := time.Now()
	for _, module := range modules {
		writer, err := newDailyWriter(rootDir, module, now)
		if err != nil {
			closeWriters()
			return fmt.Errorf("init %s logger: %w", module, err)
		}
		writers = append(writers, writer)
		loggers = append(loggers, newLogger(writer, levelValue, jsonEncoding, module))
	}

	next := &manager{
		app:      loggers[0],
		business: loggers[1],
		gorm:     loggers[2],
		access:   loggers[3],
		writers:  writers,
	}

	currentMu.Lock()
	previous := current
	current = next
	currentMu.Unlock()

	previous.close()
	return nil
}

// App returns the startup/lifecycle/error logger.
func App() *Logger {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.app
}

// Business returns the business-event logger.
func Business() *Logger {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.business
}

// GORM returns the database logger module. Pass a gorm adapter built with
// NewGormLogger to gorm.Config.Logger.
func GORM() *Logger {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.gorm
}

// Access returns the HTTP access logger.
func Access() *Logger {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.access
}

// Sync flushes the current loggers and their files.
func Sync() {
	currentMu.RLock()
	m := current
	currentMu.RUnlock()
	m.sync()
}

// Close flushes and closes all file-backed writers. It is safe to call more
// than once; later writes reopen the current day's file if necessary.
func Close() {
	currentMu.RLock()
	m := current
	currentMu.RUnlock()
	m.close()
}

// Debug logs at debug level with request_id automatically added from ctx.
func (l *Logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if l == nil || l.zap == nil {
		return
	}
	l.zap.Debug(msg, withRequestID(ctx, fields)...)
}

// Info logs at info level with request_id automatically added from ctx.
func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	if l == nil || l.zap == nil {
		return
	}
	l.zap.Info(msg, withRequestID(ctx, fields)...)
}

// Warn logs at warn level with request_id automatically added from ctx.
func (l *Logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if l == nil || l.zap == nil {
		return
	}
	l.zap.Warn(msg, withRequestID(ctx, fields)...)
}

// Error logs at error level with request_id automatically added from ctx.
func (l *Logger) Error(ctx context.Context, msg string, fields ...zap.Field) {
	if l == nil || l.zap == nil {
		return
	}
	l.zap.Error(msg, withRequestID(ctx, fields)...)
}

// With returns a child logger with additional fixed fields.
func (l *Logger) With(fields ...zap.Field) *Logger {
	if l == nil || l.zap == nil {
		return l
	}
	return &Logger{zap: l.zap.With(fields...)}
}

// Zap exposes the underlying logger for integrations that require it.
func (l *Logger) Zap() *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l.zap
}

// Writer returns an io.Writer that routes plain lines (for example gin's
// default writer) into the app logger at info level.
func Writer() io.Writer { return plainWriter{} }

// StdLogger returns a *log.Logger that writes to the app logger at error level,
// suitable for http.Server.ErrorLog.
func StdLogger() *stdlog.Logger {
	l, err := zap.NewStdLogAt(App().Zap(), zapcore.ErrorLevel)
	if err != nil {
		return stdlog.New(io.Discard, "", 0)
	}
	return l
}

type plainWriter struct{}

func (plainWriter) Write(p []byte) (int, error) {
	if msg := strings.TrimSpace(string(p)); msg != "" {
		App().Info(context.Background(), msg)
	}
	return len(p), nil
}

func newLogger(writer io.Writer, level zapcore.Level, json bool, module string) *Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	var encoder zapcore.Encoder
	if json {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(writer), level)
	zapLogger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	).With(zap.String("module", module))

	return &Logger{zap: zapLogger}
}

func newFallbackManager() *manager {
	return &manager{
		app:      newFallbackLogger(moduleApp),
		business: newFallbackLogger(moduleBusiness),
		gorm:     newFallbackLogger(moduleGORM),
		access:   newFallbackLogger(moduleAccess),
	}
}

func newFallbackLogger(module string) *Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stderr),
		zapcore.DebugLevel,
	)
	zapLogger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	).With(zap.String("module", module))
	return &Logger{zap: zapLogger}
}

func (m *manager) sync() {
	if m == nil {
		return
	}
	for _, logger := range []*Logger{m.app, m.business, m.gorm, m.access} {
		if logger != nil && logger.zap != nil {
			_ = logger.zap.Sync()
		}
	}
}

func (m *manager) close() {
	if m == nil {
		return
	}
	m.sync()
	for _, writer := range m.writers {
		_ = writer.Close()
	}
	m.writers = nil
}

func withRequestID(ctx context.Context, fields []zap.Field) []zap.Field {
	id := requestid.FromContext(ctx)
	if id == "" {
		return fields
	}
	for _, field := range fields {
		if field.Key == "request_id" {
			return fields
		}
	}
	return append([]zap.Field{zap.String("request_id", id)}, fields...)
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
