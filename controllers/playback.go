package controllers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"vinylfo/models"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PlaybackController struct {
	db              *gorm.DB
	playbackManager *PlaybackManager
}

type PlaybackManager struct {
	sync.RWMutex
	sessions     map[string]*PlaybackSessionState
	currentTrack *models.Track
	playlistID   string
	playlistName string
}

type PlaybackSessionState struct {
	IsPlaying       bool
	IsPaused        bool
	Position        int
	PlaybackSession *models.PlaybackSession
}

func NewPlaybackManager() *PlaybackManager {
	return &PlaybackManager{
		sessions: make(map[string]*PlaybackSessionState),
	}
}

func NewPlaybackController(db *gorm.DB) *PlaybackController {
	return &PlaybackController{
		db:              db,
		playbackManager: NewPlaybackManager(),
	}
}

func (pm *PlaybackManager) StartPlayback(playlistID string, session *models.PlaybackSession) {
	pm.Lock()
	defer pm.Unlock()
	log.Printf("[DEBUG] StartPlayback called with playlistID=%s, session.PlaylistName=%s\n", playlistID, session.PlaylistName)
	pm.sessions[playlistID] = &PlaybackSessionState{
		IsPlaying:       true,
		IsPaused:        false,
		Position:        0,
		PlaybackSession: session,
	}
	pm.playlistID = playlistID
	pm.playlistName = session.PlaylistName
	log.Printf("[DEBUG] StartPlayback complete, sessions count=%d, isPlaying=%v\n", len(pm.sessions), pm.sessions[playlistID].IsPlaying)
}

func (pm *PlaybackManager) PausePlayback(playlistID string) {
	pm.Lock()
	defer pm.Unlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		sess.IsPlaying = false
		sess.IsPaused = true
		sess.PlaybackSession.Status = "paused"
		sess.PlaybackSession.LastPlayedAt = time.Now()
	}
}

func (pm *PlaybackManager) ResumePlayback(playlistID string) {
	pm.Lock()
	defer pm.Unlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		sess.IsPaused = false
		sess.IsPlaying = true
		sess.PlaybackSession.Status = "playing"
	}
}

func (pm *PlaybackManager) StopPlayback(playlistID string) {
	pm.Lock()
	defer pm.Unlock()
	delete(pm.sessions, playlistID)
	if pm.playlistID == playlistID {
		pm.playlistID = ""
		pm.playlistName = ""
		pm.currentTrack = nil
	}
}

func (pm *PlaybackManager) IsPlaying(playlistID string) bool {
	pm.RLock()
	defer pm.RUnlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		log.Printf("[DEBUG] IsPlaying(%s): found session, returning %v\n", playlistID, sess.IsPlaying)
		return sess.IsPlaying
	}
	log.Printf("[DEBUG] IsPlaying(%s): no session found, sessions=%v\n", playlistID, len(pm.sessions))
	return false
}

func (pm *PlaybackManager) IsPaused(playlistID string) bool {
	pm.RLock()
	defer pm.RUnlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		return sess.IsPaused
	}
	return false
}

func (pm *PlaybackManager) GetPosition(playlistID string) int {
	pm.RLock()
	defer pm.RUnlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		return sess.Position
	}
	return 0
}

func (pm *PlaybackManager) GetSession(playlistID string) *models.PlaybackSession {
	pm.RLock()
	defer pm.RUnlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		return sess.PlaybackSession
	}
	return nil
}

func (pm *PlaybackManager) UpdatePosition(playlistID string, position int) {
	pm.Lock()
	defer pm.Unlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		sess.Position = position
		sess.PlaybackSession.QueuePosition = position
	}
}

func (pm *PlaybackManager) GetCurrentTrack() *models.Track {
	pm.RLock()
	defer pm.RUnlock()
	return pm.currentTrack
}

func (pm *PlaybackManager) SetCurrentTrack(playlistID string, track *models.Track) {
	pm.Lock()
	defer pm.Unlock()
	pm.currentTrack = track
	pm.playlistID = playlistID
	if sess, ok := pm.sessions[playlistID]; ok {
		sess.PlaybackSession.TrackID = track.ID
	}
}

func (pm *PlaybackManager) GetCurrentPlaylistID() string {
	pm.RLock()
	defer pm.RUnlock()
	return pm.playlistID
}

func (pm *PlaybackManager) GetCurrentPlaylistName() string {
	pm.RLock()
	defer pm.RUnlock()
	return pm.playlistName
}

func (pm *PlaybackManager) HasSession(playlistID string) bool {
	pm.RLock()
	defer pm.RUnlock()
	_, ok := pm.sessions[playlistID]
	return ok
}

func (pm *PlaybackManager) UpdateSessionState(playlistID string, updateFunc func(*PlaybackSessionState)) {
	pm.Lock()
	defer pm.Unlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		updateFunc(sess)
	}
}

func (c *PlaybackController) GetPlaybackState(ctx *gin.Context) {
	playlistID := ctx.Query("playlist_id")
	if playlistID == "" {
		playlistID = c.playbackManager.GetCurrentPlaylistID()
	}

	var playbackState *models.PlaybackSession

	if playlistID != "" {
		playbackState = c.playbackManager.GetSession(playlistID)

		// Always refresh from database to get latest QueueIndex
		var session models.PlaybackSession
		result := c.db.First(&session, "playlist_id = ?", playlistID)
		if result.Error == nil {
			playbackState = &session
		}
	}

	if playbackState == nil {
		ctx.JSON(200, gin.H{
			"has_state":  false,
			"is_playing": false,
			"is_paused":  false,
		})
		return
	}

	inMemoryPosition := c.playbackManager.GetPosition(playlistID)
	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)

	currentPosition := inMemoryPosition
	if !isPlaying && !isPaused {
		currentPosition = playbackState.QueuePosition
	}

	var trackWithAlbum map[string]interface{}
	if playbackState.TrackID > 0 {
		var dbTrack models.Track
		c.db.First(&dbTrack, playbackState.TrackID)
		var album models.Album
		c.db.First(&album, dbTrack.AlbumID)
		trackWithAlbum = c.buildTrackResponse(dbTrack, album)
	}

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	response := gin.H{
		"position":       currentPosition,
		"server_time":    time.Now().UTC().Format(time.RFC3339),
		"last_update":    time.Now().UTC().Format(time.RFC3339),
		"playlist_id":    playlistID,
		"playlist_name":  playbackState.PlaylistName,
		"queue_index":    playbackState.QueueIndex,
		"queue":          queueTracks,
		"queue_position": playbackState.QueuePosition,
		"status":         playbackState.Status,
		"is_playing":     isPlaying,
		"is_paused":      isPaused,
	}

	if trackWithAlbum != nil {
		response["track"] = trackWithAlbum
	}

	ctx.JSON(200, response)
}

func (c *PlaybackController) Previous(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}

	playlistID := req.PlaylistID
	if playlistID == "" {
		utils.BadRequest(ctx, "No playlist ID provided")
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		utils.NotFound(ctx, "No session found")
		return
	}

	if playbackState.QueueIndex <= 0 {
		utils.BadRequest(ctx, "No previous track in queue")
		return
	}

	playbackState.QueueIndex--
	playbackState.QueuePosition = 0

	trackID, ok := c.getTrackIDAtOrder(playlistID, playbackState.QueueIndex+1)
	if !ok {
		utils.NotFound(ctx, "Track not found at order")
		return
	}
	playbackState.TrackID = trackID

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)

	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)

	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)
	if isPlaying && !isPaused {
		c.playbackManager.ResumePlayback(playlistID)
	}

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to previous track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) Stop(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}

	playlistID := req.PlaylistID
	if playlistID == "" {
		utils.BadRequest(ctx, "No playlist ID provided")
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error == nil {
		c.db.Delete(&playbackState)
	}

	c.playbackManager.StopPlayback(playlistID)

	ctx.JSON(200, gin.H{"status": "Playback stopped", "playlist_id": playlistID})
}

func (c *PlaybackController) Clear(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}

	playlistID := req.PlaylistID
	if playlistID == "" {
		utils.BadRequest(ctx, "No playlist ID provided")
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error == nil {
		playbackState.PlaylistID = ""
		playbackState.PlaylistName = ""
		playbackState.QueueIndex = 0
		playbackState.QueuePosition = 0
		playbackState.TrackID = 0
		playbackState.Status = "stopped"
		c.db.Save(&playbackState)
	}

	c.db.Where("session_id = ?", playlistID).Delete(&models.SessionPlaylist{})

	c.playbackManager.StopPlayback(playlistID)

	ctx.JSON(200, gin.H{"status": "Playback state cleared"})
}

func (c *PlaybackController) RestoreSession(ctx *gin.Context) {
	// Read and parse body manually to avoid consuming it
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Printf("[DEBUG] RestoreSession: failed to read body=%s\n", err.Error())
		ctx.JSON(400, gin.H{"error": "Failed to read request body"})
		return
	}

	log.Printf("[DEBUG] RestoreSession: raw body=%s\n", string(body))

	// Parse JSON manually
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[DEBUG] RestoreSession: parse error=%s\n", err.Error())
		ctx.JSON(400, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	playlistID := req.PlaylistID
	log.Printf("[DEBUG] RestoreSession: playlistID=%s\n", playlistID)

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		log.Printf("[DEBUG] RestoreSession: session not found for playlistID=%s\n", playlistID)
		ctx.JSON(404, gin.H{"error": "Session not found"})
		return
	}

	playlistSize := c.getPlaylistSize(playlistID)
	log.Printf("[DEBUG] RestoreSession: playlistSize=%d\n", playlistSize)
	if playlistSize == 0 {
		ctx.JSON(400, gin.H{"error": "Session has no tracks"})
		return
	}

	if playbackState.QueueIndex >= playlistSize {
		playbackState.QueueIndex = 0
	}

	trackID, ok := c.getTrackIDAtOrder(playlistID, playbackState.QueueIndex+1)
	if !ok {
		ctx.JSON(404, gin.H{"error": "Track not found at queue index"})
		return
	}

	var track models.Track
	result = c.db.First(&track, trackID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	var album models.Album
	c.db.First(&album, track.AlbumID)

	playbackState.TrackID = trackID
	playbackState.Status = "playing"
	playbackState.LastPlayedAt = time.Now()
	c.db.Save(&playbackState)

	c.playbackManager.SetCurrentTrack(playlistID, &track)
	c.playbackManager.StartPlayback(playlistID, &playbackState)

	queueWithAlbums := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"message":        "Session restored",
		"track":          c.buildTrackResponse(track, album),
		"queue":          queueWithAlbums,
		"queue_index":    playbackState.QueueIndex,
		"queue_position": playbackState.QueuePosition,
		"playlist_id":    playlistID,
		"playlist_name":  playbackState.PlaylistName,
		"is_playing":     true,
		"is_paused":      false,
	})
}

func (c *PlaybackController) buildTrackResponse(track models.Track, album models.Album) map[string]interface{} {
	var youtubeVideoDuration int
	var youtubeVideoID string

	var youtubeMatch models.TrackYouTubeMatch
	if result := c.db.Where("track_id = ? AND status = ?", track.ID, "matched").First(&youtubeMatch); result.Error == nil {
		youtubeVideoDuration = youtubeMatch.VideoDuration
		youtubeVideoID = youtubeMatch.YouTubeVideoID
	}

	return map[string]interface{}{
		"id":                     track.ID,
		"album_id":               track.AlbumID,
		"album_title":            album.Title,
		"album_artist":           album.Artist,
		"title":                  track.Title,
		"duration":               track.Duration,
		"youtube_video_id":       youtubeVideoID,
		"youtube_video_duration": youtubeVideoDuration,
		"track_number":           track.TrackNumber,
		"audio_file_url":         track.AudioFileURL,
		"release_year":           album.ReleaseYear,
		"album_genre":            album.Genre,
		"created_at":             track.CreatedAt,
		"updated_at":             track.UpdatedAt,
	}
}

func (c *PlaybackController) getQueueTracks(playlistID string) []map[string]interface{} {
	var playlistEntries []models.SessionPlaylist
	c.db.Where("session_id = ?", playlistID).Order("`order` ASC").Find(&playlistEntries)
	log.Printf("[DEBUG] getQueueTracks: playlistID=%s, entriesCount=%d\n", playlistID, len(playlistEntries))

	var queueTracks []map[string]interface{}
	if len(playlistEntries) > 0 {
		var trackIDs []uint
		for _, entry := range playlistEntries {
			trackIDs = append(trackIDs, entry.TrackID)
		}

		var allQueueTracks []models.Track
		c.db.Find(&allQueueTracks, trackIDs)

		trackMap := make(map[uint]models.Track)
		albumIDSet := make(map[uint]bool)
		for _, track := range allQueueTracks {
			trackMap[track.ID] = track
			albumIDSet[track.AlbumID] = true
		}

		var albumIDs []uint
		for albumID := range albumIDSet {
			albumIDs = append(albumIDs, albumID)
		}

		var albums []models.Album
		c.db.Find(&albums, albumIDs)
		albumMap := make(map[uint]models.Album)
		for _, album := range albums {
			albumMap[album.ID] = album
		}

		for _, entry := range playlistEntries {
			if track, ok := trackMap[entry.TrackID]; ok {
				album := albumMap[track.AlbumID]
				queueTracks = append(queueTracks, c.buildTrackResponse(track, album))
			}
		}
	}
	return queueTracks
}

func (c *PlaybackController) getTrackIDAtOrder(playlistID string, order int) (uint, bool) {
	var entry models.SessionPlaylist
	result := c.db.Where("session_id = ? AND `order` = ?", playlistID, order).First(&entry)
	if result.Error != nil {
		return 0, false
	}
	return entry.TrackID, true
}

func (c *PlaybackController) getPlaylistSize(playlistID string) int {
	var count int64
	c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Count(&count)
	return int(count)
}

func (c *PlaybackController) StartPlaylist(ctx *gin.Context) {
	var req struct {
		PlaylistID   string `json:"playlist_id"`
		PlaylistName string `json:"playlist_name"`
		TrackIDs     []uint `json:"track_ids"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(req.TrackIDs) == 0 {
		ctx.JSON(400, gin.H{"error": "No tracks in playlist"})
		return
	}

	for _, trackID := range req.TrackIDs {
		var track models.Track
		if err := c.db.First(&track, trackID).Error; err != nil {
			ctx.JSON(404, gin.H{"error": "Track not found", "track_id": trackID})
			return
		}
	}

	var firstTrack models.Track
	result := c.db.First(&firstTrack, req.TrackIDs[0])
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "First track not found"})
		return
	}

	var album models.Album
	c.db.First(&album, firstTrack.AlbumID)

	var playbackState models.PlaybackSession
	c.db.FirstOrCreate(&playbackState, models.PlaybackSession{PlaylistID: req.PlaylistID})

	playbackState.PlaylistID = req.PlaylistID
	playbackState.PlaylistName = req.PlaylistName
	playbackState.QueueIndex = 0
	playbackState.QueuePosition = 0
	playbackState.TrackID = req.TrackIDs[0]
	playbackState.Status = "playing"

	c.db.Save(&playbackState)

	c.db.Where("session_id = ?", req.PlaylistID).Delete(&models.SessionPlaylist{})
	var playlistEntries []models.SessionPlaylist
	for i, trackID := range req.TrackIDs {
		entry := models.SessionPlaylist{
			SessionID: req.PlaylistID,
			TrackID:   trackID,
			Order:     i + 1,
		}
		playlistEntries = append(playlistEntries, entry)
	}
	log.Printf("[DEBUG] StartPlaylist: Creating %d SessionPlaylist entries for playlistID=%s\n", len(playlistEntries), req.PlaylistID)
	c.db.Create(&playlistEntries)

	c.playbackManager.StartPlayback(playbackState.PlaylistID, &playbackState)
	c.playbackManager.SetCurrentTrack(playbackState.PlaylistID, &firstTrack)

	queueWithAlbums := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"message":     "Playlist playback started",
		"track":       c.buildTrackResponse(firstTrack, album),
		"queue":       queueWithAlbums,
		"queue_index": 0,
	})
}

func (c *PlaybackController) UpdateProgress(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
		TrackID    uint   `json:"track_id"`
		Position   int    `json:"position_seconds"`
		QueueIndex int    `json:"queue_index"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", req.PlaylistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	playbackState.TrackID = req.TrackID
	playbackState.QueuePosition = req.Position
	playbackState.QueueIndex = req.QueueIndex

	c.db.Save(&playbackState)
	c.playbackManager.UpdatePosition(req.PlaylistID, req.Position)

	ctx.JSON(200, gin.H{"status": "Progress saved"})
}

func (c *PlaybackController) GetState(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", ctx.Query("playlist_id"))
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	ctx.JSON(200, gin.H{
		"playlist_id":    playbackState.PlaylistID,
		"playlist_name":  playbackState.PlaylistName,
		"track_id":       playbackState.TrackID,
		"queue_index":    playbackState.QueueIndex,
		"queue_position": playbackState.QueuePosition,
		"status":         playbackState.Status,
	})
}

func (c *PlaybackController) Pause(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}
	playlistID := req.PlaylistID

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	c.playbackManager.PausePlayback(playlistID)
	playbackState.QueuePosition = c.playbackManager.GetPosition(playlistID)
	playbackState.Status = "paused"
	c.db.Save(&playbackState)

	ctx.JSON(200, gin.H{
		"status":      "Playback paused",
		"is_playing":  false,
		"is_paused":   true,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) Resume(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}
	playlistID := req.PlaylistID

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	c.playbackManager.ResumePlayback(playlistID)
	playbackState.Status = "playing"
	c.db.Save(&playbackState)

	ctx.JSON(200, gin.H{
		"status":      "Playback resumed",
		"is_playing":  true,
		"is_paused":   false,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) Skip(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.PlaylistID = c.playbackManager.GetCurrentPlaylistID()
	}
	playlistID := req.PlaylistID

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	playlistSize := c.getPlaylistSize(playlistID)
	if playbackState.QueueIndex >= playlistSize-1 {
		ctx.JSON(400, gin.H{"error": "No next track in queue"})
		return
	}

	playbackState.QueueIndex++
	playbackState.QueuePosition = 0

	trackID, ok := c.getTrackIDAtOrder(playlistID, playbackState.QueueIndex+1)
	if !ok {
		ctx.JSON(404, gin.H{"error": "Track not found at order"})
		return
	}
	playbackState.TrackID = trackID

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)

	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)

	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)
	if isPlaying && !isPaused {
		c.playbackManager.ResumePlayback(playlistID)
	}

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to next track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) PlayIndex(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
		QueueIndex int    `json:"queue_index"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	playlistID := req.PlaylistID
	queueIndex := req.QueueIndex

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	playlistSize := c.getPlaylistSize(playlistID)
	if queueIndex < 0 || queueIndex >= playlistSize {
		ctx.JSON(400, gin.H{"error": "Invalid queue index"})
		return
	}

	playbackState.QueueIndex = queueIndex
	playbackState.QueuePosition = 0

	trackID, ok := c.getTrackIDAtOrder(playlistID, queueIndex+1)
	if !ok {
		ctx.JSON(404, gin.H{"error": "Track not found at order"})
		return
	}
	playbackState.TrackID = trackID

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)

	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)

	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)
	if isPlaying && !isPaused {
		c.playbackManager.ResumePlayback(playlistID)
	}

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Playing track at index",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) GetCurrent(ctx *gin.Context) {
	playlistID := c.playbackManager.GetCurrentPlaylistID()
	playlistName := c.playbackManager.GetCurrentPlaylistName()

	ctx.JSON(200, gin.H{
		"is_playing":    c.playbackManager.IsPlaying(playlistID),
		"is_paused":     c.playbackManager.IsPaused(playlistID),
		"position":      c.playbackManager.GetPosition(playlistID),
		"playlist_id":   playlistID,
		"playlist_name": playlistName,
		"track":         c.playbackManager.GetCurrentTrack(),
	})
}

func (c *PlaybackController) Start(ctx *gin.Context) {
	ctx.JSON(200, gin.H{"status": "Playback started"})
}

func (c *PlaybackController) SimulateTimer(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.playbackManager.Lock()
			for _, sess := range c.playbackManager.sessions {
				if sess.IsPlaying && !sess.IsPaused {
					if c.playbackManager.currentTrack != nil {
						if sess.Position < c.playbackManager.currentTrack.Duration {
							sess.Position++
						}
					}
				}
			}
			c.playbackManager.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (c *PlaybackController) GetPlaybackManager() *PlaybackManager {
	return c.playbackManager
}
