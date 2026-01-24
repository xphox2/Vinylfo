package controllers

import (
	"context"
	"log"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"

	"github.com/gin-gonic/gin"
)

func (c *DiscogsController) GetSyncProgress(ctx *gin.Context) {
	type ProgressResponse struct {
		IsRunning            bool                     `json:"is_running"`
		IsPaused             bool                     `json:"is_paused"`
		IsActive             bool                     `json:"is_active"`
		CurrentPage          int                      `json:"current_page"`
		TotalPages           int                      `json:"total_pages"`
		Processed            int                      `json:"processed"`
		Total                int                      `json:"total"`
		SyncMode             string                   `json:"sync_mode"`
		CurrentFolder        int                      `json:"current_folder"`
		FolderIndex          int                      `json:"folder_index"`
		TotalFolders         int                      `json:"total_folders"`
		FolderName           string                   `json:"folder_name"`
		Folders              []map[string]interface{} `json:"folders"`
		APIRemaining         int                      `json:"api_remaining"`
		AnonRemaining        int                      `json:"anon_remaining"`
		LastBatch            *SyncBatch               `json:"last_batch"`
		HasSavedProgress     bool                     `json:"has_saved_progress"`
		SavedStatus          string                   `json:"saved_status"`
		SavedFolderID        int                      `json:"saved_folder_id"`
		SavedFolderName      string                   `json:"saved_folder_name"`
		SavedProcessed       int                      `json:"saved_processed"`
		SavedTotalAlbums     int                      `json:"saved_total_albums"`
		SavedLastActivity    time.Time                `json:"saved_last_activity"`
		IsStalled            bool                     `json:"is_stalled"`
		IsRateLimited        bool                     `json:"is_rate_limited"`
		RateLimitSecondsLeft int                      `json:"rate_limit_seconds_left"`
		RateLimitRetryAt     *time.Time               `json:"rate_limit_retry_at,omitempty"`
		RateLimitMessage     string                   `json:"rate_limit_message,omitempty"`
		LastActivity         time.Time                `json:"last_activity"`
	}

	state := getSyncState()

	var savedProgress models.SyncProgress
	var hasSavedProgress bool
	var savedFolderName string
	var savedProcessed int
	var savedTotalAlbums int
	var savedLastActivity time.Time
	var savedStatus string

	ctxDB, cancelDB := context.WithTimeout(ctx.Request.Context(), 150*time.Millisecond)
	defer cancelDB()

	err := c.db.WithContext(ctxDB).Raw("SELECT id, folder_id, folder_name, current_page, processed, total_albums, last_activity_at, status FROM sync_progresses ORDER BY id DESC LIMIT 1").Scan(&savedProgress).Error
	if err != nil {
		log.Printf("GetSyncProgress: saved progress lookup failed: %v", err)
	} else if savedProgress.ID > 0 {
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
			savedStatus = savedProgress.Status
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
		if time.Since(state.LastActivity) > 180*time.Second {
			if !discogs.GetGlobalRateLimiter().IsRateLimited() {
				isStalled = true
			}
		}
	}

	rateLimiter := discogs.GetGlobalRateLimiter()
	isRateLimited := rateLimiter.IsRateLimited()
	rateLimitSecondsLeft := rateLimiter.GetSecondsUntilReset()

	// Log rate limit state for debugging
	if isRateLimited || rateLimitSecondsLeft > 0 {
		log.Printf("GetSyncProgress: RATE LIMIT STATE - isRateLimited=%v, secondsLeft=%d", isRateLimited, rateLimitSecondsLeft)
	}

	// If rate limit has expired but flag wasn't cleared, clear it now
	if isRateLimited && rateLimitSecondsLeft <= 0 {
		rateLimiter.ClearRateLimit()
		isRateLimited = false
		log.Printf("GetSyncProgress: cleared expired rate limit flag")
	}

	response := ProgressResponse{
		IsRunning:            state.IsRunning(),
		IsPaused:             state.IsPaused(),
		IsActive:             state.IsActive(),
		CurrentPage:          state.CurrentPage,
		TotalPages:           state.TotalPages,
		Processed:            state.Processed,
		Total:                state.Total,
		SyncMode:             state.SyncMode,
		CurrentFolder:        state.CurrentFolder,
		FolderIndex:          state.FolderIndex,
		TotalFolders:         totalFolders,
		FolderName:           folderName,
		Folders:              state.Folders,
		APIRemaining:         state.APIRemaining,
		AnonRemaining:        state.AnonRemaining,
		LastBatch:            state.LastBatch,
		HasSavedProgress:     hasSavedProgress,
		SavedStatus:          savedStatus,
		SavedFolderID:        savedProgress.FolderID,
		SavedFolderName:      savedFolderName,
		SavedProcessed:       savedProcessed,
		SavedTotalAlbums:     savedTotalAlbums,
		SavedLastActivity:    savedLastActivity,
		IsStalled:            isStalled,
		IsRateLimited:        isRateLimited,
		RateLimitSecondsLeft: rateLimitSecondsLeft,
		RateLimitRetryAt:     state.RateLimitRetryAt,
		RateLimitMessage:     state.RateLimitMessage,
		LastActivity:         state.LastActivity,
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
