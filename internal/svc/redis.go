package svc

import (
	"context"
	"fmt"
	"time"

	"gin-scaffold/infra/logger"
	"gin-scaffold/internal/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const redisStartupTimeout = 5 * time.Second

// openRedis creates the shared Redis client and verifies connectivity before
// the HTTP server accepts traffic.
func openRedis(c config.RedisConf) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:                  c.Addr(),
		ClientName:            "gin-scaffold",
		Password:              c.Password,
		DB:                    c.DB,
		PoolSize:              c.PoolSize,
		MinIdleConns:          c.MinIdleConns,
		DialTimeout:           time.Duration(c.DialTimeoutSeconds) * time.Second,
		ReadTimeout:           time.Duration(c.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:          time.Duration(c.WriteTimeoutSeconds) * time.Second,
		ConnMaxIdleTime:       time.Duration(c.IdleTimeoutSeconds) * time.Second,
		ContextTimeoutEnabled: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), redisStartupTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis %s db %d: %w", c.Addr(), c.DB, err)
	}

	logger.App().Info(context.Background(), "redis connected",
		zap.String("addr", c.Addr()),
		zap.Int("db", c.DB),
		zap.Int("pool_size", c.PoolSize),
		zap.Int("min_idle_conns", c.MinIdleConns),
	)
	return client, nil
}
