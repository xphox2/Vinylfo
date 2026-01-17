package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// intPtr returns a pointer to the given int, or nil if the value is 0
func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func downloadImage(imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("invalid content type: %s", contentType)
	}

	return data, contentType, nil
}

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

type SyncBatch = sync.SyncBatch

var syncManager = sync.DefaultLegacyManager

func getSyncState() sync.LegacySyncState {
	return syncManager.GetState()
}

func updateSyncState(fn func(*sync.LegacySyncState)) {
	syncManager.UpdateState(fn)
}

func ResetSyncState() {
	syncManager.Reset()
}

func setSyncState(state sync.LegacySyncState) {
	syncManager.UpdateState(func(s *sync.LegacySyncState) {
		*s = state
	})
}

func removeFirstAlbumFromBatch(s *sync.LegacySyncState) {
	if s.LastBatch != nil && len(s.LastBatch.Albums) > 0 {
		s.LastBatch.Albums = s.LastBatch.Albums[1:]
		if len(s.LastBatch.Albums) == 0 {
			s.LastBatch = nil
		}
	}
}

func isSyncComplete(state sync.LegacySyncState) bool {
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

func (c *DiscogsController) StartSync(ctx *gin.Context) {
	logToFile("StartSync: called")
	state := getSyncState()
	logToFile("StartSync: IsRunning=%v, IsPaused=%v", state.IsRunning, state.IsPaused)

	if state.IsRunning {
		logToFile("StartSync: sync already in progress")
		ctx.JSON(400, gin.H{"error": "Sync already in progress"})
		return
	}

	if state.IsPaused {
		logToFile("StartSync: sync is paused, resuming...")
		c.ResumeSyncFromPause(ctx)
		return
	}

	existingProgress := loadSyncProgress(c.db)
	if existingProgress != nil {
		logToFile("StartSync: Found existing sync progress, status=%s, is_running=%v, is_paused=%v", existingProgress.Status, state.IsRunning, state.IsPaused)

		// If sync is not running and not paused, it means the previous sync completed
		// Archive it to history and start fresh
		if !state.IsRunning && !state.IsPaused && existingProgress.Status == "completed" {
			logToFile("StartSync: Previous sync completed, archiving to history and starting fresh")
			archiveSyncToHistory(c.db, existingProgress)
			c.db.Exec("DELETE FROM sync_progresses WHERE id = ?", existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if existingProgress.Status == "completed" {
			// Even if state isn't synced, if DB shows completed, archive and start fresh
			logToFile("StartSync: DB shows sync completed, archiving to history and starting fresh")
			archiveSyncToHistory(c.db, existingProgress)
			c.db.Exec("DELETE FROM sync_progresses WHERE id = ?", existingProgress.ID)
			ResetSyncState()
			existingProgress = nil
		} else if !state.IsRunning && !state.IsPaused {
			// No progress in DB, start fresh
			logToFile("StartSync: No active sync, clearing state and starting fresh")
			ResetSyncState()
			existingProgress = nil
		} else {
			// Sync is actually in progress or paused
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
		c.db.Delete(&models.SyncProgress{})
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
			updateSyncState(func(s *sync.LegacySyncState) {
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

	state = sync.LegacySyncState{
		IsRunning:    true,
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
		updateSyncState(func(s *sync.LegacySyncState) {
			s.IsRunning = false
		})
		logToFile("StartSync: failed to get client - nil returned")
		ctx.JSON(500, gin.H{"error": "Failed to initialize Discogs client. Please reconnect your Discogs account in Settings."})
		return
	}

	if input.SyncMode == "all-folders" {
		folders, err := client.GetUserFolders(config.DiscogsUsername)
		if err != nil {
			updateSyncState(func(s *sync.LegacySyncState) {
				s.IsRunning = false
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
		updateSyncState(func(s *sync.LegacySyncState) {
			s.IsRunning = false
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

	go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize, input.SyncMode, state.CurrentFolder, &state.Folders)

	ctx.JSON(200, gin.H{
		"message":   "Sync started",
		"sync_mode": input.SyncMode,
		"folder_id": state.CurrentFolder,
	})
}

func processSyncBatches(db *gorm.DB, client *discogs.Client, username string, batchSize int, syncMode string, currentFolder int, folders *[]map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logToFile("Sync: PANIC in processSyncBatches: %v", r)
			// Reset sync state on panic
			updateSyncState(func(s *sync.LegacySyncState) {
				s.IsRunning = false
				s.IsPaused = false
				s.LastActivity = time.Time{}
			})
		}
	}()

	logToFile("Sync: processSyncBatches STARTING")

	// Get initial state for logging
	initialState := getSyncState()
	logToFile("Sync: initial state - IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d",
		initialState.IsRunning, initialState.IsPaused, initialState.Processed, initialState.Total)

	for {
		logToFile("Sync: ========== LOOP ITERATION START ==========")

		// ALWAYS update last activity timestamp
		logToFile("Sync: updating last activity")
		updateSyncState(func(s *sync.LegacySyncState) {
			s.LastActivity = time.Now()
		})

		// ALWAYS read fresh state
		logToFile("Sync: reading fresh state")
		state := getSyncState()
		lastBatchAlbums := 0
		if state.LastBatch != nil {
			lastBatchAlbums = len(state.LastBatch.Albums)
		}
		logToFile("Sync: loop TOP - IsRunning=%v, IsPaused=%v, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.IsRunning, state.IsPaused, state.Processed,
			state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		if !state.IsRunning {
			logToFile("Sync: complete (not running), Processed=%d/%d", state.Processed, state.Total)
			return
		}

		logToFile("Sync: NOT PAUSED - proceeding with batch processing, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.Processed,
			state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		// Determine if we need to fetch new data from API
		var needFetch bool
		page := state.CurrentPage
		folderID := state.CurrentFolder

		if state.LastBatch == nil || len(state.LastBatch.Albums) == 0 {
			needFetch = true

			if syncMode == "all-folders" && len(*folders) > 0 {
				if state.CurrentPage > 1 {
					// Move to next folder
					if state.FolderIndex >= len(*folders)-1 {
						logToFile("processSyncBatches: all folders synced complete. Total processed: %d", state.Processed)
						updateSyncState(func(s *sync.LegacySyncState) {
							s.IsRunning = false
							s.IsPaused = false
							s.Total = state.Processed // Use actual processed count as final total
							s.LastBatch = nil
							s.LastActivity = time.Time{}
						})
						saveSyncProgress(db)
						if progress := loadSyncProgress(db); progress != nil {
							archiveSyncToHistory(db, progress)
							db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
						}
						db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
						return
					}
					state.FolderIndex++
					state.CurrentFolder = (*folders)[state.FolderIndex]["id"].(int)
					state.CurrentPage = 1
					folderID = state.CurrentFolder
					page = 1
					logToFile("processSyncBatches: moving to folder %d (%s)", state.CurrentFolder, (*folders)[state.FolderIndex]["name"])
				}
			} else {
				// Will increment page after successful fetch (in the fetch block below)
			}
		} else {
			// Use current batch data
			needFetch = false
		}

		// Check if sync was stopped during our processing
		if !state.IsRunning {
			return
		}

		// Get current releases to process
		currentReleases := []map[string]interface{}{}
		if state.LastBatch != nil {
			currentReleases = state.LastBatch.Albums
		}

		if needFetch {
			logToFile("processSyncBatches: fetching page %d from API for folder %d", page, folderID)
			time.Sleep(500 * time.Millisecond)

			// Check if sync was stopped or paused before making API call
			if !state.IsRunning || state.IsPaused {
				return
			}

			var releases []map[string]interface{}
			var err error
			var totalItems int

			if syncMode == "all-folders" && folderID > 0 {
				releases, totalItems, err = client.GetUserCollectionByFolder(username, folderID, page, batchSize)
			} else if syncMode == "all-folders" {
				releases, totalItems, err = client.GetUserCollectionByFolder(username, 0, page, batchSize)
			} else if syncMode == "specific" {
				releases, totalItems, err = client.GetUserCollectionByFolder(username, folderID, page, batchSize)
			} else {
				releases, err = client.GetUserCollection(username, page, batchSize)
			}

			// Update total if API reports a different count (collection may have changed)
			if totalItems > 0 {
				currentState := getSyncState()
				if totalItems != currentState.Total {
					logToFile("processSyncBatches: API reports total=%d, updating from %d", totalItems, currentState.Total)
					updateSyncState(func(s *sync.LegacySyncState) {
						s.Total = totalItems
					})
				}
			}

			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Page") && strings.Contains(errStr, "outside of valid range") {
					logToFile("processSyncBatches: reached end of pagination (page %d doesn't exist)", page)

					// Handle end of current folder
					if syncMode == "all-folders" && len(*folders) > 0 && state.FolderIndex < len(*folders)-1 {
						// Move to next folder
						logToFile("processSyncBatches: more folders to process, continuing")
						updateSyncState(func(s *sync.LegacySyncState) {
							s.CurrentPage = 1
							s.LastBatch = nil
						})
						continue
					}

					// All folders/pagination complete - mark as completed with actual count
					logToFile("processSyncBatches: all pagination complete. Processed: %d, Total: %d", state.Processed, state.Total)
					updateSyncState(func(s *sync.LegacySyncState) {
						s.IsRunning = false
						s.IsPaused = false
						s.Total = state.Processed // Use actual processed count as final total
						s.LastBatch = nil
						s.LastActivity = time.Time{}
					})
					saveSyncProgress(db)
					if progress := loadSyncProgress(db); progress != nil {
						archiveSyncToHistory(db, progress)
						db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
					}
					db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
					return
				}
				logToFile("processSyncBatches: failed to fetch page %d: %v", page, err)
				updateSyncState(func(s *sync.LegacySyncState) {
					s.IsRunning = false
					s.IsPaused = false
					s.LastBatch = nil
					s.LastActivity = time.Time{}
				})
				return
			}

			if len(releases) == 0 {
				logToFile("processSyncBatches: received empty releases list at page %d", page)

				// Update total to reflect actual processed count if we've gone past the initial estimate
				checkState := getSyncState()
				if checkState.Processed > checkState.Total {
					updateSyncState(func(s *sync.LegacySyncState) {
						s.Total = checkState.Processed
					})
					logToFile("processSyncBatches: adjusted total to %d (was %d)", checkState.Processed, checkState.Total)
				}

				// Handle empty page - move to next folder or complete
				if syncMode == "all-folders" && len(*folders) > 0 {
					if state.FolderIndex < len(*folders)-1 {
						logToFile("processSyncBatches: moving to next folder after empty page in folder %d", state.CurrentFolder)
						updateSyncState(func(s *sync.LegacySyncState) {
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

				// No more albums to sync
				currentState := getSyncState()
				logToFile("processSyncBatches: sync complete. Processed=%d, Total=%d, Mode=%s", currentState.Processed, currentState.Total, syncMode)

				updateSyncState(func(s *sync.LegacySyncState) {
					s.IsRunning = false
					s.IsPaused = false
					s.Total = currentState.Processed // Use actual processed count as final total
					s.LastBatch = nil
					s.LastActivity = time.Time{}
				})
				saveSyncProgress(db)
				if progress := loadSyncProgress(db); progress != nil {
					archiveSyncToHistory(db, progress)
					db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
				}
				db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
				return
			}

			apiRem := client.GetAPIRemaining()
			anonRem := client.GetAPIRemainingAnon()
			logToFile("processSyncBatches: fetched %d albums from page %d folder %d, api_remaining=%d", len(releases), page, folderID, apiRem)

			if len(releases) < batchSize {
				logToFile("processSyncBatches: received fewer albums than page size (%d < %d), sync complete", len(releases), batchSize)
				currentState := getSyncState()
				updateSyncState(func(s *sync.LegacySyncState) {
					s.IsRunning = false
					s.IsPaused = false
					s.Total = currentState.Processed
					s.LastBatch = nil
					s.LastActivity = time.Time{}
				})
				saveSyncProgress(db)
				if progress := loadSyncProgress(db); progress != nil {
					archiveSyncToHistory(db, progress)
					db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
				}
				db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
				return
			}

			updateSyncState(func(s *sync.LegacySyncState) {
				s.LastBatch = &SyncBatch{
					ID:     page,
					Albums: releases,
				}
				s.APIRemaining = apiRem
				s.AnonRemaining = anonRem
				s.CurrentPage = page + 1
			})

			// Check if sync was stopped or paused after API call
			state = getSyncState()
			if !state.IsRunning || state.IsPaused {
				return
			}

			// Use the newly set batch
			currentReleases = releases
		} else {
			logToFile("processSyncBatches: processing batch %d with %d albums", state.CurrentPage, len(currentReleases))
		}

		// Re-check running state after potential API call delays
		state = getSyncState()
		if !state.IsRunning || state.IsPaused {
			return
		}

		releases := currentReleases
		if len(releases) == 0 {
			logToFile("processSyncBatches: no releases to process, continuing loop")
			time.Sleep(200 * time.Millisecond)
			continue
		}

		logToFile("Sync: processing batch of %d albums, Processed=%d/%d", len(releases), state.Processed, state.Total)

		for _, album := range releases {
			// Check if sync stopped or paused before processing each album
			// Note: We need to check global state here since it could have changed
			currentCheck := getSyncState()
			if !currentCheck.IsRunning || currentCheck.IsPaused {
				return
			}

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
			var result *gorm.DB

			// Check for existing album - prefer DiscogsID match, fall back to title+artist
			if discogsID > 0 {
				result = db.Where("discogs_id = ?", discogsID).First(&existingAlbum)
				if result.Error == gorm.ErrRecordNotFound {
					// No match by DiscogsID, check by title+artist as fallback
					result = db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)
				}
			} else {
				result = db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)
			}

			if result.Error == gorm.ErrRecordNotFound {
				maxRetries := 3
				var newAlbum models.Album
				var tx *gorm.DB
				var createErr error

				imageData, imageType, imageErr := downloadImage(coverImage)
				imageFailed := imageErr != nil
				if imageErr != nil {
					logToFile("processSyncBatches: failed to download image for %s - %s: %v", artist, title, imageErr)
				}

				for attempt := 0; attempt <= maxRetries; attempt++ {
					newAlbum = models.Album{
						Title:                 title,
						Artist:                artist,
						ReleaseYear:           year,
						CoverImageURL:         coverImage,
						DiscogsCoverImage:     imageData,
						DiscogsCoverImageType: imageType,
						CoverImageFailed:      imageFailed,
						DiscogsID:             intPtr(discogsID),
						DiscogsFolderID:       albumFolderID,
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
						saveSyncProgress(db)
						break
					}

					logToFile("processSyncBatches: Created album: %s - %s (folder: %d)", artist, title, albumFolderID)

					if discogsID > 0 {
						success, errMsg := fetchTracksForAlbum(tx, client, newAlbum.ID, discogsID, title, artist)
						if !success {
							logToFile("processSyncBatches: Failed to fetch tracks for album %s - %s: %s", artist, title, errMsg)
							tx.Rollback()
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
						saveSyncProgress(db)
						break
					}
					updateSyncState(func(s *sync.LegacySyncState) {
						s.Processed++
					})
					saveSyncProgress(db)
					updateSyncState(func(s *sync.LegacySyncState) {
						removeFirstAlbumFromBatch(s)
					})
					saveSyncProgress(db)
					logToFile("processSyncBatches: Album synced successfully: %s - %s, Processed=%d", artist, title, getSyncState().Processed)
					break
				}
				continue
			} else {
				// Album exists - check if we should update metadata from Discogs
				updated := false
				updates := make(map[string]interface{})

				// Update DiscogsID if it was previously missing
				if existingAlbum.DiscogsID == nil && discogsID > 0 {
					updates["discogs_id"] = discogsID
					updated = true
				}

				// Update folder ID if changed
				if albumFolderID > 0 && existingAlbum.DiscogsFolderID != albumFolderID {
					updates["discogs_folder_id"] = albumFolderID
					updated = true
				}

				// Update cover image if we have one and it's different or was missing
				if coverImage != "" && existingAlbum.CoverImageURL != coverImage {
					updates["cover_image_url"] = coverImage
					// Also download the new image
					if imageData, imageType, err := downloadImage(coverImage); err == nil && imageData != nil {
						updates["discogs_cover_image"] = imageData
						updates["discogs_cover_image_type"] = imageType
						updates["cover_image_failed"] = false
					}
					updated = true
				}

				// Update year if we have one and existing is 0
				if year > 0 && existingAlbum.ReleaseYear == 0 {
					updates["release_year"] = year
					updated = true
				}

				if updated {
					if err := db.Model(&existingAlbum).Updates(updates).Error; err != nil {
						logToFile("Sync: failed to update album %s - %s: %v", artist, title, err)
					} else {
						logToFile("Sync: updated existing album: %s - %s", artist, title)
					}
				} else {
					logToFile("Sync: album exists (no updates needed): %s - %s", artist, title)
				}

				// Note: Track re-sync is NOT done here to keep sync fast.
				// Use the separate "Refresh Tracks" feature to re-sync tracks for existing albums.

				updateSyncState(func(s *sync.LegacySyncState) {
					s.Processed++
				})
				saveSyncProgress(db)
				updateSyncState(func(s *sync.LegacySyncState) {
					removeFirstAlbumFromBatch(s)
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

		// Check if LastBatch is empty after processing - if so, fetch next page
		state = getSyncState()
		if state.LastBatch == nil || len(state.LastBatch.Albums) == 0 {
			// Before fetching next page, check if paused
			// This ensures we finish processing current batch before pausing
			if state.IsPaused {
				logToFile("Sync: PAUSED - entering wait loop (batch empty), current state IsPaused=%v, IsRunning=%v", state.IsPaused, state.IsRunning)

				// Poll for resume without timeout - wait until not paused
				waitStart := time.Now()
				for {
					checkState := getSyncState()
					elapsed := time.Since(waitStart)

					// Log every check to help debug
					if elapsed.Seconds() < 1 || elapsed.Seconds() > 5 || checkState.IsPaused != state.IsPaused {
						logToFile("Sync: wait loop check #%d - IsPaused=%v, IsRunning=%v, elapsed=%v",
							int(elapsed.Seconds()*10), checkState.IsPaused, checkState.IsRunning, elapsed)
					}

					if !checkState.IsPaused {
						logToFile("Sync: RESUME DETECTED after %v", elapsed)
						break
					}

					if !checkState.IsRunning {
						logToFile("Sync: sync stopped while paused")
						return
					}

					time.Sleep(100 * time.Millisecond)
				}

				logToFile("Sync: continuing loop after wait, about to re-read state")
				continue
			}

			logToFile("processSyncBatches: batch empty, will fetch next page")
			// LastBatch will be cleared and next page fetched in next loop iteration
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// Save progress - LastBatch still has failed albums that need retry
		saveSyncProgress(db)

		time.Sleep(200 * time.Millisecond)
	}

	apiRem := client.GetAPIRemaining()
	anonRem := client.GetAPIRemainingAnon()
	updateSyncState(func(s *sync.LegacySyncState) {
		s.LastBatch = nil
		s.APIRemaining = apiRem
		s.AnonRemaining = anonRem
	})
	time.Sleep(100 * time.Millisecond)
	saveSyncProgress(db)
	if progress := loadSyncProgress(db); progress != nil {
		archiveSyncToHistory(db, progress)
		db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
	}
	db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
}

func fetchTracksForAlbum(db *gorm.DB, client *discogs.Client, albumID uint, discogsID int, albumTitle, artist string) (bool, string) {
	logToFile("fetchTracksForAlbum: fetching tracks for album ID %d, discogs ID %d", albumID, discogsID)

	// Check pause state before making API call - if paused, abort early
	checkState := getSyncState()
	if checkState.IsPaused {
		logToFile("fetchTracksForAlbum: sync was paused, aborting track fetch for %s - %s", artist, albumTitle)
		return false, "sync paused"
	}

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

	// Check pause state before second API call
	checkState = getSyncState()
	if checkState.IsPaused {
		logToFile("fetchTracksForAlbum: sync was paused, aborting metadata fetch for %s - %s", artist, albumTitle)
		return false, "sync paused"
	}

	logToFile("fetchTracksForAlbum: Fetching full album metadata for discogs ID %d", discogsID)
	fullAlbumData, err := client.GetAlbum(discogsID)
	if err == nil {
		updates := make(map[string]interface{})

		if v, ok := fullAlbumData["genre"].(string); ok && v != "" {
			updates["genre"] = v
		}
		if v, ok := fullAlbumData["style"].(string); ok && v != "" {
			updates["style"] = v
		}
		if v, ok := fullAlbumData["label"].(string); ok && v != "" {
			updates["label"] = v
		}
		if v, ok := fullAlbumData["country"].(string); ok && v != "" {
			updates["country"] = v
		}
		if v, ok := fullAlbumData["cover_image"].(string); ok && v != "" {
			updates["cover_image_url"] = v

			imageData, imageType, imageErr := downloadImage(v)
			if imageErr != nil {
				logToFile("fetchTracksForAlbum: failed to download image for album %s - %s: %v", albumTitle, artist, imageErr)
				updates["cover_image_failed"] = true
			} else if len(imageData) > 0 {
				updates["discogs_cover_image"] = imageData
				updates["discogs_cover_image_type"] = imageType
				updates["cover_image_failed"] = false
			}
		}

		if len(updates) > 0 {
			if err := db.Model(&models.Album{}).Where("id = ?", albumID).Updates(updates).Error; err != nil {
				logToFile("fetchTracksForAlbum: Failed to update album metadata: %v", err)
			} else {
				logToFile("fetchTracksForAlbum: Updated album metadata: genre=%v, style=%v, label=%v, country=%v, cover_image=%v",
					updates["genre"], updates["style"], updates["label"], updates["country"], updates["cover_image_url"])
			}
		}
	} else {
		logToFile("fetchTracksForAlbum: Failed to fetch full album metadata: %v", err)
	}

	logToFile("fetchTracksForAlbum: Removing existing tracks for album ID %d before syncing %d new tracks", albumID, len(tracks))
	if err := db.Where("album_id = ?", albumID).Delete(&models.Track{}).Error; err != nil {
		logToFile("fetchTracksForAlbum: Warning - failed to delete existing tracks: %v", err)
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
		switch v := track["duration"].(type) {
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

	// Use raw SQL with a short timeout to avoid hanging on locked tables
	// This prevents GetSyncProgress from blocking when sync goroutine has transactions
	err := db.Raw("SELECT id, folder_id, folder_name, current_page, processed, total_albums, last_activity_at, status, last_batch_json FROM sync_progresses ORDER BY id DESC LIMIT 1").Scan(&progress).Error

	if err != nil || progress.ID == 0 {
		return nil
	}

	state := getSyncState()
	maxAge := 30 * time.Minute
	if state.IsPaused {
		maxAge = 4 * time.Hour
	}

	if time.Since(progress.LastActivityAt) > maxAge {
		// Use raw SQL to avoid transaction issues
		db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
		return nil
	}

	return &progress
}

func archiveSyncToHistory(db *gorm.DB, progress *models.SyncProgress) {
	var history models.SyncHistory
	history.SyncMode = progress.SyncMode
	history.FolderID = progress.FolderID
	history.FolderName = progress.FolderName
	history.Processed = progress.Processed
	history.TotalAlbums = progress.TotalAlbums
	history.Status = progress.Status
	history.StartedAt = progress.CreatedAt
	history.CompletedAt = progress.LastActivityAt
	if progress.LastActivityAt.IsZero() {
		history.CompletedAt = time.Now()
	}
	history.DurationSecs = int(history.CompletedAt.Sub(history.StartedAt).Seconds())
	if history.DurationSecs < 1 {
		history.DurationSecs = 1
	}

	db.Create(&history)
	logToFile("archiveSyncToHistory: archived sync with %d/%d albums in %d seconds", history.Processed, history.TotalAlbums, history.DurationSecs)
}

func restoreLastBatch(db *gorm.DB, state *sync.LegacySyncState) {
	progress := loadSyncProgress(db)
	if progress == nil || progress.LastBatchJSON == "" {
		logToFile("restoreLastBatch: no batch to restore")
		return
	}

	var batch SyncBatch
	if err := json.Unmarshal([]byte(progress.LastBatchJSON), &batch); err == nil {
		state.LastBatch = &batch
		logToFile("restoreLastBatch: restored batch with %d albums from page %d, processed=%d",
			len(batch.Albums), batch.ID, state.Processed)
	} else {
		logToFile("restoreLastBatch: failed to unmarshal batch: %v", err)
	}
}

func saveSyncProgress(db *gorm.DB) {
	state := getSyncState()

	var progress models.SyncProgress
	db.FirstOrCreate(&progress, models.SyncProgress{ID: 1})

	progress.SyncMode = state.SyncMode
	progress.FolderID = state.CurrentFolder
	progress.FolderIndex = state.FolderIndex
	progress.CurrentPage = state.CurrentPage
	progress.Processed = state.Processed
	progress.TotalAlbums = state.Total
	progress.LastActivityAt = time.Now()

	if !state.IsRunning && !state.IsPaused {
		progress.Status = "completed"
	} else if state.IsPaused {
		progress.Status = "paused"
	} else {
		progress.Status = "running"
	}

	var folderNames []string
	for _, f := range state.Folders {
		if name, ok := f["name"].(string); ok {
			folderNames = append(folderNames, name)
		}
	}
	if len(folderNames) > 0 && state.FolderIndex < len(folderNames) {
		progress.FolderName = folderNames[state.FolderIndex]
	}

	if state.LastBatch != nil && len(state.LastBatch.Albums) > 0 {
		batchJSON, err := json.Marshal(state.LastBatch)
		if err == nil {
			progress.LastBatchJSON = string(batchJSON)
		}
	} else {
		progress.LastBatchJSON = ""
	}

	db.Save(&progress)
	logToFile("saveSyncProgress: saved progress - page=%d, processed=%d, total=%d, has_batch=%v",
		progress.CurrentPage, progress.Processed, progress.TotalAlbums, progress.LastBatchJSON != "")
}

type DiscogsController struct {
	db *gorm.DB
}

func NewDiscogsController(db *gorm.DB) *DiscogsController {
	return &DiscogsController{db: db}
}

func (c *DiscogsController) GetSyncProgress(ctx *gin.Context) {
	// Add a timeout context to prevent hanging
	ctxWithTimeout, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
	defer cancel()

	// Use a separate read-only database connection to avoid blocking
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

	// Check context timeout before making DB call
	select {
	case <-ctxWithTimeout.Done():
		logToFile("GetSyncProgress: TIMEOUT - context cancelled")
		ctx.JSON(503, gin.H{"error": "Service timeout - sync may be busy"})
		return
	default:
	}

	// Query saved progress with a separate DB connection to avoid locks
	var savedProgress models.SyncProgress
	var hasSavedProgress bool
	var savedFolderName string
	var savedProcessed int
	var savedTotalAlbums int
	var savedLastActivity time.Time

	// Use raw SQL with shorter timeout to avoid hanging
	err := c.db.WithContext(ctxWithTimeout).Raw("SELECT id, folder_id, folder_name, current_page, processed, total_albums, last_activity_at, status FROM sync_progresses ORDER BY id DESC LIMIT 1").Scan(&savedProgress).Error
	if err == nil && savedProgress.ID > 0 {
		maxAge := 30 * time.Minute
		if state.IsPaused {
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
		state.IsRunning, state.IsPaused, state.Processed, state.Total,
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
	if state.IsRunning && !state.IsPaused && state.LastActivity.IsZero() == false {
		// Stall timeout must be longer than Discogs API rate limit reset (60s)
		// Using 90s to give buffer for rate limit wait + processing time
		if time.Since(state.LastActivity) > 90*time.Second {
			isStalled = true
		}
	}

	response := ProgressResponse{
		IsRunning:         state.IsRunning,
		IsPaused:          state.IsPaused,
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
	logToFile("GetSyncProgress: IsRunning=%v, Processed=%d, Total=%d, IsStalled=%v", state.IsRunning, state.Processed, state.Total, isStalled)
}

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
			DiscogsID:       intPtr(discogsID),
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

	updateSyncState(func(s *sync.LegacySyncState) {
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
	updateSyncState(func(s *sync.LegacySyncState) {
		s.IsRunning = false
		s.IsPaused = false
		s.Processed = 0 // Reset processed count
		s.LastBatch = nil
		s.LastActivity = time.Time{}
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
	state.Processed = existingProgress.Processed
	state.Total = existingProgress.TotalAlbums
	restoreLastBatch(c.db, &state)
	setSyncState(state)

	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		updateSyncState(func(s *sync.LegacySyncState) {
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
				updateSyncState(func(s *sync.LegacySyncState) {
					s.Folders = folders
				})
				logToFile("ResumeSync: restored %d folders from Discogs", len(folders))
			} else {
				logToFile("ResumeSync: failed to fetch folders: %v", err)
			}
		}
	}

	logToFile("ResumeSync: starting sync from folder %d, page %d, processed=%d, folders_count=%d",
		existingProgress.FolderID, existingProgress.CurrentPage, existingProgress.Processed, len(getSyncState().Folders))

	go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize, existingProgress.SyncMode, existingProgress.FolderID, &state.Folders)

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

	updateSyncState(func(s *sync.LegacySyncState) {
		s.Processed += len(s.LastBatch.Albums)
		s.LastBatch = nil
	})

	ctx.JSON(200, gin.H{
		"message": "Batch skipped",
	})
}

func (c *DiscogsController) CancelSync(ctx *gin.Context) {
	updateSyncState(func(s *sync.LegacySyncState) {
		s.IsRunning = false
		s.IsPaused = false
		s.Processed = 0 // Reset processed count
		s.LastBatch = nil
		s.LastActivity = time.Time{}
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
		logToFile("PauseSync: No sync in progress, cannot pause")
		ctx.JSON(400, gin.H{"error": "No sync in progress"})
		return
	}

	if state.IsPaused {
		logToFile("PauseSync: Sync is already paused")
		ctx.JSON(400, gin.H{"error": "Sync is already paused"})
		return
	}

	logToFile("PauseSync: setting IsPaused=true, current state - IsRunning=%v, Processed=%d, Total=%d, LastBatch=%v",
		state.IsRunning, state.Processed, state.Total, state.LastBatch != nil && len(state.LastBatch.Albums) > 0)

	// Save current progress including LastBatch before setting paused state
	saveSyncProgress(c.db)

	// IMPORTANT: Signal pause FIRST, then update state
	logToFile("PauseSync: calling RequestPause()...")
	pauseSuccess := syncManager.RequestPause()
	logToFile("PauseSync: RequestPause() returned %v", pauseSuccess)
	stateAfterPause := getSyncState()
	logToFile("PauseSync: after RequestPause - IsRunning=%v, IsPaused=%v",
		stateAfterPause.IsRunning, stateAfterPause.IsPaused)

	// Now update the legacy state
	updateSyncState(func(s *sync.LegacySyncState) {
		s.IsPaused = true
	})

	// Save again after setting paused state to persist the paused status
	saveSyncProgress(c.db)

	var config models.AppConfig
	if err := c.db.First(&config).Error; err == nil {
		logToFile("PauseSync: config state - IsConnected=%v, Username=%s, HasTokens=%v",
			config.IsDiscogsConnected, config.DiscogsUsername,
			config.DiscogsAccessToken != "" && config.DiscogsAccessSecret != "")
	}

	newState := getSyncState()
	logToFile("PauseSync: after setting - IsPaused=%v, IsRunning=%v", newState.IsPaused, newState.IsRunning)

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

	if state.IsRunning && state.IsPaused {
		// Load progress once to avoid nested database calls
		progress := loadSyncProgress(c.db)
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

		// IMPORTANT: Signal resume FIRST, then update state
		logToFile("ResumeSyncFromPause: calling RequestResume()...")
		resumeSuccess := syncManager.RequestResume()
		logToFile("ResumeSyncFromPause: RequestResume() returned %v, restarting worker...", resumeSuccess)

		updateSyncState(func(s *sync.LegacySyncState) {
			s.IsPaused = false
			s.Processed = progress.Processed
			s.Total = progress.TotalAlbums
			s.CurrentPage = progress.CurrentPage
			// Keep LastBatch - will restore below
		})

		// Restore LastBatch from database so worker continues with remaining albums
		// instead of re-fetching the same page from API
		state = getSyncState()
		restoreLastBatch(c.db, &state)
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
			updateSyncState(func(s *sync.LegacySyncState) {
				s.IsRunning = false
				s.IsPaused = false
				s.LastActivity = time.Time{}
			})
			ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
			return
		}

		newState := getSyncState()
		logToFile("ResumeSyncFromPause: restarting sync worker at page %d, processed=%d", newState.CurrentPage, newState.Processed)
		go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize,
			newState.SyncMode, newState.CurrentFolder, &newState.Folders)

		ctx.JSON(200, gin.H{
			"message":    "Sync resumed from pause",
			"sync_state": newState,
		})
		return
	}

	if !state.IsRunning {
		existingProgress := loadSyncProgress(c.db)
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

		state.IsRunning = true
		state.SyncMode = existingProgress.SyncMode
		state.CurrentFolder = existingProgress.FolderID
		state.FolderIndex = existingProgress.FolderIndex
		state.CurrentPage = existingProgress.CurrentPage
		state.Processed = existingProgress.Processed
		state.Total = existingProgress.TotalAlbums
		state.IsPaused = false
		setSyncState(state)

		// Restore LastBatch from database so worker continues with remaining albums
		// instead of re-fetching the same page from API
		state = getSyncState()
		restoreLastBatch(c.db, &state)
		setSyncState(state)

		batchCount := 0
		if state.LastBatch != nil {
			batchCount = len(state.LastBatch.Albums)
		}
		logToFile("ResumeSyncFromPause: resuming from page %d with %d albums processed, restored LastBatch with %d albums",
			state.CurrentPage, state.Processed, batchCount)

		client := c.getDiscogsClientWithOAuth()
		if client == nil {
			updateSyncState(func(s *sync.LegacySyncState) {
				s.IsRunning = false
				s.IsPaused = false
				s.Processed = 0
				s.LastActivity = time.Time{}
			})
			ctx.JSON(500, gin.H{"error": "Failed to get Discogs client"})
			return
		}

		if existingProgress.SyncMode == "all-folders" {
			folders, err := client.GetUserFolders(config.DiscogsUsername)
			if err == nil {
				updateSyncState(func(s *sync.LegacySyncState) {
					s.Folders = folders
				})
				logToFile("ResumeSyncFromPause: restored %d folders from Discogs", len(folders))
			} else {
				logToFile("ResumeSyncFromPause: failed to fetch folders: %v", err)
			}
		}

		logToFile("ResumeSyncFromPause: resuming sync from folder %d, page %d, processed=%d, folders_count=%d",
			existingProgress.FolderID, existingProgress.CurrentPage, existingProgress.Processed, len(getSyncState().Folders))

		go processSyncBatches(c.db, client, config.DiscogsUsername, config.SyncBatchSize,
			existingProgress.SyncMode, existingProgress.FolderID, &state.Folders)

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

// RefreshTracks re-syncs tracks for all albums that have a DiscogsID.
// This fetches the latest tracklist from Discogs and replaces local tracks.
// Use this to sync track additions/removals that happened on Discogs.
func (c *DiscogsController) RefreshTracks(ctx *gin.Context) {
	client := c.getDiscogsClientWithOAuth()
	if client == nil {
		ctx.JSON(500, gin.H{"error": "Failed to get Discogs client - please connect to Discogs first"})
		return
	}

	// Get all albums with a DiscogsID
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

	updated := 0
	failed := 0

	for _, album := range albums {
		if album.DiscogsID == nil {
			continue
		}

		discogsID := *album.DiscogsID
		logToFile("RefreshTracks: Refreshing tracks for album %d: %s - %s (DiscogsID: %d)",
			album.ID, album.Artist, album.Title, discogsID)

		success, errMsg := fetchTracksForAlbum(c.db, client, album.ID, discogsID, album.Title, album.Artist)
		if success {
			updated++
			logToFile("RefreshTracks: Successfully refreshed tracks for %s - %s", album.Artist, album.Title)
		} else {
			failed++
			logToFile("RefreshTracks: Failed to refresh tracks for %s - %s: %s", album.Artist, album.Title, errMsg)
		}

		// Small delay to respect rate limits
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

// FindUnlinkedAlbums finds albums that have a DiscogsID but are no longer in the user's Discogs collection.
// This helps identify albums that were removed from Discogs and may need cleanup.
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

	// Get all local albums with a DiscogsID
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

	// Build a set of local DiscogsIDs for quick lookup
	localDiscogsIDs := make(map[int]models.Album)
	for _, album := range localAlbums {
		if album.DiscogsID != nil && *album.DiscogsID > 0 {
			localDiscogsIDs[*album.DiscogsID] = album
		}
	}

	// Fetch all releases from Discogs collection (all pages)
	discogsIDs := make(map[int]bool)
	page := 1
	batchSize := 100 // Max per page for efficiency

	for {
		releases, _, err := client.GetUserCollectionByFolder(config.DiscogsUsername, 0, page, batchSize)
		if err != nil {
			// Check if we've reached the end of pagination
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

		// Small delay to respect rate limits
		time.Sleep(200 * time.Millisecond)
	}

	// Find albums that are local but not in Discogs
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

// DeleteUnlinkedAlbums deletes specified albums and their tracks from the local database.
// This is used to clean up albums that are no longer in the user's Discogs collection.
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
		// Start transaction
		tx := c.db.Begin()

		// Delete tracks first (foreign key constraint)
		if err := tx.Where("album_id = ?", albumID).Delete(&models.Track{}).Error; err != nil {
			logToFile("DeleteUnlinkedAlbums: Failed to delete tracks for album %d: %v", albumID, err)
			tx.Rollback()
			failed++
			continue
		}

		// Delete the album
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
