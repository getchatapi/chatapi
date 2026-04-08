package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	cleanupInterval = time.Minute
	idleTTL         = 5 * time.Minute
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is a per-key token bucket rate limiter with automatic eviction of
// idle entries. Keys are typically user IDs.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	limit   rate.Limit
	burst   int
	stop    chan struct{}
}

// New creates a Limiter allowing rps events per second with the given burst
// size per key. It starts a background goroutine to evict idle entries;
// call Stop when the limiter is no longer needed.
func New(rps float64, burst int) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		limit:   rate.Limit(rps),
		burst:   burst,
		stop:    make(chan struct{}),
	}
	go l.cleanup()
	return l
}

// Allow reports whether the key is within its rate limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	e, ok := l.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = time.Now()
	l.mu.Unlock()
	return e.limiter.Allow()
}

// Stop shuts down the background cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stop)
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-idleTTL)
			l.mu.Lock()
			for key, e := range l.entries {
				if e.lastSeen.Before(cutoff) {
					delete(l.entries, key)
				}
			}
			l.mu.Unlock()
		case <-l.stop:
			return
		}
	}
}
