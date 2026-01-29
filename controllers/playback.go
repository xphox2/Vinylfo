package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

	sseClientsMux sync.RWMutex
	sseClients    map[string]*playbackSSEClient
}

type playbackSSEClient struct {
	playlistID string
	ch         chan PlaybackEvent
}

type PlaybackEvent struct {
	Type string `json:"type"`
	Data gin.H  `json:"data"`
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
	Revision        int64
	PlaybackSession *models.PlaybackSession
}

// advanceAfterTrackEnd moves the session to the next track (or stops at end).
// This is used by the server-side timer so playback continues even when no UI is open.
func (c *PlaybackController) advanceAfterTrackEnd(playlistID string) error {
	if playlistID == "" {
		return nil
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		return result.Error
	}

	playlistSize := c.getPlaylistSize(playlistID)
	if playlistSize == 0 {
		// No queue to advance; stop playback.
		c.db.Delete(&playbackState)
		c.playbackManager.StopPlayback(playlistID)
		return nil
	}

	// End of queue: stop playback.
	if playbackState.QueueIndex >= playlistSize-1 {
		c.db.Delete(&playbackState)
		c.playbackManager.StopPlayback(playlistID)
		return nil
	}

	playbackState.QueueIndex++
	playbackState.QueuePosition = 0
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()

	trackID, ok := c.getTrackIDAtOrder(playlistID, playbackState.QueueIndex+1)
	if !ok {
		// Queue is inconsistent; stop playback to avoid looping on a broken state.
		c.db.Delete(&playbackState)
		c.playbackManager.StopPlayback(playlistID)
		return nil
	}
	playbackState.TrackID = trackID
	playbackState.Status = "playing"
	playbackState.LastPlayedAt = time.Now()

	var newTrack models.Track
	if err := c.db.First(&newTrack, playbackState.TrackID).Error; err != nil {
		return err
	}

	// Persist new playback state.
	c.db.Save(&playbackState)

	// Update in-memory state (authoritative for /playback/current while playing).
	c.playbackManager.SetCurrentTrack(playlistID, &newTrack)
	c.playbackManager.UpdatePosition(playlistID, 0)
	c.playbackManager.ResumePlayback(playlistID)
	c.playbackManager.UpdateSessionState(playlistID, func(sess *PlaybackSessionState) {
		sess.IsPlaying = true
		sess.IsPaused = false
		sess.Position = 0
		if sess.PlaybackSession != nil {
			sess.PlaybackSession.TrackID = playbackState.TrackID
			sess.PlaybackSession.QueueIndex = playbackState.QueueIndex
			sess.PlaybackSession.QueuePosition = 0
			sess.PlaybackSession.Status = playbackState.Status
			sess.PlaybackSession.LastPlayedAt = playbackState.LastPlayedAt
		}
	})

	c.broadcastState(playlistID)

	return nil
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
		sseClients:      make(map[string]*playbackSSEClient),
	}
}

func (c *PlaybackController) broadcastEvent(playlistID string, event PlaybackEvent) {
	c.sseClientsMux.RLock()
	defer c.sseClientsMux.RUnlock()

	for _, client := range c.sseClients {
		if client.playlistID != "" && client.playlistID != playlistID {
			continue
		}
		select {
		case client.ch <- event:
		default:
			// Client channel full; drop.
		}
	}
}

func (c *PlaybackController) broadcastState(playlistID string) {
	state := c.buildPlaybackStateResponse(playlistID)
	c.broadcastEvent(playlistID, PlaybackEvent{Type: "state", Data: state})
}

func (c *PlaybackController) broadcastPosition(playlistID string, position int) {
	c.broadcastEvent(playlistID, PlaybackEvent{Type: "position", Data: gin.H{
		"playlist_id": playlistID,
		"position":    position,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}})
}

func (c *PlaybackController) buildPlaybackStateResponse(requestPlaylistID string) gin.H {
	playlistID := requestPlaylistID
	if playlistID == "" {
		playlistID = c.playbackManager.GetCurrentPlaylistID()
	}

	var playbackState *models.PlaybackSession
	if playlistID != "" {
		playbackState = c.playbackManager.GetSession(playlistID)

		var session models.PlaybackSession
		result := c.db.First(&session, "playlist_id = ?", playlistID)
		if result.Error == nil {
			playbackState = &session
		} else if playbackState == nil {
			log.Printf("[Playback] GetPlaybackState: no session found for playlist_id=%s: %v", playlistID, result.Error)
		}
	}

	if playbackState == nil {
		return gin.H{
			"has_state":  false,
			"is_playing": false,
			"is_paused":  false,
		}
	}

	isPlaying := c.playbackManager.IsPlaying(playlistID)
	isPaused := c.playbackManager.IsPaused(playlistID)

	var currentPosition int
	if isPlaying && !isPaused {
		elapsed := int(time.Since(playbackState.UpdatedAt).Seconds())
		currentPosition = playbackState.BasePositionSeconds + elapsed
	} else {
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
		"has_state":             true,
		"position":              currentPosition,
		"revision":              playbackState.Revision,
		"base_position_seconds": playbackState.BasePositionSeconds,
		"updated_at":            playbackState.UpdatedAt.Format(time.RFC3339),
		"server_time":           time.Now().UTC().Format(time.RFC3339),
		"last_update":           time.Now().UTC().Format(time.RFC3339),
		"playlist_id":           playlistID,
		"playlist_name":         playbackState.PlaylistName,
		"queue_index":           playbackState.QueueIndex,
		"queue":                 queueTracks,
		"queue_position":        playbackState.QueuePosition,
		"status":                playbackState.Status,
		"is_playing":            isPlaying,
		"is_paused":             isPaused,
	}

	if trackWithAlbum != nil {
		response["track"] = trackWithAlbum
	}

	return response
}

// StreamEvents is the SSE endpoint for real-time playback sync (player tabs).
func (c *PlaybackController) StreamEvents(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	playlistID := ctx.Query("playlist_id")

	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	clientChan := make(chan PlaybackEvent, 50)

	c.sseClientsMux.Lock()
	c.sseClients[clientID] = &playbackSSEClient{playlistID: playlistID, ch: clientChan}
	c.sseClientsMux.Unlock()

	defer func() {
		c.sseClientsMux.Lock()
		if client, ok := c.sseClients[clientID]; ok {
			delete(c.sseClients, clientID)
			close(client.ch)
		}
		c.sseClientsMux.Unlock()
	}()

	// Initial state snapshot.
	clientChan <- PlaybackEvent{Type: "state", Data: c.buildPlaybackStateResponse(playlistID)}

	ctx.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-clientChan:
			if !ok {
				return false
			}
			data, _ := json.Marshal(event)
			ctx.SSEvent("message", string(data))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return true
		case <-ctx.Request.Context().Done():
			return false
		}
	})
}

func (pm *PlaybackManager) StartPlayback(playlistID string, session *models.PlaybackSession) {
	pm.Lock()
	defer pm.Unlock()
	log.Printf("[DEBUG] StartPlayback called with playlistID=%s, session.PlaylistName=%s\n", playlistID, session.PlaylistName)
	pm.sessions[playlistID] = &PlaybackSessionState{
		IsPlaying:       true,
		IsPaused:        false,
		Position:        0,
		Revision:        1,
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
		sess.Revision++
		sess.PlaybackSession.Status = "paused"
		sess.PlaybackSession.LastPlayedAt = time.Now()
		sess.PlaybackSession.Revision = sess.Revision
	}
}

func (pm *PlaybackManager) ResumePlayback(playlistID string) {
	pm.Lock()
	defer pm.Unlock()
	if sess, ok := pm.sessions[playlistID]; ok {
		sess.IsPaused = false
		sess.IsPlaying = true
		sess.Revision++
		sess.PlaybackSession.Status = "playing"
		sess.PlaybackSession.Revision = sess.Revision
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
	ctx.JSON(200, c.buildPlaybackStateResponse(playlistID))
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
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
	playbackState.Revision++

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
	c.broadcastState(playlistID)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to previous track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"revision":    playbackState.Revision,
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
	c.broadcastState(playlistID)

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
	c.broadcastState(playlistID)

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
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
	playbackState.LastPlayedAt = time.Now()
	c.db.Save(&playbackState)

	c.playbackManager.SetCurrentTrack(playlistID, &track)
	c.playbackManager.StartPlayback(playlistID, &playbackState)
	c.broadcastState(playlistID)

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
		log.Printf("[DEBUG] buildTrackResponse: Found YouTube match for track %d: videoID=%s, status=%s", track.ID, youtubeVideoID, youtubeMatch.Status)
	} else {
		log.Printf("[DEBUG] buildTrackResponse: No YouTube match found for track %d (status=matched). Error: %v", track.ID, result.Error)
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
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
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
	c.broadcastState(playbackState.PlaylistID)

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

	if playbackState.Status == "playing" && !c.playbackManager.IsPaused(req.PlaylistID) {
		ctx.JSON(400, gin.H{"error": "Cannot update progress while playing - use /playback/seek instead"})
		return
	}

	playbackState.TrackID = req.TrackID
	playbackState.QueueIndex = req.QueueIndex
	playbackState.QueuePosition = req.Position
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
	c.db.Save(&playbackState)

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

	if playbackState.Status == "paused" || c.playbackManager.IsPaused(playlistID) {
		ctx.JSON(200, gin.H{
			"status":      "Playback already paused",
			"is_playing":  false,
			"is_paused":   true,
			"revision":    playbackState.Revision,
			"playlist_id": playlistID,
		})
		return
	}

	c.playbackManager.PausePlayback(playlistID)
	elapsed := int(time.Since(playbackState.UpdatedAt).Seconds())
	playbackState.QueuePosition = playbackState.BasePositionSeconds + elapsed
	playbackState.BasePositionSeconds = 0
	playbackState.Status = "paused"
	playbackState.UpdatedAt = time.Now()
	playbackState.Revision++
	c.db.Save(&playbackState)
	c.broadcastState(playlistID)

	ctx.JSON(200, gin.H{
		"status":      "Playback paused",
		"is_playing":  false,
		"is_paused":   true,
		"revision":    playbackState.Revision,
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
	playbackState.BasePositionSeconds = playbackState.QueuePosition
	playbackState.UpdatedAt = time.Now()
	playbackState.Status = "playing"
	playbackState.Revision++
	c.db.Save(&playbackState)
	c.broadcastState(playlistID)

	ctx.JSON(200, gin.H{
		"status":      "Playback resumed",
		"is_playing":  true,
		"is_paused":   false,
		"revision":    playbackState.Revision,
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
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
	playbackState.Revision++

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
	c.broadcastState(playlistID)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to next track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"revision":    playbackState.Revision,
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
	playbackState.BasePositionSeconds = 0
	playbackState.UpdatedAt = time.Now()
	playbackState.Revision++

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
	c.broadcastState(playlistID)

	queueTracks := c.getQueueTracks(playbackState.PlaylistID)

	ctx.JSON(200, gin.H{
		"status":      "Playing track at index",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"revision":    playbackState.Revision,
		"is_playing":  true,
		"playlist_id": playlistID,
	})
}

func (c *PlaybackController) GetCurrent(ctx *gin.Context) {
	playlistID := c.playbackManager.GetCurrentPlaylistID()
	playlistName := c.playbackManager.GetCurrentPlaylistName()

	if playlistID == "" {
		var session models.PlaybackSession
		result := c.db.Order("updated_at DESC").First(&session)
		if result.Error == nil {
			playlistID = session.PlaylistID
			playlistName = session.PlaylistName

			var track models.Track
			if session.TrackID > 0 {
				c.db.First(&track, session.TrackID)
			}

			var album models.Album
			if track.AlbumID > 0 {
				c.db.First(&album, track.AlbumID)
			}

			trackResponse := c.buildTrackResponse(track, album)

			ctx.JSON(200, gin.H{
				"is_playing":    false,
				"is_paused":     true,
				"position":      session.QueuePosition,
				"playlist_id":   playlistID,
				"playlist_name": playlistName,
				"track":         trackResponse,
				"queue_index":   session.QueueIndex,
				"revision":      session.Revision,
				"status":        session.Status,
				"has_state":     true,
			})
			return
		}
	}

	ctx.JSON(200, gin.H{
		"is_playing":    c.playbackManager.IsPlaying(playlistID),
		"is_paused":     c.playbackManager.IsPaused(playlistID),
		"position":      c.playbackManager.GetPosition(playlistID),
		"playlist_id":   playlistID,
		"playlist_name": playlistName,
		"track":         c.playbackManager.GetCurrentTrack(),
		"has_state":     playlistID != "",
	})
}

func (c *PlaybackController) Seek(ctx *gin.Context) {
	var req struct {
		PlaylistID string `json:"playlist_id"`
		Position   int    `json:"position_seconds"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "Invalid request: "+err.Error())
		return
	}

	playlistID := req.PlaylistID
	if playlistID == "" {
		playlistID = c.playbackManager.GetCurrentPlaylistID()
	}
	if playlistID == "" {
		utils.BadRequest(ctx, "No playlist ID provided")
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		utils.NotFound(ctx, "No playback state found")
		return
	}

	var track models.Track
	if playbackState.TrackID > 0 {
		c.db.First(&track, playbackState.TrackID)
	}

	if track.Duration > 0 && req.Position > track.Duration {
		req.Position = track.Duration
	}
	if req.Position < 0 {
		req.Position = 0
	}

	c.playbackManager.UpdatePosition(playlistID, req.Position)
	playbackState.QueuePosition = req.Position
	playbackState.BasePositionSeconds = req.Position
	playbackState.UpdatedAt = time.Now()
	playbackState.LastPlayedAt = time.Now()
	playbackState.Revision++
	c.db.Save(&playbackState)
	c.broadcastState(playlistID)

	c.playbackManager.UpdateSessionState(playlistID, func(sess *PlaybackSessionState) {
		sess.Revision = playbackState.Revision
	})

	var album models.Album
	if track.AlbumID > 0 {
		c.db.First(&album, track.AlbumID)
	}

	log.Printf("[Playback] Seek: playlistID=%s, position=%d, revision=%d", playlistID, req.Position, playbackState.Revision)

	ctx.JSON(200, gin.H{
		"status":      "Seeked",
		"position":    req.Position,
		"revision":    playbackState.Revision,
		"playlist_id": playlistID,
		"track":       c.buildTrackResponse(track, album),
		"is_playing":  c.playbackManager.IsPlaying(playlistID),
		"is_paused":   c.playbackManager.IsPaused(playlistID),
		"last_update": time.Now().UTC().Format(time.RFC3339),
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
			var advancePlaylistID string
			var positionPlaylistID string
			var positionToBroadcast int

			c.playbackManager.Lock()
			playlistID := c.playbackManager.playlistID
			sess := c.playbackManager.sessions[playlistID]
			track := c.playbackManager.currentTrack
			if playlistID != "" && sess != nil && track != nil && sess.IsPlaying && !sess.IsPaused {
				elapsed := int(time.Since(sess.PlaybackSession.UpdatedAt).Seconds())
				currentPosition := sess.PlaybackSession.BasePositionSeconds + elapsed

				if track.Duration <= 0 || currentPosition < track.Duration {
					sess.Position = currentPosition
					positionPlaylistID = playlistID
					positionToBroadcast = currentPosition
				} else {
					advancePlaylistID = playlistID
				}
			}
			c.playbackManager.Unlock()

			if positionPlaylistID != "" {
				c.broadcastPosition(positionPlaylistID, positionToBroadcast)
			}

			if advancePlaylistID != "" {
				if err := c.advanceAfterTrackEnd(advancePlaylistID); err != nil {
					log.Printf("[Playback] advanceAfterTrackEnd failed (playlist_id=%s): %v", advancePlaylistID, err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *PlaybackController) GetPlaybackManager() *PlaybackManager {
	return c.playbackManager
}
