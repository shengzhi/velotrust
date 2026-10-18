package cache

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

// 静态编译检查
var _ Cache = (*RedisCache)(nil)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr, password string, db int) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisCache{client: rdb}
}

func (r *RedisCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	if r == nil || r.client == nil {
		return false, errors.New("redis client is not initialized")
	}
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil // Cache Miss
	} else if err != nil {
		return false, err
	}

	// 自动反序列化到目标结构体指针中
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	if r == nil || r.client == nil {
		return errors.New("redis client is not initialized")
	}
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	if r == nil || r.client == nil {
		return errors.New("redis client is not initialized")
	}
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) Exist(ctx context.Context, key string) (bool, error) {
	if r == nil || r.client == nil {
		return false, errors.New("redis client is not initialized")
	}

	// Exists() 返回的是存在该 Key 的个数 (int64)
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

func (r *RedisCache) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return errors.New("redis client is not initialized")
	}
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}
