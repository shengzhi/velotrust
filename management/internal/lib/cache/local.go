package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var _ Cache = (*LocalCache)(nil)

// LocalCache is a synchronous, process-local cache. Unlike the template's
// buffered cache, Set is immediately visible to Get; ceremony state must never
// be acknowledged before it is actually stored.
type LocalCache struct {
	mu      sync.RWMutex
	entries map[string]localEntry
}

type localEntry struct {
	data      []byte
	expiresAt time.Time
}

func NewLocalCache() *LocalCache { return &LocalCache{entries: make(map[string]localEntry)} }

func (c *LocalCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	if c == nil || key == "" {
		return errors.New("cache and key are required")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	entry := localEntry{data: data}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	return nil
}

func (c *LocalCache) Get(_ context.Context, key string, destination any) (bool, error) {
	if c == nil || key == "" || destination == nil {
		return false, errors.New("cache, key, and destination are required")
	}
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if !entry.expiresAt.IsZero() && !entry.expiresAt.After(time.Now()) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return false, nil
	}
	if err := json.Unmarshal(entry.data, destination); err != nil {
		return false, err
	}
	return true, nil
}

func (c *LocalCache) Delete(_ context.Context, key string) error {
	if c == nil || key == "" {
		return errors.New("cache and key are required")
	}
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
	return nil
}

func (c *LocalCache) Exist(ctx context.Context, key string) (bool, error) {
	var raw json.RawMessage
	return c.Get(ctx, key, &raw)
}
