package controllers

import (
	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsController struct {
	db *gorm.DB
}

func NewSettingsController(db *gorm.DB) *SettingsController {
	return &SettingsController{db: db}
}

func (c *SettingsController) Get(ctx *gin.Context) {
	var config models.AppConfig
	result := c.db.First(&config)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch settings"})
		return
	}

	ctx.JSON(200, gin.H{
		"discogs_connected":    config.IsDiscogsConnected,
		"discogs_username":     config.DiscogsUsername,
		"sync_confirm_batches": config.SyncConfirmBatches,
		"sync_batch_size":      config.SyncBatchSize,
		"auto_apply_safe":      config.AutoApplySafeUpdates,
		"auto_sync_new":        config.AutoSyncNewAlbums,
		"items_per_page":       config.ItemsPerPage,
		"last_sync_at":         config.LastSyncAt,
	})
}

func (c *SettingsController) Update(ctx *gin.Context) {
	var input struct {
		SyncConfirmBatches   *bool `json:"sync_confirm_batches"`
		SyncBatchSize        *int  `json:"sync_batch_size"`
		AutoApplySafeUpdates *bool `json:"auto_apply_safe_updates"`
		AutoSyncNewAlbums    *bool `json:"auto_sync_new_albums"`
		ItemsPerPage         *int  `json:"items_per_page"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})

	if input.SyncConfirmBatches != nil {
		updates["sync_confirm_batches"] = *input.SyncConfirmBatches
	}
	if input.SyncBatchSize != nil {
		if *input.SyncBatchSize < 1 || *input.SyncBatchSize > 200 {
			ctx.JSON(400, gin.H{"error": "Batch size must be between 1 and 200"})
			return
		}
		updates["sync_batch_size"] = *input.SyncBatchSize
	}
	if input.AutoApplySafeUpdates != nil {
		updates["auto_apply_safe_updates"] = *input.AutoApplySafeUpdates
	}
	if input.AutoSyncNewAlbums != nil {
		updates["auto_sync_new_albums"] = *input.AutoSyncNewAlbums
	}
	if input.ItemsPerPage != nil {
		if *input.ItemsPerPage < 10 || *input.ItemsPerPage > 100 {
			ctx.JSON(400, gin.H{"error": "Items per page must be between 10 and 100"})
			return
		}
		updates["items_per_page"] = *input.ItemsPerPage
	}

	if len(updates) == 0 {
		ctx.JSON(400, gin.H{"error": "No valid fields to update"})
		return
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
		"tracks",
		"albums",
		"sync_logs",
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
		"note":    "All music data and sync progress has been cleared. Your OAuth settings and preferences have been preserved.",
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
