package models

import (
	"time"
)

type PKCEState struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	State         string    `gorm:"size:256;uniqueIndex" json:"state"`
	CodeVerifier  string    `gorm:"size:256" json:"code_verifier"`
	CodeChallenge string    `gorm:"size:256" json:"code_challenge"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func (PKCEState) TableName() string {
	return "pkce_states"
}
