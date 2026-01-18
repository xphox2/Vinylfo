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
	isPlaying      bool
	isPaused       bool
	currentTrack   *models.Track
	currentSession *models.PlaybackSession
	position       int
}

func NewPlaybackManager() *PlaybackManager {
	return &PlaybackManager{
		isPlaying: false,
		isPaused:  false,
		position:  0,
	}
}

func NewPlaybackController(db *gorm.DB) *PlaybackController {
	return &PlaybackController{
		db:              db,
		playbackManager: NewPlaybackManager(),
	}
}

func (pm *PlaybackManager) StartPlayback(session *models.PlaybackSession) error {
	pm.Lock()
	defer pm.Unlock()
	pm.position = 0
	pm.currentSession = session
	pm.isPlaying = true
	pm.isPaused = false
	return nil
}

func (pm *PlaybackManager) PausePlayback() error {
	pm.Lock()
	defer pm.Unlock()
	if pm.isPlaying {
		pm.isPaused = true
		pm.isPlaying = false
	}
	return nil
}

func (pm *PlaybackManager) ResumePlayback() error {
	pm.Lock()
	defer pm.Unlock()
	if pm.isPaused {
		pm.isPaused = false
		pm.isPlaying = true
	}
	return nil
}

func (pm *PlaybackManager) StopPlayback() error {
	pm.Lock()
	defer pm.Unlock()
	pm.isPlaying = false
	pm.isPaused = false
	pm.position = 0
	pm.currentSession = nil
	pm.currentTrack = nil
	return nil
}

func (pm *PlaybackManager) IsPlaying() bool {
	pm.RLock()
	defer pm.RUnlock()
	return pm.isPlaying
}

func (pm *PlaybackManager) IsPaused() bool {
	pm.RLock()
	defer pm.RUnlock()
	return pm.isPaused
}

func (pm *PlaybackManager) GetCurrentPosition() int {
	pm.RLock()
	defer pm.RUnlock()
	return pm.position
}

func (pm *PlaybackManager) GetCurrentSession() *models.PlaybackSession {
	pm.RLock()
	defer pm.RUnlock()
	return pm.currentSession
}

func (pm *PlaybackManager) UpdatePosition(position int) {
	pm.Lock()
	defer pm.Unlock()
	pm.position = position
}

func (pm *PlaybackManager) GetCurrentTrack() *models.Track {
	pm.RLock()
	defer pm.RUnlock()
	return pm.currentTrack
}

func (pm *PlaybackManager) SetCurrentTrack(track *models.Track) {
	pm.Lock()
	defer pm.Unlock()
	pm.currentTrack = track
}

func (pm *PlaybackManager) SetCurrentSession(session *models.PlaybackSession) {
	pm.Lock()
	defer pm.Unlock()
	pm.currentSession = session
}

func (c *PlaybackController) GetPlaybackState(ctx *gin.Context) {
	session := c.playbackManager.GetCurrentSession()
	var playbackState models.PlaybackSession
	c.db.First(&playbackState, 1)

	inMemoryPosition := c.playbackManager.GetCurrentPosition()
	isPlaying := c.playbackManager.IsPlaying()
	isPaused := c.playbackManager.IsPaused()

	currentPosition := inMemoryPosition
	if !isPlaying && !isPaused && playbackState.ID > 0 {
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
		"session":       session,
		"queue_index":   playbackState.QueueIndex,
		"queue":         queueTracks,
		"playlist_id":   playbackState.PlaylistID,
		"playlist_name": playbackState.PlaylistName,
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
	c.db.FirstOrCreate(&playbackState, models.PlaybackSession{ID: 1})

	queueJSON, _ := json.Marshal(req.TrackIDs)

	playbackState.PlaylistID = req.PlaylistID
	playbackState.PlaylistName = req.PlaylistName
	playbackState.Queue = string(queueJSON)
	playbackState.QueueIndex = 0
	playbackState.QueuePosition = 0
	playbackState.TrackID = req.TrackIDs[0]
	playbackState.Progress = 0
	playbackState.StartTime = time.Now()

	c.db.Save(&playbackState)

	c.playbackManager.SetCurrentTrack(&firstTrack)
	c.playbackManager.SetCurrentSession(&playbackState)
	c.playbackManager.UpdatePosition(0)
	c.playbackManager.ResumePlayback()

	queueWithAlbums := c.getQueueTracks(string(queueJSON))

	ctx.JSON(200, gin.H{
		"message":       "Playlist playback started",
		"track":         c.buildTrackResponse(firstTrack, album),
		"queue":         queueWithAlbums,
		"queue_index":   0,
		"playlist_name": req.PlaylistName,
	})
}

func (c *PlaybackController) UpdateProgress(ctx *gin.Context) {
	var req struct {
		TrackID    uint `json:"track_id"`
		Position   int  `json:"position_seconds"`
		QueueIndex int  `json:"queue_index"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var playbackState models.PlaybackSession
	c.db.FirstOrCreate(&playbackState, models.PlaybackSession{ID: 1})

	playbackState.TrackID = req.TrackID
	playbackState.QueuePosition = req.Position
	playbackState.QueueIndex = req.QueueIndex
	playbackState.Progress = req.Position

	c.db.Save(&playbackState)
	c.playbackManager.UpdatePosition(req.Position)

	ctx.JSON(200, gin.H{"status": "Progress saved"})
}

func (c *PlaybackController) GetState(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, 1)

	if result.Error != nil {
		ctx.JSON(200, gin.H{"has_state": false})
		return
	}

	if time.Since(playbackState.UpdatedAt) > 24*time.Hour {
		ctx.JSON(200, gin.H{"has_state": false, "stale": true})
		return
	}

	currentPosition := c.playbackManager.GetCurrentPosition()
	isPlaying := c.playbackManager.IsPlaying()
	isPaused := c.playbackManager.IsPaused()
	now := time.Now()

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
		"server_time":    now.UTC().Format(time.RFC3339),
		"last_update":    now.UTC().Format(time.RFC3339),
	})
}

func (c *PlaybackController) Pause(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	if err := c.db.First(&playbackState, 1).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	c.playbackManager.PausePlayback()
	playbackState.Progress = c.playbackManager.GetCurrentPosition()
	c.db.Save(&playbackState)

	ctx.JSON(200, gin.H{
		"status":     "Playback paused",
		"is_playing": false,
		"is_paused":  true,
	})
}

func (c *PlaybackController) Resume(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	if err := c.db.First(&playbackState, 1).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	c.playbackManager.ResumePlayback()
	playbackState.Progress = c.playbackManager.GetCurrentPosition()
	c.db.Save(&playbackState)

	ctx.JSON(200, gin.H{
		"status":     "Playback resumed",
		"is_playing": true,
		"is_paused":  false,
	})
}

func (c *PlaybackController) Skip(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	if err := c.db.First(&playbackState, 1).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
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

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)
	newTrack.AlbumTitle = album.Title

	c.playbackManager.SetCurrentTrack(&newTrack)
	c.playbackManager.SetCurrentSession(&playbackState)
	c.playbackManager.UpdatePosition(0)
	c.playbackManager.ResumePlayback()

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.Queue)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to next track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
	})
}

func (c *PlaybackController) Previous(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	if err := c.db.First(&playbackState, 1).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
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

	var newTrack models.Track
	c.db.First(&newTrack, playbackState.TrackID)
	var album models.Album
	c.db.First(&album, newTrack.AlbumID)
	newTrack.AlbumTitle = album.Title

	c.playbackManager.SetCurrentTrack(&newTrack)
	c.playbackManager.SetCurrentSession(&playbackState)
	c.playbackManager.UpdatePosition(0)
	c.playbackManager.ResumePlayback()

	c.db.Save(&playbackState)

	queueTracks := c.getQueueTracks(playbackState.Queue)

	ctx.JSON(200, gin.H{
		"status":      "Skipped to previous track",
		"track":       c.buildTrackResponse(newTrack, album),
		"queue":       queueTracks,
		"queue_index": playbackState.QueueIndex,
		"is_playing":  true,
	})
}

func (c *PlaybackController) Stop(ctx *gin.Context) {
	c.playbackManager.StopPlayback()
	ctx.JSON(200, gin.H{"status": "Playback stopped"})
}

func (c *PlaybackController) Clear(ctx *gin.Context) {
	var playbackState models.PlaybackSession
	c.db.FirstOrCreate(&playbackState, models.PlaybackSession{ID: 1})

	playbackState.PlaylistID = ""
	playbackState.PlaylistName = ""
	playbackState.Queue = "[]"
	playbackState.QueueIndex = 0
	playbackState.QueuePosition = 0
	playbackState.TrackID = 0
	playbackState.Progress = 0

	c.db.Save(&playbackState)

	c.playbackManager.StopPlayback()
	c.playbackManager.SetCurrentTrack(nil)
	c.playbackManager.SetCurrentSession(nil)

	ctx.JSON(200, gin.H{"status": "Playback state cleared"})
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
	ctx.JSON(200, gin.H{
		"is_playing": c.playbackManager.IsPlaying(),
		"is_paused":  c.playbackManager.IsPaused(),
		"position":   c.playbackManager.GetCurrentPosition(),
		"session":    c.playbackManager.GetCurrentSession(),
		"track":      c.playbackManager.GetCurrentTrack(),
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
		if c.playbackManager.isPlaying && !c.playbackManager.isPaused {
			if c.playbackManager.currentTrack != nil {
				if c.playbackManager.position < c.playbackManager.currentTrack.Duration {
					c.playbackManager.position++
				}
			}
		}
		c.playbackManager.Unlock()
	}
}

func (c *PlaybackController) GetPlaybackManager() *PlaybackManager {
	return c.playbackManager
}
