// Package logger builds one zap logger per application module. All modules
// share a single root directory and can rotate by local date and/or file size.
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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	moduleApp    = "app"
	moduleGORM   = "gorm"
	moduleAccess = "access"
)

// Logger wraps a module-scoped zap logger and adds request_id from ctx.
type Logger struct {
	zap *zap.Logger
}

type manager struct {
	app     *Logger
	gorm    *Logger
	access  *Logger
	writers []*dailyWriter
}

var (
	currentMu sync.RWMutex
	current   = newFallbackManager()
)

// Config controls file-backed module logging. It is populated directly from
// the application config's log section.
type Config struct {
	RootDir     string `mapstructure:"rootDir"`
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	DailyRotate bool   `mapstructure:"dailyRotate"`
	Timezone    string `mapstructure:"timezone"`
	MaxSizeMB   int    `mapstructure:"maxSizeMB"`
	MaxAgeDays  int    `mapstructure:"maxAgeDays"`
	Compress    bool   `mapstructure:"compress"`
}

// Init replaces the process loggers with file-backed module loggers. rootDir
// is the only path setting. With daily rotation enabled, files are written to
// <rootDir>/<YYYY-MM>/<YYYY-MM-DD>_<module>.log; otherwise they are written to
// <rootDir>/<module>.log. level accepts debug|info|warn|error; format accepts
// json|console and controls the file encoder.
func Init(cfg Config) error {
	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		return errors.New("log root dir is required")
	}
	if !cfg.DailyRotate && cfg.MaxSizeMB == 0 && (cfg.MaxAgeDays > 0 || cfg.Compress) {
		return errors.New("log max size must be positive when daily rotation is disabled and retention or compression is enabled")
	}
	timezone := strings.TrimSpace(cfg.Timezone)
	if timezone == "" {
		timezone = "Local"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load log timezone %q: %w", timezone, err)
	}

	modules := []string{moduleApp, moduleGORM, moduleAccess}
	loggers := make([]*Logger, 0, len(modules))
	writers := make([]*dailyWriter, 0, len(modules))

	closeWriters := func() {
		for _, writer := range writers {
			_ = writer.Close()
		}
	}

	jsonEncoding := strings.EqualFold(cfg.Format, "json")
	levelValue := parseLevel(cfg.Level)
	now := time.Now()
	writerOptions := dailyWriterOptions{
		location:     location,
		dailyRotate:  cfg.DailyRotate,
		maxSizeBytes: int64(cfg.MaxSizeMB) * 1024 * 1024,
		maxAgeDays:   cfg.MaxAgeDays,
		compress:     cfg.Compress,
	}
	for _, module := range modules {
		writer, err := newDailyWriter(rootDir, module, now, writerOptions)
		if err != nil {
			closeWriters()
			return fmt.Errorf("init %s logger: %w", module, err)
		}
		writers = append(writers, writer)
		loggers = append(loggers, newLogger(writer, levelValue, jsonEncoding, module))
	}

	next := &manager{
		app:     loggers[0],
		gorm:    loggers[1],
		access:  loggers[2],
		writers: writers,
	}

	currentMu.Lock()
	previous := current
	current = next
	currentMu.Unlock()

	previous.close()
	return nil
}

// App returns the application logger for internal lifecycle, error, and
// business-event logs.
func App() *Logger {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.app
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

// AccessWriter returns an io.Writer that routes Gin's plain output into the
// access logger at info level.
func AccessWriter() io.Writer { return plainWriter{level: zapcore.InfoLevel} }

// AccessErrorWriter returns an io.Writer that routes Gin's plain error output
// into the access logger at error level.
func AccessErrorWriter() io.Writer { return plainWriter{level: zapcore.ErrorLevel} }

// StdLogger returns a *log.Logger that writes to the app logger at error level,
// suitable for http.Server.ErrorLog.
func StdLogger() *stdlog.Logger {
	l, err := zap.NewStdLogAt(App().Zap(), zapcore.ErrorLevel)
	if err != nil {
		return stdlog.New(io.Discard, "", 0)
	}
	return l
}

type plainWriter struct {
	level zapcore.Level
}

func (w plainWriter) Write(p []byte) (int, error) {
	if msg := strings.TrimSpace(string(p)); msg != "" {
		if w.level >= zapcore.ErrorLevel {
			Access().Error(context.Background(), msg)
		} else {
			Access().Info(context.Background(), msg)
		}
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
		app:    newFallbackLogger(moduleApp),
		gorm:   newFallbackLogger(moduleGORM),
		access: newFallbackLogger(moduleAccess),
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
	for _, logger := range []*Logger{m.app, m.gorm, m.access} {
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
	id := RequestIDFromContext(ctx)
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
