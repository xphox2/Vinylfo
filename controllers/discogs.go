package controllers

import (
	"log"
	"os"
	"strconv"
	"strings"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/services"
	"vinylfo/sync"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getDiscogsClient() *discogs.Client {
	client := discogs.NewClient("")
	return client
}

type DiscogsController struct {
	db              *gorm.DB
	progressService *services.SyncProgressService
}

func NewDiscogsController(db *gorm.DB) *DiscogsController {
	return &DiscogsController{
		db:              db,
		progressService: services.NewSyncProgressService(db),
	}
}

func (c *DiscogsController) getDiscogsClientWithOAuth() *discogs.Client {
	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		log.Printf("OAUTH: ERROR loading config from database: %v", err)
		return nil
	}

	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")

	oauth := &discogs.OAuthConfig{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    config.DiscogsAccessToken,
		AccessSecret:   config.DiscogsAccessSecret,
	}
	return discogs.NewClientWithOAuth("", oauth)
}

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

func (c *DiscogsController) GetStatus(ctx *gin.Context) {
	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		log.Printf("GetStatus: ERROR loading config: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to load config"})
		return
	}

	ctx.JSON(200, gin.H{
		"is_connected":   config.IsDiscogsConnected,
		"username":       config.DiscogsUsername,
		"batch_size":     config.SyncBatchSize,
		"last_sync_at":   config.LastSyncAt,
		"sync_mode":      config.SyncMode,
		"sync_folder_id": config.SyncFolderID,
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
			log.Printf("CreateAlbum: Failed to get OAuth client, skipping Discogs fetch")
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
				album.DiscogsID = utils.IntPtr(input.DiscogsID)

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
				log.Printf("CreateAlbum: Failed to fetch from Discogs: %v", err)
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
			log.Printf("CreateAlbum: failed to download image: %v", imageErr)
			album.CoverImageFailed = true
		} else {
			album.DiscogsCoverImage = imageData
			album.DiscogsCoverImageType = imageType
		}
	} else if album.CoverImageURL != "" {
		imageData, imageType, imageErr := downloadImage(album.CoverImageURL)
		if imageErr != nil {
			log.Printf("CreateAlbum: failed to download image from Discogs: %v", imageErr)
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
