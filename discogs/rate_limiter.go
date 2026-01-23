package discogs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	RateLimitWindow = 60 * time.Second
	AuthRequests    = 60
	AnonRequests    = 25
)

// ErrRateLimited is returned when the API is rate limited
// The caller should handle this by pausing and waiting for the rate limit to clear
var ErrRateLimited = errors.New("rate limited by Discogs API")

// RateLimitError contains details about the rate limit
type RateLimitError struct {
	RetryAfter int
	ResetAt    time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited by Discogs API, retry after %d seconds", e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

var (
	globalRateLimiter     *RateLimiter
	globalRateLimiterOnce sync.Once
)

func GetGlobalRateLimiter() *RateLimiter {
	globalRateLimiterOnce.Do(func() {
		globalRateLimiter = NewRateLimiter()
	})
	return globalRateLimiter
}

type RateLimiter struct {
	sync.RWMutex
	windowStart              time.Time
	authRemaining            int
	anonRemaining            int
	lastAuthLimit            int
	lastAnonLimit            int
	rateLimitCallback        func(retryAfter int)
	rateLimitClearedCallback func()
	isRateLimited            bool
	rateLimitResetAt         time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		windowStart:   time.Now(),
		authRemaining: AuthRequests,
		anonRemaining: AnonRequests,
		lastAuthLimit: AuthRequests,
		lastAnonLimit: AnonRequests,
		isRateLimited: false,
	}
}

func (rl *RateLimiter) SetRateLimitCallback(callback func(retryAfter int)) {
	rl.Lock()
	rl.rateLimitCallback = callback
	rl.Unlock()
}

func (rl *RateLimiter) SetRateLimitClearedCallback(callback func()) {
	rl.Lock()
	rl.rateLimitClearedCallback = callback
	rl.Unlock()
}

func (rl *RateLimiter) IsRateLimited() bool {
	rl.RLock()
	defer rl.RUnlock()
	return rl.isRateLimited
}

func (rl *RateLimiter) ClearRateLimit() {
	rl.Lock()
	rl.isRateLimited = false
	rl.Unlock()
}

// Wait checks if there are enough remaining API requests before making a call.
// If remaining requests are below threshold, it returns a RateLimitError instead of blocking.
// The caller should handle this by pausing and waiting for the rate limit to clear.
func (rl *RateLimiter) Wait(isAuth bool) error {
	rl.Lock()
	defer rl.Unlock()

	// If already rate limited, return error immediately
	if rl.isRateLimited {
		secondsLeft := int(time.Until(rl.rateLimitResetAt).Seconds())
		if secondsLeft < 0 {
			secondsLeft = 0
		}
		logToFile("RATELIMIT: Wait() - already rate limited, returning error (seconds left: %d)", secondsLeft)
		return &RateLimitError{
			RetryAfter: secondsLeft,
			ResetAt:    rl.rateLimitResetAt,
		}
	}

	now := time.Now()
	elapsed := now.Sub(rl.windowStart)

	// Reset window if it has elapsed
	if elapsed >= RateLimitWindow {
		rl.windowStart = time.Now()
		if rl.lastAuthLimit > 0 {
			rl.authRemaining = rl.lastAuthLimit
		} else {
			rl.authRemaining = AuthRequests
		}
		if rl.lastAnonLimit > 0 {
			rl.anonRemaining = rl.lastAnonLimit
		} else {
			rl.anonRemaining = AnonRequests
		}
		logToFile("RATELIMIT: Wait() - window reset, auth_rem=%d, anon_rem=%d", rl.authRemaining, rl.anonRemaining)
	}

	remaining := rl.authRemaining
	if !isAuth {
		remaining = rl.anonRemaining
	}

	remainingThreshold := 2
	if remaining <= remainingThreshold {
		// Calculate time until window resets
		sleepTime := rl.windowStart.Add(RateLimitWindow).Sub(time.Now())
		if sleepTime < 0 {
			sleepTime = 0
		}
		retryAfter := int(sleepTime.Seconds())
		if retryAfter <= 0 {
			retryAfter = 60 // Default to 60 seconds if calculation gives 0
		}

		logToFile("RATELIMIT: Wait() - preemptive rate limit triggered (remaining=%d <= threshold=%d), setting state and returning error",
			remaining, remainingThreshold)

		// Set rate limit state (this also calls the callback)
		rl.isRateLimited = true
		rl.rateLimitResetAt = time.Now().Add(time.Duration(retryAfter) * time.Second)
		callback := rl.rateLimitCallback

		// Call callback outside of lock
		if callback != nil {
			// Release lock temporarily to call callback
			rl.Unlock()
			callback(retryAfter)
			rl.Lock()
		}

		// Start async countdown in a goroutine
		go rl.StartRateLimitCountdown(retryAfter)

		return &RateLimitError{
			RetryAfter: retryAfter,
			ResetAt:    rl.rateLimitResetAt,
		}
	}

	return nil
}

func (rl *RateLimiter) GetDebugInfo() string {
	rl.RLock()
	defer rl.RUnlock()
	return fmt.Sprintf("auth_rem=%d, anon_rem=%d, window_elapsed=%.2fs",
		rl.authRemaining, rl.anonRemaining, time.Since(rl.windowStart).Seconds())
}

func (rl *RateLimiter) Decrement(isAuth bool) {
	rl.Lock()
	defer rl.Unlock()

	if isAuth {
		rl.authRemaining--
	} else {
		rl.anonRemaining--
	}

	logToFile("RATELIMIT: DECREMENTED - auth_rem=%d, anon_rem=%d",
		rl.authRemaining, rl.anonRemaining)
}

func (rl *RateLimiter) UpdateFromHeaders(resp *http.Response) {
	rl.Lock()
	defer rl.Unlock()

	rlAuth := resp.Header.Get("X-Discogs-Ratelimit-Auth")
	rlAuthRem := resp.Header.Get("X-Discogs-Ratelimit-Auth-Remaining")
	rlAnon := resp.Header.Get("X-Discogs-Ratelimit")
	rlAnonRem := resp.Header.Get("X-Discogs-Ratelimit-Remaining")

	logToFile("RATELIMIT HEADERS: Auth=%s/%s, Anon=%s/%s",
		rlAuth, rlAuthRem, rlAnon, rlAnonRem)

	authLimitSet := false
	if rlAuth != "" && rlAuth != "/" {
		if limit, err := strconv.Atoi(rlAuth); err == nil {
			rl.lastAuthLimit = limit
			authLimitSet = true
			if rlAuthRem != "" {
				if rem, err := strconv.Atoi(rlAuthRem); err == nil {
					rl.authRemaining = rem
				}
			} else {
				rl.authRemaining = limit
			}
		}
	}

	if !authLimitSet && rl.lastAuthLimit == 0 {
		rl.lastAuthLimit = AuthRequests
		if rl.authRemaining == 0 {
			rl.authRemaining = AuthRequests
		}
	}

	if rlAnon != "" {
		if limit, err := strconv.Atoi(rlAnon); err == nil {
			rl.lastAnonLimit = limit
			if rlAnonRem != "" {
				if rem, err := strconv.Atoi(rlAnonRem); err == nil {
					rl.anonRemaining = rem
				} else {
					rl.anonRemaining = limit
				}
			} else {
				rl.anonRemaining = limit
			}
		}
	} else if rlAnonRem != "" && rl.lastAnonLimit == 0 {
		if rem, err := strconv.Atoi(rlAnonRem); err == nil {
			rl.anonRemaining = rem
		}
	}
}

// SetRateLimitState sets the rate limit state without blocking
// Returns a RateLimitError that the caller can use to handle the rate limit
func (rl *RateLimiter) SetRateLimitState(retryAfter int) *RateLimitError {
	sleepTime := time.Duration(retryAfter) * time.Second
	if sleepTime <= 0 {
		sleepTime = RateLimitWindow
	}

	resetAt := time.Now().Add(sleepTime)

	rl.Lock()
	rl.isRateLimited = true
	rl.rateLimitResetAt = resetAt
	callback := rl.rateLimitCallback
	rl.Unlock()

	if callback != nil {
		callback(retryAfter)
	}

	return &RateLimitError{
		RetryAfter: retryAfter,
		ResetAt:    resetAt,
	}
}

// StartRateLimitCountdown starts an async countdown that clears the rate limit when done
// This should be called from a goroutine to avoid blocking
func (rl *RateLimiter) StartRateLimitCountdown(retryAfter int) {
	sleepTime := time.Duration(retryAfter) * time.Second
	if sleepTime <= 0 {
		sleepTime = RateLimitWindow
	}

	endTime := time.Now().Add(sleepTime)

	// Use ticker-based waiting to allow periodic state checks
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(endTime) {
		<-ticker.C
		// Check if rate limit was manually cleared
		rl.RLock()
		stillLimited := rl.isRateLimited
		rl.RUnlock()
		if !stillLimited {
			logToFile("RATELIMIT: countdown cancelled - rate limit was manually cleared")
			return
		}
	}

	rl.Lock()
	rl.windowStart = time.Now()
	rl.authRemaining = rl.lastAuthLimit
	rl.anonRemaining = rl.lastAnonLimit
	rl.isRateLimited = false
	rl.rateLimitResetAt = time.Time{}
	clearedCallback := rl.rateLimitClearedCallback
	rl.Unlock()

	logToFile("RATELIMIT: countdown completed, calling cleared callback")
	if clearedCallback != nil {
		clearedCallback()
	}
}

// WaitForReset is DEPRECATED - use SetRateLimitState and StartRateLimitCountdown instead
// This is kept for backwards compatibility but now sets state and starts async countdown
func (rl *RateLimiter) WaitForReset(retryAfter int) {
	rl.SetRateLimitState(retryAfter)
	rl.StartRateLimitCountdown(retryAfter)
}

func (rl *RateLimiter) GetRateLimitResetAt() time.Time {
	rl.RLock()
	defer rl.RUnlock()
	return rl.rateLimitResetAt
}

func (rl *RateLimiter) GetSecondsUntilReset() int {
	rl.RLock()
	defer rl.RUnlock()
	if rl.rateLimitResetAt.IsZero() {
		return 0
	}
	seconds := int(time.Until(rl.rateLimitResetAt).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func (rl *RateLimiter) GetRemaining() int {
	rl.RLock()
	defer rl.RUnlock()
	return rl.authRemaining
}

func (rl *RateLimiter) GetRemainingAnon() int {
	rl.RLock()
	defer rl.RUnlock()
	return rl.anonRemaining
}
