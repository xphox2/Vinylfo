package discogs

import (
	"testing"
	"time"
)

// TestRateLimiterWaitRealImplementation tests the actual Wait() function
func TestRateLimiterWaitRealImplementation(t *testing.T) {
	tests := []struct {
		name          string
		authRemaining int
		anonRemaining int
		isAuth        bool
		shouldWait    bool
		description   string
	}{
		{
			name:          "Auth request with plenty auth remaining, anon low",
			authRemaining: 50,
			anonRemaining: 1,
			isAuth:        true,
			shouldWait:    false,
			description:   "Should NOT wait for anon when making auth request",
		},
		{
			name:          "Auth request with auth low",
			authRemaining: 2,
			anonRemaining: 25,
			isAuth:        true,
			shouldWait:    true,
			description:   "Should wait when auth is low",
		},
		{
			name:          "Anon request with anon low",
			authRemaining: 50,
			anonRemaining: 2,
			isAuth:        false,
			shouldWait:    true,
			description:   "Should wait when anon is low",
		},
		{
			name:          "Anon request with anon okay, auth low",
			authRemaining: 2,
			anonRemaining: 20,
			isAuth:        false,
			shouldWait:    false,
			description:   "Should NOT wait for auth when making anon request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter()
			rl.authRemaining = tt.authRemaining
			rl.anonRemaining = tt.anonRemaining

			start := time.Now()
			ch := make(chan bool, 1)

			// Run Wait in goroutine to check if it would wait
			go func() {
				rl.Wait(tt.isAuth)
				ch <- true
			}()

			// Check if function waits or returns immediately
			select {
			case <-ch:
				elapsed := time.Since(start)
				waited := elapsed > 100*time.Millisecond

				if waited != tt.shouldWait {
					t.Errorf("%s: waited=%v, want %v (auth=%d, anon=%d, isAuth=%v)",
						tt.description, waited, tt.shouldWait, tt.authRemaining, tt.anonRemaining, tt.isAuth)
				}
			case <-time.After(500 * time.Millisecond):
				t.Errorf("Test timed out - Wait() hung (auth=%d, anon=%d, isAuth=%v)",
					tt.authRemaining, tt.anonRemaining, tt.isAuth)
			}
		})
	}
}
