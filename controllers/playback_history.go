package controllers

import (
	"time"

	"vinylfo/models"

	"github.com/gin-gonic/gin"
)

func (c *PlaybackController) GetHistory(ctx *gin.Context) {
	var history []models.TrackHistory
	result := c.db.Order("listen_count DESC, last_played DESC").Find(&history)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}

	if len(history) == 0 {
		ctx.JSON(200, []interface{}{})
		return
	}

	var trackIDs []uint
	for _, h := range history {
		trackIDs = append(trackIDs, h.TrackID)
	}

	var tracks []models.Track
	c.db.Find(&tracks, trackIDs)
	trackMap := make(map[uint]models.Track)
	for _, track := range tracks {
		trackMap[track.ID] = track
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		if track, ok := trackMap[h.TrackID]; ok {
			resultWithTracks = append(resultWithTracks, HistoryWithTrack{
				History: h,
				Track:   track,
			})
		}
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

	if len(history) == 0 {
		ctx.JSON(200, []interface{}{})
		return
	}

	var trackIDs []uint
	for _, h := range history {
		trackIDs = append(trackIDs, h.TrackID)
	}

	var tracks []models.Track
	c.db.Find(&tracks, trackIDs)
	trackMap := make(map[uint]models.Track)
	for _, track := range tracks {
		trackMap[track.ID] = track
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		if track, ok := trackMap[h.TrackID]; ok {
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

	if len(history) == 0 {
		ctx.JSON(200, []interface{}{})
		return
	}

	var trackIDs []uint
	for _, h := range history {
		trackIDs = append(trackIDs, h.TrackID)
	}

	var tracks []models.Track
	c.db.Find(&tracks, trackIDs)
	trackMap := make(map[uint]models.Track)
	for _, track := range tracks {
		trackMap[track.ID] = track
	}

	type HistoryWithTrack struct {
		History models.TrackHistory
		Track   models.Track
	}
	var resultWithTracks []HistoryWithTrack
	for _, h := range history {
		if track, ok := trackMap[h.TrackID]; ok {
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
