package session

import "sync"

// ActiveRegistry holds only process-local stream wakeups. Durable delivery
// targets and acknowledgements live in PostgreSQL; each HUB instance polls
// that shared state and wakes only the Gateway stream it currently owns.
type ActiveRegistry struct {
	mu      sync.RWMutex
	streams map[string]chan string
}

func NewActiveRegistry() *ActiveRegistry {
	return &ActiveRegistry{streams: make(map[string]chan string)}
}

func (r *ActiveRegistry) Register(nodeID string) (<-chan string, func()) {
	channel := make(chan string, 1)
	r.mu.Lock()
	previous := r.streams[nodeID]
	r.streams[nodeID] = channel
	r.mu.Unlock()
	if previous != nil {
		select {
		case previous <- "ACCESS_CONFIGURATION_CHANGED":
		default:
		}
	}
	return channel, func() {
		r.mu.Lock()
		if r.streams[nodeID] == channel {
			delete(r.streams, nodeID)
		}
		r.mu.Unlock()
	}
}

func (r *ActiveRegistry) Notify(nodeID, reason string) bool {
	r.mu.RLock()
	channel := r.streams[nodeID]
	r.mu.RUnlock()
	if channel == nil {
		return false
	}
	select {
	case channel <- reason:
		return true
	default:
	}
	// If a generic configuration wakeup is already queued, a revocation must
	// replace it so the Gateway blocks forwarding before it reconnects.
	if reason == "ACCESS_REVOKED" {
		select {
		case <-channel:
		default:
		}
		select {
		case channel <- reason:
		default:
		}
	}
	return true
}
