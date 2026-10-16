package netstate

import (
	"context"
	"testing"
	"time"
)

func TestWaitUnblocksWhenNetworkReturns(t *testing.T) {
	state := New()
	state.Set(false)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		_, err := state.Wait(ctx)
		done <- err == nil
	}()

	state.Set(true)
	select {
	case ok := <-done:
		if !ok || !state.IsOnline() {
			t.Fatal("Wait did not return after the network came back")
		}
	case <-ctx.Done():
		t.Fatal("Wait did not return when network came back")
	}
}

func TestWaitReturnsContextErrorWhileOffline(t *testing.T) {
	state := New()
	state.Set(false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	waited, err := state.Wait(ctx)
	if !waited || err != context.Canceled {
		t.Fatalf("Wait() = (%t, %v), want (true, context.Canceled)", waited, err)
	}
}

func TestNilStateIsOnline(t *testing.T) {
	var state *State
	if !state.IsOnline() {
		t.Fatal("nil State should report online")
	}
	if waited, err := state.Wait(context.Background()); waited || err != nil {
		t.Fatalf("nil State Wait() = (%t, %v), want (false, nil)", waited, err)
	}
}
