package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

const workCodesCacheKey = "banking:work-codes:v1"

type WorkCodeCache interface {
	Get(ctx context.Context) ([]model.WorkCode, bool, error)
	Set(ctx context.Context, workCodes []model.WorkCode) error
}

type redisWorkCodeCache struct {
	client redis.Cmdable
	ttl    time.Duration
}

func NewWorkCodeRedisCache(client *redis.Client, ttl time.Duration) WorkCodeCache {
	if client == nil || ttl <= 0 {
		return nil
	}

	return &redisWorkCodeCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *redisWorkCodeCache) Get(ctx context.Context) ([]model.WorkCode, bool, error) {
	bytes, err := c.client.Get(ctx, workCodesCacheKey).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var workCodes []model.WorkCode
	if err := json.Unmarshal(bytes, &workCodes); err != nil {
		return nil, false, err
	}

	return workCodes, true, nil
}

func (c *redisWorkCodeCache) Set(ctx context.Context, workCodes []model.WorkCode) error {
	bytes, err := json.Marshal(workCodes)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, workCodesCacheKey, bytes, c.ttl).Err()
}
