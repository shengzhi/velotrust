// SPDX-License-Identifier: BSD-3-Clause

// Package netstate tracks operating system reported network availability.
package netstate

import (
	"context"
	"log/slog"
	"sync"
)

// State holds operating system reported network availability.
// Create instances with New; the zero value is not usable.
type State struct {
	mu      sync.Mutex
	online  bool
	changed chan struct{}
}

// New creates a State that starts online.
func New() *State {
	return &State{online: true, changed: make(chan struct{})}
}

// Set records whether the operating system reports a usable network.
// Transitions wake all callers waiting in Wait.
func (s *State) Set(online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.online == online {
		return
	}
	s.online = online
	close(s.changed)
	s.changed = make(chan struct{})
	slog.Info("network availability changed", "online", online)
}

// IsOnline reports whether a usable network is available. A nil receiver is online.
func (s *State) IsOnline() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online
}

// Changed returns a channel closed on the next availability transition.
// Read it again after a transition to observe the next one. On a nil receiver it returns nil.
func (s *State) Changed() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changed
}

// Wait blocks while offline. It reports whether it waited and returns ctx's error on cancellation.
// A nil receiver returns immediately as online.
func (s *State) Wait(ctx context.Context) (bool, error) {
	if s == nil {
		return false, nil
	}
	waited := false
	for {
		s.mu.Lock()
		if s.online {
			s.mu.Unlock()
			return waited, nil
		}
		changed := s.changed
		s.mu.Unlock()

		if !waited {
			waited = true
			slog.Debug("network is offline; pausing connection attempts")
		}
		select {
		case <-ctx.Done():
			return waited, ctx.Err()
		case <-changed:
		}
	}
}
