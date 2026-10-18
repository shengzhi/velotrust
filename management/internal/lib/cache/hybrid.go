package cache

import (
	"context"
	"errors"
	"time"
)

type HybridCache struct {
	local *LocalCache
	redis *RedisCache
}

func NewHybridCache(local *LocalCache, redis *RedisCache) *HybridCache {
	return &HybridCache{local: local, redis: redis}
}

func (h *HybridCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	if h == nil || h.local == nil || h.redis == nil {
		return false, errors.New("hybrid cache is not fully initialized")
	}
	if found, err := h.local.Get(ctx, key, dest); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	if found, err := h.redis.Get(ctx, key, dest); err != nil {
		return false, err
	} else if found {
		if err := h.local.Set(ctx, key, dest, time.Minute); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func (h *HybridCache) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	if h == nil || h.local == nil || h.redis == nil {
		return errors.New("hybrid cache is not fully initialized")
	}
	if err := h.redis.Set(ctx, key, val, ttl); err != nil {
		return err
	}
	return h.local.Set(ctx, key, val, ttl)
}

func (h *HybridCache) Delete(ctx context.Context, key string) error {
	if h == nil || h.local == nil || h.redis == nil {
		return errors.New("hybrid cache is not fully initialized")
	}
	localErr := h.local.Delete(ctx, key)
	redisErr := h.redis.Delete(ctx, key)
	return errors.Join(localErr, redisErr)
}

// Exist 实现了多级缓存的高效联动判断
func (h *HybridCache) Exist(ctx context.Context, key string) (bool, error) {
	if h == nil || h.local == nil || h.redis == nil {
		return false, errors.New("hybrid cache is not fully initialized")
	}
	if found, err := h.local.Exist(ctx, key); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	return h.redis.Exist(ctx, key)
}
