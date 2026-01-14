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
