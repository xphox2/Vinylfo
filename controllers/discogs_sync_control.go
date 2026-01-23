package controllers

import (
	"context"
	"log"
	"time"

	"vinylfo/models"
	"vinylfo/services"
	"vinylfo/sync"

	"github.com/gin-gonic/gin"
)

func (c *DiscogsController) StopSync(ctx *gin.Context) {
	updateSyncState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
		s.Processed = 0
		s.LastBatch = nil
		s.LastActivity = time.Time{}
	})

	c.progressService.Clear()

	ctx.JSON(200, gin.H{
		"message":    "Sync stopped",
		"sync_state": getSyncState(),
	})
}

func (c *DiscogsController) ClearProgress(ctx *gin.Context) {
	c.progressService.Clear()
	ctx.JSON(200, gin.H{"message": "Progress cleared"})
}

func (c *DiscogsController) ResumeSync(ctx *gin.Context) {
	state := getSyncState()
	existingProgress := c.progressService.Load(state)
	if existingProgress == nil {
		ctx.JSON(400, gin.H{"error": "No sync in progress to resume"})
		return
	}

	if state.IsRunning() {
		ctx.JSON(400, gin.H{"error": "Sync is already running"})
		return
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

	if !config.IsDiscogsConnected {
		ctx.JSON(400, gin.H{"error": "Discogs not connected"})
		return
	}

	state.Status = sync.SyncStatusRunning
	state.SyncMode = existingProgress.SyncMode
	state.CurrentFolder = existingProgress.FolderID
	state.FolderIndex = existingProgress.FolderIndex
	state.CurrentPage = existingProgress.CurrentPage
	state.Processed = existingProgress.Processed
	state.Total = existingProgress.TotalAlbums
	c.progressService.RestoreLastBatch(&state)
	setSyncState(state)

	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		updateSyncState(func(s *sync.SyncState) {
			s.Status = sync.SyncStatusIdle
		})
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
		return
	}

	if existingProgress.SyncMode == "all-folders" {
		folders, err := client.GetUserFolders(config.DiscogsUsername)
		if err == nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Folders = folders
			})
			log.Printf("ResumeSync: restored %d folders from Discogs", len(folders))
		} else {
			log.Printf("ResumeSync: failed to fetch folders: %v", err)
		}
	}

	log.Printf("ResumeSync: starting sync from folder %d, page %d, processed=%d, folders_count=%d",
		existingProgress.FolderID, existingProgress.CurrentPage, existingProgress.Processed, len(getSyncState().Folders))

	updatedState := getSyncState()
	workerCtx, workerCancel := context.WithCancel(context.Background())
	worker := services.NewSyncWorker(c.db, client, syncManager, services.SyncConfig{
		Username:      config.DiscogsUsername,
		BatchSize:     config.SyncBatchSize,
		SyncMode:      existingProgress.SyncMode,
		CurrentFolder: existingProgress.FolderID,
		Folders:       &updatedState.Folders,
	}, workerCtx, workerCancel)
	go worker.Run()

	ctx.JSON(200, gin.H{
		"message":    "Sync resumed",
		"sync_state": state,
	})
}

func (c *DiscogsController) CancelSync(ctx *gin.Context) {
	updateSyncState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
		s.Processed = 0
		s.LastBatch = nil
		s.LastActivity = time.Time{}
	})

	c.progressService.Clear()

	ctx.JSON(200, gin.H{
		"message":    "Sync cancelled",
		"sync_state": getSyncState(),
	})
}

func (c *DiscogsController) PauseSync(ctx *gin.Context) {
	state := getSyncState()
	if !state.IsRunning() {
		log.Printf("PauseSync: No sync in progress, cannot pause")
		ctx.JSON(400, gin.H{"error": "No sync in progress"})
		return
	}

	if state.IsPaused() {
		log.Printf("PauseSync: Sync is already paused")
		ctx.JSON(400, gin.H{"error": "Sync is already paused"})
		return
	}

	log.Printf("PauseSync: setting IsPaused=true, current state - IsRunning=%v, Processed=%d, Total=%d, LastBatch=%v",
		state.IsRunning(), state.Processed, state.Total, state.LastBatch != nil && len(state.LastBatch.Albums) > 0)

	c.progressService.Save(state)

	log.Printf("PauseSync: calling RequestPause()...")
	pauseSuccess := syncManager.RequestPause()
	log.Printf("PauseSync: RequestPause() returned %v", pauseSuccess)
	stateAfterPause := getSyncState()
	log.Printf("PauseSync: after RequestPause - IsRunning=%v, IsPaused=%v",
		stateAfterPause.IsRunning(), stateAfterPause.IsPaused())

	updateSyncState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusPaused
	})

	c.progressService.Save(getSyncState())

	var config models.AppConfig
	if err := c.db.First(&config).Error; err == nil {
		log.Printf("PauseSync: config state - IsConnected=%v, Username=%s, HasTokens=%v",
			config.IsDiscogsConnected, config.DiscogsUsername,
			config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != "")
	}

	newState := getSyncState()
	log.Printf("PauseSync: after setting - IsPaused=%v, IsRunning=%v", newState.IsPaused(), newState.IsRunning())

	ctx.JSON(200, gin.H{
		"message":    "Sync paused",
		"sync_state": newState,
	})
}

func (c *DiscogsController) ResumeSyncFromPause(ctx *gin.Context) {
	state := getSyncState()

	if state.IsRunning() && !state.IsPaused() {
		ctx.JSON(400, gin.H{"error": "Sync is already running"})
		return
	}

	if state.IsPaused() {
		progress := c.progressService.Load(state)
		if progress == nil {
			ctx.JSON(400, gin.H{"error": "No paused sync to resume"})
			return
		}

		var config models.AppConfig
		if err := c.db.First(&config).Error; err != nil {
			log.Printf("ResumeSyncFromPause: ERROR loading config: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if !config.IsDiscogsConnected {
			log.Printf("ResumeSyncFromPause: Discogs not connected, rejecting resume")
			ctx.JSON(400, gin.H{"error": "Discogs not connected"})
			return
		}

		if state.WorkerID != "" && sync.IsWorkerRunning(state.WorkerID) {
			log.Printf("ResumeSyncFromPause: worker %s still active, resuming it", state.WorkerID)
			resumeSuccess := syncManager.RequestResume()
			log.Printf("ResumeSyncFromPause: RequestResume() returned %v", resumeSuccess)

			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusRunning
			})

			newState := getSyncState()
			ctx.JSON(200, gin.H{
				"message":    "Sync resumed",
				"sync_state": newState,
			})
			return
		}

		log.Printf("ResumeSyncFromPause: worker %s not active, starting new worker", state.WorkerID)
		log.Printf("ResumeSyncFromPause: calling RequestResume()...")
		resumeSuccess := syncManager.RequestResume()
		log.Printf("ResumeSyncFromPause: RequestResume() returned %v, restarting worker...", resumeSuccess)

		updateSyncState(func(s *sync.SyncState) {
			s.Status = sync.SyncStatusRunning
			s.Processed = progress.Processed
			s.Total = progress.TotalAlbums
			s.CurrentPage = progress.CurrentPage
		})

		state = getSyncState()
		c.progressService.RestoreLastBatch(&state)
		setSyncState(state)

		newStateAfterRestore := getSyncState()
		batchCount := 0
		if newStateAfterRestore.LastBatch != nil {
			batchCount = len(newStateAfterRestore.LastBatch.Albums)
		}
		log.Printf("ResumeSyncFromPause: restored progress - page=%d, processed=%d, total=%d, restored LastBatch with %d albums",
			progress.CurrentPage, progress.Processed, progress.TotalAlbums, batchCount)

		client := c.getDiscogsClientWithOAuth()
		if client == nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
				s.LastActivity = time.Time{}
			})
			ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
			return
		}

		newState := getSyncState()
		log.Printf("ResumeSyncFromPause: restarting sync worker at page %d, processed=%d", newState.CurrentPage, newState.Processed)
		workerCtx, workerCancel := context.WithCancel(context.Background())
		worker := services.NewSyncWorker(c.db, client, syncManager, services.SyncConfig{
			Username:      config.DiscogsUsername,
			BatchSize:     config.SyncBatchSize,
			SyncMode:      newState.SyncMode,
			CurrentFolder: newState.CurrentFolder,
			Folders:       &newState.Folders,
		}, workerCtx, workerCancel)
		go worker.Run()

		ctx.JSON(200, gin.H{
			"message":    "Sync resumed from pause",
			"sync_state": newState,
		})
		return
	}

	if !state.IsRunning() {
		existingProgress := c.progressService.Load(state)
		if existingProgress == nil {
			ctx.JSON(400, gin.H{"error": "No paused sync to resume"})
			return
		}

		var config models.AppConfig
		if err := c.db.First(&config).Error; err != nil {
			log.Printf("ResumeSyncFromPause: ERROR loading config: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		log.Printf("ResumeSyncFromPause: config state - IsConnected=%v, Username=%s, HasAccessToken=%v, HasAccessSecret=%v",
			config.IsDiscogsConnected, config.DiscogsUsername,
			config.DiscogsAccessToken != "", config.DiscogsAccessSecret != "")

		if !config.IsDiscogsConnected && (config.DiscogsAccessToken == "" || config.DiscogsAccessSecret == "") {
			log.Printf("ResumeSyncFromPause: Discogs not connected, rejecting resume")
			ctx.JSON(400, gin.H{"error": "Discogs not connected"})
			return
		}

		log.Printf("ResumeSyncFromPause: proceeding with resume, IsConnected=%v, HasTokens=%v",
			config.IsDiscogsConnected, config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != "")

		state.Status = sync.SyncStatusRunning
		state.SyncMode = existingProgress.SyncMode
		state.CurrentFolder = existingProgress.FolderID
		state.FolderIndex = existingProgress.FolderIndex
		state.CurrentPage = existingProgress.CurrentPage
		state.Processed = existingProgress.Processed
		state.Total = existingProgress.TotalAlbums
		state.Status = sync.SyncStatusRunning
		setSyncState(state)

		state = getSyncState()
		c.progressService.RestoreLastBatch(&state)
		setSyncState(state)

		batchCount := 0
		if state.LastBatch != nil {
			batchCount = len(state.LastBatch.Albums)
		}
		log.Printf("ResumeSyncFromPause: resuming from page %d with %d albums processed, restored LastBatch with %d albums",
			state.CurrentPage, state.Processed, batchCount)

		client := c.getDiscogsClientWithOAuth()
		if client == nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
				s.Processed = 0
				s.LastActivity = time.Time{}
			})
			ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
			return
		}

		if existingProgress.SyncMode == "all-folders" {
			folders, err := client.GetUserFolders(config.DiscogsUsername)
			if err == nil {
				updateSyncState(func(s *sync.SyncState) {
					s.Folders = folders
				})
				log.Printf("ResumeSyncFromPause: restored %d folders from Discogs", len(folders))
			} else {
				log.Printf("ResumeSyncFromPause: failed to fetch folders: %v", err)
			}
		}

		log.Printf("ResumeSyncFromPause: resuming sync from folder %d, page %d, processed=%d, folders_count=%d",
			existingProgress.FolderID, existingProgress.CurrentPage, existingProgress.Processed, len(getSyncState().Folders))

		updatedState := getSyncState()
		workerCtx, workerCancel := context.WithCancel(context.Background())
		worker := services.NewSyncWorker(c.db, client, syncManager, services.SyncConfig{
			Username:      config.DiscogsUsername,
			BatchSize:     config.SyncBatchSize,
			SyncMode:      existingProgress.SyncMode,
			CurrentFolder: existingProgress.FolderID,
			Folders:       &updatedState.Folders,
		}, workerCtx, workerCancel)
		go worker.Run()

		ctx.JSON(200, gin.H{
			"message":    "Sync resumed from pause",
			"sync_state": state,
		})
		return
	}

	ctx.JSON(200, gin.H{
		"message":    "Sync resumed",
		"sync_state": getSyncState(),
	})
}
