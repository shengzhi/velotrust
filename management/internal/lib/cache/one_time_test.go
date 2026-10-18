package cache

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalOneTimeStoreConsumesOnlyOnceConcurrently(t *testing.T) {
	store := NewLocalOneTimeStore()
	if err := store.Put(context.Background(), "challenge", map[string]string{"value": "state"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var state map[string]string
			ok, err := store.Consume(context.Background(), "challenge", &state)
			if err != nil {
				t.Errorf("consume state: %v", err)
				return
			}
			if ok {
				if state["value"] != "state" {
					t.Errorf("unexpected state: %#v", state)
				}
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("challenge should be consumed exactly once, got %d successes", got)
	}
}

func TestLocalOneTimeStoreExpiresAndRejectsInvalidCalls(t *testing.T) {
	store := NewLocalOneTimeStore()
	if err := store.Put(context.Background(), "expired", "state", -time.Second); err == nil {
		t.Fatal("non-positive TTL was accepted")
	}
	if err := store.Put(context.Background(), "expired", "state", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	var state string
	if ok, err := store.Consume(context.Background(), "expired", &state); err != nil || ok {
		t.Fatalf("expired state should be unavailable: ok=%v err=%v", ok, err)
	}
}

func TestLocalOneTimeStorePrunesExpiredStateAndCapsMemory(t *testing.T) {
	store := NewLocalOneTimeStore()
	store.entries["expired"] = oneTimeEntry{data: []byte(`"old"`), expiresAt: time.Now().Add(-time.Second)}
	store.lastSweep = time.Now().Add(-time.Minute)
	if err := store.Put(context.Background(), "fresh", "state", time.Minute); err != nil {
		t.Fatal(err)
	}
	if len(store.entries) != 1 {
		t.Fatalf("expired ceremony data was not pruned: %d entries remain", len(store.entries))
	}
	store.mu.Lock()
	for i := 0; i < maxLocalOneTimeEntries; i++ {
		store.entries["active-"+strconv.Itoa(i)] = oneTimeEntry{data: []byte(`"state"`), expiresAt: time.Now().Add(time.Hour)}
	}
	store.mu.Unlock()
	if err := store.Put(context.Background(), "overflow", "state", time.Minute); err == nil {
		t.Fatal("local ceremony store accepted data beyond its capacity")
	}
}
