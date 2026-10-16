package refcounter

import (
	"errors"
	"testing"
)

func TestCounterLifecycle(t *testing.T) {
	adds, removes := 0, 0
	counter := New[string, string, string](
		func(key, value string) (string, error) {
			adds++
			return value, nil
		},
		func(key, value string) error {
			removes++
			return nil
		},
	)

	first, err := counter.IncrementWithID("route-a", "10.0.0.0/24", "peer-a")
	if err != nil || first.Count != 1 || first.Out != "peer-a" {
		t.Fatalf("first IncrementWithID() = %+v, %v", first, err)
	}
	second, err := counter.Increment("10.0.0.0/24", "peer-b")
	if err != nil || second.Count != 2 || second.Out != "peer-a" || adds != 1 {
		t.Fatalf("second Increment() = %+v, %v; adds=%d", second, err, adds)
	}
	if err := counter.DecrementWithID("route-a"); err != nil {
		t.Fatal(err)
	}
	if removes != 0 {
		t.Fatalf("removed before final reference: %d", removes)
	}
	if _, err := counter.Decrement("10.0.0.0/24"); err != nil {
		t.Fatal(err)
	}
	if removes != 1 {
		t.Fatalf("remove count = %d, want 1", removes)
	}
}

func TestCounterIgnoreAndFlushError(t *testing.T) {
	removeErr := errors.New("remove failed")
	counter := New[string, struct{}, struct{}](
		func(string, struct{}) (struct{}, error) { return struct{}{}, ErrIgnore },
		func(string, struct{}) error { return removeErr },
	)
	if ref, err := counter.Increment("ignored", struct{}{}); err != nil || ref.Count != 0 {
		t.Fatalf("ignored Increment() = %+v, %v", ref, err)
	}
	if err := counter.Flush(); err != nil {
		t.Fatalf("Flush() after ignored key = %v", err)
	}

	counter = New[string, struct{}, struct{}](
		func(string, struct{}) (struct{}, error) { return struct{}{}, nil },
		func(string, struct{}) error { return removeErr },
	)
	if _, err := counter.Increment("key", struct{}{}); err != nil {
		t.Fatal(err)
	}
	if err := counter.Flush(); !errors.Is(err, removeErr) {
		t.Fatalf("Flush() error = %v, want wrapped remove error", err)
	}
}
