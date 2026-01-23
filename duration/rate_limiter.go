package duration

import (
	"log"
	"sync"
	"time"
)

// RateLimiter implements a simple sliding window rate limiter with retry-after support
type RateLimiter struct {
	mu               sync.Mutex
	requestsPerMin   int
	windowStart      time.Time
	requestsInWindow int
	blockedUntil     time.Time // When set, all requests wait until this time
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

	// First check if we're blocked due to a rate limit response
	if rl.blockedUntil.After(now) {
		sleepTime := rl.blockedUntil.Sub(now)
		rl.mu.Unlock()
		log.Printf("Waiting %v due to rate limit block", sleepTime)
		time.Sleep(sleepTime)
		rl.mu.Lock()
		now = time.Now()
	}

	elapsed := now.Sub(rl.windowStart)

	// Reset window if minute has passed
	if elapsed >= time.Minute {
		rl.windowStart = now
		rl.requestsInWindow = 0
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

// SetBlockedUntil sets a time until which all requests should wait
// This is used when an API returns a rate limit response with Retry-After
func (rl *RateLimiter) SetBlockedUntil(until time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Only extend the block, never shorten it
	if until.After(rl.blockedUntil) {
		rl.blockedUntil = until
		log.Printf("Rate limiter blocked until %v", until)
	}
}

// WaitForRetryAfter blocks for the specified number of seconds
// This should be called when an API returns 429 or 503 with Retry-After header
func (rl *RateLimiter) WaitForRetryAfter(seconds int) {
	if seconds <= 0 {
		seconds = 60 // Default to 60 seconds if not specified
	}

	waitDuration := time.Duration(seconds) * time.Second
	until := time.Now().Add(waitDuration)

	rl.SetBlockedUntil(until)

	log.Printf("Rate limit hit - waiting %d seconds before retry", seconds)
	time.Sleep(waitDuration)

	// Reset the window after waiting
	rl.mu.Lock()
	rl.windowStart = time.Now()
	rl.requestsInWindow = 0
	rl.mu.Unlock()
}
