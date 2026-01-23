package controllers

import (
	"net/http"
	"strconv"

	"vinylfo/duration"
	"vinylfo/models"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsController struct {
	db      *gorm.DB
	youtube *duration.YouTubeOAuthClient
}

func NewSettingsController(db *gorm.DB) *SettingsController {
	return &SettingsController{
		db:      db,
		youtube: duration.NewYouTubeOAuthClient(db),
	}
}

func (c *SettingsController) Get(ctx *gin.Context) {
	var config models.AppConfig
	result := c.db.First(&config)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch settings"})
		return
	}

	if config.LogRetentionCount == 0 {
		config.LogRetentionCount = 10
	}

	ctx.JSON(200, gin.H{
		"discogs_connected":     config.IsDiscogsConnected,
		"discogs_username":      config.DiscogsUsername,
		"last_sync_at":          config.LastSyncAt,
		"items_per_page":        config.ItemsPerPage,
		"sync_mode":             config.SyncMode,
		"sync_folder_id":        config.SyncFolderID,
		"youtube_connected":     c.youtube.IsAuthenticated(),
		"youtube_is_configured": c.youtube.IsConfigured(),
		"log_retention_count":   config.LogRetentionCount,
	})
}

func (c *SettingsController) Update(ctx *gin.Context) {
	var input struct {
		ItemsPerPage      *int `json:"items_per_page"`
		LogRetentionCount *int `json:"log_retention_count"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if input.ItemsPerPage == nil && input.LogRetentionCount == nil {
		ctx.JSON(400, gin.H{"error": "No valid fields to update"})
		return
	}

	updates := make(map[string]interface{})

	if input.ItemsPerPage != nil {
		if *input.ItemsPerPage < 10 || *input.ItemsPerPage > 100 {
			ctx.JSON(400, gin.H{"error": "Items per page must be between 10 and 100"})
			return
		}
		updates["items_per_page"] = *input.ItemsPerPage
	}

	if input.LogRetentionCount != nil {
		if *input.LogRetentionCount < 1 || *input.LogRetentionCount > 100 {
			ctx.JSON(400, gin.H{"error": "Log retention count must be between 1 and 100"})
			return
		}
		updates["log_retention_count"] = *input.LogRetentionCount
	}

	result := c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(updates)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update settings"})
		return
	}

	c.Get(ctx)
}

func (c *SettingsController) ResetDatabase(ctx *gin.Context) {
	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to start transaction"})
		return
	}

	tables := []string{
		"track_histories",
		"session_notes",
		"session_sharings",
		"session_playlists",
		"playback_sessions",
		"duration_sources",
		"duration_resolver_progress",
		"duration_resolutions",
		"tracks",
		"albums",
		"sync_logs",
		"sync_progresses",
	}

	for _, table := range tables {
		if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
			tx.Rollback()
			ctx.JSON(500, gin.H{"error": "Failed to delete from " + table})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to commit transaction"})
		return
	}

	ResetSyncState()

	ctx.JSON(200, gin.H{
		"message": "Database reset successful",
		"note":    "All music data, sync progress, and duration resolution data has been cleared. Your OAuth settings and preferences have been preserved.",
	})
}

func (c *SettingsController) SeedDatabase(ctx *gin.Context) {
	var albumCount int64
	c.db.Model(&models.Album{}).Count(&albumCount)
	if albumCount > 0 {
		ctx.JSON(400, gin.H{
			"error":   "Database already has data",
			"message": "Please reset the database first before seeding sample data.",
		})
		return
	}

	sampleAlbums := []models.Album{
		{
			Title:         "Abbey Road",
			Artist:        "The Beatles",
			ReleaseYear:   1969,
			Genre:         "Rock",
			CoverImageURL: "https://example.com/abbey_road.jpg",
		},
		{
			Title:         "Rumours",
			Artist:        "Fleetwood Mac",
			ReleaseYear:   1977,
			Genre:         "Rock",
			CoverImageURL: "https://example.com/rumours.jpg",
		},
		{
			Title:         "Dark Side of the Moon",
			Artist:        "Pink Floyd",
			ReleaseYear:   1973,
			Genre:         "Progressive Rock",
			CoverImageURL: "https://example.com/dark_side.jpg",
		},
		{
			Title:         "Thriller",
			Artist:        "Michael Jackson",
			ReleaseYear:   1982,
			Genre:         "Pop",
			CoverImageURL: "https://example.com/thriller.jpg",
		},
	}

	for i, album := range sampleAlbums {
		if err := c.db.Create(&album).Error; err != nil {
			ctx.JSON(500, gin.H{"error": "Failed to create album: " + album.Title})
			return
		}

		var tracks []models.Track
		switch i {
		case 0:
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Come Together", Duration: 259, TrackNumber: 1},
				{AlbumID: album.ID, Title: "Something", Duration: 182, TrackNumber: 2},
				{AlbumID: album.ID, Title: "Maxwell's Silver Hammer", Duration: 207, TrackNumber: 3},
				{AlbumID: album.ID, Title: "Oh! Darling", Duration: 193, TrackNumber: 4},
			}
		case 1:
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Monday Madonna", Duration: 247, TrackNumber: 1},
				{AlbumID: album.ID, Title: "Ho Hey", Duration: 225, TrackNumber: 2},
				{AlbumID: album.ID, Title: "Dreams", Duration: 206, TrackNumber: 3},
				{AlbumID: album.ID, Title: "Don't Stop", Duration: 206, TrackNumber: 4},
			}
		case 2:
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Speak to Me", Duration: 20, TrackNumber: 1},
				{AlbumID: album.ID, Title: "Breathe", Duration: 161, TrackNumber: 2},
				{AlbumID: album.ID, Title: "On the Run", Duration: 220, TrackNumber: 3},
				{AlbumID: album.ID, Title: "Time", Duration: 237, TrackNumber: 4},
			}
		case 3:
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Wanna Be Startin' Somethin'", Duration: 258, TrackNumber: 1},
				{AlbumID: album.ID, Title: "Baby Be Mine", Duration: 225, TrackNumber: 2},
				{AlbumID: album.ID, Title: "The Girl is Mine", Duration: 192, TrackNumber: 3},
				{AlbumID: album.ID, Title: "Thriller", Duration: 258, TrackNumber: 4},
			}
		}

		for _, track := range tracks {
			if err := c.db.Create(&track).Error; err != nil {
				ctx.JSON(500, gin.H{"error": "Failed to create track: " + track.Title})
				return
			}
		}
	}

	ctx.JSON(200, gin.H{
		"message": "Sample data seeded successfully",
		"note":    "Added 4 sample albums with tracks. You can now browse your collection.",
	})
}

func (c *SettingsController) GetAuditLogs(ctx *gin.Context) {
	eventType := ctx.Query("event_type")
	limitStr := ctx.DefaultQuery("limit", "50")
	offsetStr := ctx.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	logs, total, err := utils.GetAuditLogs(eventType, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
		return
	}

	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		result = append(result, gin.H{
			"id":           log.ID,
			"event_type":   log.EventType,
			"event_action": log.EventAction,
			"user_id":      log.UserID,
			"ip_address":   log.IPAddress,
			"resource":     log.Resource,
			"status":       log.Status,
			"error_msg":    log.ErrorMsg,
			"created_at":   log.CreatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"logs":   result,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (c *SettingsController) CleanupAuditLogs(ctx *gin.Context) {
	var input struct {
		DaysRetained int `json:"days_retained"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.DaysRetained = 90
	}

	if input.DaysRetained < 7 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Minimum retention period is 7 days"})
		return
	}

	deleted, err := utils.CleanupOldAuditLogs(input.DaysRetained)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup audit logs"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "Audit logs cleaned up successfully",
		"deleted_count": deleted,
		"days_retained": input.DaysRetained,
	})
}

func (c *SettingsController) GetLogSettings(ctx *gin.Context) {
	var config models.AppConfig
	result := c.db.First(&config)
	if result.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	logCount, err := utils.GetLogFileCount("logs")
	if err != nil {
		logCount = 0
	}

	if config.LogRetentionCount == 0 {
		config.LogRetentionCount = 10
	}

	ctx.JSON(http.StatusOK, gin.H{
		"log_retention_count": config.LogRetentionCount,
		"current_log_files":   logCount,
	})
}

func (c *SettingsController) UpdateLogSettings(ctx *gin.Context) {
	var input struct {
		LogRetentionCount int `json:"log_retention_count"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.LogRetentionCount = 10
	}

	if input.LogRetentionCount < 1 || input.LogRetentionCount > 100 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Log retention count must be between 1 and 100"})
		return
	}

	result := c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("log_retention_count", input.LogRetentionCount)
	if result.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update log settings"})
		return
	}

	c.GetLogSettings(ctx)
}

func (c *SettingsController) CleanupLogs(ctx *gin.Context) {
	var config models.AppConfig
	result := c.db.First(&config)
	if result.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	retentionCount := config.LogRetentionCount
	if retentionCount <= 0 {
		retentionCount = 10
	}

	deleted, err := utils.CleanupOldLogs(retentionCount, "logs")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup logs"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "Logs cleaned up successfully",
		"deleted_count": deleted,
	})
}
