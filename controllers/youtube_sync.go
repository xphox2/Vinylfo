package controllers

import (
	"net/http"
	"strconv"

	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// YouTubeSyncController handles YouTube playlist sync operations
type YouTubeSyncController struct {
	db      *gorm.DB
	service *services.YouTubeSyncService
}

// NewYouTubeSyncController creates a new sync controller
func NewYouTubeSyncController(db *gorm.DB) *YouTubeSyncController {
	service, err := services.NewYouTubeSyncService(db)
	if err != nil {
		// Service creation failed, but we'll handle it in the endpoints
		return &YouTubeSyncController{db: db, service: nil}
	}
	return &YouTubeSyncController{db: db, service: service}
}

// MatchTrack matches a single track to YouTube videos
// POST /api/youtube/match-track/:track_id?force=true
func (c *YouTubeSyncController) MatchTrack(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	force := ctx.Query("force") == "true"
	apiFallback := ctx.Query("api_fallback") == "true"

	result, err := c.service.MatchTrack(ctx.Request.Context(), uint(trackID), force, apiFallback)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// MatchPlaylist matches all tracks in a playlist to YouTube videos
// POST /api/youtube/match-playlist/:playlist_id?force=true
func (c *YouTubeSyncController) MatchPlaylist(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	playlistID := ctx.Param("playlist_id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	force := ctx.Query("force") == "true"

	var input struct {
		IncludeReview      bool `json:"include_review"`
		YouTubeApiFallback bool `json:"youtube_api_fallback"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.YouTubeApiFallback = false
	}

	result, err := c.service.MatchPlaylist(ctx.Request.Context(), playlistID, force, input.YouTubeApiFallback)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// GetMatches returns match status for all tracks in a playlist
// GET /api/youtube/matches/:playlist_id
func (c *YouTubeSyncController) GetMatches(ctx *gin.Context) {
	playlistID := ctx.Param("playlist_id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	// Get all tracks in the playlist
	var playlistTracks []models.SessionPlaylist
	if err := c.db.Where("session_id = ?", playlistID).Order("`order` ASC").Find(&playlistTracks).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get playlist tracks"})
		return
	}

	type TrackMatchStatus struct {
		TrackID    uint                           `json:"track_id"`
		TrackTitle string                         `json:"track_title"`
		Artist     string                         `json:"artist"`
		Duration   int                            `json:"duration"`
		AlbumTitle string                         `json:"album_title"`
		Match      *models.TrackYouTubeMatch      `json:"match,omitempty"`
		Candidates []models.TrackYouTubeCandidate `json:"candidates,omitempty"`
		Status     string                         `json:"status"` // matched, needs_review, unavailable, pending
	}

	var results []TrackMatchStatus

	for _, pt := range playlistTracks {
		// Get track info
		var track models.Track
		if err := c.db.First(&track, pt.TrackID).Error; err != nil {
			continue
		}

		var album models.Album
		c.db.First(&album, track.AlbumID)

		status := TrackMatchStatus{
			TrackID:    track.ID,
			TrackTitle: track.Title,
			Artist:     album.Artist,
			Duration:   track.Duration,
			AlbumTitle: album.Title,
			Status:     "pending",
		}

		// Get match if exists
		var match models.TrackYouTubeMatch
		if err := c.db.Where("track_id = ?", track.ID).First(&match).Error; err == nil {
			status.Match = &match
			status.Status = match.Status

			// Get candidates if needs review
			if match.NeedsReview {
				var candidates []models.TrackYouTubeCandidate
				c.db.Where("track_id = ?", track.ID).Order("rank").Find(&candidates)
				status.Candidates = candidates
			}
		}

		results = append(results, status)
	}

	// Get playlist info including YouTube sync status
	var session models.PlaybackSession
	var playlistName string
	var youtubeSyncInfo gin.H
	if err := c.db.Where("playlist_id = ?", playlistID).First(&session).Error; err == nil {
		playlistName = session.PlaylistName
		youtubeSyncInfo = gin.H{
			"youtube_playlist_id":   session.YouTubePlaylistID,
			"youtube_playlist_name": session.YouTubePlaylistName,
			"synced_at":             session.YouTubeSyncedAt,
		}
	} else {
		// Try to get playlist name from SessionPlaylist if no PlaybackSession exists
		var sp models.SessionPlaylist
		if err := c.db.Where("session_id = ?", playlistID).First(&sp).Error; err == nil {
			playlistName = sp.SessionID
		}
		youtubeSyncInfo = gin.H{
			"youtube_playlist_id":   "",
			"youtube_playlist_name": "",
			"synced_at":             nil,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"tracks":        results,
		"youtube_sync":  youtubeSyncInfo,
		"playlist_name": playlistName,
	})
}

// UpdateMatch manually sets or overrides a match for a track
// PUT /api/youtube/matches/:track_id
func (c *YouTubeSyncController) UpdateMatch(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	var input struct {
		YouTubeVideoID string `json:"youtube_video_id" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	match, err := c.service.SetManualMatch(ctx.Request.Context(), uint(trackID), input.YouTubeVideoID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, match)
}

// DeleteMatch marks a track as unavailable (no good match exists)
// DELETE /api/youtube/matches/:track_id
func (c *YouTubeSyncController) DeleteMatch(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	if err := c.service.MarkUnavailable(uint(trackID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "unavailable"})
}

// SyncPlaylist syncs matched tracks to a YouTube playlist
// POST /api/youtube/sync-playlist/:playlist_id
func (c *YouTubeSyncController) SyncPlaylist(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	playlistID := ctx.Param("playlist_id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	var input struct {
		YouTubePlaylistID  string `json:"youtube_playlist_id"`
		PlaylistName       string `json:"playlist_name"`
		IncludeNeedsReview bool   `json:"include_needs_review"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req := services.SyncPlaylistRequest{
		PlaylistID:         playlistID,
		YouTubePlaylistID:  input.YouTubePlaylistID,
		PlaylistName:       input.PlaylistName,
		IncludeNeedsReview: input.IncludeNeedsReview,
	}

	result, err := c.service.SyncPlaylistToYouTube(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// GetSyncStatus returns the sync status for a playlist
// GET /api/youtube/sync-status/:playlist_id
func (c *YouTubeSyncController) GetSyncStatus(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	playlistID := ctx.Param("playlist_id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	status, err := c.service.GetPlaylistSyncStatus(playlistID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, status)
}

// GetCandidates returns all candidates for a track needing review
// GET /api/youtube/candidates/:track_id
func (c *YouTubeSyncController) GetCandidates(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	// Get track info
	var track models.Track
	if err := c.db.First(&track, trackID).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Track not found"})
		return
	}

	var album models.Album
	c.db.First(&album, track.AlbumID)

	candidates, err := c.service.GetCandidates(uint(trackID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"track": gin.H{
			"id":       track.ID,
			"title":    track.Title,
			"artist":   album.Artist,
			"album":    album.Title,
			"duration": track.Duration,
		},
		"candidates": candidates,
	})
}

// SelectCandidate selects a candidate as the match for a track
// POST /api/youtube/candidates/:track_id/select/:candidate_id
func (c *YouTubeSyncController) SelectCandidate(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	candidateID, err := strconv.ParseUint(ctx.Param("candidate_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid candidate ID"})
		return
	}

	match, err := c.service.SelectCandidate(uint(trackID), uint(candidateID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, match)
}

// GetTrackMatch returns the current match for a specific track
// GET /api/youtube/match/:track_id
func (c *YouTubeSyncController) GetTrackMatch(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "YouTube sync service not available"})
		return
	}

	trackID, err := strconv.ParseUint(ctx.Param("track_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid track ID"})
		return
	}

	match, err := c.service.GetMatch(uint(trackID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "No match found for track"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, match)
}

// ClearWebCache clears the YouTube web search cache
// POST /api/youtube/clear-cache
func (c *YouTubeSyncController) ClearWebCache(ctx *gin.Context) {
	if c.service == nil || c.service.WebSearcher() == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web search service not available"})
		return
	}

	if err := c.service.WebSearcher().ClearCache(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cache: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "cleared"})
}
