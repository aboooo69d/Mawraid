package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLimiterHitWindow(t *testing.T) {
	m := NewMemoryLimiter()
	defer m.Close()
	ctx := context.Background()

	// max 3 hits per window
	for i := 1; i <= 3; i++ {
		allowed, _, err := m.Hit(ctx, "k", 3, time.Minute)
		if err != nil {
			t.Fatalf("Hit %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("hit %d rejected before the limit", i)
		}
	}
	allowed, retry, err := m.Hit(ctx, "k", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Error("4th hit allowed past max=3")
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("retry = %s, want within (0, 1m]", retry)
	}
}

func TestMemoryLimiterPeekDoesNotIncrement(t *testing.T) {
	m := NewMemoryLimiter()
	defer m.Close()
	ctx := context.Background()

	_, _, _ = m.Hit(ctx, "peek", 5, time.Minute)
	_, _, _ = m.Hit(ctx, "peek", 5, time.Minute)

	for i := 0; i < 10; i++ {
		count, _, err := m.Peek(ctx, "peek")
		if err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("Peek changed the counter: got %d, want 2", count)
		}
	}
}

func TestMemoryLimiterReset(t *testing.T) {
	m := NewMemoryLimiter()
	defer m.Close()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, _ = m.Hit(ctx, "reset", 5, time.Minute)
	}
	if err := m.Reset(ctx, "reset"); err != nil {
		t.Fatal(err)
	}
	count, _, _ := m.Peek(ctx, "reset")
	if count != 0 {
		t.Errorf("counter after Reset = %d, want 0", count)
	}
	allowed, _, _ := m.Hit(ctx, "reset", 5, time.Minute)
	if !allowed {
		t.Error("hit after reset rejected")
	}
}

func TestMemoryLimiterWindowExpiry(t *testing.T) {
	m := NewMemoryLimiter()
	defer m.Close()
	ctx := context.Background()

	_, _, _ = m.Hit(ctx, "exp", 1, 40*time.Millisecond)
	allowed, _, _ := m.Hit(ctx, "exp", 1, 40*time.Millisecond)
	if allowed {
		t.Fatal("second hit allowed with max=1")
	}
	time.Sleep(60 * time.Millisecond)
	allowed, _, _ = m.Hit(ctx, "exp", 1, time.Minute)
	if !allowed {
		t.Error("hit not allowed after window expiry")
	}
	count, _, _ := m.Peek(ctx, "exp")
	if count != 1 {
		t.Errorf("counter after expiry = %d, want fresh window (1)", count)
	}
}
