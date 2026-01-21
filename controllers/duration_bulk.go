package controllers

import (
	"context"
	"net/http"
	"sync"

	"vinylfo/duration"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
)

var (
	bulkWorker       *services.DurationWorker
	bulkStateManager *duration.StateManager
	bulkCancel       context.CancelFunc
	bulkMutex        sync.Mutex
)

func (c *DurationController) StartBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager != nil && bulkStateManager.IsRunning() {
		ctx.JSON(http.StatusConflict, gin.H{"error": "Bulk resolution already running"})
		return
	}

	bulkStateManager = duration.NewStateManager()
	workerCtx, cancel := context.WithCancel(context.Background())
	bulkCancel = cancel

	bulkWorker = services.NewDurationWorker(
		c.db,
		c.resolverService,
		bulkStateManager,
		workerCtx,
		cancel,
	)

	go bulkWorker.Run()

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Bulk resolution started",
		"status":  "running",
	})
}

func (c *DurationController) PauseBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager == nil || !bulkStateManager.IsRunning() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No bulk resolution running"})
		return
	}

	bulkStateManager.RequestPause()
	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution paused"})
}

func (c *DurationController) ResumeBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager == nil || !bulkStateManager.IsPaused() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No paused bulk resolution"})
		return
	}

	bulkStateManager.RequestResume()
	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution resumed"})
}

func (c *DurationController) CancelBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkCancel != nil {
		bulkCancel()
	}
	if bulkStateManager != nil {
		bulkStateManager.RequestStop()
		bulkStateManager.Reset()
	}

	progressService := services.NewDurationProgressService(c.db)
	progressService.Reset()

	bulkStateManager = nil
	bulkWorker = nil
	bulkCancel = nil

	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution cancelled"})
}

func (c *DurationController) GetBulkProgress(ctx *gin.Context) {
	if bulkStateManager == nil {
		progressService := services.NewDurationProgressService(c.db)
		saved, err := progressService.Load()
		if err != nil || saved == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"status":           "idle",
				"total_tracks":     0,
				"processed_tracks": 0,
			})
			return
		}

		if saved.Status == "running" {
			saved.Status = "failed"
			saved.LastError = "Worker stopped unexpectedly"
			c.db.Save(saved)
			ctx.JSON(http.StatusOK, gin.H{
				"status":             "failed",
				"total_tracks":       saved.TotalTracks,
				"processed_tracks":   saved.ProcessedTracks,
				"resolved_count":     saved.ResolvedCount,
				"needs_review_count": saved.NeedsReviewCount,
				"failed_count":       saved.FailedCount,
				"last_error":         "Worker stopped unexpectedly",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status":             saved.Status,
			"total_tracks":       saved.TotalTracks,
			"processed_tracks":   saved.ProcessedTracks,
			"resolved_count":     saved.ResolvedCount,
			"needs_review_count": saved.NeedsReviewCount,
			"failed_count":       saved.FailedCount,
			"last_activity":      saved.LastActivityAt,
		})
		return
	}

	state := bulkStateManager.GetState()
	percentComplete := 0.0
	if state.TotalTracks > 0 {
		percentComplete = float64(state.ProcessedTracks) / float64(state.TotalTracks) * 100
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":             string(state.Status),
		"total_tracks":       state.TotalTracks,
		"processed_tracks":   state.ProcessedTracks,
		"resolved_count":     state.ResolvedCount,
		"needs_review_count": state.NeedsReviewCount,
		"failed_count":       state.FailedCount,
		"skipped_count":      state.SkippedCount,
		"current_track":      state.CurrentTrack,
		"last_error":         state.LastError,
		"percent_complete":   percentComplete,
	})
}
