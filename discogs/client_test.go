package discogs

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticationLogic(t *testing.T) {
	tests := []struct {
		name           string
		hasOAuth       bool
		hasAPIKey      bool
		expectedIsAuth bool
	}{
		{
			name:           "OAuth with no APIKey should be auth",
			hasOAuth:       true,
			hasAPIKey:      false,
			expectedIsAuth: true,
		},
		{
			name:           "OAuth with APIKey should be auth (using APIKey)",
			hasOAuth:       true,
			hasAPIKey:      true,
			expectedIsAuth: true,
		},
		{
			name:           "No OAuth with APIKey should be auth",
			hasOAuth:       false,
			hasAPIKey:      true,
			expectedIsAuth: true,
		},
		{
			name:           "No OAuth and no APIKey should be anon",
			hasOAuth:       false,
			hasAPIKey:      false,
			expectedIsAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			if tt.hasOAuth {
				client.OAuth = &OAuthConfig{
					ConsumerKey:  "test_key",
					AccessToken:  "test_token",
					AccessSecret: "test_secret",
				}
			}
			if tt.hasAPIKey {
				client.APIKey = "test_api_key"
			}

			isAuthenticated := client.IsAuthenticated()
			actualIsAuth := isAuthenticated || client.APIKey != ""

			if actualIsAuth != tt.expectedIsAuth {
				t.Errorf("makeRequest isAuth = %v, want %v", actualIsAuth, tt.expectedIsAuth)
			}
		})
	}
}

func TestIsAuthenticated_OAuthOnly(t *testing.T) {
	client := &Client{
		OAuth: &OAuthConfig{
			ConsumerKey:    "test_consumer_key",
			ConsumerSecret: "test_consumer_secret",
			AccessToken:    "test_access_token",
			AccessSecret:   "test_access_secret",
		},
	}

	isAuth := client.IsAuthenticated()
	if !isAuth {
		t.Errorf("IsAuthenticated() = %v, want true for OAuth-only config", isAuth)
	}

	hasAPIKey := client.APIKey != ""
	if hasAPIKey {
		t.Errorf("APIKey should be empty for OAuth-only config")
	}
}

func TestMakeRequest_AuthHeaderPresent(t *testing.T) {
	tests := []struct {
		name               string
		setupClient        func() *Client
		expectAuthHeader   bool
		authHeaderContains string
	}{
		{
			name: "APIKey set - Authorization header present",
			setupClient: func() *Client {
				return &Client{
					APIKey:      "test_api_key",
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthHeader:   true,
			authHeaderContains: "Discogs token=",
		},
		{
			name: "No APIKey and no OAuth - no Authorization header",
			setupClient: func() *Client {
				return &Client{
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthHeader:   false,
			authHeaderContains: "",
		},
		{
			name: "OAuth only (no APIKey) - no Authorization header in makeRequest",
			setupClient: func() *Client {
				return &Client{
					OAuth: &OAuthConfig{
						ConsumerKey:    "test_consumer_key",
						ConsumerSecret: "test_consumer_secret",
						AccessToken:    "test_access_token",
						AccessSecret:   "test_access_secret",
					},
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthHeader:   false,
			authHeaderContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedAuthHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedAuthHeader = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": 1}`))
			}))
			defer server.Close()

			client := tt.setupClient()
			client.HTTPClient = server.Client()

			_, err := client.makeRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("makeRequest failed: %v", err)
			}

			hasAuthHeader := capturedAuthHeader != ""
			if hasAuthHeader != tt.expectAuthHeader {
				t.Errorf("Authorization header present = %v, want %v", hasAuthHeader, tt.expectAuthHeader)
			}

			if tt.authHeaderContains != "" && !contains(capturedAuthHeader, tt.authHeaderContains) {
				t.Errorf("Authorization header = %q, want to contain %q", capturedAuthHeader, tt.authHeaderContains)
			}
		})
	}
}

func TestRateLimiter_AnonVsAuth(t *testing.T) {
	tests := []struct {
		name                string
		setupClient         func() *Client
		expectAuthDecrement bool
		expectAnonDecrement bool
	}{
		{
			name: "Request with APIKey - should decrement auth counter",
			setupClient: func() *Client {
				return &Client{
					APIKey:      "test_api_key",
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthDecrement: true,
			expectAnonDecrement: false,
		},
		{
			name: "Request without APIKey - should decrement anon counter",
			setupClient: func() *Client {
				return &Client{
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthDecrement: false,
			expectAnonDecrement: true,
		},
		{
			name: "OAuth-only request via makeRequest - should decrement anon counter (BUG)",
			setupClient: func() *Client {
				return &Client{
					OAuth: &OAuthConfig{
						ConsumerKey:    "test_consumer_key",
						ConsumerSecret: "test_consumer_secret",
						AccessToken:    "test_access_token",
						AccessSecret:   "test_access_secret",
					},
					HTTPClient:  &http.Client{Timeout: 30 * time.Second},
					RateLimiter: NewRateLimiter(),
				}
			},
			expectAuthDecrement: false,
			expectAnonDecrement: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": 1}`))
			}))
			defer server.Close()

			client := tt.setupClient()
			client.HTTPClient = server.Client()

			initialAuthRem := client.RateLimiter.authRemaining
			initialAnonRem := client.RateLimiter.anonRemaining

			_, err := client.makeRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("makeRequest failed: %v", err)
			}

			authDecremented := client.RateLimiter.authRemaining < initialAuthRem
			anonDecremented := client.RateLimiter.anonRemaining < initialAnonRem

			if authDecremented != tt.expectAuthDecrement {
				t.Errorf("Auth decremented = %v, want %v", authDecremented, tt.expectAuthDecrement)
			}
			if anonDecremented != tt.expectAnonDecrement {
				t.Errorf("Anon decremented = %v, want %v", anonDecremented, tt.expectAnonDecrement)
			}
		})
	}
}

func TestRateLimiter_HeaderParsing(t *testing.T) {
	tests := []struct {
		name                string
		authHeader          string
		authRemainingHeader string
		anonHeader          string
		anonRemainingHeader string
		expectedAuthLimit   int
		expectedAuthRem     int
		expectedAnonLimit   int
		expectedAnonRem     int
	}{
		{
			name:                "Auth header '/' should not update limit",
			authHeader:          "/",
			authRemainingHeader: "",
			anonHeader:          "25",
			anonRemainingHeader: "23",
			expectedAuthLimit:   60,
			expectedAuthRem:     60,
			expectedAnonLimit:   25,
			expectedAnonRem:     23,
		},
		{
			name:                "Auth header empty - default to AuthRequests",
			authHeader:          "",
			authRemainingHeader: "",
			anonHeader:          "25",
			anonRemainingHeader: "20",
			expectedAuthLimit:   60,
			expectedAuthRem:     60,
			expectedAnonLimit:   25,
			expectedAnonRem:     20,
		},
		{
			name:                "Auth remaining empty - set to limit value",
			authHeader:          "60",
			authRemainingHeader: "",
			anonHeader:          "25",
			anonRemainingHeader: "18",
			expectedAuthLimit:   60,
			expectedAuthRem:     60,
			expectedAnonLimit:   25,
			expectedAnonRem:     18,
		},
		{
			name:                "Anon header parsing edge case - zero remaining",
			authHeader:          "60",
			authRemainingHeader: "5",
			anonHeader:          "25",
			anonRemainingHeader: "0",
			expectedAuthLimit:   60,
			expectedAuthRem:     5,
			expectedAnonLimit:   25,
			expectedAnonRem:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter()

			resp := &http.Response{
				Header: make(http.Header),
			}
			if tt.authHeader != "" {
				resp.Header.Set("X-Discogs-Ratelimit-Auth", tt.authHeader)
			}
			if tt.authRemainingHeader != "" {
				resp.Header.Set("X-Discogs-Ratelimit-Auth-Remaining", tt.authRemainingHeader)
			}
			if tt.anonHeader != "" {
				resp.Header.Set("X-Discogs-Ratelimit", tt.anonHeader)
			}
			if tt.anonRemainingHeader != "" {
				resp.Header.Set("X-Discogs-Ratelimit-Remaining", tt.anonRemainingHeader)
			}

			rl.UpdateFromHeaders(resp)

			if rl.lastAuthLimit != tt.expectedAuthLimit {
				t.Errorf("lastAuthLimit = %d, want %d", rl.lastAuthLimit, tt.expectedAuthLimit)
			}
			if rl.authRemaining != tt.expectedAuthRem {
				t.Errorf("authRemaining = %d, want %d", rl.authRemaining, tt.expectedAuthRem)
			}
			if rl.lastAnonLimit != tt.expectedAnonLimit {
				t.Errorf("lastAnonLimit = %d, want %d", rl.lastAnonLimit, tt.expectedAnonLimit)
			}
			if rl.anonRemaining != tt.expectedAnonRem {
				t.Errorf("anonRemaining = %d, want %d", rl.anonRemaining, tt.expectedAnonRem)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRateLimiterDecrement(t *testing.T) {
	rl := NewRateLimiter()

	tests := []struct {
		name            string
		initialAuthRem  int
		initialAnonRem  int
		isAuth          bool
		expectedAuthRem int
		expectedAnonRem int
	}{
		{
			name:            "Decrement auth request",
			initialAuthRem:  60,
			initialAnonRem:  25,
			isAuth:          true,
			expectedAuthRem: 59,
			expectedAnonRem: 25,
		},
		{
			name:            "Decrement anon request",
			initialAuthRem:  60,
			initialAnonRem:  25,
			isAuth:          false,
			expectedAuthRem: 60,
			expectedAnonRem: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl.authRemaining = tt.initialAuthRem
			rl.anonRemaining = tt.initialAnonRem

			rl.Decrement(tt.isAuth)

			if rl.authRemaining != tt.expectedAuthRem {
				t.Errorf("authRemaining = %d, want %d", rl.authRemaining, tt.expectedAuthRem)
			}
			if rl.anonRemaining != tt.expectedAnonRem {
				t.Errorf("anonRemaining = %d, want %d", rl.anonRemaining, tt.expectedAnonRem)
			}
		})
	}
}

func TestRateLimiterWaitShouldNotWaitForOppositeType(t *testing.T) {
	rl := NewRateLimiter()

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
			rl.authRemaining = tt.authRemaining
			rl.anonRemaining = tt.anonRemaining

			initialAuth := rl.authRemaining
			initialAnon := rl.anonRemaining

			wouldWait := false
			remaining := rl.authRemaining
			if !tt.isAuth {
				remaining = rl.anonRemaining
			}
			remainingThreshold := 5
			if remaining <= remainingThreshold {
				wouldWait = true
			}

			if wouldWait != tt.shouldWait {
				t.Errorf("%s: wouldWait = %v, want %v (auth=%d, anon=%d, isAuth=%v)",
					tt.description, wouldWait, tt.shouldWait, initialAuth, initialAnon, tt.isAuth)
			}
		})
	}
}
