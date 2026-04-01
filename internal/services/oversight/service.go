// Package oversight manages the await_response primitive: blocking until a human
// replies in a room. It is intentionally in-memory — waiters are transient and
// do not survive a restart.
package oversight

import (
	"fmt"
	"sync"
	"time"

	"github.com/hastenr/chatapi/internal/models"
)

// Service holds in-flight waiters keyed by "tenantID:roomID".
type Service struct {
	mu      sync.Mutex
	waiters map[string][]*waiter
}

type waiter struct {
	ch chan *models.Message
}

// NewService creates a new oversight service.
func NewService() *Service {
	return &Service{waiters: make(map[string][]*waiter)}
}

// WaitForResponse blocks until any message arrives in the room via NotifyWaiters,
// or until timeout elapses. Only call this from goroutines that can block
// (e.g. MCP tool handlers running in their own goroutine).
func (s *Service) WaitForResponse(tenantID, roomID string, timeout time.Duration) (*models.Message, error) {
	w := &waiter{ch: make(chan *models.Message, 1)}
	key := tenantID + ":" + roomID

	s.mu.Lock()
	s.waiters[key] = append(s.waiters[key], w)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		list := s.waiters[key]
		for i, ww := range list {
			if ww == w {
				s.waiters[key] = append(list[:i], list[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()

	select {
	case msg := <-w.ch:
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout")
	}
}

// NotifyWaiters unblocks all goroutines waiting on this room.
// Call this after every human-initiated message send (not bot sends).
func (s *Service) NotifyWaiters(tenantID, roomID string, msg *models.Message) {
	key := tenantID + ":" + roomID

	s.mu.Lock()
	list := s.waiters[key]
	s.waiters[key] = nil
	s.mu.Unlock()

	for _, w := range list {
		select {
		case w.ch <- msg:
		default:
		}
	}
}
