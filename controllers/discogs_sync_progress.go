package controllers

import (
	"context"
	"log"
	"time"

	"vinylfo/models"

	"github.com/gin-gonic/gin"
)

func (c *DiscogsController) GetSyncProgress(ctx *gin.Context) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
	defer cancel()

	type ProgressResponse struct {
		IsRunning         bool                     `json:"is_running"`
		IsPaused          bool                     `json:"is_paused"`
		CurrentPage       int                      `json:"current_page"`
		TotalPages        int                      `json:"total_pages"`
		Processed         int                      `json:"processed"`
		Total             int                      `json:"total"`
		SyncMode          string                   `json:"sync_mode"`
		CurrentFolder     int                      `json:"current_folder"`
		FolderIndex       int                      `json:"folder_index"`
		TotalFolders      int                      `json:"total_folders"`
		FolderName        string                   `json:"folder_name"`
		Folders           []map[string]interface{} `json:"folders"`
		APIRemaining      int                      `json:"api_remaining"`
		AnonRemaining     int                      `json:"anon_remaining"`
		LastBatch         *SyncBatch               `json:"last_batch"`
		HasSavedProgress  bool                     `json:"has_saved_progress"`
		SavedStatus       string                   `json:"saved_status"`
		SavedFolderID     int                      `json:"saved_folder_id"`
		SavedFolderName   string                   `json:"saved_folder_name"`
		SavedProcessed    int                      `json:"saved_processed"`
		SavedTotalAlbums  int                      `json:"saved_total_albums"`
		SavedLastActivity time.Time                `json:"saved_last_activity"`
		IsStalled         bool                     `json:"is_stalled"`
		LastActivity      time.Time                `json:"last_activity"`
	}

	state := getSyncState()

	select {
	case <-ctxWithTimeout.Done():
		log.Printf("GetSyncProgress: TIMEOUT - context cancelled")
		ctx.JSON(503, gin.H{"error": "Service timeout - sync may be busy"})
		return
	default:
	}

	var savedProgress models.SyncProgress
	var hasSavedProgress bool
	var savedFolderName string
	var savedProcessed int
	var savedTotalAlbums int
	var savedLastActivity time.Time

	err := c.db.WithContext(ctxWithTimeout).Raw("SELECT id, folder_id, folder_name, current_page, processed, total_albums, last_activity_at, status FROM sync_progresses ORDER BY id DESC LIMIT 1").Scan(&savedProgress).Error
	if err == nil && savedProgress.ID > 0 {
		maxAge := 30 * time.Minute
		if state.IsPaused() {
			maxAge = 4 * time.Hour
		}
		if time.Since(savedProgress.LastActivityAt) <= maxAge {
			hasSavedProgress = true
			savedFolderName = savedProgress.FolderName
			savedProcessed = savedProgress.Processed
			savedTotalAlbums = savedProgress.TotalAlbums
			savedLastActivity = savedProgress.LastActivityAt
		}
	}

	log.Printf("GetSyncProgress: IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d, LastBatch=%v, savedProgress=%v",
		state.IsRunning(), state.IsPaused(), state.Processed, state.Total,
		state.LastBatch != nil && len(state.LastBatch.Albums) > 0, hasSavedProgress)

	totalFolders := 0
	folderName := ""
	if len(state.Folders) > 0 {
		totalFolders = len(state.Folders)
		if state.FolderIndex < len(state.Folders) {
			if name, ok := state.Folders[state.FolderIndex]["name"].(string); ok {
				folderName = name
			}
		}
	}

	isStalled := false
	if state.IsRunning() && !state.IsPaused() && state.LastActivity.IsZero() == false {
		if time.Since(state.LastActivity) > 90*time.Second {
			isStalled = true
		}
	}

	response := ProgressResponse{
		IsRunning:         state.IsRunning(),
		IsPaused:          state.IsPaused(),
		CurrentPage:       state.CurrentPage,
		TotalPages:        state.TotalPages,
		Processed:         state.Processed,
		Total:             state.Total,
		SyncMode:          state.SyncMode,
		CurrentFolder:     state.CurrentFolder,
		FolderIndex:       state.FolderIndex,
		TotalFolders:      totalFolders,
		FolderName:        folderName,
		Folders:           state.Folders,
		APIRemaining:      state.APIRemaining,
		AnonRemaining:     state.AnonRemaining,
		LastBatch:         state.LastBatch,
		HasSavedProgress:  hasSavedProgress,
		SavedStatus:       "running",
		SavedFolderID:     savedProgress.FolderID,
		SavedFolderName:   savedFolderName,
		SavedProcessed:    savedProcessed,
		SavedTotalAlbums:  savedTotalAlbums,
		SavedLastActivity: savedLastActivity,
		IsStalled:         isStalled,
		LastActivity:      state.LastActivity,
	}

	ctx.JSON(200, response)
	log.Printf("GetSyncProgress: IsRunning=%v, Processed=%d, Total=%d, IsStalled=%v", state.IsRunning(), state.Processed, state.Total, isStalled)
}

func (c *DiscogsController) GetSyncHistory(ctx *gin.Context) {
	var history []models.SyncHistory
	result := c.db.Order("completed_at DESC").Find(&history)
	if result.Error != nil {
		log.Printf("GetSyncHistory: failed to fetch history: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to fetch sync history"})
		return
	}

	ctx.JSON(200, gin.H{
		"history": history,
		"count":   len(history),
	})
}
