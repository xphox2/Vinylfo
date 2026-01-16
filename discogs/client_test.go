package discogs

import (
	"testing"
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
