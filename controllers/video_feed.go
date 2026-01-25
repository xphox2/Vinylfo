package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"vinylfo/duration"
	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VideoFeedController struct {
	db                 *gorm.DB
	playbackController *PlaybackController
	sseClients         map[string]chan VideoFeedEvent
	sseClientsMux      sync.RWMutex
	lastState          *VideoFeedState
	lastStateMux       sync.RWMutex
	youtubeOAuth       *duration.YouTubeOAuthClient
}

type VideoFeedState struct {
	TrackID     uint      `json:"track_id"`
	IsPlaying   bool      `json:"is_playing"`
	IsPaused    bool      `json:"is_paused"`
	Position    int       `json:"position"`
	LastUpdated time.Time `json:"last_updated"`
}

type VideoFeedEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type VideoTrackInfo struct {
	TrackID        uint    `json:"track_id"`
	TrackTitle     string  `json:"track_title"`
	Artist         string  `json:"artist"`
	AlbumTitle     string  `json:"album_title"`
	AlbumArtURL    string  `json:"album_art_url"`
	Duration       int     `json:"duration"`
	HasVideo       bool    `json:"has_video"`
	YouTubeVideoID string  `json:"youtube_video_id,omitempty"`
	VideoTitle     string  `json:"video_title,omitempty"`
	VideoDuration  int     `json:"video_duration,omitempty"`
	ThumbnailURL   string  `json:"thumbnail_url,omitempty"`
	MatchScore     float64 `json:"match_score,omitempty"`
}

func NewVideoFeedController(db *gorm.DB, playbackController *PlaybackController, youtubeOAuth *duration.YouTubeOAuthClient) *VideoFeedController {
	vfc := &VideoFeedController{
		db:                 db,
		playbackController: playbackController,
		sseClients:         make(map[string]chan VideoFeedEvent),
		youtubeOAuth:       youtubeOAuth,
	}

	// Start background state broadcaster
	go vfc.stateMonitor()

	return vfc
}

// GetVideoFeedPage serves the video feed HTML template
func (c *VideoFeedController) GetVideoFeedPage(ctx *gin.Context) {
	ctx.HTML(200, "video-feed.html", gin.H{
		"overlay":         ctx.DefaultQuery("overlay", "bottom"),
		"theme":           ctx.DefaultQuery("theme", "dark"),
		"transition":      ctx.DefaultQuery("transition", "fade"),
		"showVisualizer":  ctx.DefaultQuery("showVisualizer", "true"),
		"quality":         ctx.DefaultQuery("quality", "auto"),
		"overlayDuration": ctx.DefaultQuery("overlayDuration", "5"),
	})
}

// GetCurrentYouTubeVideo returns the current track's YouTube video info
func (c *VideoFeedController) GetCurrentYouTubeVideo(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	currentTrack := pm.GetCurrentTrack()
	playlistID := pm.GetCurrentPlaylistID()

	if currentTrack == nil {
		ctx.JSON(200, gin.H{
			"has_track":  false,
			"is_playing": false,
			"is_paused":  false,
		})
		return
	}

	trackInfo := c.buildVideoTrackInfo(currentTrack)

	ctx.JSON(200, gin.H{
		"has_track":   true,
		"track":       trackInfo,
		"is_playing":  pm.IsPlaying(playlistID),
		"is_paused":   pm.IsPaused(playlistID),
		"position":    pm.GetPosition(playlistID),
		"playlist_id": playlistID,
	})
}

// GetNextTrackPreload returns the next track's info for preloading
func (c *VideoFeedController) GetNextTrackPreload(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(200, gin.H{"has_next": false})
		return
	}

	session := pm.GetSession(playlistID)
	if session == nil {
		ctx.JSON(200, gin.H{"has_next": false})
		return
	}

	// Get playlist size
	var count int64
	c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Count(&count)

	nextIndex := session.QueueIndex + 1
	if nextIndex >= int(count) {
		ctx.JSON(200, gin.H{"has_next": false})
		return
	}

	// Get next track ID
	var nextEntry models.SessionPlaylist
	result := c.db.Where("session_id = ? AND `order` = ?", playlistID, nextIndex+1).First(&nextEntry)
	if result.Error != nil {
		ctx.JSON(200, gin.H{"has_next": false})
		return
	}

	var nextTrack models.Track
	c.db.First(&nextTrack, nextEntry.TrackID)

	trackInfo := c.buildVideoTrackInfo(&nextTrack)

	ctx.JSON(200, gin.H{
		"has_next": true,
		"track":    trackInfo,
	})
}

// StreamEvents is the SSE endpoint for real-time updates
func (c *VideoFeedController) StreamEvents(ctx *gin.Context) {
	// Set SSE headers
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("X-Accel-Buffering", "no")

	// Create client channel
	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	clientChan := make(chan VideoFeedEvent, 10)

	c.sseClientsMux.Lock()
	c.sseClients[clientID] = clientChan
	c.sseClientsMux.Unlock()

	// Clean up on disconnect
	defer func() {
		c.sseClientsMux.Lock()
		delete(c.sseClients, clientID)
		close(clientChan)
		c.sseClientsMux.Unlock()
	}()

	// Send initial state
	c.sendCurrentState(clientChan)

	// Stream events
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

// Play triggers video playback
func (c *VideoFeedController) Play(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No active playlist"})
		return
	}

	pm.ResumePlayback(playlistID)
	c.broadcastEvent(VideoFeedEvent{
		Type: "playback_state",
		Data: gin.H{
			"is_playing": true,
			"is_paused":  false,
		},
	})

	ctx.JSON(200, gin.H{"status": "Playing"})
}

// Pause pauses video playback
func (c *VideoFeedController) Pause(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No active playlist"})
		return
	}

	pm.PausePlayback(playlistID)
	c.broadcastEvent(VideoFeedEvent{
		Type: "playback_state",
		Data: gin.H{
			"is_playing": false,
			"is_paused":  true,
		},
	})

	ctx.JSON(200, gin.H{"status": "Paused"})
}

// Stop stops video playback (pauses and resets position, but preserves session)
func (c *VideoFeedController) Stop(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID != "" {
		// Just pause, don't delete the session - user can resume later
		pm.PausePlayback(playlistID)
		pm.UpdatePosition(playlistID, 0) // Reset position to start

		// Update database state
		var playbackState models.PlaybackSession
		if c.db.First(&playbackState, "playlist_id = ?", playlistID).Error == nil {
			playbackState.Status = "stopped"
			playbackState.QueuePosition = 0
			c.db.Save(&playbackState)
		}
	}

	c.broadcastEvent(VideoFeedEvent{
		Type: "playback_state",
		Data: gin.H{
			"is_playing": false,
			"is_paused":  true,
			"stopped":    true,
		},
	})

	ctx.JSON(200, gin.H{"status": "Stopped"})
}

// Next skips to next video
func (c *VideoFeedController) Next(ctx *gin.Context) {
	// Use the existing playback controller's Skip method logic
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No active playlist"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	// Get playlist size
	var count int64
	c.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Count(&count)

	if playbackState.QueueIndex >= int(count)-1 {
		ctx.JSON(400, gin.H{"error": "No next track in queue"})
		return
	}

	playbackState.QueueIndex++
	playbackState.QueuePosition = 0

	// Get track at new position
	var entry models.SessionPlaylist
	c.db.Where("session_id = ? AND `order` = ?", playlistID, playbackState.QueueIndex+1).First(&entry)
	playbackState.TrackID = entry.TrackID

	var newTrack models.Track
	c.db.First(&newTrack, entry.TrackID)

	pm.SetCurrentTrack(playlistID, &newTrack)
	pm.UpdatePosition(playlistID, 0)

	c.db.Save(&playbackState)

	trackInfo := c.buildVideoTrackInfo(&newTrack)

	pm.SetCurrentTrack(playlistID, &newTrack)
	pm.UpdatePosition(playlistID, 0)

	c.db.Save(&playbackState)

	c.broadcastEvent(VideoFeedEvent{
		Type: "track_changed",
		Data: gin.H{
			"track":       trackInfo,
			"queue_index": playbackState.QueueIndex,
			"is_playing":  true,
			"is_paused":   false,
			"position":    0,
		},
	})

	ctx.JSON(200, gin.H{
		"status": "Skipped to next track",
		"track":  trackInfo,
	})
}

// Previous goes to previous video
func (c *VideoFeedController) Previous(ctx *gin.Context) {
	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No active playlist"})
		return
	}

	var playbackState models.PlaybackSession
	result := c.db.First(&playbackState, "playlist_id = ?", playlistID)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No playback state found"})
		return
	}

	if playbackState.QueueIndex <= 0 {
		ctx.JSON(400, gin.H{"error": "No previous track in queue"})
		return
	}

	playbackState.QueueIndex--
	playbackState.QueuePosition = 0

	// Get track at new position
	var entry models.SessionPlaylist
	c.db.Where("session_id = ? AND `order` = ?", playlistID, playbackState.QueueIndex+1).First(&entry)
	playbackState.TrackID = entry.TrackID

	var newTrack models.Track
	c.db.First(&newTrack, entry.TrackID)

	pm.SetCurrentTrack(playlistID, &newTrack)
	pm.UpdatePosition(playlistID, 0)

	c.db.Save(&playbackState)

	trackInfo := c.buildVideoTrackInfo(&newTrack)

	pm.SetCurrentTrack(playlistID, &newTrack)
	pm.UpdatePosition(playlistID, 0)

	c.db.Save(&playbackState)

	c.broadcastEvent(VideoFeedEvent{
		Type: "track_changed",
		Data: gin.H{
			"track":       trackInfo,
			"queue_index": playbackState.QueueIndex,
			"is_playing":  true,
			"is_paused":   false,
			"position":    0,
		},
	})

	ctx.JSON(200, gin.H{
		"status": "Skipped to previous track",
		"track":  trackInfo,
	})
}

// Seek seeks video to position
func (c *VideoFeedController) Seek(ctx *gin.Context) {
	var req struct {
		Position int `json:"position"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	pm := c.playbackController.GetPlaybackManager()
	playlistID := pm.GetCurrentPlaylistID()

	if playlistID == "" {
		ctx.JSON(400, gin.H{"error": "No active playlist"})
		return
	}

	pm.UpdatePosition(playlistID, req.Position)

	c.broadcastEvent(VideoFeedEvent{
		Type: "position_update",
		Data: gin.H{
			"position": req.Position,
		},
	})

	ctx.JSON(200, gin.H{
		"status":   "Seeked",
		"position": req.Position,
	})
}

// buildVideoTrackInfo builds the video track info response
func (c *VideoFeedController) buildVideoTrackInfo(track *models.Track) VideoTrackInfo {
	var album models.Album
	c.db.First(&album, track.AlbumID)

	info := VideoTrackInfo{
		TrackID:     track.ID,
		TrackTitle:  duration.NormalizeTitle(track.Title),
		Artist:      duration.NormalizeArtistName(album.Artist),
		AlbumTitle:  duration.NormalizeTitle(album.Title),
		AlbumArtURL: fmt.Sprintf("/albums/%d/image", album.ID),
		Duration:    track.Duration,
		HasVideo:    false,
	}

	// Get YouTube match if exists
	var youtubeMatch models.TrackYouTubeMatch
	result := c.db.Where("track_id = ? AND (match_method IN (?, ?, ?) OR status = ?)", track.ID, "web_search", "api_search", "manual", "reviewed").First(&youtubeMatch)

	if result.Error == nil {
		info.HasVideo = true
		info.YouTubeVideoID = youtubeMatch.YouTubeVideoID
		info.VideoTitle = youtubeMatch.VideoTitle
		info.VideoDuration = youtubeMatch.VideoDuration
		info.ThumbnailURL = youtubeMatch.ThumbnailURL
		info.MatchScore = youtubeMatch.MatchScore
	} else if result.Error == gorm.ErrRecordNotFound {
		// Fallback: query directly with raw SQL
		var row map[string]interface{}
		rowResult := c.db.Raw(`SELECT * FROM track_youtube_matches WHERE track_id = ? AND (match_method IN ('web_search', 'api_search', 'manual') OR status = 'reviewed') LIMIT 1`, track.ID).Scan(&row)
		if rowResult.Error == nil && len(row) > 0 {
			if v, ok := row["youtube_video_id"].(string); ok && v != "" {
				info.HasVideo = true
				info.YouTubeVideoID = v
				if t, ok := row["video_title"].(string); ok {
					info.VideoTitle = t
				}
				if d, ok := row["video_duration"].(int64); ok {
					info.VideoDuration = int(d)
				}
				if thumb, ok := row["thumbnail_url"].(string); ok {
					info.ThumbnailURL = thumb
				}
				if s, ok := row["match_score"].(float64); ok {
					info.MatchScore = s
				}
			}
		}
	}

	return info
}

// GetYouTubeVideoDuration fetches the duration for a YouTube video ID
// This can be used when the cached duration is missing or needs refresh
func (c *VideoFeedController) GetYouTubeVideoDuration(ctx *gin.Context) {
	videoID := ctx.Query("video_id")
	if videoID == "" {
		ctx.JSON(400, gin.H{"error": "video_id is required"})
		return
	}

	var youtubeMatch models.TrackYouTubeMatch
	result := c.db.Where("youtube_video_id = ?", videoID).First(&youtubeMatch)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "No YouTube match found for this video ID"})
		return
	}

	ctx.JSON(200, gin.H{
		"video_id":       videoID,
		"video_duration": youtubeMatch.VideoDuration,
		"video_title":    youtubeMatch.VideoTitle,
	})
}

// RefreshYouTubeDuration fetches the duration from YouTube API and updates the database
func (c *VideoFeedController) RefreshYouTubeDuration(ctx *gin.Context) {
	var req struct {
		VideoID string `json:"video_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.VideoID = ctx.Query("video_id")
	}

	if req.VideoID == "" {
		ctx.JSON(400, gin.H{"error": "video_id is required"})
		return
	}

	var duration int
	var err error

	if c.youtubeOAuth != nil && c.youtubeOAuth.IsAuthenticated() {
		duration, err = c.fetchYouTubeVideoDurationWithOAuth(req.VideoID)
		if err != nil {
			log.Printf("[VideoFeed] OAuth failed, trying API key: %v", err)
			duration, err = fetchYouTubeVideoDuration(req.VideoID)
		}
	} else {
		duration, err = fetchYouTubeVideoDuration(req.VideoID)
	}

	if err != nil {
		log.Printf("[VideoFeed] Failed to fetch duration for %s: %v", req.VideoID, err)
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to fetch duration: %v", err)})
		return
	}

	pm := c.playbackController.GetPlaybackManager()
	currentTrack := pm.GetCurrentTrack()
	if currentTrack == nil {
		ctx.JSON(404, gin.H{"error": "No track currently playing"})
		return
	}

	var youtubeMatch models.TrackYouTubeMatch
	result := c.db.Where("track_id = ?", currentTrack.ID).First(&youtubeMatch)
	if result.Error != nil {
		log.Printf("[VideoFeed] No YouTube match found for track %d, creating one with video ID %s", currentTrack.ID, req.VideoID)
		youtubeMatch = models.TrackYouTubeMatch{
			TrackID:        currentTrack.ID,
			YouTubeVideoID: req.VideoID,
			VideoDuration:  duration,
			MatchScore:     1.0,
			Status:         "matched",
		}
		if err := c.db.Create(&youtubeMatch).Error; err != nil {
			ctx.JSON(500, gin.H{"error": "Failed to create YouTube match"})
			return
		}
	} else {
		youtubeMatch.VideoDuration = duration
		c.db.Save(&youtubeMatch)
	}

	log.Printf("[VideoFeed] Updated duration for track %d (video: %s): %d seconds", currentTrack.ID, req.VideoID, duration)

	ctx.JSON(200, gin.H{
		"video_id":       req.VideoID,
		"video_duration": duration,
		"success":        true,
	})
}

// RefreshAllYouTubeDurations fetches durations for all tracks with YouTube matches that have no duration
func (c *VideoFeedController) RefreshAllYouTubeDurations(ctx *gin.Context) {
	if c.youtubeOAuth == nil || !c.youtubeOAuth.IsAuthenticated() {
		ctx.JSON(401, gin.H{"error": "YouTube not connected. Please connect to YouTube first."})
		return
	}

	var matches []models.TrackYouTubeMatch
	result := c.db.Where("video_duration = 0 OR video_duration IS NULL").Where("status = ?", "matched").Find(&matches)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to find matches"})
		return
	}

	updated := 0
	failed := 0

	for _, match := range matches {
		if match.YouTubeVideoID == "" {
			continue
		}

		duration, err := c.fetchYouTubeVideoDurationWithOAuth(match.YouTubeVideoID)
		if err != nil {
			log.Printf("[VideoFeed] Failed to fetch duration for %s: %v", match.YouTubeVideoID, err)
			failed++
			continue
		}

		match.VideoDuration = duration
		c.db.Save(&match)
		updated++
	}

	ctx.JSON(200, gin.H{
		"total_matches": len(matches),
		"updated":       updated,
		"failed":        failed,
		"success":       true,
	})
}

var youtubeDurationRegex = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

// fetchYouTubeVideoDurationWithOAuth uses the user's OAuth token to fetch video duration
func (c *VideoFeedController) fetchYouTubeVideoDurationWithOAuth(videoID string) (int, error) {
	if c.youtubeOAuth == nil {
		return 0, fmt.Errorf("OAuth client not available")
	}

	if !c.youtubeOAuth.IsAuthenticated() {
		return 0, fmt.Errorf("not authenticated with YouTube")
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=contentDetails&id=%s", videoID)

	ctx := context.Background()
	resp, err := c.youtubeOAuth.MakeAuthenticatedRequest(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("YouTube API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Items) == 0 {
		return 0, fmt.Errorf("video not found")
	}

	durationStr := result.Items[0].ContentDetails.Duration
	matches := youtubeDurationRegex.FindStringSubmatch(durationStr)
	if matches == nil {
		return 0, fmt.Errorf("failed to parse duration: %s", durationStr)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])

	return hours*3600 + minutes*60 + seconds, nil
}

// fetchYouTubeVideoDuration is a standalone function that tries to use API key as fallback
func fetchYouTubeVideoDuration(videoID string) (int, error) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("YouTube API key not configured (set YOUTUBE_API_KEY environment variable)")
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=contentDetails&id=%s&key=%s", videoID, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("YouTube API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Items) == 0 {
		return 0, fmt.Errorf("video not found")
	}

	durationStr := result.Items[0].ContentDetails.Duration
	matches := youtubeDurationRegex.FindStringSubmatch(durationStr)
	if matches == nil {
		return 0, fmt.Errorf("failed to parse duration: %s", durationStr)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])

	return hours*3600 + minutes*60 + seconds, nil
}

// stateMonitor monitors playback state and broadcasts changes
func (c *VideoFeedController) stateMonitor() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	log.Println("[VideoFeed] stateMonitor started")

	for range ticker.C {
		pm := c.playbackController.GetPlaybackManager()
		currentTrack := pm.GetCurrentTrack()
		playlistID := pm.GetCurrentPlaylistID()

		var trackID uint
		if currentTrack != nil {
			trackID = currentTrack.ID
		}

		isPlaying := pm.IsPlaying(playlistID)
		isPaused := pm.IsPaused(playlistID)
		position := pm.GetPosition(playlistID)

		c.lastStateMux.Lock()

		// Detect what changed
		trackChanged := c.lastState == nil || c.lastState.TrackID != trackID
		playStateChanged := c.lastState != nil &&
			(c.lastState.IsPlaying != isPlaying || c.lastState.IsPaused != isPaused)
		positionChanged := c.lastState != nil && c.lastState.Position != position

		// Debug logging for state changes
		if playStateChanged {
			log.Printf("[VideoFeed] State change detected - isPlaying: %v->%v, isPaused: %v->%v\n",
				c.lastState.IsPlaying, isPlaying, c.lastState.IsPaused, isPaused)
		}

		if trackChanged || playStateChanged {
			oldState := c.lastState
			c.lastState = &VideoFeedState{
				TrackID:     trackID,
				IsPlaying:   isPlaying,
				IsPaused:    isPaused,
				Position:    position,
				LastUpdated: time.Now(),
			}
			c.lastStateMux.Unlock()

			// Track changed - send full track info
			if trackChanged {
				log.Printf("[VideoFeed] Broadcasting track_changed (trackID: %d)\n", trackID)
				if currentTrack != nil {
					trackInfo := c.buildVideoTrackInfo(currentTrack)
					c.broadcastEvent(VideoFeedEvent{
						Type: "track_changed",
						Data: gin.H{
							"track":      trackInfo,
							"is_playing": isPlaying,
							"is_paused":  isPaused,
							"position":   position,
						},
					})
				} else {
					c.broadcastEvent(VideoFeedEvent{
						Type: "no_track",
						Data: gin.H{
							"is_playing": false,
							"is_paused":  false,
						},
					})
				}
			} else if playStateChanged && oldState != nil {
				// Only play/pause state changed - send playback_state event
				log.Printf("[VideoFeed] Broadcasting playback_state - isPlaying: %v, isPaused: %v\n", isPlaying, isPaused)
				c.broadcastEvent(VideoFeedEvent{
					Type: "playback_state",
					Data: gin.H{
						"is_playing": isPlaying,
						"is_paused":  isPaused,
					},
				})
			}
		} else if positionChanged && isPlaying {
			c.lastState.Position = position
			c.lastStateMux.Unlock()

			// Only broadcast position every 5 seconds to reduce traffic
			if position%5 == 0 {
				c.broadcastEvent(VideoFeedEvent{
					Type: "position_update",
					Data: gin.H{
						"position": position,
					},
				})
			}
		} else {
			c.lastStateMux.Unlock()
		}
	}
}

// sendCurrentState sends the current state to a new client
func (c *VideoFeedController) sendCurrentState(clientChan chan VideoFeedEvent) {
	pm := c.playbackController.GetPlaybackManager()
	currentTrack := pm.GetCurrentTrack()
	playlistID := pm.GetCurrentPlaylistID()

	if currentTrack != nil {
		trackInfo := c.buildVideoTrackInfo(currentTrack)
		clientChan <- VideoFeedEvent{
			Type: "initial_state",
			Data: gin.H{
				"track":      trackInfo,
				"is_playing": pm.IsPlaying(playlistID),
				"is_paused":  pm.IsPaused(playlistID),
				"position":   pm.GetPosition(playlistID),
			},
		}
	} else {
		clientChan <- VideoFeedEvent{
			Type: "initial_state",
			Data: gin.H{
				"has_track":  false,
				"is_playing": false,
				"is_paused":  false,
			},
		}
	}
}

// broadcastEvent sends an event to all connected SSE clients
func (c *VideoFeedController) broadcastEvent(event VideoFeedEvent) {
	c.sseClientsMux.RLock()
	defer c.sseClientsMux.RUnlock()

	clientCount := len(c.sseClients)
	sentCount := 0

	for _, clientChan := range c.sseClients {
		select {
		case clientChan <- event:
			sentCount++
		default:
			// Client channel full, skip
			log.Printf("[VideoFeed] WARNING: Client channel full, skipping event\n")
		}
	}

	log.Printf("[VideoFeed] Broadcast %s event to %d/%d clients\n", event.Type, sentCount, clientCount)
}
