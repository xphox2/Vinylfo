package sync_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"vinylfo/discogs"
	"vinylfo/sync"
)

// TestRateLimiterCallbacksAreCalled verifies that rate limit callbacks are invoked
func TestRateLimiterCallbacksAreCalled(t *testing.T) {
	rl := discogs.NewRateLimiter()

	var callbackCalled int32
	var clearedCallbackCalled int32

	rl.SetRateLimitCallback(func(retryAfter int) {
		atomic.StoreInt32(&callbackCalled, 1)
		if retryAfter <= 0 {
			t.Error("Expected retryAfter to be positive")
		}
	})

	rl.SetRateLimitClearedCallback(func() {
		atomic.StoreInt32(&clearedCallbackCalled, 1)
	})

	// Use a short wait time for testing
	go rl.WaitForReset(1) // 1 second wait

	// Give time for callback to be called
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&callbackCalled) != 1 {
		t.Error("Rate limit callback was not called")
	}

	// Wait for the reset to complete
	time.Sleep(1100 * time.Millisecond)

	if atomic.LoadInt32(&clearedCallbackCalled) != 1 {
		t.Error("Rate limit cleared callback was not called")
	}
}

// TestRateLimiterIsRateLimitedFlag verifies the isRateLimited flag behavior
func TestRateLimiterIsRateLimitedFlag(t *testing.T) {
	rl := discogs.NewRateLimiter()

	if rl.IsRateLimited() {
		t.Error("New rate limiter should not be rate limited")
	}

	// Start a short rate limit wait in background
	done := make(chan bool)
	go func() {
		rl.WaitForReset(1)
		done <- true
	}()

	// Give time for the flag to be set
	time.Sleep(100 * time.Millisecond)

	if !rl.IsRateLimited() {
		t.Error("Rate limiter should be rate limited during WaitForReset")
	}

	// Wait for completion
	<-done

	if rl.IsRateLimited() {
		t.Error("Rate limiter should not be rate limited after WaitForReset completes")
	}
}

// TestRateLimiterSecondsUntilReset verifies the countdown is accurate
func TestRateLimiterSecondsUntilReset(t *testing.T) {
	rl := discogs.NewRateLimiter()

	if rl.GetSecondsUntilReset() != 0 {
		t.Error("New rate limiter should have 0 seconds until reset")
	}

	// Start a 3 second rate limit wait in background
	go rl.WaitForReset(3)

	// Give time for the rate limit to be set
	time.Sleep(100 * time.Millisecond)

	seconds := rl.GetSecondsUntilReset()
	if seconds < 1 || seconds > 3 {
		t.Errorf("Expected seconds until reset to be between 1-3, got %d", seconds)
	}

	// Wait a bit and check it's decreasing
	time.Sleep(1100 * time.Millisecond)

	newSeconds := rl.GetSecondsUntilReset()
	if newSeconds >= seconds {
		t.Errorf("Seconds until reset should decrease over time: was %d, now %d", seconds, newSeconds)
	}
}

// TestRateLimiterClearRateLimit verifies manual clearing works
func TestRateLimiterClearRateLimit(t *testing.T) {
	rl := discogs.NewRateLimiter()

	// Start a long rate limit wait in background
	go rl.WaitForReset(60)

	// Give time for the flag to be set
	time.Sleep(100 * time.Millisecond)

	if !rl.IsRateLimited() {
		t.Error("Rate limiter should be rate limited")
	}

	// Manually clear
	rl.ClearRateLimit()

	if rl.IsRateLimited() {
		t.Error("Rate limiter should not be rate limited after ClearRateLimit")
	}
}

// TestSyncStateManagerPauseResume verifies pause/resume state transitions
func TestSyncStateManagerPauseResume(t *testing.T) {
	sm := sync.NewStateManager()
	// Initialize with running state
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusRunning
	})

	state := sm.GetState()
	if !state.IsRunning() {
		t.Error("State should be running")
	}

	// Request pause
	if !sm.RequestPause() {
		t.Error("RequestPause should succeed when running")
	}

	state = sm.GetState()
	if !state.IsPaused() {
		t.Error("State should be paused after RequestPause")
	}

	// Request pause again (should fail)
	if sm.RequestPause() {
		t.Error("RequestPause should fail when already paused")
	}

	// Request resume
	if !sm.RequestResume() {
		t.Error("RequestResume should succeed when paused")
	}

	state = sm.GetState()
	if !state.IsRunning() {
		t.Error("State should be running after RequestResume")
	}

	// Request resume again (should fail)
	if sm.RequestResume() {
		t.Error("RequestResume should fail when already running")
	}
}

// TestSyncStateManagerWaitForResume verifies WaitForResume unblocks on status change
func TestSyncStateManagerWaitForResume(t *testing.T) {
	sm := sync.NewStateManager()
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusPaused
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sm.WaitForResume(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Resume the state
	sm.RequestResume()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("WaitForResume returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("WaitForResume did not unblock after RequestResume")
	}
}

// TestSyncStateManagerWaitForResumeContextCancel verifies WaitForResume respects context
func TestSyncStateManagerWaitForResumeContextCancel(t *testing.T) {
	sm := sync.NewStateManager()
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusPaused
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := sm.WaitForResume(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got: %v", err)
	}
}

// TestSyncStateRateLimitFields verifies rate limit state fields
func TestSyncStateRateLimitFields(t *testing.T) {
	sm := sync.NewStateManager()

	// Set rate limit state
	retryAt := time.Now().Add(60 * time.Second)
	sm.SetRateLimitState(retryAt, "API rate limit - retry after 60 seconds")

	state := sm.GetState()
	if state.RateLimitRetryAt == nil {
		t.Error("RateLimitRetryAt should be set")
	}
	if state.RateLimitMessage == "" {
		t.Error("RateLimitMessage should be set")
	}

	// Clear rate limit state
	sm.ClearRateLimitState()

	state = sm.GetState()
	if state.RateLimitRetryAt != nil {
		t.Error("RateLimitRetryAt should be nil after clear")
	}
	if state.RateLimitMessage != "" {
		t.Error("RateLimitMessage should be empty after clear")
	}
}

// TestSyncStateIsActive verifies IsActive includes both running and paused
func TestSyncStateIsActive(t *testing.T) {
	tests := []struct {
		status   sync.SyncStatus
		isActive bool
	}{
		{sync.SyncStatusIdle, false},
		{sync.SyncStatusRunning, true},
		{sync.SyncStatusPaused, true},
		{sync.SyncStatusStopping, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			state := sync.SyncState{Status: tt.status}
			if state.IsActive() != tt.isActive {
				t.Errorf("Status %s: IsActive() = %v, want %v", tt.status, state.IsActive(), tt.isActive)
			}
		})
	}
}

// TestRateLimitRecoveryFlow simulates the full rate limit recovery flow
func TestRateLimitRecoveryFlow(t *testing.T) {
	rl := discogs.NewRateLimiter()
	sm := sync.NewStateManager()

	// Start in running state
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusRunning
		s.Processed = 50
		s.Total = 100
	})

	// Set up rate limit callback (simulates sync_worker.go behavior)
	rl.SetRateLimitCallback(func(retryAfter int) {
		retryAt := time.Now().Add(time.Duration(retryAfter) * time.Second)
		sm.UpdateState(func(s *sync.SyncState) {
			s.RateLimitRetryAt = &retryAt
			s.RateLimitMessage = "API rate limit"
		})
		// Only pause if running
		state := sm.GetState()
		if state.IsRunning() {
			sm.SetStatus(sync.SyncStatusPaused)
		}
	})

	rl.SetRateLimitClearedCallback(func() {
		sm.ClearRateLimitState()
		// Only resume if paused
		state := sm.GetState()
		if state.IsPaused() {
			sm.RequestResume()
		}
	})

	// Trigger rate limit with short duration
	done := make(chan bool)
	go func() {
		rl.WaitForReset(1)
		done <- true
	}()

	// Give time for rate limit to be set
	time.Sleep(100 * time.Millisecond)

	// Verify paused state
	state := sm.GetState()
	if !state.IsPaused() {
		t.Error("State should be paused during rate limit")
	}
	if state.RateLimitRetryAt == nil {
		t.Error("RateLimitRetryAt should be set")
	}
	if !rl.IsRateLimited() {
		t.Error("Rate limiter should indicate rate limited")
	}

	// Wait for rate limit to clear
	<-done

	// Give time for callbacks to complete
	time.Sleep(100 * time.Millisecond)

	// Verify resumed state
	state = sm.GetState()
	if !state.IsRunning() {
		t.Errorf("State should be running after rate limit cleared, got %s", state.Status)
	}
	if state.RateLimitRetryAt != nil {
		t.Error("RateLimitRetryAt should be cleared")
	}
	if rl.IsRateLimited() {
		t.Error("Rate limiter should not indicate rate limited")
	}
}

// TestRateLimitDoesNotResumeIfManuallyCancelled verifies user cancellation is respected
func TestRateLimitDoesNotResumeIfManuallyCancelled(t *testing.T) {
	rl := discogs.NewRateLimiter()
	sm := sync.NewStateManager()

	// Start in running state
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusRunning
	})

	rl.SetRateLimitCallback(func(retryAfter int) {
		state := sm.GetState()
		if state.IsRunning() {
			sm.SetStatus(sync.SyncStatusPaused)
		}
	})

	rl.SetRateLimitClearedCallback(func() {
		sm.ClearRateLimitState()
		// Only resume if paused (not if user cancelled)
		state := sm.GetState()
		if state.IsPaused() {
			sm.RequestResume()
		}
	})

	// Trigger rate limit
	done := make(chan bool)
	go func() {
		rl.WaitForReset(1)
		done <- true
	}()

	// Give time for rate limit to be set
	time.Sleep(100 * time.Millisecond)

	// User cancels (sets to idle)
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
	})

	// Wait for rate limit to clear
	<-done
	time.Sleep(100 * time.Millisecond)

	// Verify still idle (not auto-resumed)
	state := sm.GetState()
	if state.Status != sync.SyncStatusIdle {
		t.Errorf("State should remain idle after user cancel, got %s", state.Status)
	}
}

// TestRateLimitTickerBasedWaiting verifies WaitForReset doesn't block completely
func TestRateLimitTickerBasedWaiting(t *testing.T) {
	rl := discogs.NewRateLimiter()

	// Track seconds updates
	var updates int32

	// Start rate limit wait
	go func() {
		rl.WaitForReset(3)
	}()

	// Monitor GetSecondsUntilReset changes
	time.Sleep(100 * time.Millisecond)
	lastSeconds := rl.GetSecondsUntilReset()

	for i := 0; i < 4; i++ {
		time.Sleep(800 * time.Millisecond)
		newSeconds := rl.GetSecondsUntilReset()
		if newSeconds != lastSeconds {
			atomic.AddInt32(&updates, 1)
			lastSeconds = newSeconds
		}
	}

	// Should see at least 2 updates (countdown ticking)
	if atomic.LoadInt32(&updates) < 2 {
		t.Errorf("Expected at least 2 countdown updates, got %d", atomic.LoadInt32(&updates))
	}
}

// TestSyncProgressResponseDuringRateLimit simulates what the progress endpoint returns
func TestSyncProgressResponseDuringRateLimit(t *testing.T) {
	rl := discogs.NewRateLimiter()
	sm := sync.NewStateManager()

	// Set up initial state
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusRunning
		s.Processed = 30
		s.Total = 100
	})

	// Simulate rate limit hit
	retryAt := time.Now().Add(60 * time.Second)
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusPaused
		s.RateLimitRetryAt = &retryAt
		s.RateLimitMessage = "API rate limit - retry after 60 seconds"
	})

	// Start the rate limiter (in real scenario this happens in WaitForReset)
	go rl.WaitForReset(2)
	time.Sleep(100 * time.Millisecond)

	// Simulate what GetSyncProgress controller does
	state := sm.GetState()
	isRateLimited := rl.IsRateLimited()
	rateLimitSecondsLeft := rl.GetSecondsUntilReset()

	// Verify the response fields that frontend needs
	if !state.IsPaused() {
		t.Error("is_paused should be true during rate limit")
	}
	if state.IsRunning() {
		t.Error("is_running should be false during rate limit")
	}
	if !isRateLimited {
		t.Error("is_rate_limited should be true")
	}
	if rateLimitSecondsLeft <= 0 {
		t.Error("rate_limit_seconds_left should be positive")
	}
	if state.Processed != 30 {
		t.Errorf("processed should be 30, got %d", state.Processed)
	}
}

// TestExpiredRateLimitAutoClears verifies expired rate limits are detected
func TestExpiredRateLimitAutoClears(t *testing.T) {
	rl := discogs.NewRateLimiter()

	// Start a very short rate limit
	done := make(chan bool)
	go func() {
		rl.WaitForReset(1)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	if !rl.IsRateLimited() {
		t.Error("Should be rate limited")
	}

	// Wait for it to expire
	<-done
	time.Sleep(100 * time.Millisecond)

	// After WaitForReset completes, GetSecondsUntilReset should return 0
	if rl.GetSecondsUntilReset() != 0 {
		t.Errorf("Seconds until reset should be 0 after expiry, got %d", rl.GetSecondsUntilReset())
	}

	if rl.IsRateLimited() {
		t.Error("Should not be rate limited after WaitForReset completes")
	}
}

// TestConcurrentStateAccess verifies thread safety of state manager
func TestConcurrentStateAccess(t *testing.T) {
	sm := sync.NewStateManager()
	sm.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusRunning
		s.Processed = 0
		s.Total = 100
	})

	done := make(chan bool)

	// Goroutine 1: Update processed count
	go func() {
		for i := 0; i < 100; i++ {
			sm.UpdateState(func(s *sync.SyncState) {
				s.Processed++
			})
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Read state frequently
	go func() {
		for i := 0; i < 200; i++ {
			state := sm.GetState()
			_ = state.Processed // Access field
			time.Sleep(500 * time.Microsecond)
		}
		done <- true
	}()

	// Goroutine 3: Toggle pause/resume
	go func() {
		for i := 0; i < 10; i++ {
			sm.RequestPause()
			time.Sleep(5 * time.Millisecond)
			sm.RequestResume()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Final state should be consistent
	state := sm.GetState()
	if state.Processed != 100 {
		t.Errorf("Expected processed = 100, got %d", state.Processed)
	}
}
