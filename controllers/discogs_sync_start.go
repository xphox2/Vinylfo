package controllers

import (
	"context"
	"log"
	"os"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/services"
	"vinylfo/sync"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
)

func (c *DiscogsController) StartSync(ctx *gin.Context) {
	log.Printf("StartSync: called")
	state := getSyncState()
	log.Printf("StartSync: IsRunning=%v, IsPaused=%v", state.IsRunning(), state.IsPaused())

	if state.IsRunning() {
		log.Printf("StartSync: sync already in progress")
		ctx.JSON(400, gin.H{"error": "Sync already in progress"})
		return
	}

	if state.IsPaused() {
		log.Printf("StartSync: sync is paused, resuming...")
		c.ResumeSyncFromPause(ctx)
		return
	}

	existingProgress := c.progressService.Load(state)
	if existingProgress != nil {
		log.Printf("StartSync: Found existing sync progress, status=%s, is_running=%v, is_paused=%v", existingProgress.Status, state.IsRunning(), state.IsPaused())

		if !state.IsRunning() && !state.IsPaused() && existingProgress.Status == "completed" {
			log.Printf("StartSync: Previous sync completed, archiving to history and starting fresh")
			c.progressService.ArchiveToHistory(existingProgress)
			c.progressService.Delete(existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if existingProgress.Status == "completed" {
			log.Printf("StartSync: DB shows sync completed, archiving to history and starting fresh")
			c.progressService.ArchiveToHistory(existingProgress)
			c.progressService.Delete(existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if !state.IsRunning() && !state.IsPaused() {
			log.Printf("StartSync: No active sync, clearing state and starting fresh")
			ResetSyncState()
			existingProgress = nil
		} else {
			log.Printf("StartSync: Returning existing progress to frontend")
			ctx.JSON(200, gin.H{
				"message":       "Existing sync in progress",
				"has_progress":  true,
				"can_resume":    true,
				"can_start_new": true,
				"sync_mode":     existingProgress.SyncMode,
				"folder_id":     existingProgress.FolderID,
				"folder_name":   existingProgress.FolderName,
				"folder_index":  existingProgress.FolderIndex,
				"current_page":  existingProgress.CurrentPage,
				"processed":     existingProgress.Processed,
				"total_albums":  existingProgress.TotalAlbums,
				"last_activity": existingProgress.LastActivityAt,
			})
			return
		}
	}

	var input struct {
		SyncMode string `json:"sync_mode"`
		FolderID int    `json:"folder_id"`
		ForceNew bool   `json:"force_new"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.SyncMode = "all-folders"
		input.FolderID = 0
		input.ForceNew = false
	}

	if input.SyncMode == "" {
		input.SyncMode = "all-folders"
	}

	if input.ForceNew && existingProgress != nil {
		c.progressService.Clear()
		ResetSyncState()
		existingProgress = nil
		log.Printf("StartSync: Cleared existing progress, starting fresh")
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		log.Printf("StartSync: Failed to load config: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to load application configuration"})
		return
	}

	c.db.Model(&config).Where("id = ?", 1).Updates(map[string]interface{}{
		"sync_mode":      input.SyncMode,
		"sync_folder_id": input.FolderID,
	})

	log.Printf("StartSync: config loaded, IsDiscogsConnected=%v, DiscogsUsername=%s",
		config.IsDiscogsConnected, config.DiscogsUsername)
	log.Printf("StartSync: ConsumerKey=%s, AccessToken=%s",
		utils.MaskValue(config.DiscogsConsumerKey), utils.MaskValue(config.DiscogsAccessToken))
	log.Printf("StartSync: sync_mode=%s, folder_id=%d",
		input.SyncMode, input.FolderID)

	if !config.IsDiscogsConnected {
		log.Printf("StartSync: Discogs not connected")
		ctx.JSON(400, gin.H{"error": "Discogs account is not connected. Please connect your Discogs account in Settings first."})
		return
	}

	if config.DiscogsUsername == "" {
		log.Printf("StartSync: Username is empty, fetching from Discogs...")
		consumerKey := config.DiscogsConsumerKey
		if consumerKey == "" {
			consumerKey = os.Getenv("DISCOGS_CONSUMER_KEY")
		}
		consumerSecret := config.DiscogsConsumerSecret
		if consumerSecret == "" {
			consumerSecret = os.Getenv("DISCOGS_CONSUMER_SECRET")
		}
		oauth := &discogs.OAuthConfig{
			ConsumerKey:    consumerKey,
			ConsumerSecret: consumerSecret,
			AccessToken:    config.DiscogsAccessToken,
			AccessSecret:   config.DiscogsAccessSecret,
		}
		client := discogs.NewClientWithOAuth("", oauth)
		username, err := client.GetUserIdentity()
		if err != nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
			})
			log.Printf("StartSync: Failed to fetch username: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch Discogs username", "details": err.Error()})
			return
		}
		c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("discogs_username", username)
		config.DiscogsUsername = username
		log.Printf("StartSync: Fetched username: %s", username)
	}

	state = sync.SyncState{
		Status:       sync.SyncStatusRunning,
		CurrentPage:  1,
		SyncMode:     input.SyncMode,
		LastActivity: time.Now(),
	}

	if input.SyncMode == "specific" {
		state.CurrentFolder = input.FolderID
	}

	log.Printf("StartSync: getting client with OAuth...")
	client := c.getDiscogsClientWithOAuth()

	if client == nil {
		updateSyncState(func(s *sync.SyncState) {
			s.Status = sync.SyncStatusIdle
		})
		log.Printf("StartSync: failed to get client - nil returned")
		ctx.JSON(500, gin.H{"error": "Failed to initialize Discogs client. Please reconnect your Discogs account in Settings."})
		return
	}

	if input.SyncMode == "all-folders" {
		folders, err := client.GetUserFolders(config.DiscogsUsername)
		if err != nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
			})
			log.Printf("StartSync: GetUserFolders FAILED: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch Discogs folders", "details": err.Error()})
			return
		}
		state.Folders = folders
		state.FolderIndex = 0
		if len(folders) > 0 {
			firstFolderID := folders[0]["id"].(int)
			state.CurrentFolder = firstFolderID
			log.Printf("StartSync: Starting all-folders sync, first folder ID: %d", firstFolderID)
		}
	}

	var releases []map[string]interface{}
	var err error
	var totalItems int

	if input.SyncMode == "all-folders" {
		folderID := 0
		if len(state.Folders) > 0 {
			folderID = state.CurrentFolder
		}
		releases, totalItems, err = client.GetUserCollectionByFolder(config.DiscogsUsername, folderID, 1, config.SyncBatchSize)
	} else if input.SyncMode == "specific" {
		releases, totalItems, err = client.GetUserCollectionByFolder(config.DiscogsUsername, input.FolderID, 1, config.SyncBatchSize)
	} else {
		releases, err = client.GetUserCollection(config.DiscogsUsername, 1, config.SyncBatchSize)
		totalItems = len(releases)
	}

	if err != nil {
		updateSyncState(func(s *sync.SyncState) {
			s.Status = sync.SyncStatusIdle
		})
		log.Printf("StartSync: Failed to start sync - %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to start sync: " + err.Error()})
		return
	}

	log.Printf("StartSync: Success! Got %d releases, total items: %d", len(releases), totalItems)

	state.Total = totalItems
	state.CurrentPage = 1
	state.Processed = 0

	batch := SyncBatch{
		ID:     1,
		Albums: releases,
	}
	state.LastBatch = &batch

	setSyncState(state)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	worker := services.NewSyncWorker(c.db, client, syncManager, services.SyncConfig{
		Username:      config.DiscogsUsername,
		BatchSize:     config.SyncBatchSize,
		SyncMode:      input.SyncMode,
		CurrentFolder: state.CurrentFolder,
		Folders:       &state.Folders,
	}, workerCtx, workerCancel)
	go worker.Run()

	ctx.JSON(200, gin.H{
		"message":   "Sync started",
		"sync_mode": input.SyncMode,
		"folder_id": state.CurrentFolder,
	})
}
