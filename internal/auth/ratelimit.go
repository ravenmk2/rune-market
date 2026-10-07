package auth

import (
	"sync"
	"time"
)

// ipLimiter is an in-memory per-IP token bucket (design §13: 10 req/min on
// login and register). Process-local by design; resets on restart.
type ipLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64
	refill   float64 // tokens per second
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newIPLimiter(capacity int, per time.Duration) *ipLimiter {
	return &ipLimiter{
		buckets:  map[string]*bucket{},
		capacity: float64(capacity),
		refill:   float64(capacity) / per.Seconds(),
	}
}

func (l *ipLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) > 1024 {
		l.sweep()
	}

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.refill
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep drops buckets idle for over 10 minutes; called opportunistically.
func (l *ipLimiter) sweep() {
	cutoff := time.Now().Add(-10 * time.Minute)
	for k, b := range l.buckets {
		if b.last.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}
