package controllers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func logToFile(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	f, _ := os.OpenFile("sync_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}

func isLockTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Lock wait timeout") || strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "try restarting transaction")
}

func maskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func getDiscogsClient() *discogs.Client {
	client := discogs.NewClient("")
	return client
}

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

func (c *DiscogsController) GetStatus(ctx *gin.Context) {
	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

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
				album.DiscogsID = input.DiscogsID

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
						input.Tracks = append(input.Tracks, struct {
							Title       string `json:"title"`
							Duration    int    `json:"duration"`
							TrackNumber int    `json:"track_number"`
							DiscNumber  int    `json:"disc_number"`
							Side        string `json:"side"`
							Position    string `json:"position"`
						}{
							Title:    t["title"].(string),
							Duration: duration,
							Side:     t["position"].(string),
							Position: t["position"].(string),
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

type SyncState struct {
	IsRunning     bool                     `json:"is_running"`
	IsPaused      bool                     `json:"is_paused"`
	CurrentPage   int                      `json:"current_page"`
	TotalPages    int                      `json:"total_pages"`
	Processed     int                      `json:"processed"`
	Total         int                      `json:"total"`
	SyncMode      string                   `json:"sync_mode"`
	CurrentFolder int                      `json:"current_folder"`
	Folders       []map[string]interface{} `json:"folders,omitempty"`
	FolderIndex   int                      `json:"folder_index"`
	APIRemaining  int                      `json:"api_remaining"`
	AnonRemaining int                      `json:"anon_remaining"`
	LastBatch     *SyncBatch               `json:"last_batch,omitempty"`
	LastReview    *discogs.BatchReview     `json:"last_review,omitempty"`
}

type SyncBatch struct {
	ID          int                      `json:"id"`
	Albums      []map[string]interface{} `json:"albums"`
	ProcessedAt *time.Time               `json:"processed_at,omitempty"`
}

var syncState = SyncState{IsRunning: false}
var syncStateMu sync.RWMutex

func getSyncState() SyncState {
	syncStateMu.RLock()
	defer syncStateMu.RUnlock()
	return syncState
}

func setSyncState(state SyncState) {
	syncStateMu.Lock()
	defer syncStateMu.Unlock()
	syncState = state
}

func updateSyncState(fn func(*SyncState)) {
	syncStateMu.Lock()
	defer syncStateMu.Unlock()
	fn(&syncState)
}

func isSyncComplete(state SyncState) bool {
	if state.IsPaused {
		return false
	}
	if !state.IsRunning {
		return true
	}
	if state.LastBatch != nil && len(state.LastBatch.Albums) > 0 {
		return false
	}
	return true
}

func ResetSyncState() {
	updateSyncState(func(s *SyncState) {
		s.IsRunning = false
		s.IsPaused = false
		s.CurrentPage = 1
		s.TotalPages = 0
		s.Processed = 0
		s.Total = 0
		s.LastBatch = nil
		s.LastReview = nil
	})
}

func (c *DiscogsController) StartSync(ctx *gin.Context) {
	logToFile("StartSync: called")
	logToFile("StartSync: IsRunning=%v, IsPaused=%v", getSyncState().IsRunning, getSyncState().IsPaused)

	if getSyncState().IsRunning {
		logToFile("StartSync: sync already in progress")
		ctx.JSON(400, gin.H{"error": "Sync already in progress"})
		return
	}

	if getSyncState().IsPaused {
		logToFile("StartSync: sync is paused, resuming...")
		c.ResumeSyncFromPause(ctx)
		return
	}

	existingProgress := loadSyncProgress(c.db)
	if existingProgress != nil {
		logToFile("StartSync: Found existing sync progress, can resume")
		ctx.JSON(200, gin.H{
			"message":       "Existing sync in progress",
			"has_progress":  true,
			"can_resume":    true,
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

	var input struct {
		SyncMode string `json:"sync_mode"`
		FolderID int    `json:"folder_id"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.SyncMode = "all-folders"
		input.FolderID = 0
	}

	if input.SyncMode == "" {
		input.SyncMode = "all-folders"
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
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
		ctx.JSON(400, gin.H{"error": "Discogs not connected"})
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
			updateSyncState(func(s *SyncState) {
				s.IsRunning = false
			})
			logToFile("StartSync: Failed to fetch username: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch Discogs username", "details": err.Error()})
			return
		}
		c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("discogs_username", username)
		config.DiscogsUsername = username
		logToFile("StartSync: Fetched username: %s", username)
	}

	state := SyncState{
		IsRunning:   true,
		CurrentPage: 1,
		SyncMode:    input.SyncMode,
	}

	if input.SyncMode == "specific" {
		state.CurrentFolder = input.FolderID
	}

	logToFile("StartSync: getting client with OAuth...")
	client := c.getDiscogsClientWithOAuth()

	if client == nil {
		updateSyncState(func(s *SyncState) {
			s.IsRunning = false
		})
		logToFile("StartSync: failed to get client - nil returned")
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
		return
	}

	if input.SyncMode == "all-folders" {
		folders, err := client.GetUserFolders(config.DiscogsUsername)
		if err != nil {
			updateSyncState(func(s *SyncState) {
				s.IsRunning = false
			})
			logToFile("StartSync: GetUserFolders FAILED: %v", err)
			ctx.JSON(500, gin.H{"error": "Failed to fetch folders", "details": err.Error()})
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
	}

	if err != nil {
		updateSyncState(func(s *SyncState) {
			s.IsRunning = false
		})
		logToFile("StartSync: Failed to start sync - %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to start sync", "details": err.Error()})
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

	go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize, input.SyncMode, state.CurrentFolder, &state.Folders)

	ctx.JSON(200, gin.H{
		"message":   "Sync started",
		"sync_mode": input.SyncMode,
		"folder_id": state.CurrentFolder,
	})
}

func processSyncBatches(db *gorm.DB, client *discogs.Client, username string, batchSize int, syncMode string, currentFolder int, folders *[]map[string]interface{}) {
	for {
		state := getSyncState()
		logToFile("Sync: loop - IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d",
			state.IsRunning, state.IsPaused, state.Processed, state.Total)

		if !state.IsRunning {
			logToFile("Sync: complete, Processed=%d/%d", state.Processed, state.Total)
			return
		}

		if state.IsPaused {
			logToFile("Sync: paused, waiting...")
			time.Sleep(2 * time.Second)
			continue
		}

		var needFetch bool
		var page int
		var folderID int

		updateSyncState(func(s *SyncState) {
			if s.LastBatch == nil || len(s.LastBatch.Albums) == 0 {
				needFetch = true
				page = s.CurrentPage
				folderID = s.CurrentFolder

				if syncMode == "all-folders" && len(*folders) > 0 {
					if s.CurrentPage > 1 {
						s.FolderIndex++
						if s.FolderIndex >= len(*folders) {
							logToFile("processSyncBatches: all folders synced complete. Total processed: %d", s.Processed)
							s.IsRunning = false
							db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
							return
						}
						s.CurrentFolder = (*folders)[s.FolderIndex]["id"].(int)
						s.CurrentPage = 1
						folderID = s.CurrentFolder
						logToFile("processSyncBatches: moving to folder %d (%s)", s.CurrentFolder, (*folders)[s.FolderIndex]["name"])
					}
				} else {
					s.CurrentPage++
					page = s.CurrentPage
				}
			} else {
				needFetch = false
				page = s.CurrentPage
				folderID = s.CurrentFolder
			}
		})

		if !getSyncState().IsRunning {
			return
		}

		state = getSyncState()
		var currentReleases []map[string]interface{}
		if state.LastBatch != nil {
			currentReleases = state.LastBatch.Albums
		}

		if needFetch {
			logToFile("processSyncBatches: fetching page %d from API for folder %d", page, folderID)
			time.Sleep(500 * time.Millisecond)

			var releases []map[string]interface{}
			var err error

			if syncMode == "all-folders" && folderID > 0 {
				releases, _, err = client.GetUserCollectionByFolder(username, folderID, page, batchSize)
			} else if syncMode == "all-folders" {
				releases, _, err = client.GetUserCollectionByFolder(username, 0, page, batchSize)
			} else if syncMode == "specific" {
				releases, _, err = client.GetUserCollectionByFolder(username, folderID, page, batchSize)
			} else {
				releases, err = client.GetUserCollection(username, page, batchSize)
			}

			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Page") && strings.Contains(errStr, "outside of valid range") {
					logToFile("processSyncBatches: reached end of pagination (page %d doesn't exist)", page)
					if syncMode == "all-folders" && len(*folders) > 0 && getSyncState().FolderIndex < len(*folders)-1 {
						logToFile("processSyncBatches: more folders to process, continuing")
						updateSyncState(func(s *SyncState) {
							s.CurrentPage = 1
							s.LastBatch = nil
						})
						continue
					}
					logToFile("processSyncBatches: all pagination complete. Processed: %d, Total: %d", getSyncState().Processed, getSyncState().Total)
					updateSyncState(func(s *SyncState) {
						s.IsRunning = false
						s.CurrentPage = page
					})
					saveSyncProgress(db)
					db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
					return
				}
				logToFile("processSyncBatches: failed to fetch page %d: %v", page, err)
				updateSyncState(func(s *SyncState) {
					s.IsRunning = false
				})
				return
			}

			if len(releases) == 0 {
				logToFile("processSyncBatches: received empty releases list at page %d", page)

				if syncMode == "all-folders" && len(*folders) > 0 {
					state := getSyncState()
					if state.FolderIndex < len(*folders)-1 {
						logToFile("processSyncBatches: moving to next folder after empty page in folder %d", state.CurrentFolder)
						updateSyncState(func(s *SyncState) {
							s.FolderIndex++
							s.CurrentFolder = (*folders)[s.FolderIndex]["id"].(int)
							s.CurrentPage = 1
							s.LastBatch = nil
						})
						logToFile("processSyncBatches: moving to folder %d (%s)",
							state.CurrentFolder, (*folders)[state.FolderIndex]["name"])
						continue
					}
				}

				currentState := getSyncState()
				logToFile("processSyncBatches: sync complete. Processed=%d, Total=%d, Mode=%s", currentState.Processed, currentState.Total, syncMode)

				if syncMode == "specific" {
					logToFile("processSyncBatches: folder-specific sync complete. Total processed: %d", currentState.Processed)
					updateSyncState(func(s *SyncState) {
						s.IsRunning = false
						s.Processed = currentState.Processed
						s.Total = currentState.Total
					})
					saveSyncProgress(db)
					db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
					return
				}

				if syncMode == "all-folders" && len(*folders) > 0 && currentState.FolderIndex < len(*folders)-1 {
					updateSyncState(func(s *SyncState) {
						s.FolderIndex++
						s.CurrentFolder = (*folders)[s.FolderIndex]["id"].(int)
						s.CurrentPage = 1
						s.LastBatch = nil
					})
					logToFile("processSyncBatches: moving to next folder %d (%s) after empty page",
						currentState.CurrentFolder, (*folders)[currentState.FolderIndex]["name"])
					continue
				}

				logToFile("processSyncBatches: all folders synced complete. Total processed: %d", currentState.Processed)
				updateSyncState(func(s *SyncState) {
					s.IsRunning = false
					s.Processed = currentState.Processed
					s.Total = currentState.Total
				})
				saveSyncProgress(db)
				db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
				return
			}

			apiRem := client.GetAPIRemaining()
			anonRem := client.GetAPIRemainingAnon()
			updateSyncState(func(s *SyncState) {
				s.LastBatch = &SyncBatch{
					ID:     page,
					Albums: releases,
				}
				s.APIRemaining = apiRem
				s.AnonRemaining = anonRem
			})
			logToFile("processSyncBatches: fetched %d albums from page %d folder %d, api_remaining=%d", len(releases), page, folderID, apiRem)
		} else {
			logToFile("processSyncBatches: processing batch %d with %d albums", state.CurrentPage, len(currentReleases))
		}

		state = getSyncState()
		releases := state.LastBatch.Albums

		logToFile("Sync: processing batch of %d albums, Processed=%d/%d", len(releases), state.Processed, state.Total)

		for _, album := range releases {
			title, _ := album["title"].(string)
			artist, _ := album["artist"].(string)
			year, _ := album["year"].(int)
			coverImage, _ := album["cover_image"].(string)
			discogsID := 0
			if v, ok := album["discogs_id"].(int); ok {
				discogsID = v
			}
			albumFolderID := 0
			if f, ok := album["folder_id"].(int); ok {
				albumFolderID = f
			}
			if albumFolderID == 0 {
				albumFolderID = state.CurrentFolder
			}

			var existingAlbum models.Album
			result := db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)

			if result.Error == gorm.ErrRecordNotFound {
				maxRetries := 3
				var newAlbum models.Album
				var tx *gorm.DB
				var createErr error

				for attempt := 0; attempt <= maxRetries; attempt++ {
					newAlbum = models.Album{
						Title:           title,
						Artist:          artist,
						ReleaseYear:     year,
						CoverImageURL:   coverImage,
						DiscogsID:       discogsID,
						DiscogsFolderID: albumFolderID,
					}
					tx = db.Begin()
					if tx.Error != nil {
						if attempt < maxRetries && isLockTimeout(tx.Error) {
							tx.Rollback()
							time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
							continue
						}
						logToFile("processSyncBatches: failed to start transaction for album: %s - %s", artist, title)
						db.Create(&models.SyncLog{
							DiscogsID:  discogsID,
							AlbumTitle: title,
							Artist:     artist,
							ErrorType:  "album",
							ErrorMsg:   fmt.Sprintf("Failed to start transaction: %v", tx.Error),
						})
						updateSyncState(func(s *SyncState) {
							s.Processed++
						})
						break
					}

					createErr = tx.Create(&newAlbum).Error
					if createErr != nil {
						tx.Rollback()
						if attempt < maxRetries && isLockTimeout(createErr) {
							time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
							continue
						}
						logToFile("processSyncBatches: failed to create album: %s - %s: %v", artist, title, createErr)
						db.Create(&models.SyncLog{
							DiscogsID:  discogsID,
							AlbumTitle: title,
							Artist:     artist,
							ErrorType:  "album",
							ErrorMsg:   fmt.Sprintf("Failed to create album: %v", createErr),
						})
						updateSyncState(func(s *SyncState) {
							s.Processed++
						})
						saveSyncProgress(db)
						break
					}

					logToFile("processSyncBatches: Created album: %s - %s (folder: %d)", artist, title, albumFolderID)

					if discogsID > 0 {
						success, errMsg := fetchTracksForAlbum(tx, client, newAlbum.ID, discogsID, title, artist)
						if !success {
							logToFile("processSyncBatches: Failed to fetch tracks for album %s - %s: %s", artist, title, errMsg)
							tx.Rollback()
							updateSyncState(func(s *SyncState) {
								s.Processed++
							})
							saveSyncProgress(db)
							break
						}
						logToFile("processSyncBatches: Successfully synced album with tracks: %s - %s", artist, title)
					}

					if err := tx.Commit().Error; err != nil {
						if attempt < maxRetries && isLockTimeout(err) {
							time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
							continue
						}
						logToFile("processSyncBatches: failed to commit album: %s - %s: %v", artist, title, err)
						updateSyncState(func(s *SyncState) {
							s.Processed++
						})
						saveSyncProgress(db)
						break
					}
					updateSyncState(func(s *SyncState) {
						s.Processed++
					})
					saveSyncProgress(db)
					logToFile("processSyncBatches: Album synced successfully: %s - %s, Processed=%d", artist, title, getSyncState().Processed)
					break
				}
				continue
			} else {
				logToFile("Sync: album exists: %s - %s", artist, title)
				updateSyncState(func(s *SyncState) {
					s.Processed++
					if s.Processed%5 == 0 {
						s.APIRemaining = client.GetAPIRemaining()
						s.AnonRemaining = client.GetAPIRemainingAnon()
					}
				})
				saveSyncProgress(db)
				logToFile("Sync: processed=%d/%d", getSyncState().Processed, getSyncState().Total)
				continue
			}
		}

		updateSyncState(func(s *SyncState) {
			s.LastBatch = nil
		})

		saveSyncProgress(db)

		time.Sleep(200 * time.Millisecond)
	}

	apiRem := client.GetAPIRemaining()
	anonRem := client.GetAPIRemainingAnon()
	updateSyncState(func(s *SyncState) {
		s.LastBatch = nil
		s.APIRemaining = apiRem
		s.AnonRemaining = anonRem
	})
	time.Sleep(100 * time.Millisecond)
	saveSyncProgress(db)
	db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
}

func fetchTracksForAlbum(db *gorm.DB, client *discogs.Client, albumID uint, discogsID int, albumTitle, artist string) (bool, string) {
	logToFile("fetchTracksForAlbum: fetching tracks for album ID %d, discogs ID %d", albumID, discogsID)

	tracks, err := client.GetTracksForAlbum(discogsID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch tracks: %v", err)
		logToFile("fetchTracksForAlbum: %s", errMsg)
		db.Create(&models.SyncLog{
			DiscogsID:  discogsID,
			AlbumTitle: albumTitle,
			Artist:     artist,
			ErrorType:  "tracks",
			ErrorMsg:   errMsg,
		})
		return false, errMsg
	}

	if len(tracks) == 0 {
		errMsg := "No tracks found for release"
		logToFile("fetchTracksForAlbum: %s", errMsg)
		db.Create(&models.SyncLog{
			DiscogsID:  discogsID,
			AlbumTitle: albumTitle,
			Artist:     artist,
			ErrorType:  "tracks",
			ErrorMsg:   errMsg,
		})
		return false, errMsg
	}

	for _, track := range tracks {
		title := ""
		if t, ok := track["title"].(string); ok {
			title = t
		}
		position := ""
		if p, ok := track["position"].(string); ok {
			position = p
		}
		duration := 0
		if d, ok := track["duration"].(float64); ok {
			duration = int(d)
		}

		maxTrackRetries := 3
		var newTrack models.Track
		var trackErr error

		for attempt := 0; attempt <= maxTrackRetries; attempt++ {
			newTrack = models.Track{
				AlbumID:     albumID,
				AlbumTitle:  albumTitle,
				Title:       title,
				Duration:    duration,
				TrackNumber: 0,
				DiscNumber:  0,
				Side:        position,
				Position:    position,
			}
			trackErr = db.Create(&newTrack).Error
			if trackErr != nil {
				if attempt < maxTrackRetries && isLockTimeout(trackErr) {
					time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
					continue
				}
				errMsg := fmt.Sprintf("Failed to create track %s: %v", title, trackErr)
				logToFile("fetchTracksForAlbum: %s", errMsg)
				db.Create(&models.SyncLog{
					DiscogsID:  discogsID,
					AlbumTitle: albumTitle,
					Artist:     artist,
					ErrorType:  "track",
					ErrorMsg:   errMsg,
				})
				return false, errMsg
			}
			break
		}
	}

	return true, ""
}

func loadSyncProgress(db *gorm.DB) *models.SyncProgress {
	var progress models.SyncProgress
	if err := db.Order("id DESC").First(&progress).Error; err != nil {
		return nil
	}

	if time.Since(progress.LastActivityAt) > 30*time.Minute {
		db.Delete(&progress)
		return nil
	}

	return &progress
}

func saveSyncProgress(db *gorm.DB) {
	state := getSyncState()
	if !state.IsRunning {
		return
	}

	var progress models.SyncProgress
	db.FirstOrCreate(&progress, models.SyncProgress{ID: 1})

	progress.SyncMode = state.SyncMode
	progress.FolderID = state.CurrentFolder
	progress.FolderIndex = state.FolderIndex
	progress.CurrentPage = state.CurrentPage
	progress.Processed = state.Processed
	progress.TotalAlbums = state.Total
	progress.LastActivityAt = time.Now()

	var folderNames []string
	for _, f := range state.Folders {
		if name, ok := f["name"].(string); ok {
			folderNames = append(folderNames, name)
		}
	}
	if len(folderNames) > 0 && state.FolderIndex < len(folderNames) {
		progress.FolderName = folderNames[state.FolderIndex]
	}

	db.Save(&progress)
}

type DiscogsController struct {
	db *gorm.DB
}

func NewDiscogsController(db *gorm.DB) *DiscogsController {
	return &DiscogsController{db: db}
}

func (c *DiscogsController) GetSyncProgress(ctx *gin.Context) {
	state := getSyncState()

	var savedProgress *models.SyncProgress
	if state.IsRunning {
		savedProgress = loadSyncProgress(c.db)
	}

	logToFile("GetSyncProgress: IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d, LastBatch=%v",
		state.IsRunning, state.IsPaused, state.Processed, state.Total,
		state.LastBatch != nil && len(state.LastBatch.Albums) > 0)

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

	savedFolderID := 0
	savedFolderName := ""
	savedProcessed := 0
	savedTotalAlbums := 0
	var savedLastActivity time.Time
	if savedProgress != nil {
		savedFolderID = savedProgress.FolderID
		savedFolderName = savedProgress.FolderName
		savedProcessed = savedProgress.Processed
		savedTotalAlbums = savedProgress.TotalAlbums
		savedLastActivity = savedProgress.LastActivityAt
	}

	ctx.JSON(200, gin.H{
		"is_running":          state.IsRunning,
		"is_paused":           state.IsPaused,
		"current_page":        state.CurrentPage,
		"total_pages":         state.TotalPages,
		"processed":           state.Processed,
		"total":               state.Total,
		"sync_mode":           state.SyncMode,
		"current_folder":      state.CurrentFolder,
		"folder_index":        state.FolderIndex,
		"total_folders":       totalFolders,
		"folder_name":         folderName,
		"folders":             state.Folders,
		"api_remaining":       state.APIRemaining,
		"anon_remaining":      state.AnonRemaining,
		"last_batch":          state.LastBatch,
		"has_saved_progress":  savedProgress != nil,
		"saved_status":        "running",
		"saved_folder_id":     savedFolderID,
		"saved_folder_name":   savedFolderName,
		"saved_processed":     savedProcessed,
		"saved_total_albums":  savedTotalAlbums,
		"saved_last_activity": savedLastActivity,
	})
	logToFile("GetSyncProgress: IsRunning=%v, Processed=%d, Total=%d", state.IsRunning, state.Processed, state.Total)
}

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
			DiscogsID:       discogsID,
			DiscogsFolderID: folderID,
		}
		if err := tx.Create(&newAlbum).Error; err != nil {
			tx.Rollback()
			continue
		}

		if discogsID > 0 {
			client := c.getDiscogsClientWithOAuth()
			if client != nil {
				success, _ := fetchTracksForAlbum(tx, client, newAlbum.ID, discogsID, title, artist)
				if !success {
					tx.Rollback()
					continue
				}
			}
		}

		if err := tx.Commit().Error; err != nil {
			logToFile("ApplyBatch: failed to commit transaction for album %s - %s: %v", artist, title, err)
			tx.Rollback()
			continue
		}
	}

	updateSyncState(func(s *SyncState) {
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

func (c *DiscogsController) StopSync(ctx *gin.Context) {
	updateSyncState(func(s *SyncState) {
		s.IsRunning = false
	})

	c.db.Delete(&models.SyncProgress{}, "1=1")

	ctx.JSON(200, gin.H{
		"message":    "Sync stopped",
		"sync_state": getSyncState(),
	})
}

func (c *DiscogsController) ClearProgress(ctx *gin.Context) {
	c.db.Delete(&models.SyncProgress{}, "1=1")
	ctx.JSON(200, gin.H{"message": "Progress cleared"})
}

func (c *DiscogsController) ResumeSync(ctx *gin.Context) {
	existingProgress := loadSyncProgress(c.db)
	if existingProgress == nil {
		ctx.JSON(400, gin.H{"error": "No sync in progress to resume"})
		return
	}

	state := getSyncState()
	if state.IsRunning {
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

	state.IsRunning = true
	state.SyncMode = existingProgress.SyncMode
	state.CurrentFolder = existingProgress.FolderID
	state.FolderIndex = existingProgress.FolderIndex
	state.CurrentPage = existingProgress.CurrentPage
	setSyncState(state)

	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		updateSyncState(func(s *SyncState) {
			s.IsRunning = false
		})
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
		return
	}

	if existingProgress.SyncMode == "all-folders" {
		client := c.getDiscogsClientWithOAuth()
		if client != nil {
			folders, err := client.GetUserFolders(config.DiscogsUsername)
			if err == nil {
				updateSyncState(func(s *SyncState) {
					s.Folders = folders
				})
			}
		}
	}

	foldersState := getSyncState()
	go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize, existingProgress.SyncMode, existingProgress.FolderID, &foldersState.Folders)

	ctx.JSON(200, gin.H{
		"message":    "Sync resumed",
		"sync_state": state,
	})
}

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

	updateSyncState(func(s *SyncState) {
		s.Processed += len(s.LastBatch.Albums)
		s.LastBatch = nil
	})

	ctx.JSON(200, gin.H{
		"message": "Batch skipped",
	})
}

func (c *DiscogsController) CancelSync(ctx *gin.Context) {
	updateSyncState(func(s *SyncState) {
		s.IsRunning = false
	})

	c.db.Delete(&models.SyncProgress{}, "1=1")

	ctx.JSON(200, gin.H{
		"message":    "Sync cancelled",
		"sync_state": getSyncState(),
	})
}

func (c *DiscogsController) PauseSync(ctx *gin.Context) {
	state := getSyncState()
	if !state.IsRunning {
		ctx.JSON(400, gin.H{"error": "No sync in progress"})
		return
	}

	if state.IsPaused {
		ctx.JSON(400, gin.H{"error": "Sync is already paused"})
		return
	}

	logToFile("PauseSync: setting IsPaused=true, current state - IsRunning=%v, Processed=%d, Total=%d",
		state.IsRunning, state.Processed, state.Total)

	updateSyncState(func(s *SyncState) {
		s.IsPaused = true
	})

	logToFile("PauseSync: sync paused at processed=%d, total=%d", state.Processed, state.Total)
	saveSyncProgress(c.db)

	newState := getSyncState()
	logToFile("PauseSync: after setting - IsPaused=%v", newState.IsPaused)

	ctx.JSON(200, gin.H{
		"message":    "Sync paused",
		"sync_state": newState,
	})
}

func (c *DiscogsController) ResumeSyncFromPause(ctx *gin.Context) {
	state := getSyncState()
	if state.IsRunning && !state.IsPaused {
		ctx.JSON(400, gin.H{"error": "Sync is already running"})
		return
	}

	if !state.IsRunning && !state.IsPaused {
		existingProgress := loadSyncProgress(c.db)
		if existingProgress == nil {
			ctx.JSON(400, gin.H{"error": "No paused sync to resume"})
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

		state.IsRunning = true
		state.SyncMode = existingProgress.SyncMode
		state.CurrentFolder = existingProgress.FolderID
		state.FolderIndex = existingProgress.FolderIndex
		state.CurrentPage = existingProgress.CurrentPage
		state.IsPaused = false
		setSyncState(state)

		client := c.getDiscogsClientWithOAuth()
		if client == nil {
			updateSyncState(func(s *SyncState) {
				s.IsRunning = false
				s.IsPaused = false
			})
			ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
			return
		}

		foldersState := getSyncState()
		logToFile("ResumeSyncFromPause: resuming sync from folder %d, page %d, processed=%d",
			existingProgress.FolderID, existingProgress.CurrentPage, existingProgress.Processed)

		go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize,
			existingProgress.SyncMode, existingProgress.FolderID, &foldersState.Folders)

		ctx.JSON(200, gin.H{
			"message":    "Sync resumed from pause",
			"sync_state": state,
		})
		return
	}

	updateSyncState(func(s *SyncState) {
		s.IsPaused = false
	})

	logToFile("ResumeSyncFromPause: sync resumed from pause, processed=%d, total=%d", state.Processed, state.Total)

	ctx.JSON(200, gin.H{
		"message":    "Sync resumed from pause",
		"sync_state": getSyncState(),
	})
}

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
