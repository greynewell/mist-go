package parallel

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterTryTake(t *testing.T) {
	rl := NewRateLimiter(3, time.Second)

	// Should be able to take 3 tokens immediately.
	for i := 0; i < 3; i++ {
		if !rl.TryTake() {
			t.Fatalf("TryTake %d should succeed", i)
		}
	}

	// Fourth should fail.
	if rl.TryTake() {
		t.Error("TryTake should fail after exhausting tokens")
	}
}

func TestRateLimiterWait(t *testing.T) {
	rl := NewRateLimiter(100, 100*time.Millisecond)

	ctx := context.Background()
	// Drain tokens.
	for i := 0; i < 100; i++ {
		rl.TryTake()
	}

	// Wait should succeed after tokens refill.
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("Wait took %v, expected much less", elapsed)
	}
}

func TestRateLimiterWaitCancelled(t *testing.T) {
	rl := NewRateLimiter(1, time.Hour) // very slow refill

	// Drain the single token.
	rl.TryTake()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(10, 50*time.Millisecond)

	// Drain all tokens.
	for i := 0; i < 10; i++ {
		rl.TryTake()
	}

	// Wait for refill.
	time.Sleep(100 * time.Millisecond)

	// Should have tokens again.
	if !rl.TryTake() {
		t.Error("expected token after refill")
	}
}

func TestNewRateLimiterMinimumRate(t *testing.T) {
	rl := NewRateLimiter(0, time.Second)
	if rl.rate != 1 {
		t.Errorf("rate = %d, want 1", rl.rate)
	}
}
