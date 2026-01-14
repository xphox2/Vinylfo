package models

import (
	"time"
)

type AppConfig struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DiscogsConsumerKey    string    `gorm:"size:255" json:"discogs_consumer_key"`
	DiscogsConsumerSecret string    `gorm:"size:255" json:"discogs_consumer_secret"`
	DiscogsAccessToken    string    `gorm:"size:255" json:"discogs_access_token"`
	DiscogsAccessSecret   string    `gorm:"size:255" json:"discogs_access_secret"`
	DiscogsUsername       string    `gorm:"size:255" json:"discogs_username"`
	IsDiscogsConnected    bool      `gorm:"default:false" json:"is_discogs_connected"`
	SyncConfirmBatches    bool      `gorm:"default:true" json:"sync_confirm_batches"`
	SyncBatchSize         int       `gorm:"default:50" json:"sync_batch_size"`
	AutoApplySafeUpdates  bool      `gorm:"default:false" json:"auto_apply_safe_updates"`
	AutoSyncNewAlbums     bool      `gorm:"default:false" json:"auto_sync_new_albums"`
	LastSyncAt            time.Time `json:"last_sync_at"`
	Theme                 string    `gorm:"size:50;default:'light'" json:"theme"`
	ItemsPerPage          int       `gorm:"default:25" json:"items_per_page"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
