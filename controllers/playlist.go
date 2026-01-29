package controllers

import (
	"log"
	"time"

	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PlaylistController struct {
	db *gorm.DB
}

func NewPlaylistController(db *gorm.DB) *PlaylistController {
	return &PlaylistController{db: db}
}

func (c *PlaylistController) GetSessions(ctx *gin.Context) {
	var sessions []models.PlaybackSession
	result := c.db.Find(&sessions)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch playback sessions"})
		return
	}

	// Build response with queue counts
	type SessionResponse struct {
		PlaylistID    string    `json:"playlist_id"`
		PlaylistName  string    `json:"playlist_name"`
		TrackID       uint      `json:"track_id"`
		QueueIndex    int       `json:"queue_index"`
		QueuePosition int       `json:"queue_position"`
		Status        string    `json:"status"`
		QueueCount    int       `json:"queue_count"`
		StartedAt     time.Time `json:"started_at"`
		LastPlayedAt  time.Time `json:"last_played_at"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	var response []SessionResponse
	for _, s := range sessions {
		var queueCount int64
		c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", s.PlaylistID).Count(&queueCount)

		response = append(response, SessionResponse{
			PlaylistID:    s.PlaylistID,
			PlaylistName:  s.PlaylistName,
			TrackID:       s.TrackID,
			QueueIndex:    s.QueueIndex,
			QueuePosition: s.QueuePosition,
			Status:        s.Status,
			QueueCount:    int(queueCount),
			StartedAt:     s.StartedAt,
			LastPlayedAt:  s.LastPlayedAt,
			CreatedAt:     s.CreatedAt,
			UpdatedAt:     s.UpdatedAt,
		})
	}

	ctx.JSON(200, response)
}

func (c *PlaylistController) GetSessionByID(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	var session models.PlaybackSession
	result := c.db.First(&session, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Playback session not found"})
		return
	}
	ctx.JSON(200, session)
}

func (c *PlaylistController) CreateSession(ctx *gin.Context) {
	var session models.PlaybackSession
	if err := ctx.ShouldBindJSON(&session); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&session)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create playback session"})
		return
	}
	ctx.JSON(201, session)
}

func (c *PlaylistController) UpdateSession(ctx *gin.Context) {
	id := ctx.Param("id")
	var session models.PlaybackSession
	result := c.db.First(&session, "playlist_id = ?", id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Playback session not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&session); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&session)
	if result.Error != nil {
		log.Printf("UpdateSession DB error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to update playback session"})
		return
	}
	ctx.JSON(200, session)
}

func (c *PlaylistController) DeleteSession(ctx *gin.Context) {
	id := ctx.Param("id")
	var session models.PlaybackSession
	result := c.db.First(&session, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Playback session not found"})
		return
	}

	result = c.db.Delete(&session)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete playback session"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Playback session deleted successfully"})
}

func (c *PlaylistController) CreatePlaylistSession(ctx *gin.Context) {
	type PlaylistSessionRequest struct {
		PlaylistID   string `json:"playlist_id"`
		PlaylistName string `json:"playlist_name"`
		TrackIDs     []uint `json:"track_ids"`
	}

	var req PlaylistSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.PlaylistID == "" {
		ctx.JSON(400, gin.H{"error": "playlist_id is required"})
		return
	}

	var existingSession models.PlaybackSession
	result := c.db.First(&existingSession, "playlist_id = ?", req.PlaylistID)
	if result.Error == nil {
		ctx.JSON(400, gin.H{"error": "A session for this playlist already exists"})
		return
	}

	session := models.PlaybackSession{
		PlaylistID:   req.PlaylistID,
		PlaylistName: req.PlaylistName,
		TrackID:      0,
		Status:       "stopped",
	}

	result = c.db.Create(&session)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create playback session"})
		return
	}

	var playlistEntries []models.SessionPlaylist
	for i, trackID := range req.TrackIDs {
		entry := models.SessionPlaylist{
			SessionID: req.PlaylistID,
			TrackID:   trackID,
			Order:     i + 1,
		}
		playlistEntries = append(playlistEntries, entry)
	}

	result = c.db.Create(&playlistEntries)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create playlist entries"})
		return
	}

	ctx.JSON(201, gin.H{
		"session":     session,
		"playlist":    playlistEntries,
		"track_count": len(req.TrackIDs),
	})
}

func (c *PlaylistController) GetSessionPlaylistTracks(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	var playlistEntries []models.SessionPlaylist
	result := c.db.Where("session_id = ?", sessionID).Order("`order` ASC").Find(&playlistEntries)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch playlist entries"})
		return
	}

	var tracks []models.Track
	var trackIDs []uint
	for _, entry := range playlistEntries {
		trackIDs = append(trackIDs, entry.TrackID)
	}

	if len(trackIDs) > 0 {
		result = c.db.Find(&tracks, trackIDs)
		if result.Error != nil {
			ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
			return
		}
	}

	ctx.JSON(200, gin.H{
		"session_id": sessionID,
		"tracks":     tracks,
		"count":      len(tracks),
	})
}

func (c *PlaylistController) GetAllPlaylists(ctx *gin.Context) {
	var sessionIDs []string

	result := c.db.Model(&models.SessionPlaylist{}).
		Distinct("session_id").
		Pluck("session_id", &sessionIDs)
	if result.Error != nil {
		log.Printf("GetAllPlaylists DB error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to fetch playlists"})
		return
	}

	var playlists []models.SessionPlaylist
	for _, sessionID := range sessionIDs {
		var sp models.SessionPlaylist
		c.db.Where("session_id = ?", sessionID).Order("created_at ASC").First(&sp)
		playlists = append(playlists, sp)
	}

	ctx.JSON(200, playlists)
}

func (c *PlaylistController) CreateNewPlaylist(ctx *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request: name is required"})
		return
	}

	if req.Name == "" {
		ctx.JSON(400, gin.H{"error": "Playlist name cannot be empty"})
		return
	}

	var count int64
	c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", req.Name).Count(&count)
	if count > 0 {
		ctx.JSON(400, gin.H{"error": "A playlist with this name already exists"})
		return
	}

	playlist := models.SessionPlaylist{
		SessionID: req.Name,
		TrackID:   0,
		Order:     0,
	}

	result := c.db.Create(&playlist)
	if result.Error != nil {
		log.Printf("CreateNewPlaylist DB error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to create playlist"})
		return
	}
	ctx.JSON(201, gin.H{
		"session_id": req.Name,
		"message":    "Playlist created successfully",
	})
}

func (c *PlaylistController) GetPlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	// For playlist management, fetch all tracks without pagination
	page := 1
	limit := 100000 // Effectively no limit for playlist management

	offset := (page - 1) * limit

	var playlistEntries []models.SessionPlaylist
	var total int64

	c.db.Model(&models.SessionPlaylist{}).Where("session_id = ? AND track_id > 0", sessionID).Count(&total)

	result := c.db.Where("session_id = ? AND track_id > 0", sessionID).Order("`order` ASC").Offset(offset).Limit(limit).Find(&playlistEntries)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch playlist"})
		return
	}

	type TrackResult struct {
		ID             uint   `json:"id"`
		AlbumID        uint   `json:"album_id"`
		Title          string `json:"title"`
		Duration       int    `json:"duration"`
		TrackNumber    int    `json:"track_number"`
		DiscNumber     int    `json:"disc_number"`
		Side           string `json:"side"`
		Position       string `json:"position"`
		AudioFileURL   string `json:"audio_file_url"`
		AlbumTitle     string `json:"album_title"`
		DurationSource string `json:"duration_source"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	if len(playlistEntries) == 0 {
		ctx.JSON(200, gin.H{
			"session_id": sessionID,
			"tracks":     []TrackResult{},
			"count":      0,
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": int(total) / limit,
		})
		return
	}

	var tracks []TrackResult
	trackIDSet := make(map[uint]int)
	for i, entry := range playlistEntries {
		trackIDSet[entry.TrackID] = i
	}

	var trackIDs []uint
	for _, entry := range playlistEntries {
		trackIDs = append(trackIDs, entry.TrackID)
	}

	if len(trackIDs) > 0 {
		result = c.db.Table("tracks").Select("tracks.*, albums.title as album_title").
			Joins("left join albums on tracks.album_id = albums.id").
			Where("tracks.id IN ?", trackIDs).
			Find(&tracks)
		if result.Error != nil {
			log.Printf("GetPlaylist tracks query error: %v", result.Error)
			ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
			return
		}
	}

	sortedTracks := make([]TrackResult, len(playlistEntries))
	for _, track := range tracks {
		if order, ok := trackIDSet[track.ID]; ok {
			sortedTracks[order] = track
		}
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"session_id": sessionID,
		"tracks":     sortedTracks,
		"count":      len(sortedTracks),
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	})
}

func (c *PlaylistController) UpdatePlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	var req struct {
		DraggedTrackID uint `json:"dragged_track_id"`
		TargetTrackID  uint `json:"target_track_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var draggedEntry models.SessionPlaylist
	result := c.db.Where("session_id = ? AND track_id = ?", sessionID, req.DraggedTrackID).First(&draggedEntry)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Dragged track not found in playlist"})
		return
	}

	var targetEntry models.SessionPlaylist
	result = c.db.Where("session_id = ? AND track_id = ?", sessionID, req.TargetTrackID).First(&targetEntry)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Target track not found in playlist"})
		return
	}

	draggedOrder := draggedEntry.Order
	targetOrder := targetEntry.Order

	result = c.db.Model(&draggedEntry).Update("order", targetOrder)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update order"})
		return
	}

	result = c.db.Model(&targetEntry).Update("order", draggedOrder)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update order"})
		return
	}

	ctx.JSON(200, gin.H{"message": "Playlist updated successfully"})
}

func (c *PlaylistController) DeletePlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	// Get all track IDs in this playlist first
	var playlistTracks []models.SessionPlaylist
	if err := c.db.Where("session_id = ?", sessionID).Find(&playlistTracks).Error; err == nil {
		var trackIDs []uint
		for _, pt := range playlistTracks {
			trackIDs = append(trackIDs, pt.TrackID)
		}

		// Clean up YouTube match data for tracks in this playlist
		if len(trackIDs) > 0 {
			c.db.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeMatch{})
			c.db.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeCandidate{})
		}
	}

	// Delete SessionPlaylist entries
	result := c.db.Where("session_id = ?", sessionID).Delete(&models.SessionPlaylist{})
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete playlist"})
		return
	}

	// Delete the PlaybackSession itself (including YouTube sync info)
	c.db.Where("playlist_id = ?", sessionID).Delete(&models.PlaybackSession{})

	ctx.JSON(200, gin.H{"message": "Playlist deleted successfully"})
}

func (c *PlaylistController) DeletePlaylistWithSessions(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	// Delete session notes
	c.db.Where("session_id = ?", sessionID).Delete(&models.SessionNote{})

	// Delete session sharing
	c.db.Where("session_id = ?", sessionID).Delete(&models.SessionSharing{})

	// Get all track IDs in this playlist first
	var playlistTracks []models.SessionPlaylist
	if err := c.db.Where("session_id = ?", sessionID).Find(&playlistTracks).Error; err == nil {
		var trackIDs []uint
		for _, pt := range playlistTracks {
			trackIDs = append(trackIDs, pt.TrackID)
		}

		// Clean up YouTube match data for tracks in this playlist
		if len(trackIDs) > 0 {
			c.db.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeMatch{})
			c.db.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeCandidate{})
		}
	}

	// Delete SessionPlaylist entries
	result := c.db.Where("session_id = ?", sessionID).Delete(&models.SessionPlaylist{})
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete playlist"})
		return
	}

	// Delete the PlaybackSession itself (including YouTube sync info)
	c.db.Where("playlist_id = ?", sessionID).Delete(&models.PlaybackSession{})

	ctx.JSON(200, gin.H{"message": "Playlist and all related sessions deleted successfully"})
}

func (c *PlaylistController) AddTrackToPlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	var req struct {
		TrackID uint `json:"track_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var maxOrder int
	result := c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", sessionID).Select("MAX(`order`)").Scan(&maxOrder)
	if result.Error != nil {
		maxOrder = 0
	}

	entry := models.SessionPlaylist{
		SessionID: sessionID,
		TrackID:   req.TrackID,
		Order:     maxOrder + 1,
	}

	result = c.db.Create(&entry)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to add track to playlist"})
		return
	}
	ctx.JSON(201, entry)
}

func (c *PlaylistController) RemoveTrackFromPlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	trackID := ctx.Param("track_id")

	result := c.db.Where("session_id = ? AND track_id = ?", sessionID, trackID).Delete(&models.SessionPlaylist{})
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to remove track from playlist"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Track removed from playlist"})
}

func (c *PlaylistController) ShufflePlaylist(ctx *gin.Context) {
	sessionID := ctx.Param("id")

	var entries []models.SessionPlaylist
	result := c.db.Where("session_id = ?", sessionID).Order("`order` ASC").Find(&entries)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch playlist"})
		return
	}

	trackIDs := make([]uint, len(entries))
	for i, entry := range entries {
		trackIDs[i] = entry.TrackID
	}

	for i := len(trackIDs) - 1; i > 0; i-- {
		j := i % len(trackIDs)
		trackIDs[i], trackIDs[j] = trackIDs[j], trackIDs[i]
	}

	for i, entry := range entries {
		entry.Order = i + 1
		c.db.Model(&entry).Update("order", entry.Order)
	}

	ctx.JSON(200, gin.H{"message": "Playlist shuffled"})
}
