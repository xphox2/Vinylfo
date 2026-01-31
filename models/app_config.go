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
	YouTubeAccessToken  string    `gorm:"column:youtube_access_token;type:text" json:"-"`
	YouTubeRefreshToken string    `gorm:"column:youtube_refresh_token;type:text" json:"-"`
	YouTubeTokenExpiry  time.Time `gorm:"column:youtube_token_expiry" json:"-"`
	YouTubeConnected    bool      `gorm:"column:youtube_connected;default:false" json:"youtube_connected"`
	LogRetentionCount   int       `gorm:"default:10" json:"log_retention_count"`

	// Feed Settings - Video Feed
	FeedVideoTheme           string `gorm:"size:20;default:'dark'" json:"feed_video_theme"`
	FeedVideoOverlay         string `gorm:"size:20;default:'bottom'" json:"feed_video_overlay"`
	FeedVideoTransition      string `gorm:"size:20;default:'fade'" json:"feed_video_transition"`
	FeedVideoQuality         string `gorm:"size:10;default:'auto'" json:"feed_video_quality"`
	FeedVideoShowVisualizer  bool   `gorm:"default:true" json:"feed_video_show_visualizer"`
	FeedVideoOverlayDuration int    `gorm:"default:5" json:"feed_video_overlay_duration"`
	FeedVideoShowBackground  bool   `gorm:"default:true" json:"feed_video_show_background"`
	FeedVideoEnableAudio     bool   `gorm:"default:false" json:"feed_video_enable_audio"`

	// Feed Settings - Album Art Feed
	FeedArtTheme          string `gorm:"size:20;default:'dark'" json:"feed_art_theme"`
	FeedArtAnimation      bool   `gorm:"default:true" json:"feed_art_animation"`
	FeedArtAnimDuration   int    `gorm:"default:20" json:"feed_art_anim_duration"`
	FeedArtFit            string `gorm:"size:20;default:'cover'" json:"feed_art_fit"`
	FeedArtShowBackground bool   `gorm:"default:true" json:"feed_art_show_background"`

	// Feed Settings - Track Info Feed
	FeedTrackTheme          string `gorm:"size:20;default:'dark'" json:"feed_track_theme"`
	FeedTrackSpeed          int    `gorm:"default:5" json:"feed_track_speed"`
	FeedTrackDirection      string `gorm:"size:10;default:'rtl'" json:"feed_track_direction"`
	FeedTrackSeparator      string `gorm:"size:50;default:'*'" json:"feed_track_separator"`
	FeedTrackPrefix         string `gorm:"size:100;default:'Now Playing:'" json:"feed_track_prefix"`
	FeedTrackSuffix         string `gorm:"size:100;default:''" json:"feed_track_suffix"`
	FeedTrackShowArtist     bool   `gorm:"default:true" json:"feed_track_show_artist"`
	FeedTrackShowAlbum      bool   `gorm:"default:true" json:"feed_track_show_album"`
	FeedTrackShowDuration   bool   `gorm:"default:true" json:"feed_track_show_duration"`
	FeedTrackShowBackground bool   `gorm:"default:true" json:"feed_track_show_background"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
