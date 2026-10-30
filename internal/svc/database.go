package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gin-scaffold/infra/logger"
	"gin-scaffold/internal/config"

	"go.uber.org/zap"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	mysqlStartupTimeout = 5 * time.Second
	gormSlowThreshold   = 200 * time.Millisecond
)

// openDatabase creates the GORM connection, applies pool limits, and verifies
// that the database is reachable before the HTTP server starts.
func openDatabase(c config.MySQLConf) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(gormmysql.Open(c.DSN()), &gorm.Config{
		Logger: logger.NewGormLoggerWithOptions(logger.GormLoggerOptions{
			SlowThreshold:     gormSlowThreshold,
			LogSQLParameters:  false,
			LogRecordNotFound: false,
		}),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open mysql %s:%d/%s: %w", c.Host, c.Port, c.Database, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get mysql connection pool: %w", err)
	}

	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetimeSeconds) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(c.ConnMaxIdleTimeSeconds) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), mysqlStartupTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("ping mysql %s:%d/%s: %w", c.Host, c.Port, c.Database, err)
	}

	logger.App().Info(context.Background(), "mysql connected",
		zap.String("host", c.Host),
		zap.Int("port", c.Port),
		zap.String("database", c.Database),
		zap.Int("max_open_conns", c.MaxOpenConns),
		zap.Int("max_idle_conns", c.MaxIdleConns),
	)

	return db, sqlDB, nil
}
