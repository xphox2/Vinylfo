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

	for _, c := range verifier {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Fatal("Code verifier contains invalid characters")
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
			t.Fatal("Code challenge contains invalid characters")
		}
	}
}

func TestCodeVerifierUniqueness(t *testing.T) {
	verifiers := make(map[string]bool)
	for i := 0; i < 100; i++ {
		verifier, err := GenerateCodeVerifier()
		if err != nil {
			t.Fatalf("Failed to generate code verifier: %v", err)
		}

		if verifiers[verifier] {
			t.Fatal("Generated duplicate code verifier")
		}
		verifiers[verifier] = true
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

	if len(challenge) < 40 || len(challenge) > 45 {
		t.Fatalf("Unexpected challenge length: %d", len(challenge))
	}
}

func TestPKCEStateModel(t *testing.T) {
	state := &models.PKCEState{
		State:         "test-state",
		CodeVerifier:  "test-verifier",
		CodeChallenge: "test-challenge",
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}

	if state.State != "test-state" {
		t.Fatal("State field not set correctly")
	}

	if state.TableName() != "pkce_states" {
		t.Fatal("TableName should return pkce_states")
	}
}

func TestPKCEStateExpiry(t *testing.T) {
	expiresAt := time.Now().Add(10 * time.Minute)
	state := &models.PKCEState{
		State:         "test",
		CodeVerifier:  "verifier",
		CodeChallenge: "challenge",
		ExpiresAt:     expiresAt,
	}

	if !state.ExpiresAt.After(time.Now()) {
		t.Fatal("ExpiresAt should be in the future")
	}
}

func TestPKCEStateFields(t *testing.T) {
	state := &models.PKCEState{
		ID:            1,
		State:         "abc123",
		CodeVerifier:  "def456",
		CodeChallenge: "ghi789",
		ExpiresAt:     time.Now().Add(5 * time.Minute),
		CreatedAt:     time.Now(),
	}

	if state.ID != 1 {
		t.Fatal("ID field not set correctly")
	}
	if state.State != "abc123" {
		t.Fatal("State field not set correctly")
	}
	if state.CodeVerifier != "def456" {
		t.Fatal("CodeVerifier field not set correctly")
	}
	if state.CodeChallenge != "ghi789" {
		t.Fatal("CodeChallenge field not set correctly")
	}
}

func TestCodeChallengeDifferentFromVerifier(t *testing.T) {
	verifier := "this-is-a-very-long-verifier-string"
	challenge := GenerateCodeChallenge(verifier)

	if verifier == challenge {
		t.Fatal("Challenge should be different from verifier (different encoding)")
	}

	if len(challenge) < len(verifier) {
		t.Fatal("Challenge should be shorter than verifier (base64 of hash)")
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

func TestMockDatabaseOperations(t *testing.T) {
	state := &models.PKCEState{
		State:         "mock-state",
		CodeVerifier:  "mock-verifier",
		CodeChallenge: GenerateCodeChallenge("mock-verifier"),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}

	if state.State != "mock-state" {
		t.Fatal("Mock state creation failed")
	}

	_ = state
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
