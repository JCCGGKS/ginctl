package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gin-scaffold/common/model"
	apperrors "gin-scaffold/internal/errors"

	"github.com/redis/go-redis/v9"
)

const userCacheKeyPrefix = "user:"

// UserCache defines cache operations for the user domain.
type UserCache interface {
	Get(ctx context.Context, id uint64) (*model.User, error)
	Set(ctx context.Context, user *model.User, ttl time.Duration) error
	Delete(ctx context.Context, id uint64) error
}

type redisUserCache struct {
	client *redis.Client
}

func newRedisUserCache(client *redis.Client) *redisUserCache {
	return &redisUserCache{client: client}
}

func (c *redisUserCache) Get(ctx context.Context, id uint64) (*model.User, error) {
	if c == nil || c.client == nil {
		return nil, apperrors.ErrCacheUnavailable
	}

	payload, err := c.client.Get(ctx, userCacheKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, apperrors.ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("get user cache: %w", err)
	}

	var record userCacheRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("decode user cache: %w", err)
	}
	return record.toModel(), nil
}

func (c *redisUserCache) Set(ctx context.Context, user *model.User, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return apperrors.ErrCacheUnavailable
	}
	if user == nil {
		return errors.New("user is nil")
	}

	payload, err := json.Marshal(userCacheRecordFromModel(user))
	if err != nil {
		return fmt.Errorf("encode user cache: %w", err)
	}
	if err := c.client.Set(ctx, userCacheKey(user.ID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("set user cache: %w", err)
	}
	return nil
}

func (c *redisUserCache) Delete(ctx context.Context, id uint64) error {
	if c == nil || c.client == nil {
		return apperrors.ErrCacheUnavailable
	}
	if err := c.client.Del(ctx, userCacheKey(id)).Err(); err != nil {
		return fmt.Errorf("delete user cache: %w", err)
	}
	return nil
}

func userCacheKey(id uint64) string {
	return fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
}

type userCacheRecord struct {
	ID           uint64    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func userCacheRecordFromModel(user *model.User) userCacheRecord {
	return userCacheRecord{
		ID:           user.ID,
		Username:     user.Username,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func (r userCacheRecord) toModel() *model.User {
	return &model.User{
		ID:           r.ID,
		Username:     r.Username,
		PasswordHash: r.PasswordHash,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

var _ UserCache = (*redisUserCache)(nil)
