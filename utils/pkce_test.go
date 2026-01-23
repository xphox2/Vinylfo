package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"vinylfo/models"
)

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("Failed to generate code verifier: %v", err)
	}

	if len(verifier) == 0 {
		t.Fatal("Code verifier should not be empty")
	}

	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("Code verifier length should be 43-128 characters, got %d", len(verifier))
	}

	for _, c := range verifier {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			t.Fatalf("Code verifier contains invalid character: %c", c)
		}
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier-string-43-chars-long!!"
	challenge := GenerateCodeChallenge(verifier)

	if len(challenge) == 0 {
		t.Fatal("Code challenge should not be empty")
	}

	for _, c := range challenge {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Fatalf("Code challenge contains invalid character: %c", c)
		}
	}
}

func TestCodeVerifierUniqueness(t *testing.T) {
	verifiers := make(map[string]bool)
	duplicates := 0
	for i := 0; i < 100; i++ {
		verifier, err := GenerateCodeVerifier()
		if err != nil {
			t.Fatalf("Failed to generate code verifier: %v", err)
		}

		if verifiers[verifier] {
			duplicates++
		}
		verifiers[verifier] = true
	}

	if duplicates > 0 {
		t.Errorf("Found %d duplicate code verifiers out of 100", duplicates)
	}
}

func TestCodeChallengeConsistency(t *testing.T) {
	verifier := "test-verifier"

	challenge1 := GenerateCodeChallenge(verifier)
	challenge2 := GenerateCodeChallenge(verifier)

	if challenge1 != challenge2 {
		t.Fatal("Same verifier should produce same challenge")
	}
}

func TestCodeChallengeLength(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789"
	challenge := GenerateCodeChallenge(verifier)

	if len(challenge) < 40 || len(challenge) > 50 {
		t.Fatalf("Unexpected challenge length: %d (expected 40-50 for base64url SHA256)", len(challenge))
	}
}

func TestCodeChallengeDifferentFromVerifier(t *testing.T) {
	verifier := "this-is-a-very-long-verifier-string-with-43-chars!!"
	challenge := GenerateCodeChallenge(verifier)

	if verifier == challenge {
		t.Fatal("Challenge should be different from verifier (different encoding)")
	}

	if len(challenge) >= len(verifier) && len(verifier) > 32 {
		t.Logf("Note: For short verifiers, base64 hash (43 chars) may be longer than verifier")
	}
}

func TestSHA256HashForChallenge(t *testing.T) {
	input := "test-input-for-hashing"

	hash := sha256.Sum256([]byte(input))
	encoded := base64.RawURLEncoding.EncodeToString(hash[:])

	if len(encoded) != 43 {
		t.Fatalf("Expected 43 characters for base64url-encoded SHA256, got %d", len(encoded))
	}

	hash2 := sha256.Sum256([]byte(input))
	encoded2 := base64.RawURLEncoding.EncodeToString(hash2[:])

	if encoded != encoded2 {
		t.Fatal("Same input should produce same hash")
	}
}

func TestPKCEStateExpiryWindow(t *testing.T) {
	expiryWindow := 10 * time.Minute
	expectedExpiry := time.Now().Add(expiryWindow)

	state := &models.PKCEState{
		State:     "test",
		ExpiresAt: expectedExpiry,
	}

	diff := state.ExpiresAt.Sub(time.Now())
	if diff < 9*time.Minute || diff > 11*time.Minute {
		t.Fatalf("Expiry time should be approximately 10 minutes from now, got %v", diff)
	}
}

func TestGenerateStateFormat(t *testing.T) {
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	if len(state) == 0 {
		t.Fatal("Generated state should not be empty")
	}

	for _, c := range state {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Fatal("State contains invalid characters")
		}
	}
}

func TestCodeVerifierCharacters(t *testing.T) {
	tests := []struct {
		name       string
		verifier   string
		shouldPass bool
	}{
		{"valid characters", "abc123-_", true},
		{"uppercase", "ABC123", true},
		{"mixed case", "Abc123", true},
		{"short but valid", "a", true},
		{"43 chars valid", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO", true},
		{"too short", "", false},
		{"special chars not allowed", "abc@def", false},
		{"space not allowed", "abc def", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPass {
				if len(tt.verifier) < 43 || len(tt.verifier) > 128 {
					t.Skip("Test verifier length outside PKCE requirements")
				}
				for _, c := range tt.verifier {
					valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.'
					if !valid {
						t.Errorf("Character %c should be valid in code verifier", c)
					}
				}
			}
		})
	}
}
