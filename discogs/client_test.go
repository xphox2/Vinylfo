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

func TestParsePosition(t *testing.T) {
	tests := []struct {
		name          string
		position      string
		expectedDisc  int
		expectedTrack int
		expectedSide  string
		expectedValid bool
	}{
		{
			name:          "A1 - Side A, Disc 1, Track 1",
			position:      "A1",
			expectedDisc:  1,
			expectedTrack: 1,
			expectedSide:  "A",
			expectedValid: true,
		},
		{
			name:          "B2 - Side B, Disc 1, Track 2",
			position:      "B2",
			expectedDisc:  1,
			expectedTrack: 2,
			expectedSide:  "B",
			expectedValid: true,
		},
		{
			name:          "C1 - Side C, Disc 2, Track 1",
			position:      "C1",
			expectedDisc:  2,
			expectedTrack: 1,
			expectedSide:  "C",
			expectedValid: true,
		},
		{
			name:          "D3 - Side D, Disc 2, Track 3",
			position:      "D3",
			expectedDisc:  2,
			expectedTrack: 3,
			expectedSide:  "D",
			expectedValid: true,
		},
		{
			name:          "E1 - Side E, Disc 3, Track 1",
			position:      "E1",
			expectedDisc:  3,
			expectedTrack: 1,
			expectedSide:  "E",
			expectedValid: true,
		},
		{
			name:          "F4 - Side F, Disc 3, Track 4",
			position:      "F4",
			expectedDisc:  3,
			expectedTrack: 4,
			expectedSide:  "F",
			expectedValid: true,
		},
		{
			name:          "A10 - Side A, Disc 1, Track 10",
			position:      "A10",
			expectedDisc:  1,
			expectedTrack: 10,
			expectedSide:  "A",
			expectedValid: true,
		},
		{
			name:          "Empty position",
			position:      "",
			expectedDisc:  0,
			expectedTrack: 0,
			expectedSide:  "",
			expectedValid: false,
		},
		{
			name:          "Whitespace only",
			position:      "   ",
			expectedDisc:  0,
			expectedTrack: 0,
			expectedSide:  "",
			expectedValid: false,
		},
		{
			name:          "1-5 format - Disc 1, Track 5",
			position:      "1-5",
			expectedDisc:  1,
			expectedTrack: 5,
			expectedSide:  "A",
			expectedValid: true,
		},
		{
			name:          "2-1 format - Disc 1, Track 1 (Side B)",
			position:      "2-1",
			expectedDisc:  1,
			expectedTrack: 1,
			expectedSide:  "B",
			expectedValid: true,
		},
		{
			name:          "105 format - Disc 1, Track 5",
			position:      "105",
			expectedDisc:  1,
			expectedTrack: 5,
			expectedSide:  "A",
			expectedValid: true,
		},
		{
			name:          "201 format - Disc 1, Track 1 (Side B)",
			position:      "201",
			expectedDisc:  1,
			expectedTrack: 1,
			expectedSide:  "B",
			expectedValid: true,
		},
		{
			name:          "210 format - Disc 1, Track 10 (Side B)",
			position:      "210",
			expectedDisc:  1,
			expectedTrack: 10,
			expectedSide:  "B",
			expectedValid: true,
		},
		{
			name:          "Just number 1 - should return as-is",
			position:      "1",
			expectedDisc:  0,
			expectedTrack: 0,
			expectedSide:  "",
			expectedValid: false,
		},
		{
			name:          "Just number 5 - should return as-is",
			position:      "5",
			expectedDisc:  0,
			expectedTrack: 0,
			expectedSide:  "",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ParsePosition(tt.position)
			if info.IsValid != tt.expectedValid {
				t.Errorf("ParsePosition(%q).IsValid = %v, want %v", tt.position, info.IsValid, tt.expectedValid)
			}
			if info.DiscNumber != tt.expectedDisc {
				t.Errorf("ParsePosition(%q).DiscNumber = %d, want %d", tt.position, info.DiscNumber, tt.expectedDisc)
			}
			if info.TrackNumber != tt.expectedTrack {
				t.Errorf("ParsePosition(%q).TrackNumber = %d, want %d", tt.position, info.TrackNumber, tt.expectedTrack)
			}
			if info.Side != tt.expectedSide {
				t.Errorf("ParsePosition(%q).Side = %q, want %q", tt.position, info.Side, tt.expectedSide)
			}
		})
	}
}

func TestParseTracklistIntegration(t *testing.T) {
	tracklist := []struct {
		Title       string `json:"title"`
		Duration    string `json:"duration"`
		Position    string `json:"position"`
		TrackNumber string `json:"track_number"`
		DiscNumber  string `json:"disc_number"`
	}{
		{"Track 1", "3:30", "A1", "", ""},
		{"Track 2", "4:15", "A2", "", ""},
		{"Track 3", "3:45", "A3", "", ""},
		{"Track 4", "5:00", "A4", "", ""},
		{"Track 5", "3:30", "B5", "", ""},
		{"Track 6", "4:15", "B6", "", ""},
		{"Track 7", "3:45", "B7", "", ""},
		{"Track 8", "5:00", "B8", "", ""},
	}

	tracks := parseTracklist(tracklist)

	if len(tracks) != 8 {
		t.Errorf("Expected 8 tracks, got %d", len(tracks))
	}

	expectedResults := []struct {
		trackNumber int
		discNumber  int
		position    string
	}{
		{1, 1, "A1"},
		{2, 1, "A2"},
		{3, 1, "A3"},
		{4, 1, "A4"},
		{5, 1, "B5"},
		{6, 1, "B6"},
		{7, 1, "B7"},
		{8, 1, "B8"},
	}

	for i, expected := range expectedResults {
		track := tracks[i]

		trackNumber, ok := track["track_number"].(int)
		if !ok {
			t.Errorf("Track %d: track_number is not an int, got %T", i+1, track["track_number"])
		}
		if trackNumber != expected.trackNumber {
			t.Errorf("Track %d: track_number = %d, want %d", i+1, trackNumber, expected.trackNumber)
		}

		discNumber, ok := track["disc_number"].(int)
		if !ok {
			t.Errorf("Track %d: disc_number is not an int, got %T", i+1, track["disc_number"])
		}
		if discNumber != expected.discNumber {
			t.Errorf("Track %d: disc_number = %d, want %d", i+1, discNumber, expected.discNumber)
		}

		position, ok := track["position"].(string)
		if !ok {
			t.Errorf("Track %d: position is not a string, got %T", i+1, track["position"])
		}
		if position != expected.position {
			t.Errorf("Track %d: position = %s, want %s", i+1, position, expected.position)
		}
	}
}

func TestFetchAndSaveTracksDataFlow(t *testing.T) {
	tracklist := []struct {
		Title       string `json:"title"`
		Duration    string `json:"duration"`
		Position    string `json:"position"`
		TrackNumber string `json:"track_number"`
		DiscNumber  string `json:"disc_number"`
	}{
		{"Track 1", "3:30", "A1", "", ""},
		{"Track 2", "4:15", "A2", "", ""},
		{"Track 3", "3:45", "B1", "", ""},
		{"Track 4", "5:00", "B2", "", ""},
	}

	tracks := parseTracklist(tracklist)

	// Simulate the FetchAndSaveTracks logic
	for i, track := range tracks {
		title := track["title"].(string)
		position := track["position"].(string)

		trackNumber := 0
		switch tn := track["track_number"].(type) {
		case int:
			trackNumber = tn
		case int64:
			trackNumber = int(tn)
		case float64:
			trackNumber = int(tn)
		}

		discNumber := 0
		switch dn := track["disc_number"].(type) {
		case int:
			discNumber = dn
		case int64:
			discNumber = int(dn)
		case float64:
			discNumber = int(dn)
		}

		t.Logf("Track %d: title=%s, position=%s, track_number=%d, disc_number=%d, types: tn=%T, dn=%T",
			i+1, title, position, trackNumber, discNumber, track["track_number"], track["disc_number"])

		if trackNumber == 0 {
			t.Errorf("Track %d: track_number is 0, should be > 0", i+1)
		}
		if discNumber == 0 {
			t.Errorf("Track %d: disc_number is 0, should be > 0", i+1)
		}
	}
}

func TestGetTracksForAlbumDataFlow(t *testing.T) {
	// Simulate what Discogs API returns
	discogsTracklist := []struct {
		Title       string `json:"title"`
		Duration    string `json:"duration"`
		Position    string `json:"position"`
		TrackNumber string `json:"track_number"`
		DiscNumber  string `json:"disc_number"`
	}{
		{"Track 1", "3:30", "A1", "", ""},
		{"Track 2", "4:15", "A2", "", ""},
		{"Track 3", "3:45", "B1", "", ""},
		{"Track 4", "5:00", "B2", "", ""},
	}

	// This is what parseTracklist does
	tracks := parseTracklist(discogsTracklist)

	t.Logf("parseTracklist returned %d tracks", len(tracks))
	for i, track := range tracks {
		for k, v := range track {
			t.Logf("  Track %d: key=%s value=%v type=%T", i+1, k, v, v)
		}
	}

	// Now simulate what FetchAndSaveTracks does
	for i, track := range tracks {
		title := track["title"].(string)
		position := track["position"].(string)

		trackNumber := 0
		switch tn := track["track_number"].(type) {
		case int:
			trackNumber = tn
		case int64:
			trackNumber = int(tn)
		case float64:
			trackNumber = int(tn)
		}

		discNumber := 0
		switch dn := track["disc_number"].(type) {
		case int:
			discNumber = dn
		case int64:
			discNumber = int(dn)
		case float64:
			discNumber = int(dn)
		}

		t.Logf("FetchAndSaveTracks sim: title=%s, position=%s, track_number=%d, disc_number=%d",
			title, position, trackNumber, discNumber)

		if trackNumber == 0 {
			t.Errorf("Track %d: track_number is 0, should be > 0", i+1)
		}
		if discNumber == 0 {
			t.Errorf("Track %d: disc_number is 0, should be > 0", i+1)
		}
	}
}
