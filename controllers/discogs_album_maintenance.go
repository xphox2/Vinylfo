package controllers

import (
	"fmt"
	"log"
	"strings"
	"time"

	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

	log.Printf("RefreshTracks: Starting track refresh for %d albums", len(albums))

	importer := services.NewAlbumImporter(c.db, client)
	updated := 0
	failed := 0

	for _, album := range albums {
		if album.DiscogsID == nil {
			continue
		}

		discogsID := *album.DiscogsID
		log.Printf("RefreshTracks: Refreshing tracks for album %d: %s - %s (DiscogsID: %d)",
			album.ID, album.Artist, album.Title, discogsID)

		success, errMsg := importer.FetchAndSaveTracks(c.db, album.ID, discogsID, album.Title, album.Artist)
		if success {
			updated++
			log.Printf("RefreshTracks: Successfully refreshed tracks for %s - %s", album.Artist, album.Title)
		} else {
			failed++
			log.Printf("RefreshTracks: Failed to refresh tracks for %s - %s: %s", album.Artist, album.Title, errMsg)
		}

		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("RefreshTracks: Completed - updated=%d, failed=%d, total=%d", updated, failed, len(albums))

	ctx.JSON(200, gin.H{
		"message": fmt.Sprintf("Track refresh completed: %d updated, %d failed", updated, failed),
		"updated": updated,
		"failed":  failed,
		"total":   len(albums),
	})
}

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

	log.Printf("FindUnlinkedAlbums: Checking %d local albums against Discogs collection", len(localAlbums))

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
			log.Printf("FindUnlinkedAlbums: Error fetching page %d: %v", page, err)
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

		log.Printf("FindUnlinkedAlbums: Fetched page %d, got %d releases, total so far: %d", page, len(releases), len(discogsIDs))
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

	log.Printf("FindUnlinkedAlbums: Found %d unlinked albums out of %d checked", len(unlinkedAlbums), len(localDiscogsIDs))

	ctx.JSON(200, gin.H{
		"message":         fmt.Sprintf("Found %d albums not in Discogs collection", len(unlinkedAlbums)),
		"unlinked_albums": unlinkedAlbums,
		"total_checked":   len(localDiscogsIDs),
		"discogs_total":   len(discogsIDs),
	})
}

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

	var deletedTracks int64
	var deletedAlbums int64
	var err error

	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to start transaction"})
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete tracks for the albums
	deletedTracks, err = deleteTracksForAlbums(tx, input.AlbumIDs)
	if err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to delete tracks: %v", err)})
		return
	}

	// Delete the albums
	deletedAlbums, err = deleteAlbumsByID(tx, input.AlbumIDs)
	if err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to delete albums: %v", err)})
		return
	}

	if err = tx.Commit().Error; err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to commit transaction: %v", err)})
		return
	}

	log.Printf("DeleteUnlinkedAlbums: Deleted %d albums and %d tracks", deletedAlbums, deletedTracks)
	ctx.JSON(200, gin.H{
		"message":       fmt.Sprintf("Deleted %d albums and %d tracks", deletedAlbums, deletedTracks),
		"deletedAlbums": deletedAlbums,
		"deletedTracks": deletedTracks,
	})
}

func deleteTracksForAlbums(tx *gorm.DB, albumIDs []uint) (int64, error) {
	result := tx.Where("album_id IN ?", albumIDs).Delete(&models.Track{})
	return result.RowsAffected, result.Error
}

func deleteAlbumsByID(tx *gorm.DB, albumIDs []uint) (int64, error) {
	result := tx.Where("id IN ?", albumIDs).Delete(&models.Album{})
	return result.RowsAffected, result.Error
}

// CleanupOrphanedTracks removes tracks with invalid data (album_id=0 or empty title)
func (c *DiscogsController) CleanupOrphanedTracks(ctx *gin.Context) {
	importer := services.NewAlbumImporter(c.db, nil)
	deletedCount, err := importer.CleanupOrphanedTracks()
	if err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to cleanup orphaned tracks: %v", err)})
		return
	}
	ctx.JSON(200, gin.H{
		"message":      fmt.Sprintf("Deleted %d orphaned tracks", deletedCount),
		"deletedCount": deletedCount,
	})
}
