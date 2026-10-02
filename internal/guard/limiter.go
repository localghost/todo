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

	mu     sync.Mutex
	hits   map[string][]hit
	calls  int
	nextID uint64
}

type hit struct {
	at time.Time
	id uint64
}

// Ticket identifies one reserved hit, for Release.
type Ticket struct {
	key string
	id  uint64
}

// NewLimiter allows at most max hits per key within window.
func NewLimiter(max int, window time.Duration, now func() time.Time) *Limiter {
	return &Limiter{max: max, window: window, now: now, hits: map[string][]hit{}}
}

// Reserve records one hit for key if fewer than max hits are in the window.
// The ticket releases exactly this hit.
func (l *Limiter) Reserve(key string) (Ticket, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls++; l.calls%1000 == 0 {
		l.sweep()
	}
	h := l.recent(key)
	if len(h) >= l.max {
		l.hits[key] = h
		return Ticket{}, false
	}
	l.nextID++
	l.hits[key] = append(h, hit{at: l.now(), id: l.nextID})
	return Ticket{key: key, id: l.nextID}, true
}

// Release removes the hit of t, for a reservation that should not count.
func (l *Limiter) Release(t Ticket) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hits[t.key]
	for i, e := range h {
		if e.id == t.id {
			h = append(h[:i:i], h[i+1:]...)
			break
		}
	}
	if len(h) == 0 {
		delete(l.hits, t.key)
		return
	}
	l.hits[t.key] = h
}

// recent returns the hits of key inside the window. Callers hold l.mu.
func (l *Limiter) recent(key string) []hit {
	cutoff := l.now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && !h[i].at.After(cutoff) {
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
