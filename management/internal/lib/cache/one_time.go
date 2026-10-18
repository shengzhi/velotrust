package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// OneTimeStore stores server-side ceremony state and atomically consumes it.
// Implementations must not return a value more than once.
type OneTimeStore interface {
	Put(context.Context, string, any, time.Duration) error
	Consume(context.Context, string, any) (bool, error)
}

type oneTimeEntry struct {
	data      []byte
	expiresAt time.Time
}

const maxLocalOneTimeEntries = 10000

// LocalOneTimeStore is intended for a single Management process only.
type LocalOneTimeStore struct {
	mu        sync.Mutex
	entries   map[string]oneTimeEntry
	lastSweep time.Time
}

func NewLocalOneTimeStore() *LocalOneTimeStore {
	return &LocalOneTimeStore{entries: make(map[string]oneTimeEntry), lastSweep: time.Now()}
}

func (s *LocalOneTimeStore) Put(_ context.Context, key string, value any, ttl time.Duration) error {
	if s == nil || key == "" || ttl <= 0 {
		return errors.New("one-time store, key, and positive TTL are required")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := time.Now()
	s.mu.Lock()
	if now.Sub(s.lastSweep) >= time.Minute || len(s.entries) >= maxLocalOneTimeEntries {
		for entryKey, entry := range s.entries {
			if !entry.expiresAt.After(now) {
				delete(s.entries, entryKey)
			}
		}
		s.lastSweep = now
	}
	if len(s.entries) >= maxLocalOneTimeEntries {
		s.mu.Unlock()
		return errors.New("local one-time store is full")
	}
	s.entries[key] = oneTimeEntry{data: data, expiresAt: now.Add(ttl)}
	s.mu.Unlock()
	return nil
}

func (s *LocalOneTimeStore) Consume(_ context.Context, key string, destination any) (bool, error) {
	if s == nil || key == "" || destination == nil {
		return false, errors.New("one-time store, key, and destination are required")
	}
	s.mu.Lock()
	entry, ok := s.entries[key]
	delete(s.entries, key)
	s.mu.Unlock()
	if !ok || !entry.expiresAt.After(time.Now()) {
		return false, nil
	}
	if err := json.Unmarshal(entry.data, destination); err != nil {
		return false, err
	}
	return true, nil
}

// RedisOneTimeStore uses GETDEL so simultaneous finish requests cannot both
// restore the same WebAuthn session state.
type RedisOneTimeStore struct {
	client *redis.Client
	prefix string
}

func NewRedisOneTimeStore(addr, password string, db int) (*RedisOneTimeStore, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, errors.New("Redis address is required")
	}
	return &RedisOneTimeStore{client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db}), prefix: "velotrust:iam:ceremony:"}, nil
}

func (s *RedisOneTimeStore) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("Redis ceremony store is not initialized")
	}
	return s.client.Ping(ctx).Err()
}

func (s *RedisOneTimeStore) Put(ctx context.Context, key string, value any, ttl time.Duration) error {
	if s == nil || s.client == nil || key == "" || ttl <= 0 {
		return errors.New("Redis ceremony store, key, and positive TTL are required")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.prefix+key, data, ttl).Err(); err != nil {
		return fmt.Errorf("store ceremony state: %w", err)
	}
	return nil
}

func (s *RedisOneTimeStore) Consume(ctx context.Context, key string, destination any) (bool, error) {
	if s == nil || s.client == nil || key == "" || destination == nil {
		return false, errors.New("Redis ceremony store, key, and destination are required")
	}
	data, err := s.client.GetDel(ctx, s.prefix+key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("consume ceremony state: %w", err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisOneTimeStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
