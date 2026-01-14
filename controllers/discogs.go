package controllers

import (
	"os"
	"strconv"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DiscogsController struct {
	db *gorm.DB
}

func NewDiscogsController(db *gorm.DB) *DiscogsController {
	return &DiscogsController{db: db}
}

func (c *DiscogsController) GetOAuthURL(ctx *gin.Context) {
	client := discogs.NewClient("")
	token, _, authURL, err := client.GetRequestToken()
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to get request token"})
		return
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token": token,
	})

	ctx.JSON(200, gin.H{
		"auth_url": authURL,
		"token":    token,
	})
}

func (c *DiscogsController) OAuthCallback(ctx *gin.Context) {
	oauthToken := ctx.Query("oauth_token")
	oauthVerifier := ctx.Query("oauth_verifier")

	if oauthToken == "" || oauthVerifier == "" {
		ctx.String(400, "Missing oauth_token or oauth_verifier")
		return
	}

	var config models.AppConfig
	c.db.First(&config)

	client := discogs.NewClient("")
	client.OAuth = &discogs.OAuthConfig{
		ConsumerKey:    os.Getenv("DISCOGS_CONSUMER_KEY"),
		ConsumerSecret: os.Getenv("DISCOGS_CONSUMER_SECRET"),
		AccessToken:    oauthToken,
		AccessSecret:   "",
	}

	accessToken, accessSecret, username, err := client.GetAccessToken(oauthToken, "", oauthVerifier)
	if err != nil {
		ctx.String(500, "Failed to get access token: %v", err)
		return
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":  accessToken,
		"discogs_access_secret": accessSecret,
		"discogs_username":      username,
		"is_discogs_connected":  true,
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

	ctx.JSON(200, gin.H{"message": "Disconnected from Discogs"})
}

func (c *DiscogsController) GetStatus(ctx *gin.Context) {
	var config models.AppConfig
	c.db.First(&config)

	ctx.JSON(200, gin.H{
		"is_connected":    config.IsDiscogsConnected,
		"username":        config.DiscogsUsername,
		"sync_confirm":    config.SyncConfirmBatches,
		"batch_size":      config.SyncBatchSize,
		"auto_apply_safe": config.AutoApplySafeUpdates,
		"auto_sync_new":   config.AutoSyncNewAlbums,
		"last_sync_at":    config.LastSyncAt,
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
	c.db.First(&config)

	client := discogs.NewClient(config.DiscogsAccessToken)
	albums, err := client.SearchAlbums(query, page)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"results": albums,
		"page":    page,
	})
}

func (c *DiscogsController) CreateAlbum(ctx *gin.Context) {
	var input struct {
		DiscogsID   int    `json:"discogs_id"`
		Title       string `json:"title"`
		Artist      string `json:"artist"`
		ReleaseYear int    `json:"release_year"`
		Genre       string `json:"genre"`
		CoverImage  string `json:"cover_image"`
		FromDiscogs bool   `json:"from_discogs"`
		Tracks      []struct {
			Title       string `json:"title"`
			Duration    int    `json:"duration"`
			TrackNumber int    `json:"track_number"`
		} `json:"tracks"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var album models.Album

	if input.FromDiscogs && input.DiscogsID > 0 {
		var config models.AppConfig
		c.db.First(&config)
		client := discogs.NewClient(config.DiscogsAccessToken)
		discogsData, err := client.GetAlbum(input.DiscogsID)
		if err == nil {
			if v, ok := discogsData["title"].(string); ok {
				album.Title = v
			}
			if v, ok := discogsData["artist"].(string); ok {
				album.Artist = v
			}
			if v, ok := discogsData["year"].(int); ok {
				album.ReleaseYear = v
			}
			if v, ok := discogsData["genre"].(string); ok {
				album.Genre = v
			}
			if v, ok := discogsData["cover_image"].(string); ok {
				album.CoverImageURL = v
			}

			if tracks, ok := discogsData["tracklist"].([]map[string]interface{}); ok {
				for _, t := range tracks {
					track := struct {
						Title       string `json:"title"`
						Duration    int    `json:"duration"`
						TrackNumber int    `json:"track_number"`
					}{}

					if v, ok := t["title"].(string); ok {
						track.Title = v
					}
					if v, ok := t["track_number"].(float64); ok {
						track.TrackNumber = int(v)
					}
					if v, ok := t["duration"].(int); ok {
						track.Duration = v
					}

					input.Tracks = append(input.Tracks, track)
				}
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
	if input.CoverImage != "" {
		album.CoverImageURL = input.CoverImage
	}

	result := c.db.Create(&album)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create album"})
		return
	}

	for _, track := range input.Tracks {
		trackModel := models.Track{
			AlbumID:     album.ID,
			AlbumTitle:  album.Title,
			Title:       track.Title,
			TrackNumber: track.TrackNumber,
			Duration:    track.Duration,
		}
		c.db.Create(&trackModel)
	}

	c.db.Preload("Tracks").First(&album, album.ID)
	ctx.JSON(201, album)
}

type SyncState struct {
	IsRunning   bool                 `json:"is_running"`
	CurrentPage int                  `json:"current_page"`
	TotalPages  int                  `json:"total_pages"`
	Processed   int                  `json:"processed"`
	Total       int                  `json:"total"`
	LastBatch   *SyncBatch           `json:"last_batch,omitempty"`
	LastReview  *discogs.BatchReview `json:"last_review,omitempty"`
}

type SyncBatch struct {
	ID          int                      `json:"id"`
	Albums      []map[string]interface{} `json:"albums"`
	ProcessedAt *time.Time               `json:"processed_at,omitempty"`
}

var syncState = SyncState{IsRunning: false}

func (c *DiscogsController) StartSync(ctx *gin.Context) {
	if syncState.IsRunning {
		ctx.JSON(400, gin.H{"error": "Sync already in progress"})
		return
	}

	var config models.AppConfig
	c.db.First(&config)

	if !config.IsDiscogsConnected {
		ctx.JSON(400, gin.H{"error": "Discogs not connected"})
		return
	}

	syncState = SyncState{
		IsRunning:   true,
		CurrentPage: 1,
	}

	client := discogs.NewClient("")
	releases, err := client.GetUserCollection(1, config.SyncBatchSize)
	if err != nil {
		syncState.IsRunning = false
		ctx.JSON(500, gin.H{"error": "Failed to start sync"})
		return
	}

	syncState.Total = 100
	syncState.TotalPages = 1

	batch := SyncBatch{
		ID:     1,
		Albums: releases,
	}
	syncState.LastBatch = &batch

	var localAlbums []models.Album
	c.db.Find(&localAlbums)

	reviewService := discogs.NewDataReviewService(config.AutoApplySafeUpdates)
	batchReview := reviewService.ReviewBatch(localAlbums, releases)
	syncState.LastReview = batchReview

	go processSyncBatches(c.db, client, config.SyncBatchSize)

	ctx.JSON(200, gin.H{
		"message":      "Sync started",
		"current_page": syncState.CurrentPage,
		"total_pages":  syncState.TotalPages,
		"batch_review": batchReview,
	})
}

func processSyncBatches(db *gorm.DB, client *discogs.Client, batchSize int) {
	// This would be a long-running process in a real implementation
	// For now, we'll just mark it as complete after processing one batch
}

func (c *DiscogsController) GetSyncProgress(ctx *gin.Context) {
	response := gin.H{
		"is_running":   syncState.IsRunning,
		"current_page": syncState.CurrentPage,
		"total_pages":  syncState.TotalPages,
		"processed":    syncState.Processed,
		"total":        syncState.Total,
	}

	if syncState.LastReview != nil {
		response["review"] = syncState.LastReview
	}

	ctx.JSON(200, response)
}

func (c *DiscogsController) GetBatchDetails(ctx *gin.Context) {
	batchID := ctx.Param("id")
	batchIDInt, _ := strconv.Atoi(batchID)

	if syncState.LastBatch == nil || syncState.LastBatch.ID != batchIDInt {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	if syncState.LastReview != nil {
		ctx.JSON(200, gin.H{
			"id":           syncState.LastBatch.ID,
			"albums":       syncState.LastBatch.Albums,
			"review":       syncState.LastReview,
			"auto_apply":   syncState.LastReview.NewAlbums + (syncState.LastReview.UpdatedAlbums - syncState.LastReview.ConflictCount),
			"needs_review": syncState.LastReview.ConflictCount,
		})
		return
	}

	ctx.JSON(200, gin.H{
		"id":     syncState.LastBatch.ID,
		"albums": syncState.LastBatch.Albums,
	})
}

func (c *DiscogsController) ConfirmBatch(ctx *gin.Context) {
	batchID := ctx.Param("id")
	batchIDInt, _ := strconv.Atoi(batchID)

	if syncState.LastBatch == nil || syncState.LastBatch.ID != batchIDInt {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	for _, album := range syncState.LastBatch.Albums {
		title, _ := album["title"].(string)
		artist, _ := album["artist"].(string)
		year, _ := album["year"].(int)
		coverImage, _ := album["cover_image"].(string)

		var existingAlbum models.Album
		result := c.db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)
		if result.Error == gorm.ErrRecordNotFound {
			newAlbum := models.Album{
				Title:         title,
				Artist:        artist,
				ReleaseYear:   year,
				CoverImageURL: coverImage,
			}
			c.db.Create(&newAlbum)
		}
	}

	syncState.CurrentPage++
	syncState.Processed += len(syncState.LastBatch.Albums)

	if syncState.CurrentPage > syncState.TotalPages {
		syncState.IsRunning = false
		c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
	}

	syncState.LastBatch = nil

	ctx.JSON(200, gin.H{
		"message":      "Batch confirmed and synced",
		"current_page": syncState.CurrentPage,
		"processed":    syncState.Processed,
	})
}

func (c *DiscogsController) SkipBatch(ctx *gin.Context) {
	batchID := ctx.Param("id")
	batchIDInt, _ := strconv.Atoi(batchID)

	if syncState.LastBatch == nil || syncState.LastBatch.ID != batchIDInt {
		ctx.JSON(404, gin.H{"error": "Batch not found"})
		return
	}

	syncState.CurrentPage++
	syncState.LastBatch = nil

	ctx.JSON(200, gin.H{
		"message":      "Batch skipped",
		"current_page": syncState.CurrentPage,
	})
}

func (c *DiscogsController) CancelSync(ctx *gin.Context) {
	syncState = SyncState{IsRunning: false}

	ctx.JSON(200, gin.H{"message": "Sync cancelled"})
}
