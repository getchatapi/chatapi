package ratelimit_test

import (
	"testing"
	"time"

	"github.com/getchatapi/chatapi/internal/ratelimit"
)

func TestAllow(t *testing.T) {
	t.Run("allows requests within burst", func(t *testing.T) {
		l := ratelimit.New(10, 5)
		defer l.Stop()

		// Burst of 5 should all be allowed immediately.
		for i := 0; i < 5; i++ {
			if !l.Allow("user-1") {
				t.Fatalf("request %d should be allowed within burst", i+1)
			}
		}
	})

	t.Run("rejects requests exceeding burst", func(t *testing.T) {
		l := ratelimit.New(1, 2)
		defer l.Stop()

		// Exhaust the burst.
		l.Allow("user-1")
		l.Allow("user-1")

		// Next request should be denied.
		if l.Allow("user-1") {
			t.Fatal("expected request to be denied after burst exhausted")
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		l := ratelimit.New(1, 1)
		defer l.Stop()

		// Exhaust user-1's quota.
		l.Allow("user-1")
		if l.Allow("user-1") {
			t.Fatal("user-1 should be rate limited")
		}

		// user-2 should still be allowed.
		if !l.Allow("user-2") {
			t.Fatal("user-2 should not be affected by user-1's limit")
		}
	})

	t.Run("refills over time", func(t *testing.T) {
		// 10 rps, burst 1 — one token available, refills after 100ms.
		l := ratelimit.New(10, 1)
		defer l.Stop()

		if !l.Allow("user-1") {
			t.Fatal("first request should be allowed")
		}
		// Deny immediately.
		if l.Allow("user-1") {
			t.Fatal("should be denied before refill")
		}
		// Wait for refill.
		time.Sleep(150 * time.Millisecond)
		if !l.Allow("user-1") {
			t.Fatal("should be allowed after refill")
		}
	})
}

func TestStop(t *testing.T) {
	l := ratelimit.New(10, 5)
	// Stop should not panic or block.
	l.Stop()
}
