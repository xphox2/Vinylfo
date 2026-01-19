package controllers

import (
	"encoding/json"
	"sync"
	"time"

	"vinylfo/models"

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
	pm.sessions[playlistID] = &PlaybackSessionState{
		IsPlaying:       true,
		IsPaused:        false,
		Position:        0,
		PlaybackSession: session,
	}
	pm.playlistID = playlistID
	pm.playlistName = session.PlaylistName
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
		return sess.IsPlaying
	}
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
		if playbackState == nil {
			var session models.PlaybackSession
			result := c.db.First(&session, "playlist_id = ?", playlistID)
			if result.Error == nil {
				playbackState = &session
			}
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

	queueTracks := c.getQueueTracks(playbackState.Queue)

	response := gin.H{
		"is_playing":    isPlaying,
		"is_paused":     isPaused,
		"position":      currentPosition,
		"server_time":   time.Now().UTC().Format(time.RFC3339),
		"last_update":   time.Now().UTC().Format(time.RFC3339),
		"playlist_id":   playlistID,
		"playlist_name": playbackState.PlaylistName,
		"queue_index":   playbackState.QueueIndex,
		"queue":         queueTracks,
		"status":        playbackState.Status,
	}

	if trackWithAlbum != nil {
		response["track"] = trackWithAlbum
	}

	ctx.JSON(200, response)
}

func (c *PlaybackController) buildTrackResponse(track models.Track, album models.Album) map[string]interface{} {
	return map[string]interface{}{
		"id":             track.ID,
		"album_id":       track.AlbumID,
		"album_title":    album.Title,
		"album_artist":   album.Artist,
		"title":          track.Title,
		"duration":       track.Duration,
		"track_number":   track.TrackNumber,
		"audio_file_url": track.AudioFileURL,
		"release_year":   album.ReleaseYear,
		"album_genre":    album.Genre,
		"created_at":     track.CreatedAt,
		"updated_at":     track.UpdatedAt,
	}
}

func (c *PlaybackController) getQueueTracks(queueJSON string) []map[string]interface{} {
	var queueTrackIDs []uint
	json.Unmarshal([]byte(queueJSON), &queueTrackIDs)

	var queueTracks []map[string]interface{}
	if len(queueTrackIDs) > 0 {
		var allQueueTracks []models.Track
		c.db.Find(&allQueueTracks, queueTrackIDs)

		trackMap := make(map[uint]models.Track)
		for _, track := range allQueueTracks {
			trackMap[track.ID] = track
		}

		for _, trackID := range queueTrackIDs {
			if track, ok := trackMap[trackID]; ok {
				var album models.Album
				c.db.First(&album, track.AlbumID)
				queueTracks = append(queueTracks, c.buildTrackResponse(track, album))
			}
		}
	}
	return queueTracks
}

func (c *PlaybackController) StartPlaylist(ctx *gin.Context) {
	var req struct {
		PlaylistID   string `json:"playlist_id" binding:"required"`
		PlaylistName string `json:"playlist_name"`
		TrackIDs     []uint `json:"track_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(req.TrackIDs) == 0 {
		ctx.JSON(400, gin.H{"error": "No tracks in playlist"})
		return
	}

	var firstTrack models.Track
	result := c.db.First(&firstTrack, req.TrackIDs[0])
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "First track not found"})
		return
	}

	var album models.Album
	c.db.First(&album, firstTrack.AlbumID)
	firstTrack.AlbumTitle = album.Title

	var playbackState models.PlaybackSession
	result = c.db.First(&playbackState, "playlist_id = ?", req.PlaylistID)
	if result.Error != nil {
		playbackState = models.PlaybackSession{
			PlaylistID:   req.PlaylistID,
			PlaylistName: req.PlaylistName,
		}
	}

	queueJSON, _ := json.Marshal(req.TrackIDs)

	playbackState.PlaylistID = req.PlaylistID
	playbackState.PlaylistName = req.PlaylistName
	playbackState.Queue = string(queueJSON)
	playbackState.QueueIndex = 0
	playbackState.QueuePosition = 0
	playbackState.TrackID = req.TrackIDs[0]
	playbackState.Status = "playing"
	playbackState.StartedAt = time.Now()
	playbackState.LastPlayedAt = time.Now()

	c.db.Save(&playbackState)

	c.playbackManager.SetCurrentTrack(req.PlaylistID, &firstTrack)
	c.playbackManager.StartPlayback(req.PlaylistID, &playbackState)

	queueWithAlbums := c.getQueueTracks(string(queueJSON))

	ctx.JSON(200, gin.H{
		"message":       "Playlist playback started",
		"track":         c.buildTrackResponse(firstTrack, album),
		"queue":         queueWithAlbums,
		"queue_index":   0,
		"playlist_id":   req.PlaylistID,
		"playlist_name": req.PlaylistName,
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

	playlistID := req.PlaylistID
	if playlistID == "" {
		playlistID = c.playbackManager.GetCurrentPlaylistID()
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No session found for playlist"})
		return
	}

	playbackState.TrackID = req.TrackID
	playbackState.QueuePosition = req.Position
	playbackState.QueueIndex = req.QueueIndex
	playbackState.LastPlayedAt = time.Now()

	c.db.Save(&playbackState)
	c.playbackManager.UpdatePosition(playlistID, req.Position)

	ctx.JSON(200, gin.H{"status": "Progress saved"})
}

func (c *PlaybackController) GetState(ctx *gin.Context) {
	playlistID := ctx.Query("playlist_id")
	if playlistID == "" {
		playlistID = c.playbackManager.GetCurrentPlaylistID()
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)

	if result.Error != nil {
		ctx.JSON(200, gin.H{"has_state": false})
		return
	}

	currentPosition := c.playbackManager.GetPosition(playlistID)
	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)

	var track models.Track
	c.db.First(&track, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, track.AlbumID)

	queueWithAlbums := c.getQueueTracks(playbackState.Queue)

	ctx.JSON(200, gin.H{
		"has_state":      true,
		"track":          c.buildTrackResponse(track, album),
		"playlist_id":    playbackState.PlaylistID,
		"playlist_name":  playbackState.PlaylistName,
		"queue":          queueWithAlbums,
		"queue_index":    playbackState.QueueIndex,
		"queue_position": currentPosition,
		"is_playing":     isPlaying,
		"is_paused":      isPaused,
		"status":         playbackState.Status,
		"server_time":    time.Now().UTC().Format(time.RFC3339),
		"last_update":    time.Now().UTC().Format(time.RFC3339),
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
	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No playlist ID provided and no active session"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No session found"})
		return
	}

	c.playbackManager.PausePlayback(playlistID)
	playbackState.Status = "paused"
	playbackState.QueuePosition = c.playbackManager.GetPosition(playlistID)
	playbackState.LastPlayedAt = time.Now()
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
	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No playlist ID provided and no active session"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No session found"})
		return
	}

	c.playbackManager.ResumePlayback(playlistID)
	playbackState.Status = "playing"
	playbackState.LastPlayedAt = time.Now()
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
	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No playlist ID provided"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No session found"})
		return
	}

	var queueTrackIDs []uint
	json.Unmarshal([]byte(playbackState.Queue), &queueTrackIDs)

	if playbackState.QueueIndex >= len(queueTrackIDs)-1 {
		ctx.JSON(400, gin.H{"error": "No next track in queue"})
		return
	}

	playbackState.QueueIndex++
	playbackState.QueuePosition = 0
	playbackState.TrackID = queueTrackIDs[playbackState.QueueIndex]
	playbackState.Status = "playing"

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)
	newTrack.AlbumTitle = album.Title

	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)
	c.playbackManager.ResumePlayback(playlistID)

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.Queue)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to next track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
		"playlist_id": playlistID,
	})
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
		ctx.JSON(400, gin.H{"error": "No playlist ID provided"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No session found"})
		return
	}

	var queueTrackIDs []uint
	json.Unmarshal([]byte(playbackState.Queue), &queueTrackIDs)

	if playbackState.QueueIndex <= 0 {
		ctx.JSON(400, gin.H{"error": "No previous track in queue"})
		return
	}

	playbackState.QueueIndex--
	playbackState.QueuePosition = 0
	playbackState.TrackID = queueTrackIDs[playbackState.QueueIndex]
	playbackState.Status = "playing"

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)
	newTrack.AlbumTitle = album.Title

	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)
	c.playbackManager.ResumePlayback(playlistID)

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.Queue)

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
		ctx.JSON(400, gin.H{"error": "No playlist ID provided"})
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
		ctx.JSON(400, gin.H{"error": "No playlist ID provided"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error == nil {
		playbackState.PlaylistID = ""
		playbackState.PlaylistName = ""
		playbackState.Queue = "[]"
		playbackState.QueueIndex = 0
		playbackState.QueuePosition = 0
		playbackState.TrackID = 0
		playbackState.Status = "stopped"
		c.db.Save(&playbackState)
	}

	c.playbackManager.StopPlayback(playlistID)

	ctx.JSON(200, gin.H{"status": "Playback state cleared"})
}

func (c *PlaybackController) RestoreSession(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	playlistID := req.PlaylistID

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session not found"})
		return
	}

	var queueTrackIDs []uint
	if err := json.Unmarshal([]byte(playbackState.Queue), &queueTrackIDs); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid queue data"})
		return
	}

	if len(queueTrackIDs) == 0 {
		ctx.JSON(400, gin.H{"error": "Session has no tracks"})
		return
	}

	var track models.Track
	result = c.db.First(&track, queueTrackIDs[playbackState.QueueIndex])
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	var album models.Album
	c.db.First(&album, track.AlbumID)
	track.AlbumTitle = album.Title

	playbackState.Status = "playing"
	playbackState.LastPlayedAt = time.Now()
	c.db.Save(&playbackState)

	c.playbackManager.SetCurrentTrack(playlistID, &track)
	c.playbackManager.StartPlayback(playlistID, &playbackState)

	queueWithAlbums := c.getQueueTracks(playbackState.Queue)

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

func (c *PlaybackController) GetHistory(ctx *gin.Context) {
	var history []models.TrackHistory
	result := c.db.Order("listen_count DESC, last_played DESC").Find(&history)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		var track models.Track
		c.db.First(&track, h.TrackID)
		resultWithTracks = append(resultWithTracks, HistoryWithTrack{
			History: h,
			Track:   track,
		})
	}

	ctx.JSON(200, resultWithTracks)
}

func (c *PlaybackController) GetMostPlayed(ctx *gin.Context) {
	var history []models.TrackHistory
	result := c.db.Order("listen_count DESC").Limit(10).Find(&history)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		var track models.Track
		c.db.First(&track, h.TrackID)
		if track.ID != 0 {
			resultWithTracks = append(resultWithTracks, HistoryWithTrack{
				History: h,
				Track:   track,
			})
		}
	}

	ctx.JSON(200, resultWithTracks)
}

func (c *PlaybackController) GetRecent(ctx *gin.Context) {
	var history []models.TrackHistory
	result := c.db.Order("last_played DESC").Limit(10).Find(&history)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		var track models.Track
		c.db.First(&track, h.TrackID)
		if track.ID != 0 {
			resultWithTracks = append(resultWithTracks, HistoryWithTrack{
				History: h,
				Track:   track,
			})
		}
	}

	ctx.JSON(200, resultWithTracks)
}

func (c *PlaybackController) GetTrackHistory(ctx *gin.Context) {
	trackID := ctx.Param("track_id")
	var history models.TrackHistory
	result := c.db.Where("track_id = ?", trackID).First(&history)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "History not found"})
		return
	}
	ctx.JSON(200, history)
}

func (c *PlaybackController) UpdateHistory(ctx *gin.Context) {
	var req struct {
		TrackID    uint   `json:"track_id"`
		PlaylistID string `json:"playlist_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var history models.TrackHistory
	c.db.Where("track_id = ?", req.TrackID).FirstOrCreate(&history, models.TrackHistory{
		TrackID:    req.TrackID,
		PlaylistID: req.PlaylistID,
	})

	history.ListenCount++
	history.LastPlayed = time.Now()
	c.db.Save(&history)

	ctx.JSON(200, gin.H{"status": "History updated"})
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

func (c *PlaybackController) SimulateTimer() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
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
	}
}

func (c *PlaybackController) GetPlaybackManager() *PlaybackManager {
	return c.playbackManager
}
