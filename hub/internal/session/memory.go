package session

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidRecord = errors.New("invalid HUB session record")
	ErrStaleSession  = errors.New("HUB session is no longer current")
)

// MemoryStore coordinates sessions within one HUB process. It deliberately
// does not claim cross-instance ownership; HA requires a shared lease store.
type MemoryStore struct {
	mu        sync.Mutex
	byID      map[string]Record
	byNode    map[string]string
	nodeLocks map[string]*sync.Mutex
	now       func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID:      make(map[string]Record),
		byNode:    make(map[string]string),
		nodeLocks: make(map[string]*sync.Mutex),
		now:       time.Now,
	}
}

func (s *MemoryStore) Acquire(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if record.SessionID == "" || record.NodeID == "" || record.InstanceID == "" || !record.LeaseUntil.After(s.now()) {
		return ErrInvalidRecord
	}
	nodeLock := s.lockForNode(record.NodeID)
	nodeLock.Lock()
	defer nodeLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !record.LeaseUntil.After(s.now()) {
		return ErrInvalidRecord
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[record.SessionID]; exists {
		return ErrInvalidRecord
	}
	if currentID, ok := s.byNode[record.NodeID]; ok {
		delete(s.byID, currentID)
	}
	s.byID[record.SessionID] = record
	s.byNode[record.NodeID] = record.SessionID
	return nil
}

func (s *MemoryStore) Renew(ctx context.Context, sessionID, instanceID string, leaseUntil time.Time) error {
	return s.WithCurrent(ctx, sessionID, instanceID, leaseUntil, func() error { return nil })
}

// WithCurrent serializes a node's authenticated work against session takeover.
// A new session for the same node waits until fn finishes; old sessions cannot
// write a heartbeat or status after a newer session has taken ownership.
func (s *MemoryStore) WithCurrent(ctx context.Context, sessionID, instanceID string, leaseUntil time.Time, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sessionID == "" || instanceID == "" || fn == nil {
		return ErrInvalidRecord
	}
	s.mu.Lock()
	record, ok := s.byID[sessionID]
	s.mu.Unlock()
	if !ok || record.InstanceID != instanceID {
		return ErrStaleSession
	}
	nodeLock := s.lockForNode(record.NodeID)
	nodeLock.Lock()
	defer nodeLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	record, ok = s.byID[sessionID]
	if !ok || record.InstanceID != instanceID || s.byNode[record.NodeID] != sessionID || !record.LeaseUntil.After(s.now()) {
		if ok && !record.LeaseUntil.After(s.now()) {
			s.deleteLocked(sessionID, record.NodeID)
		}
		s.mu.Unlock()
		return ErrStaleSession
	}
	if !leaseUntil.After(s.now()) {
		s.mu.Unlock()
		return ErrStaleSession
	}
	if leaseUntil.Before(record.LeaseUntil) {
		leaseUntil = record.LeaseUntil
	}
	record.LeaseUntil = leaseUntil
	s.byID[sessionID] = record
	s.mu.Unlock()
	return fn()
}

func (s *MemoryStore) Release(ctx context.Context, sessionID, instanceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	record, ok := s.byID[sessionID]
	s.mu.Unlock()
	if !ok || record.InstanceID != instanceID {
		return nil
	}
	nodeLock := s.lockForNode(record.NodeID)
	nodeLock.Lock()
	defer nodeLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok = s.byID[sessionID]
	if !ok || record.InstanceID != instanceID {
		return nil
	}
	s.deleteLocked(sessionID, record.NodeID)
	return nil
}

func (s *MemoryStore) ExpireBefore(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	nodes := make(map[string]struct{}, len(s.byNode))
	for nodeID := range s.byNode {
		nodes[nodeID] = struct{}{}
	}
	s.mu.Unlock()
	var expired int64
	for nodeID := range nodes {
		nodeLock := s.lockForNode(nodeID)
		nodeLock.Lock()
		s.mu.Lock()
		if sessionID, ok := s.byNode[nodeID]; ok {
			if record, exists := s.byID[sessionID]; exists && !record.LeaseUntil.After(before) {
				s.deleteLocked(sessionID, nodeID)
				expired++
			}
		}
		s.mu.Unlock()
		nodeLock.Unlock()
	}
	return expired, nil
}

func (s *MemoryStore) lockForNode(nodeID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.nodeLocks[nodeID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.nodeLocks[nodeID] = lock
	}
	return lock
}

func (s *MemoryStore) deleteLocked(sessionID, nodeID string) {
	delete(s.byID, sessionID)
	if s.byNode[nodeID] == sessionID {
		delete(s.byNode, nodeID)
	}
}

var _ Store = (*MemoryStore)(nil)
