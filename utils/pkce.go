package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"vinylfo/models"

	"gorm.io/gorm"
)

type PKCEStateRecord struct {
	State         string    `json:"state"`
	CodeVerifier  string    `json:"code_verifier"`
	CodeChallenge string    `json:"code_challenge"`
	ExpiresAt     time.Time `json:"expires_at"`
}

const (
	pkceStateExpiry = 10 * time.Minute
)

var db *gorm.DB

func InitPKCE(dbInstance *gorm.DB) {
	db = dbInstance
}

func GenerateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func CreatePKCEState() (state, codeVerifier, codeChallenge string, err error) {
	if db == nil {
		return "", "", "", fmt.Errorf("database not initialized for PKCE")
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate state: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(stateBytes)

	codeVerifier, err = GenerateCodeVerifier()
	if err != nil {
		return "", "", "", err
	}

	codeChallenge = GenerateCodeChallenge(codeVerifier)

	expiresAt := time.Now().Add(pkceStateExpiry)

	pkceState := &models.PKCEState{
		State:         state,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		ExpiresAt:     expiresAt,
	}

	if err := db.Create(pkceState).Error; err != nil {
		return "", "", "", fmt.Errorf("failed to save PKCE state: %w", err)
	}

	return state, codeVerifier, codeChallenge, nil
}

func ValidatePKCEState(state, codeVerifier string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database not initialized for PKCE")
	}

	var pkceState models.PKCEState
	if err := db.Where("state = ?", state).First(&pkceState).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, fmt.Errorf("state not found or expired")
		}
		return false, fmt.Errorf("failed to lookup PKCE state: %w", err)
	}

	if time.Now().After(pkceState.ExpiresAt) {
		db.Delete(&pkceState)
		return false, fmt.Errorf("state has expired")
	}

	expectedChallenge := pkceState.CodeChallenge
	actualChallenge := GenerateCodeChallenge(codeVerifier)

	if expectedChallenge != actualChallenge {
		return false, fmt.Errorf("code verifier does not match code challenge")
	}

	db.Delete(&pkceState)
	return true, nil
}

func CleanupExpiredPKCEStates() error {
	if db == nil {
		return fmt.Errorf("database not initialized for PKCE")
	}

	result := db.Where("expires_at < ?", time.Now()).Delete(&models.PKCEState{})
	return result.Error
}

func GetPKCEStateCount() (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized for PKCE")
	}

	var count int64
	err := db.Model(&models.PKCEState{}).Count(&count).Error
	return count, err
}

func ClearAllPKCEStates() error {
	if db == nil {
		return fmt.Errorf("database not initialized for PKCE")
	}

	return db.Delete(&models.PKCEState{}).Error
}
