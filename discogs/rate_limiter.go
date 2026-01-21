package discogs

import (
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

type RateLimiter struct {
	sync.RWMutex
	windowStart   time.Time
	authRemaining int
	anonRemaining int
	lastAuthLimit int
	lastAnonLimit int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		windowStart:   time.Now(),
		authRemaining: AuthRequests,
		anonRemaining: AnonRequests,
		lastAuthLimit: AuthRequests,
		lastAnonLimit: AnonRequests,
	}
}

func (rl *RateLimiter) Wait(isAuth bool) {
	rl.Lock()
	defer rl.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.windowStart)

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
	}

	remaining := rl.authRemaining
	if !isAuth {
		remaining = rl.anonRemaining
	}

	remainingThreshold := 5
	for remaining <= remainingThreshold {
		now := time.Now()
		sleepTime := rl.windowStart.Add(RateLimitWindow).Sub(now)
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
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

		remaining = rl.authRemaining
		if !isAuth {
			remaining = rl.anonRemaining
		}
	}
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

func (rl *RateLimiter) WaitForReset(retryAfter int) {
	rl.Lock()
	defer rl.Unlock()

	sleepTime := time.Duration(retryAfter) * time.Second
	if sleepTime <= 0 {
		sleepTime = RateLimitWindow
	}
	time.Sleep(sleepTime)
	rl.windowStart = time.Now()
	rl.authRemaining = rl.lastAuthLimit
	rl.anonRemaining = rl.lastAnonLimit
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
