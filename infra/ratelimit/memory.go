package ratelimit

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	count int
	reset time.Time
}

// Memory is a process-local fixed-window limiter. It is useful for tests and
// single-instance development, but it cannot share counters across replicas.
type Memory struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	max     int
	window  time.Duration
	calls   int
}

// NewMemory builds a process-local limiter.
func NewMemory(max int, window time.Duration) *Memory {
	max, window = normalize(max, window)
	return &Memory{
		entries: make(map[string]memoryEntry),
		max:     max,
		window:  window,
	}
}

// Allow records one request for subject and reports whether it is allowed.
func (l *Memory) Allow(_ context.Context, subject string) (Decision, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.calls++
	if l.calls%1024 == 0 {
		for key, entry := range l.entries {
			if !now.Before(entry.reset) {
				delete(l.entries, key)
			}
		}
	}

	entry, ok := l.entries[subject]
	if !ok || !now.Before(entry.reset) {
		l.entries[subject] = memoryEntry{count: 1, reset: now.Add(l.window)}
		return Decision{Allowed: true}, nil
	}
	if entry.count >= l.max {
		return Decision{Allowed: false, RetryAfter: entry.reset.Sub(now)}, nil
	}

	entry.count++
	l.entries[subject] = entry
	return Decision{Allowed: true}, nil
}

func normalize(max int, window time.Duration) (int, time.Duration) {
	if max <= 0 {
		max = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return max, window
}
