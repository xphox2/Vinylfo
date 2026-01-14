package models

import (
	"time"
)

// Album represents a music album
type Album struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title         string    `gorm:"not null;uniqueIndex" json:"title"`
	Artist        string    `gorm:"not null" json:"artist"`
	ReleaseYear   int       `json:"release_year"`
	Genre         string    `json:"genre"`
	CoverImageURL string    `json:"cover_image_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Track represents a track on an album
type Track struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AlbumID      uint      `gorm:"not null;index" json:"album_id"`
	AlbumTitle   string    `json:"album_title"`
	Title        string    `gorm:"not null" json:"title"`
	Duration     int       `json:"duration"`     // Duration in seconds
	TrackNumber  int       `json:"track_number"` // Track number on album
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
