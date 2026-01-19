package models

import (
	"time"
)

// Album represents a music album
type Album struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title                 string    `gorm:"not null;uniqueIndex:idx_title_artist" json:"title"`
	Artist                string    `gorm:"not null;uniqueIndex:idx_title_artist" json:"artist"`
	ReleaseYear           int       `json:"release_year"`
	Genre                 string    `json:"genre"`
	Label                 string    `json:"label"`
	Country               string    `json:"country"`
	ReleaseDate           string    `json:"release_date"`
	Style                 string    `json:"style"`
	DiscogsID             *int      `gorm:"uniqueIndex" json:"discogs_id"`
	DiscogsFolderID       int       `json:"discogs_folder_id"` // Folder ID from Discogs collection
	CoverImageURL         string    `json:"cover_image_url"`
	DiscogsCoverImage     []byte    `gorm:"type:longblob" json:"-"`
	DiscogsCoverImageType string    `json:"discogs_cover_image_type"`
	CoverImageFailed      bool      `json:"cover_image_failed"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// Track represents a track on an album
type Track struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AlbumID      uint   `gorm:"not null;index" json:"album_id"`
	AlbumTitle   string `json:"album_title"`
	Title        string `gorm:"not null" json:"title"`
	Duration     int    `json:"duration"`     // Duration in seconds
	TrackNumber  int    `json:"track_number"` // Track number on album
	DiscNumber   int    `json:"disc_number"`  // Which disc (1, 2, 3...)
	Side         string `json:"side"`         // Side position (A1, B2, C1, etc.)
	Position     string `json:"position"`     // Full position code
	AudioFileURL string `json:"audio_file_url"`
	ReleaseYear  int    `json:"release_year"` // From album
	AlbumGenre   string `json:"album_genre"`  // From album

	DurationSource string `gorm:"size:50;default:'discogs'" json:"duration_source"`
	// Values: "discogs" (original), "resolved" (from API consensus), "manual" (user entered)

	DurationNeedsReview bool `gorm:"default:false;index" json:"duration_needs_review"`
	// True if track was flagged for manual duration review

	DurationResolvedAt *time.Time `json:"duration_resolved_at"`
	// When the duration was resolved (if not from Discogs)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	SyncMode       string    `gorm:"size:20" json:"sync_mode"`         // "all-folders", "specific"
	FolderID       int       `gorm:"index" json:"folder_id"`           // Current folder being synced
	FolderName     string    `gorm:"size:255" json:"folder_name"`      // Current folder name
	FolderIndex    int       `json:"folder_index"`                     // Index in folders list
	TotalFolders   int       `json:"total_folders"`                    // Total folders to sync
	CurrentPage    int       `json:"current_page"`                     // Current page in folder
	Processed      int       `json:"processed"`                        // Total albums processed
	TotalAlbums    int       `json:"total_albums"`                     // Total albums to process
	Status         string    `gorm:"size:20" json:"status"`            // "running", "paused", "completed", "cancelled"
	LastBatchJSON  string    `gorm:"type:text" json:"last_batch_json"` // JSON serialized LastBatch for resume
	LastActivityAt time.Time `json:"last_activity_at"`                 // Last time sync made progress
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SyncHistory stores completed sync runs for historical reporting
type SyncHistory struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncMode     string    `gorm:"size:20" json:"sync_mode"`       // "all-folders", "specific"
	FolderID     int       `json:"folder_id"`                      // Folder that was synced (0 for all-folders)
	FolderName   string    `gorm:"size:255" json:"folder_name"`    // Folder name
	Processed    int       `json:"processed"`                      // Total albums processed
	TotalAlbums  int       `json:"total_albums"`                   // Total albums in folder/sync
	DurationSecs int       `json:"duration_secs"`                  // How long the sync took
	Status       string    `gorm:"size:20" json:"status"`          // "completed", "cancelled", "failed"
	ErrorMessage string    `gorm:"type:text" json:"error_message"` // Error if failed
	StartedAt    time.Time `json:"started_at"`                     // When sync started
	CompletedAt  time.Time `json:"completed_at"`                   // When sync finished
	CreatedAt    time.Time `json:"created_at"`
}

// =============================================================================
// DURATION RESOLUTION MODELS
// =============================================================================

// DurationSource stores a duration value retrieved from an external API.
// Multiple sources are queried for each track, and consensus determines accuracy.
type DurationSource struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ResolutionID  uint      `gorm:"not null;index" json:"resolution_id"` // FK to DurationResolution
	SourceName    string    `gorm:"size:50;not null" json:"source_name"` // "musicbrainz", "wikipedia", "spotify", "apple_music", "youtube_music"
	DurationValue int       `json:"duration_value"`                      // Duration in seconds (0 if not found)
	Confidence    float64   `json:"confidence"`                          // 0.0-1.0 how confident source is in this value
	MatchScore    float64   `json:"match_score"`                         // 0.0-1.0 how well the track matched the search query
	ExternalID    string    `gorm:"size:255" json:"external_id"`         // ID from external service (e.g., MusicBrainz recording ID)
	ExternalURL   string    `gorm:"size:512" json:"external_url"`        // URL to the track on external service (for verification)
	RawResponse   string    `gorm:"type:longtext" json:"raw_response"`   // Full JSON response for debugging/auditing
	ErrorMessage  string    `gorm:"size:500" json:"error_message"`       // Error message if query failed
	QueryDuration int       `json:"query_duration"`                      // How long the API call took in milliseconds
	QueriedAt     time.Time `json:"queried_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// DurationResolution represents a complete resolution attempt for a single track.
// It aggregates results from multiple DurationSource queries and tracks the outcome.
type DurationResolution struct {
	ID      uint `gorm:"primaryKey;autoIncrement" json:"id"`
	TrackID uint `gorm:"not null;uniqueIndex" json:"track_id"` // One resolution per track
	AlbumID uint `gorm:"not null;index" json:"album_id"`       // Denormalized for faster album-based queries

	// Resolution status
	Status string `gorm:"size:20;not null;default:'pending'" json:"status"`
	// Status values:
	//   "pending"      - Not yet processed
	//   "in_progress"  - Currently being resolved
	//   "resolved"     - Successfully resolved with consensus
	//   "needs_review" - Requires manual review (no consensus or single source)
	//   "failed"       - All API queries failed
	//   "approved"     - Manually reviewed and approved
	//   "rejected"     - Manually reviewed and rejected (keep original)

	// Resolution outcome
	OriginalDuration    int  `json:"original_duration"`     // Original duration from Discogs (usually 0)
	ResolvedDuration    *int `json:"resolved_duration"`     // Final resolved duration (nil until resolved)
	ConsensusCount      int  `json:"consensus_count"`       // How many sources agreed on the duration
	TotalSourcesQueried int  `json:"total_sources_queried"` // Total APIs that were queried
	SuccessfulQueries   int  `json:"successful_queries"`    // APIs that returned a result

	// Application tracking
	AutoApplied      bool       `gorm:"default:false" json:"auto_applied"`      // Was duration auto-applied (consensus met)?
	AppliedAt        *time.Time `json:"applied_at"`                             // When duration was applied to track
	ManuallyReviewed bool       `gorm:"default:false" json:"manually_reviewed"` // Was this manually reviewed?
	ReviewedAt       *time.Time `json:"reviewed_at"`
	ReviewedBy       string     `gorm:"size:100" json:"reviewed_by"`   // Who reviewed (for audit trail)
	ReviewNotes      string     `gorm:"type:text" json:"review_notes"` // Notes from reviewer
	ReviewAction     string     `gorm:"size:20" json:"review_action"`  // "apply", "reject", "manual", "skip"

	// Discogs submission tracking (for future Phase 8)
	DiscogsSubmittable  bool       `gorm:"default:false" json:"discogs_submittable"` // Ready for Discogs submission
	DiscogsSubmittedAt  *time.Time `json:"discogs_submitted_at"`
	DiscogsSubmissionID string     `gorm:"size:100" json:"discogs_submission_id"`

	// Related data (loaded via Preload, not a database relationship)
	Sources []DurationSource `gorm:"-" json:"sources"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DurationResolverProgress tracks the state of a bulk resolution operation.
// Persisted to database so resolution can survive application restarts.
type DurationResolverProgress struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`

	// Current status
	Status string `gorm:"size:20;not null;default:'idle'" json:"status"`
	// Status values: "idle", "running", "paused", "completed", "failed", "cancelled"

	// Progress counters
	TotalTracks      int `json:"total_tracks"`       // Total tracks to process
	ProcessedTracks  int `json:"processed_tracks"`   // Tracks processed so far
	ResolvedCount    int `json:"resolved_count"`     // Successfully resolved (consensus reached)
	NeedsReviewCount int `json:"needs_review_count"` // Flagged for manual review
	FailedCount      int `json:"failed_count"`       // All API queries failed
	SkippedCount     int `json:"skipped_count"`      // Skipped (already has duration)

	// Current position (for resume)
	CurrentTrackID  uint `json:"current_track_id"`  // Track currently being processed
	CurrentAlbumID  uint `json:"current_album_id"`  // Album currently being processed
	LastProcessedID uint `json:"last_processed_id"` // Last successfully processed track ID

	// Timing
	StartedAt      *time.Time `json:"started_at"`
	PausedAt       *time.Time `json:"paused_at"`
	ResumedAt      *time.Time `json:"resumed_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	LastActivityAt time.Time  `json:"last_activity_at"`

	// Error tracking
	LastError         string `gorm:"size:500" json:"last_error"`
	ConsecutiveErrors int    `json:"consecutive_errors"` // For circuit breaker logic

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides for GORM (optional, uses snake_case by default)
func (DurationSource) TableName() string           { return "duration_sources" }
func (DurationResolution) TableName() string       { return "duration_resolutions" }
func (DurationResolverProgress) TableName() string { return "duration_resolver_progress" }
