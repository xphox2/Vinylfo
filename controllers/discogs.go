package controllers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/services"
	"vinylfo/sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// getDiscogsClient returns a Discogs client without OAuth
func getDiscogsClient() *discogs.Client {
	client := discogs.NewClient("")
	return client
}

// DiscogsController handles all Discogs-related HTTP endpoints
type DiscogsController struct {
	db              *gorm.DB
	progressService *services.SyncProgressService
}

// NewDiscogsController creates a new DiscogsController instance
func NewDiscogsController(db *gorm.DB) *DiscogsController {
	return &DiscogsController{
		db:              db,
		progressService: services.NewSyncProgressService(db),
	}
}

// getDiscogsClientWithOAuth returns a Discogs client with OAuth credentials
func (c *DiscogsController) getDiscogsClientWithOAuth() *discogs.Client {
	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		logToFile("OAUTH: ERROR loading config from database: %v", err)
		return nil
	}

	logToFile("OAUTH: DB config - ID=%d, ConsumerKey=%s, AccessToken=%s, Username=%s, IsConnected=%v",
		config.ID, maskValue(config.DiscogsConsumerKey), maskValue(config.DiscogsAccessToken), config.DiscogsUsername, config.IsDiscogsConnected)

	consumerKey := config.DiscogsConsumerKey
	if consumerKey == "" {
		consumerKey = os.Getenv("DISCOGS_CONSUMER_KEY")
		logToFile("OAUTH: Using ConsumerKey from env var: %s", maskValue(consumerKey))
	}
	consumerSecret := config.DiscogsConsumerSecret
	if consumerSecret == "" {
		consumerSecret = os.Getenv("DISCOGS_CONSUMER_SECRET")
		logToFile("OAUTH: Using ConsumerSecret from env var")
	}

	logToFile("OAUTH: Final ConsumerKey=%s, AccessToken=%s", maskValue(consumerKey), maskValue(config.DiscogsAccessToken))

	oauth := &discogs.OAuthConfig{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    config.DiscogsAccessToken,
		AccessSecret:   config.DiscogsAccessSecret,
	}
	return discogs.NewClientWithOAuth("", oauth)
}

// Sync state management
type SyncBatch = sync.SyncBatch

var syncManager = sync.DefaultManager

func getSyncState() sync.SyncState {
	return syncManager.GetState()
}

func updateSyncState(fn func(*sync.SyncState)) {
	syncManager.UpdateState(fn)
}

func ResetSyncState() {
	syncManager.Reset()
}

func setSyncState(state sync.SyncState) {
	syncManager.UpdateState(func(s *sync.SyncState) {
		*s = state
	})
}

func removeFirstAlbumFromBatch(s *sync.SyncState) {
	if s.LastBatch != nil && len(s.LastBatch.Albums) > 0 {
		s.LastBatch.Albums = s.LastBatch.Albums[1:]
		if len(s.LastBatch.Albums) == 0 {
			s.LastBatch = nil
		}
	}
}

func isSyncComplete(state sync.SyncState) bool {
	if state.IsPaused() {
		return false
	}
	if !state.IsRunning() {
		return true
	}
	if state.LastBatch != nil && len(state.LastBatch.Albums) > 0 {
		return false
	}
	return true
}

// OAuth Handlers

// GetOAuthURL returns the OAuth authorization URL for Discogs
func (c *DiscogsController) GetOAuthURL(ctx *gin.Context) {
	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		config = models.AppConfig{ID: 1}
		c.db.FirstOrCreate(&config, models.AppConfig{ID: 1})
	}

	consumerKey := config.DiscogsConsumerKey
	if consumerKey == "" {
		consumerKey = os.Getenv("DISCOGS_CONSUMER_KEY")
	}
	consumerSecret := config.DiscogsConsumerSecret
	if consumerSecret == "" {
		consumerSecret = os.Getenv("DISCOGS_CONSUMER_SECRET")
	}

	if consumerKey == "" || consumerSecret == "" {
		ctx.JSON(500, gin.H{"error": "DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set"})
		return
	}

	oauth := &discogs.OAuthConfig{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
	}
	client := discogs.NewClientWithOAuth("", oauth)

	token, secret, authURL, err := client.GetRequestToken()
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to get request token"})
		return
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":    token,
		"discogs_access_secret":   secret,
		"discogs_consumer_key":    consumerKey,
		"discogs_consumer_secret": consumerSecret,
	})

	ctx.JSON(200, gin.H{
		"auth_url": authURL,
		"token":    token,
	})
}

// OAuthCallback handles the OAuth callback from Discogs
func (c *DiscogsController) OAuthCallback(ctx *gin.Context) {
	if ctx.Query("oauth_token") == "" || ctx.Query("oauth_verifier") == "" {
		ctx.String(400, "Missing oauth_token or oauth_verifier")
		return
	}

	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		ctx.String(500, "Failed to load config: %v", err)
		return
	}

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

	accessToken, accessSecret, username, err := client.GetAccessToken(config.DiscogsAccessToken, config.DiscogsAccessSecret, ctx.Query("oauth_verifier"))
	if err != nil {
		ctx.String(500, "Failed to get access token: %v", err)
		return
	}

	if username == "" {
		username, err = client.GetUserIdentity()
		if err != nil {
			ctx.String(500, "Failed to get user identity: %v", err)
			return
		}
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":    accessToken,
		"discogs_access_secret":   accessSecret,
		"discogs_username":        username,
		"discogs_consumer_key":    consumerKey,
		"discogs_consumer_secret": consumerSecret,
		"is_discogs_connected":    true,
	})

	ctx.Redirect(302, "/settings?discogs_connected=true")
}

// Disconnect removes Discogs OAuth credentials
func (c *DiscogsController) Disconnect(ctx *gin.Context) {
	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":  "",
		"discogs_access_secret": "",
		"discogs_username":      "",
		"is_discogs_connected":  false,
	})

	ctx.JSON(200, gin.H{
		"message": "Disconnected from Discogs",
		"note":    "The authorization has been removed from this application. To fully disconnect, please also revoke access at: https://www.discogs.com/settings/applications",
	})
}

// Status and Folder Handlers

// GetStatus returns the current Discogs connection status
func (c *DiscogsController) GetStatus(ctx *gin.Context) {
	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		logToFile("GetStatus: ERROR loading config: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

	logToFile("GetStatus: IsConnected=%v, Username=%s, HasAccessToken=%v, HasAccessSecret=%v, HasConsumerKey=%v",
		config.IsDiscogsConnected, config.DiscogsUsername,
		config.DiscogsAccessToken != "", config.DiscogsAccessSecret != "",
		config.DiscogsConsumerKey != "")

	ctx.JSON(200, gin.H{
		"is_connected":    config.IsDiscogsConnected,
		"username":        config.DiscogsUsername,
		"sync_confirm":    config.SyncConfirmBatches,
		"batch_size":      config.SyncBatchSize,
		"auto_apply_safe": config.AutoApplySafeUpdates,
		"auto_sync_new":   config.AutoSyncNewAlbums,
		"last_sync_at":    config.LastSyncAt,
		"sync_mode":       config.SyncMode,
		"sync_folder_id":  config.SyncFolderID,
	})
}

// GetFolders returns the user's Discogs folders
func (c *DiscogsController) GetFolders(ctx *gin.Context) {
	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

	if !config.IsDiscogsConnected {
		ctx.JSON(400, gin.H{"error": "Discogs not connected"})
		return
	}

	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
		return
	}

	folders, err := client.GetUserFolders(config.DiscogsUsername)
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch folders", "details": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"folders": folders,
	})
}

// Search and Preview Handlers

// Search searches for albums on Discogs
func (c *DiscogsController) Search(ctx *gin.Context) {
	query := ctx.Query("q")
	page := 1

	if p := ctx.Query("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
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

	client := c.getDiscogsClientWithOAuth()
	albums, totalPages, err := client.SearchAlbums(query, page)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"results":    albums,
		"page":       page,
		"totalPages": totalPages,
	})
}

// PreviewAlbum returns preview data for an album from Discogs
func (c *DiscogsController) PreviewAlbum(ctx *gin.Context) {
	discogsID := ctx.Param("id")
	id, err := strconv.Atoi(discogsID)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid Discogs ID"})
		return
	}

	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client - not authenticated"})
		return
	}

	discogsData, err := client.GetAlbum(id)
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch album from Discogs"})
		return
	}

	if tracks, ok := discogsData["tracklist"].([]map[string]interface{}); ok {
		if len(tracks) == 0 {
			ctx.JSON(400, gin.H{"error": "No track information available for this release"})
			return
		}
	}

	ctx.JSON(200, discogsData)
}

// Album Creation Handler

// CreateAlbum creates a new album from Discogs or manual input
func (c *DiscogsController) CreateAlbum(ctx *gin.Context) {
	var input struct {
		DiscogsID   int    `json:"discogs_id"`
		Title       string `json:"title"`
		Artist      string `json:"artist"`
		ReleaseYear int    `json:"release_year"`
		Genre       string `json:"genre"`
		Label       string `json:"label"`
		Country     string `json:"country"`
		ReleaseDate string `json:"release_date"`
		Style       string `json:"style"`
		CoverImage  string `json:"cover_image"`
		FromDiscogs bool   `json:"from_discogs"`
		Tracks      []struct {
			Title       string `json:"title"`
			Duration    int    `json:"duration"`
			TrackNumber int    `json:"track_number"`
			DiscNumber  int    `json:"disc_number"`
			Side        string `json:"side"`
			Position    string `json:"position"`
		} `json:"tracks"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var album models.Album

	if input.FromDiscogs && input.DiscogsID > 0 {
		client := c.getDiscogsClientWithOAuth()
		if client == nil {
			logToFile("CreateAlbum: Failed to get OAuth client, skipping Discogs fetch")
		} else {
			discogsData, err := client.GetAlbum(input.DiscogsID)
			if err == nil {
				if v, ok := discogsData["title"].(string); ok {
					album.Title = v
				}
				if v, ok := discogsData["artist"].(string); ok {
					album.Artist = v
				}
				switch v := discogsData["year"].(type) {
				case float64:
					album.ReleaseYear = int(v)
				case int:
					album.ReleaseYear = v
				}
				if v, ok := discogsData["genre"].(string); ok {
					album.Genre = v
				}
				if v, ok := discogsData["label"].(string); ok {
					album.Label = v
				}
				if v, ok := discogsData["country"].(string); ok {
					album.Country = v
				}
				if v, ok := discogsData["release_date"].(string); ok {
					album.ReleaseDate = v
				}
				if v, ok := discogsData["style"].(string); ok {
					album.Style = v
				}
				if v, ok := discogsData["cover_image"].(string); ok {
					album.CoverImageURL = v
				}
				album.DiscogsID = intPtr(input.DiscogsID)

				if tracks, ok := discogsData["tracklist"].([]map[string]interface{}); ok {
					input.Tracks = []struct {
						Title       string `json:"title"`
						Duration    int    `json:"duration"`
						TrackNumber int    `json:"track_number"`
						DiscNumber  int    `json:"disc_number"`
						Side        string `json:"side"`
						Position    string `json:"position"`
					}{}
					for _, t := range tracks {
						duration := 0
						switch v := t["duration"].(type) {
						case float64:
							duration = int(v)
						case int:
							duration = v
						case string:
							if v != "" {
								parts := strings.Split(v, ":")
								if len(parts) == 2 {
									if mins, err := strconv.Atoi(parts[0]); err == nil {
										if secs, err := strconv.Atoi(parts[1]); err == nil {
											duration = mins*60 + secs
										}
									}
								}
							}
						}

						trackNumber := 0
						switch tn := t["track_number"].(type) {
						case int:
							trackNumber = tn
						case int64:
							trackNumber = int(tn)
						case float64:
							trackNumber = int(tn)
						}

						discNumber := 0
						switch dn := t["disc_number"].(type) {
						case int:
							discNumber = dn
						case int64:
							discNumber = int(dn)
						case float64:
							discNumber = int(dn)
						}

						input.Tracks = append(input.Tracks, struct {
							Title       string `json:"title"`
							Duration    int    `json:"duration"`
							TrackNumber int    `json:"track_number"`
							DiscNumber  int    `json:"disc_number"`
							Side        string `json:"side"`
							Position    string `json:"position"`
						}{
							Title:       t["title"].(string),
							Duration:    duration,
							TrackNumber: trackNumber,
							DiscNumber:  discNumber,
							Side:        t["position"].(string),
							Position:    t["position"].(string),
						})
					}
				}
			} else {
				logToFile("CreateAlbum: Failed to fetch from Discogs: %v", err)
			}
		}
	}

	if input.Title != "" {
		album.Title = input.Title
	}
	if input.Artist != "" {
		album.Artist = input.Artist
	}
	if input.ReleaseYear > 0 {
		album.ReleaseYear = input.ReleaseYear
	}
	if input.Genre != "" {
		album.Genre = input.Genre
	}
	if input.Label != "" {
		album.Label = input.Label
	}
	if input.Country != "" {
		album.Country = input.Country
	}
	if input.ReleaseDate != "" {
		album.ReleaseDate = input.ReleaseDate
	}
	if input.Style != "" {
		album.Style = input.Style
	}
	if input.CoverImage != "" {
		album.CoverImageURL = input.CoverImage
		imageData, imageType, imageErr := downloadImage(input.CoverImage)
		if imageErr != nil {
			logToFile("CreateAlbum: failed to download image: %v", imageErr)
			album.CoverImageFailed = true
		} else {
			album.DiscogsCoverImage = imageData
			album.DiscogsCoverImageType = imageType
		}
	} else if album.CoverImageURL != "" {
		imageData, imageType, imageErr := downloadImage(album.CoverImageURL)
		if imageErr != nil {
			logToFile("CreateAlbum: failed to download image from Discogs: %v", imageErr)
			album.CoverImageFailed = true
		} else {
			album.DiscogsCoverImage = imageData
			album.DiscogsCoverImageType = imageType
		}
	}

	result := c.db.Create(&album)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create album"})
		return
	}

	for _, trackInput := range input.Tracks {
		track := models.Track{
			AlbumID:     album.ID,
			AlbumTitle:  album.Title,
			Title:       trackInput.Title,
			Duration:    trackInput.Duration,
			TrackNumber: trackInput.TrackNumber,
			DiscNumber:  trackInput.DiscNumber,
			Side:        trackInput.Side,
			Position:    trackInput.Position,
		}
		c.db.Create(&track)
	}

	c.db.Preload("Tracks").First(&album, album.ID)
	ctx.JSON(201, album)
}

// Sync Handlers

// StartSync initiates a new sync with Discogs
func (c *DiscogsController) StartSync(ctx *gin.Context) {
	logToFile("StartSync: called")
	state := getSyncState()
	logToFile("StartSync: IsRunning=%v, IsPaused=%v", state.IsRunning(), state.IsPaused())

	if state.IsRunning() {
		logToFile("StartSync: sync already in progress")
		ctx.JSON(400, gin.H{"error": "Sync already in progress"})
		return
	}

	if state.IsPaused() {
		logToFile("StartSync: sync is paused, resuming...")
		c.ResumeSyncFromPause(ctx)
		return
	}

	existingProgress := c.progressService.Load(state)
	if existingProgress != nil {
		logToFile("StartSync: Found existing sync progress, status=%s, is_running=%v, is_paused=%v", existingProgress.Status, state.IsRunning(), state.IsPaused())

		if !state.IsRunning() && !state.IsPaused() && existingProgress.Status == "completed" {
			logToFile("StartSync: Previous sync completed, archiving to history and starting fresh")
			c.progressService.ArchiveToHistory(existingProgress)
			c.progressService.Delete(existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if existingProgress.Status == "completed" {
			logToFile("StartSync: DB shows sync completed, archiving to history and starting fresh")
			c.progressService.ArchiveToHistory(existingProgress)
			c.progressService.Delete(existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if !state.IsRunning() && !state.IsPaused() {
			logToFile("StartSync: No active sync, clearing state and starting fresh")
			ResetSyncState()
			existingProgress = nil
		} else {
			logToFile("StartSync: Returning existing progress to frontend")
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
		logToFile("StartSync: Cleared existing progress, starting fresh")
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		logToFile("StartSync: Failed to load config: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to load application configuration"})
		return
	}

	c.db.Model(&config).Where("id = ?", 1).Updates(map[string]interface{}{
		"sync_mode":      input.SyncMode,
		"sync_folder_id": input.FolderID,
	})

	logToFile("StartSync: config loaded, IsDiscogsConnected=%v, DiscogsUsername=%s",
		config.IsDiscogsConnected, config.DiscogsUsername)
	logToFile("StartSync: ConsumerKey=%s, AccessToken=%s",
		maskValue(config.DiscogsConsumerKey), maskValue(config.DiscogsAccessToken))
	logToFile("StartSync: sync_mode=%s, folder_id=%d",
		input.SyncMode, input.FolderID)

	if !config.IsDiscogsConnected {
		logToFile("StartSync: Discogs not connected")
		ctx.JSON(400, gin.H{"error": "Discogs account is not connected. Please connect your Discogs account in Settings first."})
		return
	}

	if config.DiscogsUsername == "" {
		logToFile("StartSync: Username is empty, fetching from Discogs...")
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
			logToFile("StartSync: Failed to fetch username: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch Discogs username", "details": err.Error()})
			return
		}
		c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("discogs_username", username)
		config.DiscogsUsername = username
		logToFile("StartSync: Fetched username: %s", username)
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

	logToFile("StartSync: getting client with OAuth...")
	client := c.getDiscogsClientWithOAuth()

	if client == nil {
		updateSyncState(func(s *sync.SyncState) {
			s.Status = sync.SyncStatusIdle
		})
		logToFile("StartSync: failed to get client - nil returned")
		ctx.JSON(500, gin.H{"error": "Failed to initialize Discogs client. Please reconnect your Discogs account in Settings."})
		return
	}

	if input.SyncMode == "all-folders" {
		folders, err := client.GetUserFolders(config.DiscogsUsername)
		if err != nil {
			updateSyncState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
			})
			logToFile("StartSync: GetUserFolders FAILED: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch Discogs folders", "details": err.Error()})
			return
		}
		state.Folders = folders
		state.FolderIndex = 0
		if len(folders) > 0 {
			firstFolderID := folders[0]["id"].(int)
			state.CurrentFolder = firstFolderID
			logToFile("StartSync: Starting all-folders sync, first folder ID: %d", firstFolderID)
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
		logToFile("StartSync: Failed to start sync - %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to start sync: " + err.Error()})
		return
	}

	logToFile("StartSync: Success! Got %d releases, total items: %d", len(releases), totalItems)

	state.Total = totalItems
	state.CurrentPage = 1
	state.Processed = 0

	batch := SyncBatch{
		ID:     1,
		Albums: releases,
	}
	state.LastBatch = &batch

	setSyncState(state)

	// Start the sync worker with a background context
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

// GetSyncProgress returns the current sync progress
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
		logToFile("GetSyncProgress: TIMEOUT - context cancelled")
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

	logToFile("GetSyncProgress: IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d, LastBatch=%v, savedProgress=%v",
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
	logToFile("GetSyncProgress: IsRunning=%v, Processed=%d, Total=%d, IsStalled=%v", state.IsRunning(), state.Processed, state.Total, isStalled)
}

// GetSyncHistory returns the sync history
func (c *DiscogsController) GetSyncHistory(ctx *gin.Context) {
	var history []models.SyncHistory
	result := c.db.Order("completed_at DESC").Find(&history)
	if result.Error != nil {
		logToFile("GetSyncHistory: failed to fetch history: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to fetch sync history"})
		return
	}

	ctx.JSON(200, gin.H{
		"history": history,
		"count":   len(history),
	})
}

// ApplyBatch applies a batch of albums
func (c *DiscogsController) ApplyBatch(ctx *gin.Context) {
	var input struct {
		ApplyAlbums []int `json:"apply_albums"`
		SkipAlbums  []int `json:"skip_albums"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	state := getSyncState()
	client := c.getDiscogsClientWithOAuth()
	importer := services.NewAlbumImporter(c.db, client)

	for _, discogsID := range input.ApplyAlbums {
		var albumInBatch *map[string]interface{}
		for i, a := range state.LastBatch.Albums {
			if a["discogs_id"].(int) == discogsID {
				albumInBatch = &state.LastBatch.Albums[i]
				break
			}
		}

		if albumInBatch == nil {
			continue
		}

		title, _ := (*albumInBatch)["title"].(string)
		artist, _ := (*albumInBatch)["artist"].(string)
		year, _ := (*albumInBatch)["year"].(int)
		coverImage, _ := (*albumInBatch)["cover_image"].(string)
		folderID := 0
		if f, ok := (*albumInBatch)["folder_id"].(int); ok {
			folderID = f
		}

		tx := c.db.Begin()
		newAlbum := models.Album{
			Title:           title,
			Artist:          artist,
			ReleaseYear:     year,
			CoverImageURL:   coverImage,
			DiscogsID:       intPtr(discogsID),
			DiscogsFolderID: folderID,
		}
		if err := tx.Create(&newAlbum).Error; err != nil {
			tx.Rollback()
			continue
		}

		if discogsID > 0 && client != nil {
			success, _ := importer.FetchAndSaveTracks(tx, newAlbum.ID, discogsID, title, artist)
			if !success {
				tx.Rollback()
				continue
			}
		}

		if err := tx.Commit().Error; err != nil {
			logToFile("ApplyBatch: failed to commit transaction for album %s - %s: %v", artist, title, err)
			tx.Rollback()
			continue
		}
	}

	updateSyncState(func(s *sync.SyncState) {
		s.Processed += len(input.ApplyAlbums)
		s.Processed += len(input.SkipAlbums)
	})

	ctx.JSON(200, gin.H{
		"message":    "Batch processed",
		"applied":    len(input.ApplyAlbums),
		"skipped":    len(input.SkipAlbums),
		"sync_state": getSyncState(),
	})
}

// StopSync stops the current sync
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

// ClearProgress clears the sync progress
func (c *DiscogsController) ClearProgress(ctx *gin.Context) {
	c.progressService.Clear()
	ctx.JSON(200, gin.H{"message": "Progress cleared"})
}

// ResumeSync resumes a stopped sync
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
			logToFile("ResumeSync: restored %d folders from Discogs", len(folders))
		} else {
			logToFile("ResumeSync: failed to fetch folders: %v", err)
		}
	}

	logToFile("ResumeSync: starting sync from folder %d, page %d, processed=%d, folders_count=%d",
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

// GetBatchDetails returns details of a specific batch
func (c *DiscogsController) GetBatchDetails(ctx *gin.Context) {
	batchID := ctx.Param("id")
	id, err := strconv.Atoi(batchID)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid batch ID"})
		return
	}

	state := getSyncState()
	if state.LastBatch == nil {
		ctx.JSON(404, gin.H{"error": "No current batch"})
		return
	}

	if state.LastBatch.ID != id {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	ctx.JSON(200, gin.H{
		"batch": state.LastBatch,
	})
}

// ConfirmBatch confirms a batch
func (c *DiscogsController) ConfirmBatch(ctx *gin.Context) {
	batchID := ctx.Param("id")
	id, err := strconv.Atoi(batchID)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid batch ID"})
		return
	}

	state := getSyncState()
	if state.LastBatch == nil {
		ctx.JSON(404, gin.H{"error": "No current batch"})
		return
	}

	if state.LastBatch.ID != id {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	ctx.JSON(200, gin.H{
		"message": "Batch confirmed",
	})
}

// SkipBatch skips a batch
func (c *DiscogsController) SkipBatch(ctx *gin.Context) {
	batchID := ctx.Param("id")
	id, err := strconv.Atoi(batchID)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid batch ID"})
		return
	}

	state := getSyncState()
	if state.LastBatch == nil {
		ctx.JSON(404, gin.H{"error": "No current batch"})
		return
	}

	if state.LastBatch.ID != id {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	updateSyncState(func(s *sync.SyncState) {
		s.Processed += len(s.LastBatch.Albums)
		s.LastBatch = nil
	})

	ctx.JSON(200, gin.H{
		"message": "Batch skipped",
	})
}

// CancelSync cancels the current sync
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

// PauseSync pauses the current sync
func (c *DiscogsController) PauseSync(ctx *gin.Context) {
	state := getSyncState()
	if !state.IsRunning() {
		logToFile("PauseSync: No sync in progress, cannot pause")
		ctx.JSON(400, gin.H{"error": "No sync in progress"})
		return
	}

	if state.IsPaused() {
		logToFile("PauseSync: Sync is already paused")
		ctx.JSON(400, gin.H{"error": "Sync is already paused"})
		return
	}

	logToFile("PauseSync: setting IsPaused=true, current state - IsRunning=%v, Processed=%d, Total=%d, LastBatch=%v",
		state.IsRunning(), state.Processed, state.Total, state.LastBatch != nil && len(state.LastBatch.Albums) > 0)

	c.progressService.Save(state)

	logToFile("PauseSync: calling RequestPause()...")
	pauseSuccess := syncManager.RequestPause()
	logToFile("PauseSync: RequestPause() returned %v", pauseSuccess)
	stateAfterPause := getSyncState()
	logToFile("PauseSync: after RequestPause - IsRunning=%v, IsPaused=%v",
		stateAfterPause.IsRunning(), stateAfterPause.IsPaused())

	updateSyncState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusPaused
	})

	c.progressService.Save(getSyncState())

	var config models.AppConfig
	if err := c.db.First(&config).Error; err == nil {
		logToFile("PauseSync: config state - IsConnected=%v, Username=%s, HasTokens=%v",
			config.IsDiscogsConnected, config.DiscogsUsername,
			config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != "")
	}

	newState := getSyncState()
	logToFile("PauseSync: after setting - IsPaused=%v, IsRunning=%v", newState.IsPaused(), newState.IsRunning())

	ctx.JSON(200, gin.H{
		"message":    "Sync paused",
		"sync_state": newState,
	})
}

// ResumeSyncFromPause resumes a paused sync
func (c *DiscogsController) ResumeSyncFromPause(ctx *gin.Context) {
	state := getSyncState()

	if state.IsRunning() && !state.IsPaused() {
		ctx.JSON(400, gin.H{"error": "Sync is already running"})
		return
	}

	if state.IsRunning() && state.IsPaused() {
		progress := c.progressService.Load(state)
		if progress == nil {
			ctx.JSON(400, gin.H{"error": "No paused sync to resume"})
			return
		}

		var config models.AppConfig
		if err := c.db.First(&config).Error; err != nil {
			logToFile("ResumeSyncFromPause: ERROR loading config: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if !config.IsDiscogsConnected {
			logToFile("ResumeSyncFromPause: Discogs not connected, rejecting resume")
			ctx.JSON(400, gin.H{"error": "Discogs not connected"})
			return
		}

		logToFile("ResumeSyncFromPause: calling RequestResume()...")
		resumeSuccess := syncManager.RequestResume()
		logToFile("ResumeSyncFromPause: RequestResume() returned %v, restarting worker...", resumeSuccess)

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
		logToFile("ResumeSyncFromPause: restored progress - page=%d, processed=%d, total=%d, restored LastBatch with %d albums",
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
		logToFile("ResumeSyncFromPause: restarting sync worker at page %d, processed=%d", newState.CurrentPage, newState.Processed)
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
			logToFile("ResumeSyncFromPause: ERROR loading config: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		logToFile("ResumeSyncFromPause: config state - IsConnected=%v, Username=%s, HasAccessToken=%v, HasAccessSecret=%v",
			config.IsDiscogsConnected, config.DiscogsUsername,
			config.DiscogsAccessToken != "", config.DiscogsAccessSecret != "")

		if !config.IsDiscogsConnected && (config.DiscogsAccessToken == "" || config.DiscogsAccessSecret == "") {
			logToFile("ResumeSyncFromPause: Discogs not connected, rejecting resume")
			ctx.JSON(400, gin.H{"error": "Discogs not connected"})
			return
		}

		logToFile("ResumeSyncFromPause: proceeding with resume, IsConnected=%v, HasTokens=%v",
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
		logToFile("ResumeSyncFromPause: resuming from page %d with %d albums processed, restored LastBatch with %d albums",
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
				logToFile("ResumeSyncFromPause: restored %d folders from Discogs", len(folders))
			} else {
				logToFile("ResumeSyncFromPause: failed to fetch folders: %v", err)
			}
		}

		logToFile("ResumeSyncFromPause: resuming sync from folder %d, page %d, processed=%d, folders_count=%d",
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

// FetchUsername fetches the username from Discogs
func (c *DiscogsController) FetchUsername(ctx *gin.Context) {
	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
		return
	}

	username, err := client.GetUserIdentity()
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch username", "details": err.Error()})
		return
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("discogs_username", username)

	ctx.JSON(200, gin.H{
		"username": username,
	})
}

// RefreshTracks re-syncs tracks for all albums that have a DiscogsID
func (c *DiscogsController) RefreshTracks(ctx *gin.Context) {
	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client - please connect to Discogs first"})
		return
	}

	var albums []models.Album
	if err := c.db.Where("discogs_id IS NOT NULL AND discogs_id > 0").Find(&albums).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch albums"})
		return
	}

	if len(albums) == 0 {
		ctx.JSON(200, gin.H{
			"message": "No albums with Discogs ID found",
			"updated": 0,
			"failed":  0,
			"total":   0,
		})
		return
	}

	logToFile("RefreshTracks: Starting track refresh for %d albums", len(albums))

	importer := services.NewAlbumImporter(c.db, client)
	updated := 0
	failed := 0

	for _, album := range albums {
		if album.DiscogsID == nil {
			continue
		}

		discogsID := *album.DiscogsID
		logToFile("RefreshTracks: Refreshing tracks for album %d: %s - %s (DiscogsID: %d)",
			album.ID, album.Artist, album.Title, discogsID)

		success, errMsg := importer.FetchAndSaveTracks(c.db, album.ID, discogsID, album.Title, album.Artist)
		if success {
			updated++
			logToFile("RefreshTracks: Successfully refreshed tracks for %s - %s", album.Artist, album.Title)
		} else {
			failed++
			logToFile("RefreshTracks: Failed to refresh tracks for %s - %s: %s", album.Artist, album.Title, errMsg)
		}

		time.Sleep(100 * time.Millisecond)
	}

	logToFile("RefreshTracks: Completed - updated=%d, failed=%d, total=%d", updated, failed, len(albums))

	ctx.JSON(200, gin.H{
		"message": fmt.Sprintf("Track refresh completed: %d updated, %d failed", updated, failed),
		"updated": updated,
		"failed":  failed,
		"total":   len(albums),
	})
}

// FindUnlinkedAlbums finds albums that have a DiscogsID but are no longer in the user's Discogs collection
func (c *DiscogsController) FindUnlinkedAlbums(ctx *gin.Context) {
	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client - please connect to Discogs first"})
		return
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

	if config.DiscogsUsername == "" {
		ctx.JSON(400, gin.H{"error": "Discogs username not set"})
		return
	}

	var localAlbums []models.Album
	if err := c.db.Where("discogs_id IS NOT NULL").Find(&localAlbums).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch local albums"})
		return
	}

	if len(localAlbums) == 0 {
		ctx.JSON(200, gin.H{
			"message":         "No albums with Discogs ID found",
			"unlinked_albums": []interface{}{},
			"total_checked":   0,
		})
		return
	}

	logToFile("FindUnlinkedAlbums: Checking %d local albums against Discogs collection", len(localAlbums))

	localDiscogsIDs := make(map[int]models.Album)
	for _, album := range localAlbums {
		if album.DiscogsID != nil && *album.DiscogsID > 0 {
			localDiscogsIDs[*album.DiscogsID] = album
		}
	}

	discogsIDs := make(map[int]bool)
	page := 1
	batchSize := 100

	for {
		releases, _, err := client.GetUserCollectionByFolder(config.DiscogsUsername, 0, page, batchSize)
		if err != nil {
			if strings.Contains(err.Error(), "outside of valid range") {
				break
			}
			logToFile("FindUnlinkedAlbums: Error fetching page %d: %v", page, err)
			break
		}

		if len(releases) == 0 {
			break
		}

		for _, release := range releases {
			if discogsID, ok := release["discogs_id"].(int); ok {
				discogsIDs[discogsID] = true
			}
		}

		logToFile("FindUnlinkedAlbums: Fetched page %d, got %d releases, total so far: %d", page, len(releases), len(discogsIDs))
		page++

		time.Sleep(200 * time.Millisecond)
	}

	var unlinkedAlbums []gin.H
	for discogsID, album := range localDiscogsIDs {
		if !discogsIDs[discogsID] {
			unlinkedAlbums = append(unlinkedAlbums, gin.H{
				"id":         album.ID,
				"title":      album.Title,
				"artist":     album.Artist,
				"discogs_id": discogsID,
				"year":       album.ReleaseYear,
			})
		}
	}

	logToFile("FindUnlinkedAlbums: Found %d unlinked albums out of %d checked", len(unlinkedAlbums), len(localDiscogsIDs))

	ctx.JSON(200, gin.H{
		"message":         fmt.Sprintf("Found %d albums not in Discogs collection", len(unlinkedAlbums)),
		"unlinked_albums": unlinkedAlbums,
		"total_checked":   len(localDiscogsIDs),
		"discogs_total":   len(discogsIDs),
	})
}

// DeleteUnlinkedAlbums deletes specified albums and their tracks from the local database
func (c *DiscogsController) DeleteUnlinkedAlbums(ctx *gin.Context) {
	var input struct {
		AlbumIDs []uint `json:"album_ids"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	if len(input.AlbumIDs) == 0 {
		ctx.JSON(400, gin.H{"error": "No album IDs provided"})
		return
	}

	logToFile("DeleteUnlinkedAlbums: Deleting %d albums", len(input.AlbumIDs))

	deleted := 0
	failed := 0

	for _, albumID := range input.AlbumIDs {
		tx := c.db.Begin()

		if err := tx.Where("album_id = ?", albumID).Delete(&models.Track{}).Error; err != nil {
			logToFile("DeleteUnlinkedAlbums: Failed to delete tracks for album %d: %v", albumID, err)
			tx.Rollback()
			failed++
			continue
		}

		if err := tx.Delete(&models.Album{}, albumID).Error; err != nil {
			logToFile("DeleteUnlinkedAlbums: Failed to delete album %d: %v", albumID, err)
			tx.Rollback()
			failed++
			continue
		}

		if err := tx.Commit().Error; err != nil {
			logToFile("DeleteUnlinkedAlbums: Failed to commit deletion for album %d: %v", albumID, err)
			failed++
			continue
		}

		deleted++
		logToFile("DeleteUnlinkedAlbums: Deleted album %d", albumID)
	}

	logToFile("DeleteUnlinkedAlbums: Completed - deleted=%d, failed=%d", deleted, failed)

	ctx.JSON(200, gin.H{
		"message": fmt.Sprintf("Deleted %d albums, %d failed", deleted, failed),
		"deleted": deleted,
		"failed":  failed,
	})
}
