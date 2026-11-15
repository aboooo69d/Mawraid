// Package cache provides the Redis integration used for rate limiting and
// (in future sprints) caching. When Redis is disabled (development/tests
// without Docker), an in-process limiter with identical semantics is used;
// it protects a single instance only and is never used in production.
package cache

import (
	"context"
	"sync"
	"time"
)

// Limiter is a fixed-window counter used for OTP resend throttling, login
// attempt limiting, and generic endpoint rate limits.
type Limiter interface {
	// Hit increments the counter for key and reports whether maxHits was
	// still within the window. retryAfter > 0 tells when the window resets.
	Hit(ctx context.Context, key string, maxHits int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
	// Peek reports the current counter value and time-to-expiry WITHOUT
	// incrementing it (used for lockout checks).
	Peek(ctx context.Context, key string) (count int, retryAfter time.Duration, err error)
	// Reset clears the counter for key (e.g. after a successful login).
	Reset(ctx context.Context, key string) error
	// Close releases resources.
	Close() error
}

// --- in-memory fallback -----------------------------------------------------

type memEntry struct {
	count   int
	expires time.Time
}

// MemoryLimiter is a single-process fixed-window limiter (dev/test fallback).
type MemoryLimiter struct {
	mu   sync.Mutex
	data map[string]memEntry
	stop chan struct{}
	once sync.Once
}

// NewMemoryLimiter starts a cleanup goroutine.
func NewMemoryLimiter() *MemoryLimiter {
	m := &MemoryLimiter{data: make(map[string]memEntry), stop: make(chan struct{})}
	go m.cleanupLoop()
	return m
}

func (m *MemoryLimiter) cleanupLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-t.C:
			m.mu.Lock()
			for k, e := range m.data {
				if now.After(e.expires) {
					delete(m.data, k)
				}
			}
			m.mu.Unlock()
		}
	}
}

// Hit implements Limiter.
func (m *MemoryLimiter) Hit(_ context.Context, key string, maxHits int, window time.Duration) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	e, ok := m.data[key]
	if !ok || now.After(e.expires) {
		e = memEntry{count: 0, expires: now.Add(window)}
	}
	e.count++
	m.data[key] = e
	if e.count > maxHits {
		return false, e.expires.Sub(now), nil
	}
	return true, 0, nil
}

// Reset implements Limiter.
func (m *MemoryLimiter) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

// Peek implements Limiter.
func (m *MemoryLimiter) Peek(_ context.Context, key string) (int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || time.Now().After(e.expires) {
		return 0, 0, nil
	}
	return e.count, e.expires.Sub(time.Now()), nil
}

// Close implements Limiter.
func (m *MemoryLimiter) Close() error {
	m.once.Do(func() { close(m.stop) })
	return nil
}
