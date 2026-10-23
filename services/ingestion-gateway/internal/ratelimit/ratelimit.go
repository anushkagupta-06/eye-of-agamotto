// Package ratelimit provides a small per-key token bucket.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter allows rate events per second per key with the given burst. The key space is bounded
// by the number of tenants in the key file, so buckets are never evicted.
type Limiter struct {
	rate, burst float64
	now         func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

// New returns a limiter. rate <= 0 disables limiting.
func New(rate float64, burst int) *Limiter {
	return &Limiter{rate: rate, burst: float64(burst), now: time.Now, buckets: make(map[string]*bucket)}
}

// Allow consumes one token for key and reports whether the event may proceed.
func (l *Limiter) Allow(key string) bool {
	if l.rate <= 0 {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
