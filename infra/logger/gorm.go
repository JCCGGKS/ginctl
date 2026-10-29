package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// DefaultGORMSlowThreshold is used when a positive threshold is not supplied.
const DefaultGORMSlowThreshold = 200 * time.Millisecond

// NewGormLogger adapts the gorm module logger to gorm's logger.Interface.
// Normal SQL is emitted at info, slow queries at warn, and failures at error.
// Use db.WithContext(ctx) when executing a query so request_id is attached.
func NewGormLogger(slowThreshold time.Duration) gormlogger.Interface {
	if slowThreshold <= 0 {
		slowThreshold = DefaultGORMSlowThreshold
	}
	return &gormLogger{
		logger:        GORM(),
		slowThreshold: slowThreshold,
		level:         gormlogger.Info,
	}
}

type gormLogger struct {
	logger        *Logger
	slowThreshold time.Duration
	level         gormlogger.LogLevel
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

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == gormlogger.Silent {
		return
	}

	sql, rows := fc()
	elapsed := time.Since(begin)
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows_affected", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil:
		if l.level >= gormlogger.Error {
			l.logger.Error(ctx, "gorm query failed", append(fields, zap.Error(err))...)
		}
	case elapsed > l.slowThreshold:
		if l.level >= gormlogger.Warn {
			l.logger.Warn(ctx, "gorm slow query", fields...)
		}
	case l.level >= gormlogger.Info:
		l.logger.Info(ctx, "gorm query", fields...)
	}
}

var _ gormlogger.Interface = (*gormLogger)(nil)
