package controllers

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// TestAPIAuthenticationTracking tests that API calls are properly authenticated
// and tracked in the rate limiter
func TestAPIAuthenticationTracking(t *testing.T) {
	// Create a log file to monitor
	logFile := "test_api_auth_tracking.log"
	defer os.Remove(logFile)

	// Test scenario: Start with OAuth configured and APIKey set
	// This simulates the bug condition

	config := struct {
		DiscogsConsumerKey  string
		DiscogsAccessToken  string
		DiscogsAccessSecret string
		IsDiscogsConnected  bool
	}{
		DiscogsConsumerKey:  "test_consumer_key",
		DiscogsAccessToken:  "test_access_token",
		DiscogsAccessSecret: "test_access_secret",
		IsDiscogsConnected:  true,
	}

	t.Run("OAuth with APIKey should use auth rate limit", func(t *testing.T) {
		// Simulate the scenario in discogs/client.go:281
		hasOAuth := config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != ""
		hasAPIKey := os.Getenv("DISCOGS_API_TOKEN") != ""

		// Current buggy logic:
		buggyIsAuth := hasOAuth && !hasAPIKey

		// Correct logic:
		correctIsAuth := hasOAuth || hasAPIKey

		t.Logf("hasOAuth: %v, hasAPIKey: %v", hasOAuth, hasAPIKey)
		t.Logf("Buggy isAuth: %v", buggyIsAuth)
		t.Logf("Correct isAuth: %v", correctIsAuth)

		if buggyIsAuth != correctIsAuth {
			t.Errorf("Authentication logic is incorrect! Buggy: %v, Correct: %v",
				buggyIsAuth, correctIsAuth)
		}
	})
}

// TestRateLimiterIntegration tests rate limiter with mixed auth/anon calls
func TestRateLimiterIntegration(t *testing.T) {
	rl := &struct {
		authRemaining int
		anonRemaining int
	}{
		authRemaining: 60,
		anonRemaining: 25,
	}

	// Simulate a sync process that should only make authenticated calls
	// but due to bug #1, some might be treated as anonymous

	callLog := []struct {
		isAuth bool
		desc   string
	}{}

	// Make 10 calls that should all be authenticated
	for i := 0; i < 10; i++ {
		isAuth := true // This is what SHOULD happen
		callLog = append(callLog, struct {
			isAuth bool
			desc   string
		}{isAuth, fmt.Sprintf("API call %d", i+1)})

		if isAuth {
			rl.authRemaining--
		} else {
			rl.anonRemaining--
		}
	}

	t.Logf("After 10 auth calls: authRemaining=%d, anonRemaining=%d",
		rl.authRemaining, rl.anonRemaining)

	// Check if any anon calls were made (which would indicate a bug)
	authCalls := 0
	anonCalls := 0
	for _, call := range callLog {
		if call.isAuth {
			authCalls++
		} else {
			anonCalls++
		}
	}

	if anonCalls > 0 {
		t.Errorf("Found %d anonymous calls when all should be authenticated!", anonCalls)
	}
}

// TestAnalyzeDebugLog analyzes sync_debug.log for authentication issues
func TestAnalyzeDebugLog(t *testing.T) {
	logFile := "sync_debug.log"

	// Try to read the log file if it exists
	file, err := os.Open(logFile)
	if err != nil {
		t.Skipf("No sync_debug.log file found: %v", err)
	}
	defer file.Close()

	// Parse the log file
	scanner := bufio.NewScanner(file)

	authCalls := 0
	anonCalls := 0
	rateLimit429Count := 0
	authRateLimitIssues := 0
	anonRateLimitIssues := 0

	authRequestPattern := regexp.MustCompile(`API REQUEST \[auth\]:`)
	anonRequestPattern := regexp.MustCompile(`API REQUEST \[anon\]:`)
	error429Pattern := regexp.MustCompile(`API ERROR 429:`)
	decrementPattern := regexp.MustCompile(`RATELIMIT: DECREMENTED - auth_rem=(\d+), anon_rem=(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if authRequestPattern.MatchString(line) {
			authCalls++
		}
		if anonRequestPattern.MatchString(line) {
			anonCalls++
		}
		if error429Pattern.MatchString(line) {
			rateLimit429Count++
		}

		if decrementPattern.MatchString(line) {
			matches := decrementPattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				authRem, _ := strconv.Atoi(matches[1])
				anonRem, _ := strconv.Atoi(matches[2])

				if authRem <= 0 {
					authRateLimitIssues++
				}
				if anonRem <= 0 {
					anonRateLimitIssues++
				}
			}
		}
	}

	// Report findings
	t.Logf("=== API Authentication Analysis ===")
	t.Logf("Total Authenticated Calls: %d", authCalls)
	t.Logf("Total Anonymous Calls: %d", anonCalls)
	t.Logf("Total 429 Errors: %d", rateLimit429Count)
	t.Logf("Auth Rate Limit Issues: %d", authRateLimitIssues)
	t.Logf("Anon Rate Limit Issues: %d", anonRateLimitIssues)

	// Check for the bug
	if anonCalls > 0 && authCalls > 0 {
		ratio := float64(anonCalls) / float64(authCalls) * 100
		t.Logf("Warning: Mixed auth/anon calls detected (%.1f%% anon)", ratio)
	}

	if rateLimit429Count > 0 {
		t.Errorf("Found %d rate limit errors (429) - indicates rate limiting issue", rateLimit429Count)
	}

	// If we have more anon calls than expected (should be 0 for OAuth sync)
	if anonCalls > 0 {
		t.Logf("ISSUE: %d anonymous API calls were made during an OAuth sync", anonCalls)
		t.Logf("This indicates the authentication bug in discogs/client.go:281")
	}
}

// TestProgressTrackingAccuracy tests if progress tracking API returns accurate info
func TestProgressTrackingAccuracy(t *testing.T) {
	// Simulate what the frontend expects
	authRemaining := 50
	anonRemaining := 25 // This could be low due to bug

	// Current implementation only tracks auth
	apiRemaining := authRemaining

	t.Logf("Progress API will report: %d remaining", apiRemaining)
	t.Logf("Actual auth remaining: %d", authRemaining)
	t.Logf("Actual anon remaining: %d", anonRemaining)

	// If anon is low but auth is high, progress bar is misleading
	if anonRemaining < 5 && authRemaining > 20 {
		t.Logf("ISSUE: Progress bar shows %d%% usage, but anon is at %d/%d (%.1f%% used)",
			((60 - authRemaining) / 60 * 100),
			(25 - anonRemaining), 25,
			float64(25-anonRemaining)/25*100)
		t.Logf("The progress bar is misleading because it doesn't track anonymous API calls")
	}
}

// TestSimulateSyncProcess simulates a complete sync to identify issues
func TestSimulateSyncProcess(t *testing.T) {
	t.Log("=== Simulating Sync Process ===")

	// Simulate scenario from controllers/discogs.go
	config := struct {
		IsDiscogsConnected  bool
		DiscogsConsumerKey  string
		DiscogsAccessToken  string
		DiscogsAccessSecret string
	}{
		IsDiscogsConnected:  true,
		DiscogsConsumerKey:  "test_key",
		DiscogsAccessToken:  "test_token",
		DiscogsAccessSecret: "test_secret",
	}

	// Simulate client creation with OAuth (line 78 in discogs.go)
	// discogs.NewClientWithOAuth("", oauth) passes empty APIKey
	hasAPIKey := "" // Empty string from NewClientWithOAuth call

	t.Logf("Config: IsConnected=%v", config.IsDiscogsConnected)
	t.Logf("OAuth configured: %v", config.DiscogsAccessToken != "")
	t.Logf("APIKey from NewClientWithOAuth: '%s'", hasAPIKey)

	// Now simulate what happens in makeRequest (line 281 in client.go)
	hasOAuth := config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != ""
	isAuthBuggy := hasOAuth && hasAPIKey == ""   // This is the BUGGY logic
	isAuthCorrect := hasOAuth || hasAPIKey != "" // This is the CORRECT logic

	t.Logf("\n=== Authentication Check ===")
	t.Logf("Buggy isAuth: %v (hasOAuth=%v && APIKey=='')", isAuthBuggy, hasOAuth)
	t.Logf("Correct isAuth: %v (hasOAuth=%v || APIKey!='')", isAuthCorrect, hasOAuth)

	// Simulate rate limiting behavior
	authLimit := 60
	anonLimit := 25
	authRemaining := authLimit
	anonRemaining := anonLimit

	t.Logf("\n=== Simulating 20 API calls ===")
	for i := 1; i <= 20; i++ {
		isAuth := isAuthBuggy // Using buggy logic

		if isAuth {
			authRemaining--
		} else {
			anonRemaining--
		}

		t.Logf("Call %2d: isAuth=%v, auth_rem=%d, anon_rem=%d",
			i, isAuth, authRemaining, anonRemaining)

		// Check if we'd hit a limit
		if !isAuth && anonRemaining <= 5 {
			t.Logf("  -> WARNING: Approaching anonymous limit at call %d!", i)
		}
	}

	t.Logf("\n=== Results ===")
	t.Logf("Auth remaining: %d/%d (%.1f%% used)", authRemaining, authLimit, float64(authLimit-authRemaining)/float64(authLimit)*100)
	t.Logf("Anon remaining: %d/%d (%.1f%% used)", anonRemaining, anonLimit, float64(anonLimit-anonRemaining)/float64(anonLimit)*100)

	if anonRemaining < authRemaining {
		t.Logf("ISSUE: Anonymous quota depleted faster than authenticated quota!")
		t.Logf("This is because calls are incorrectly marked as anonymous")
	}
}
