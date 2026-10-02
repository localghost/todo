// Package guard limits requests and checks sign-up forms against spam.
package guard

import (
	"sync"
	"time"
)

// Limiter counts hits per key in a sliding time window. It lives in memory
// and resets when the app restarts.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	hits  map[string][]time.Time
	calls int
}

// NewLimiter allows at most max hits per key within window.
func NewLimiter(max int, window time.Duration, now func() time.Time) *Limiter {
	return &Limiter{max: max, window: window, now: now, hits: map[string][]time.Time{}}
}

// Reserve records one hit for key if fewer than max hits are in the window,
// and reports whether it did. A reserved hit counts until Release.
func (l *Limiter) Reserve(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls++; l.calls%1000 == 0 {
		l.sweep()
	}
	h := l.recent(key)
	if len(h) >= l.max {
		l.hits[key] = h
		return false
	}
	l.hits[key] = append(h, l.now())
	return true
}

// Release removes one hit of key, for a reservation that should not count.
func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hits[key]
	if len(h) == 0 {
		return
	}
	if h = h[:len(h)-1]; len(h) == 0 {
		delete(l.hits, key)
		return
	}
	l.hits[key] = h
}

// recent returns the hits of key inside the window. Callers hold l.mu.
func (l *Limiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && !h[i].After(cutoff) {
		i++
	}
	return h[i:]
}

// sweep drops keys without recent hits. Callers hold l.mu.
func (l *Limiter) sweep() {
	for k := range l.hits {
		if h := l.recent(k); len(h) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = h
		}
	}
}
