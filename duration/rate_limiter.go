package duration

import (
	"sync"
	"time"
)

// RateLimiter implements a simple sliding window rate limiter
type RateLimiter struct {
	mu               sync.Mutex
	requestsPerMin   int
	windowStart      time.Time
	requestsInWindow int
}

// NewRateLimiter creates a rate limiter with the specified requests per minute
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		requestsPerMin:   requestsPerMinute,
		windowStart:      time.Now(),
		requestsInWindow: 0,
	}
}

// Wait blocks until a request can be made within rate limits
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.windowStart)

	// Reset window if minute has passed
	if elapsed >= time.Minute {
		rl.windowStart = now
		requestsInWindow := 0
		rl.requestsInWindow = requestsInWindow
	}

	// If at limit, wait for window to reset
	if rl.requestsInWindow >= rl.requestsPerMin {
		sleepTime := time.Minute - elapsed
		if sleepTime > 0 {
			rl.mu.Unlock()
			time.Sleep(sleepTime)
			rl.mu.Lock()
			rl.windowStart = time.Now()
			rl.requestsInWindow = 0
		}
	}

	rl.requestsInWindow++
}

// GetRemaining returns remaining requests in current window
func (rl *RateLimiter) GetRemaining() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.windowStart)
	if elapsed >= time.Minute {
		return rl.requestsPerMin
	}

	return rl.requestsPerMin - rl.requestsInWindow
}
