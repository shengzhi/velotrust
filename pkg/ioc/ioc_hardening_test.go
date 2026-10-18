package ioc

import (
	"errors"
	"reflect"
	"testing"
)

type cycleRoot struct{}
type cycleChild struct{}

func TestSingletonCreatorCachesInitializationError(t *testing.T) {
	wantErr := errors.New("dependency unavailable")
	calls := 0
	creator := &singletonObjectCreator[Foo]{name: "Foo", createFn: func() (*Foo, error) {
		calls++
		return nil, wantErr
	}}
	container := &ObjContainer{table: make(map[reflect.Type]ICreator[any])}
	for range 2 {
		if _, err := creator.create(container, nil); !errors.Is(err, wantErr) {
			t.Fatalf("expected cached initialization error, got %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("failed singleton factory should run once, got %d calls", calls)
	}
}

func TestDependencyGraphRejectsCycles(t *testing.T) {
	root := NewNode(reflect.TypeOf(cycleRoot{}))
	child := NewNode(reflect.TypeOf(cycleChild{}))
	if err := root.Add(child); err != nil {
		t.Fatal(err)
	}
	if err := child.Add(NewNode(reflect.TypeOf(cycleRoot{}))); err == nil {
		t.Fatal("dependency graph accepted a cycle")
	}
}
