package logger

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// DefaultGORMSlowThreshold is used when a positive threshold is not supplied.
const DefaultGORMSlowThreshold = 200 * time.Millisecond

// GormLoggerOptions controls the GORM adapter. The zero value uses secure and
// quiet defaults: SQL parameters are omitted and ErrRecordNotFound is ignored.
type GormLoggerOptions struct {
	SlowThreshold     time.Duration
	LogSQLParameters  bool
	LogRecordNotFound bool
}

// NewGormLogger adapts the gorm module logger to gorm's logger.Interface.
// Normal SQL is emitted at info, slow queries at warn, and failures at error.
// SQL parameters are omitted by default so sensitive values are not written to
// gorm.log, and ErrRecordNotFound is ignored.
// Use db.WithContext(ctx) when executing a query so request_id is attached.
func NewGormLogger(slowThreshold time.Duration) gormlogger.Interface {
	return NewGormLoggerWithOptions(GormLoggerOptions{SlowThreshold: slowThreshold})
}

// NewGormLoggerWithOptions is the configurable variant of NewGormLogger.
// Set LogSQLParameters only when SQL parameter values are safe for the target
// log retention and access policy. Set LogRecordNotFound to keep record-not-found
// diagnostics; by default they are suppressed as normal query outcomes.
func NewGormLoggerWithOptions(opts GormLoggerOptions) gormlogger.Interface {
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = DefaultGORMSlowThreshold
	}
	return &gormLogger{
		logger:            GORM(),
		slowThreshold:     opts.SlowThreshold,
		logSQLParameters:  opts.LogSQLParameters,
		logRecordNotFound: opts.LogRecordNotFound,
		level:             gormlogger.Info,
	}
}

type gormLogger struct {
	logger            *Logger
	slowThreshold     time.Duration
	logSQLParameters  bool
	logRecordNotFound bool
	level             gormlogger.LogLevel
}

func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	next := *l
	next.level = level
	return &next
}

func (l *gormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Info {
		l.logger.Info(ctx, msg, zap.Any("data", data))
	}
}

func (l *gormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Warn {
		l.logger.Warn(ctx, msg, zap.Any("data", data))
	}
}

func (l *gormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Error {
		l.logger.Error(ctx, msg, zap.Any("data", data))
	}
}

// ParamsFilter implements GORM's optional ParamsFilter interface. Returning no
// vars makes GORM keep placeholders in the SQL passed to the logger.
func (l *gormLogger) ParamsFilter(_ context.Context, sql string, params ...any) (string, []any) {
	if l.logSQLParameters {
		return sql, params
	}
	return sql, nil
}

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil:
		if errors.Is(err, gormlogger.ErrRecordNotFound) && !l.logRecordNotFound {
			return
		}
		if l.level < gormlogger.Error {
			return
		}
		sql, rows := fc()
		l.logger.Error(ctx, "gorm query failed", traceFields(elapsed, rows, sql, zap.Error(err))...)
	case elapsed > l.slowThreshold:
		if l.level < gormlogger.Warn {
			return
		}
		sql, rows := fc()
		l.logger.Warn(ctx, "gorm slow query", traceFields(elapsed, rows, sql)...)
	case l.level >= gormlogger.Info:
		sql, rows := fc()
		l.logger.Info(ctx, "gorm query", traceFields(elapsed, rows, sql)...)
	}
}

func traceFields(elapsed time.Duration, rows int64, sql string, extra ...zap.Field) []zap.Field {
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows_affected", rows),
		zap.String("sql", sql),
	}
	return append(fields, extra...)
}

var _ gormlogger.Interface = (*gormLogger)(nil)
var _ interface {
	ParamsFilter(context.Context, string, ...any) (string, []any)
} = (*gormLogger)(nil)
