package models

import (
	"time"
)

// Album represents a music album
type Album struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title           string    `gorm:"not null;uniqueIndex" json:"title"`
	Artist          string    `gorm:"not null" json:"artist"`
	ReleaseYear     int       `json:"release_year"`
	Genre           string    `json:"genre"`
	Label           string    `json:"label"`
	Country         string    `json:"country"`
	ReleaseDate     string    `json:"release_date"`
	Style           string    `json:"style"`
	DiscogsID       int       `json:"discogs_id"`
	DiscogsFolderID int       `json:"discogs_folder_id"` // Folder ID from Discogs collection
	CoverImageURL   string    `json:"cover_image_url"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Track represents a track on an album
type Track struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AlbumID      uint      `gorm:"not null;index" json:"album_id"`
	AlbumTitle   string    `json:"album_title"`
	Title        string    `gorm:"not null" json:"title"`
	Duration     int       `json:"duration"`     // Duration in seconds
	TrackNumber  int       `json:"track_number"` // Track number on album
	DiscNumber   int       `json:"disc_number"`  // Which disc (1, 2, 3...)
	Side         string    `json:"side"`         // Side position (A1, B2, C1, etc.)
	Position     string    `json:"position"`     // Full position code
	AudioFileURL string    `json:"audio_file_url"`
	ReleaseYear  int       `json:"release_year"` // From album
	AlbumGenre   string    `json:"album_genre"`  // From album
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PlaybackSession represents the current playback state
type PlaybackSession struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TrackID       uint      `gorm:"not null;index" json:"track_id"`
	PlaylistID    string    `json:"playlist_id"`    // Which playlist is playing
	PlaylistName  string    `json:"playlist_name"`  // Display name of playlist
	Queue         string    `json:"queue"`          // JSON array of track IDs
	QueueIndex    int       `json:"queue_index"`    // Current position in queue
	QueuePosition int       `json:"queue_position"` // Saved position in current track (seconds)
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Duration      int       `json:"duration"` // Duration in seconds
	Progress      int       `json:"progress"` // Progress in seconds
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SessionPlaylist represents a playlist within a session
type SessionPlaylist struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID string    `gorm:"not null;index" json:"session_id"`
	TrackID   uint      `gorm:"not null;index" json:"track_id"`
	Order     int       `gorm:"not null" json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionSharing represents sharing information for sessions
type SessionSharing struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID    string    `gorm:"not null;index" json:"session_id"`
	SharingToken string    `gorm:"unique;not null" json:"sharing_token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	IsPublic     bool      `json:"is_public"`
	Notes        string    `json:"notes"`
}

// SessionNote represents a note/comment for a playback session
type SessionNote struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID string    `gorm:"not null;index" json:"session_id"`
	Content   string    `gorm:"not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrackHistory represents listening history for tracks
type TrackHistory struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TrackID     uint      `gorm:"not null;index" json:"track_id"`
	PlaylistID  string    `json:"playlist_id"` // Which playlist played from
	ListenCount int       `gorm:"default:0" json:"listen_count"`
	LastPlayed  time.Time `json:"last_played"` // When last played
	Progress    int       `json:"progress"`    // Last saved position
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SyncLog represents sync error logs
type SyncLog struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DiscogsID  int       `gorm:"index" json:"discogs_id"`
	AlbumTitle string    `gorm:"size:255" json:"album_title"`
	Artist     string    `gorm:"size:255" json:"artist"`
	ErrorType  string    `gorm:"size:50" json:"error_type"` // "album" or "tracks"
	ErrorMsg   string    `gorm:"type:text" json:"error_msg"`
	CreatedAt  time.Time `json:"created_at"`
}

// SyncProgress tracks the sync progress for resume capability
type SyncProgress struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncMode       string    `gorm:"size:20" json:"sync_mode"`    // "all-folders", "specific"
	FolderID       int       `gorm:"index" json:"folder_id"`      // Current folder being synced
	FolderName     string    `gorm:"size:255" json:"folder_name"` // Current folder name
	FolderIndex    int       `json:"folder_index"`                // Index in folders list
	TotalFolders   int       `json:"total_folders"`               // Total folders to sync
	CurrentPage    int       `json:"current_page"`                // Current page in folder
	Processed      int       `json:"processed"`                   // Total albums processed
	TotalAlbums    int       `json:"total_albums"`                // Total albums to process
	Status         string    `gorm:"size:20" json:"status"`       // "running", "paused", "completed", "cancelled"
	LastActivityAt time.Time `json:"last_activity_at"`            // Last time sync made progress
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
