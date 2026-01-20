package models

import (
	"time"
)

type AppConfig struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DiscogsAccessToken  string    `gorm:"size:255" json:"discogs_access_token"`
	DiscogsAccessSecret string    `gorm:"size:255" json:"discogs_access_secret"`
	DiscogsUsername     string    `gorm:"size:255" json:"discogs_username"`
	IsDiscogsConnected  bool      `gorm:"default:false" json:"is_discogs_connected"`
	SyncBatchSize       int       `gorm:"default:50" json:"sync_batch_size"`
	LastSyncAt          time.Time `json:"last_sync_at"`
	ItemsPerPage        int       `json:"items_per_page"`
	SyncMode            string    `gorm:"size:20;default:'all'" json:"sync_mode"`
	SyncFolderID        int       `gorm:"default:0" json:"sync_folder_id"`
	YouTubeAccessToken  string    `gorm:"column:youtube_access_token;size:500" json:"-"`
	YouTubeRefreshToken string    `gorm:"column:youtube_refresh_token;size:500" json:"-"`
	YouTubeTokenExpiry  time.Time `gorm:"column:youtube_token_expiry" json:"-"`
	YouTubeConnected    bool      `gorm:"column:youtube_connected;default:false" json:"youtube_connected"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
