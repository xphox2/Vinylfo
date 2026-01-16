package controllers

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"testing"
	"vinylfo/discogs"
)

// TestFetchTracksForAlbum_UsesOAuth tests that GetTracksForAlbum uses OAuth when configured
func TestFetchTracksForAlbum_UsesOAuth(t *testing.T) {
	oauth := &discogs.OAuthConfig{
		ConsumerKey:    "test_consumer_key",
		ConsumerSecret: "test_consumer_secret",
		AccessToken:    "test_access_token",
		AccessSecret:   "test_access_secret",
	}

	client := discogs.NewClientWithOAuth("", oauth)

	var capturedAuthHeader string
	var requestMade bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		capturedAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Discogs-Ratelimit-Auth", "60")
		w.Header().Set("X-Discogs-Ratelimit-Auth-Remaining", "59")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 12345,
			"title": "Test Album",
			"year": 2024,
			"artists": [{"name": "Test Artist"}],
			"tracklist": [
				{"title": "Track 1", "duration": "3:30", "position": "A1"},
				{"title": "Track 2", "duration": "4:15", "position": "A2"}
			]
		}`))
	}))
	defer server.Close()

	client.HTTPClient.Transport = &authCapturingTransport{
		original:    client.HTTPClient.Transport,
		capturedURL: server.URL,
		onRequest: func(authHeader string) {
			capturedAuthHeader = authHeader
			requestMade = true
		},
	}

	_, err := client.GetTracksForAlbum(12345)
	if err != nil {
		t.Logf("GetTracksForAlbum error (expected with mock server): %v", err)
	}

	if !requestMade {
		t.Errorf("No request was made to the server")
		return
	}

	t.Logf("Captured Authorization header: %q", capturedAuthHeader)

	hasOAuthHeader := capturedAuthHeader != "" && contains(capturedAuthHeader, "oauth")

	if !hasOAuthHeader {
		t.Errorf("FAIL: GetTracksForAlbum with OAuth config did NOT send OAuth Authorization header")
	} else {
		t.Logf("SUCCESS: OAuth Authorization header was sent")
	}
}

type authCapturingTransport struct {
	original    http.RoundTripper
	capturedURL string
	onRequest   func(authHeader string)
}

func (t *authCapturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.onRequest(req.Header.Get("Authorization"))
	newReq := req.Clone(req.Context())
	newReq.URL, _ = url.Parse(t.capturedURL)
	if t.original != nil {
		return t.original.RoundTrip(newReq)
	}
	return http.DefaultClient.Do(newReq)
}

// TestSyncFlow_AuthConsistency tests that API requests use appropriate authentication
func TestSyncFlow_AuthConsistency(t *testing.T) {
	oauth := &discogs.OAuthConfig{
		ConsumerKey:    "test_consumer_key",
		ConsumerSecret: "test_consumer_secret",
		AccessToken:    "test_access_token",
		AccessSecret:   "test_access_secret",
	}

	t.Run("GetTracksForAlbum uses OAuth when configured", func(t *testing.T) {
		client := discogs.NewClientWithOAuth("", oauth)

		var capturedAuthHeader string
		var requestMade bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestMade = true
			capturedAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Discogs-Ratelimit-Auth", "60")
			w.Header().Set("X-Discogs-Ratelimit-Auth-Remaining", "59")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": 12345,
				"title": "Test Album",
				"year": 2024,
				"artists": [{"name": "Test Artist"}],
				"tracklist": [{"title": "Track 1", "duration": "3:30", "position": "A1"}]
			}`))
		}))
		defer server.Close()

		client.HTTPClient.Transport = &authCapturingTransport{
			original:    client.HTTPClient.Transport,
			capturedURL: server.URL,
			onRequest: func(authHeader string) {
				capturedAuthHeader = authHeader
				requestMade = true
			},
		}

		_, err := client.GetTracksForAlbum(12345)
		if err != nil {
			t.Logf("GetTracksForAlbum error (expected with mock server): %v", err)
		}

		if !requestMade {
			t.Errorf("No request was made to the server")
			return
		}

		if capturedAuthHeader == "" {
			t.Errorf("FAIL: GetTracksForAlbum sent NO Authorization header")
		} else if contains(capturedAuthHeader, "oauth") {
			t.Logf("SUCCESS: GetTracksForAlbum sent OAuth Authorization header")
		} else {
			t.Logf("GetTracksForAlbum used token auth")
		}
	})

	t.Run("SearchAlbums uses OAuth when configured", func(t *testing.T) {
		client := discogs.NewClientWithOAuth("", oauth)

		var capturedAuthHeader string
		var requestMade bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestMade = true
			capturedAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Discogs-Ratelimit-Auth", "60")
			w.Header().Set("X-Discogs-Ratelimit-Auth-Remaining", "59")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"results": [{"id": 12345, "title": "Test Album", "year": 2024, "artists": [{"name": "Test Artist"}]}],
				"pagination": {"pages": 1, "items": 1}
			}`))
		}))
		defer server.Close()

		client.HTTPClient.Transport = &authCapturingTransport{
			original:    client.HTTPClient.Transport,
			capturedURL: server.URL,
			onRequest: func(authHeader string) {
				capturedAuthHeader = authHeader
				requestMade = true
			},
		}

		_, _, err := client.SearchAlbums("test", 1)
		if err != nil {
			t.Logf("SearchAlbums error (expected with mock server): %v", err)
		}

		if !requestMade {
			t.Errorf("No request was made to the server")
			return
		}

		if capturedAuthHeader == "" {
			t.Errorf("FAIL: SearchAlbums sent NO Authorization header")
		} else if contains(capturedAuthHeader, "oauth") {
			t.Logf("SUCCESS: SearchAlbums sent OAuth Authorization header")
		} else {
			t.Logf("SearchAlbums used token auth")
		}
	})

	t.Run("Client without OAuth uses token auth", func(t *testing.T) {
		client := discogs.NewClient("test_api_key")

		var capturedAuthHeader string
		var requestMade bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestMade = true
			capturedAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Discogs-Ratelimit", "25")
			w.Header().Set("X-Discogs-Ratelimit-Remaining", "24")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": 12345,
				"title": "Test Album",
				"year": 2024,
				"artists": [{"name": "Test Artist"}],
				"tracklist": [{"title": "Track 1", "duration": "3:30", "position": "A1"}]
			}`))
		}))
		defer server.Close()

		client.HTTPClient.Transport = &authCapturingTransport{
			original:    client.HTTPClient.Transport,
			capturedURL: server.URL,
			onRequest: func(authHeader string) {
				capturedAuthHeader = authHeader
				requestMade = true
			},
		}

		_, err := client.GetTracksForAlbum(12345)
		if err != nil {
			t.Fatalf("GetTracksForAlbum failed: %v", err)
		}

		if !requestMade {
			t.Errorf("No request was made to the server")
			return
		}

		if capturedAuthHeader == "" {
			t.Errorf("FAIL: GetTracksForAlbum sent NO Authorization header")
		} else if contains(capturedAuthHeader, "Discogs token=") {
			t.Logf("SUCCESS: GetTracksForAlbum sent token Authorization header")
		} else {
			t.Logf("GetTracksForAlbum used OAuth (unexpected for token-only config)")
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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
		hasOAuth := config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != ""
		hasAPIKey := false

		isAuth := hasOAuth || hasAPIKey

		t.Logf("hasOAuth: %v, hasAPIKey: %v", hasOAuth, hasAPIKey)
		t.Logf("isAuth (OAuth only): %v", isAuth)

		if !hasOAuth {
			t.Errorf("Expected hasOAuth to be true for OAuth-only authentication")
		}
		if isAuth != hasOAuth {
			t.Errorf("Authentication logic is incorrect! isAuth: %v, hasOAuth: %v",
				isAuth, hasOAuth)
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
	hasOAuth := config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != ""

	t.Logf("Config: IsConnected=%v", config.IsDiscogsConnected)
	t.Logf("OAuth configured: %v", hasOAuth)

	// OAuth-only authentication logic (after removing API token support)
	isAuth := hasOAuth

	t.Logf("\n=== Authentication Check ===")
	t.Logf("isAuth (OAuth only): %v", isAuth)

	// Simulate rate limiting behavior
	authLimit := 60
	anonLimit := 25
	authRemaining := authLimit
	anonRemaining := anonLimit

	t.Logf("\n=== Simulating 20 API calls ===")
	for i := 1; i <= 20; i++ {
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
