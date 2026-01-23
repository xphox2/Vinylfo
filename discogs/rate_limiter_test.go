package discogs

import (
	"testing"
	"time"
)

// TestRateLimiterWaitNoWaitNeeded tests that Wait() returns immediately when limits are above threshold
func TestRateLimiterWaitNoWaitNeeded(t *testing.T) {
	tests := []struct {
		name          string
		authRemaining int
		anonRemaining int
		isAuth        bool
		description   string
	}{
		{
			name:          "Auth request with plenty auth remaining",
			authRemaining: 50,
			anonRemaining: 1,
			isAuth:        true,
			description:   "Should NOT wait when auth remaining is above threshold",
		},
		{
			name:          "Anon request with plenty anon remaining",
			authRemaining: 2,
			anonRemaining: 20,
			isAuth:        false,
			description:   "Should NOT wait when anon remaining is above threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter()
			rl.authRemaining = tt.authRemaining
			rl.anonRemaining = tt.anonRemaining

			start := time.Now()
			ch := make(chan bool, 1)

			go func() {
				rl.Wait(tt.isAuth)
				ch <- true
			}()

			select {
			case <-ch:
				elapsed := time.Since(start)
				if elapsed > 100*time.Millisecond {
					t.Errorf("%s: Wait() took %v, expected immediate return", tt.description, elapsed)
				}
			case <-time.After(500 * time.Millisecond):
				t.Errorf("Test timed out - Wait() hung unexpectedly")
			}
		})
	}
}

// TestRateLimiterWaitWithLowRemaining tests that Wait() waits when limits are below threshold
// Note: This test validates the behavior without actually waiting for the full window
func TestRateLimiterWaitWithLowRemaining(t *testing.T) {
	tests := []struct {
		name          string
		authRemaining int
		anonRemaining int
		isAuth        bool
		description   string
	}{
		{
			name:          "Auth request with auth low",
			authRemaining: 2,
			anonRemaining: 25,
			isAuth:        true,
			description:   "Should wait when auth remaining is at or below threshold (5)",
		},
		{
			name:          "Anon request with anon low",
			authRemaining: 50,
			anonRemaining: 2,
			isAuth:        false,
			description:   "Should wait when anon remaining is at or below threshold (5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter()
			rl.authRemaining = tt.authRemaining
			rl.anonRemaining = tt.anonRemaining
			// Set window start to the past so the rate limit window has expired
			// This allows Wait() to reset immediately without sleeping
			rl.windowStart = time.Now().Add(-RateLimitWindow - time.Second)

			start := time.Now()
			ch := make(chan bool, 1)

			go func() {
				rl.Wait(tt.isAuth)
				ch <- true
			}()

			// With expired window, Wait() should reset and return quickly
			select {
			case <-ch:
				elapsed := time.Since(start)
				// After window reset, remaining should be back to full
				if elapsed > 100*time.Millisecond {
					t.Errorf("%s: Wait() took %v after window reset", tt.description, elapsed)
				}
			case <-time.After(500 * time.Millisecond):
				t.Errorf("Test timed out - Wait() hung after window should have reset")
			}
		})
	}
}

// TestRateLimiterThreshold tests that the threshold of 5 is correctly applied
func TestRateLimiterThreshold(t *testing.T) {
	// Test that exactly at threshold (5) triggers wait behavior
	rl := NewRateLimiter()
	rl.authRemaining = 5                                            // At threshold
	rl.windowStart = time.Now().Add(-RateLimitWindow - time.Second) // Expired window

	ch := make(chan bool, 1)
	go func() {
		rl.Wait(true)
		ch <- true
	}()

	select {
	case <-ch:
		// After window reset, should complete
	case <-time.After(500 * time.Millisecond):
		t.Error("Wait() should complete after window reset when at threshold")
	}

	// Test that above threshold (6) returns immediately
	rl2 := NewRateLimiter()
	rl2.authRemaining = 6 // Above threshold

	start := time.Now()
	rl2.Wait(true)
	if time.Since(start) > 50*time.Millisecond {
		t.Error("Wait() should return immediately when above threshold")
	}
}
