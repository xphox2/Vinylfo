package controllers

import (
	"log"
	"strconv"

	"vinylfo/models"
	"vinylfo/services"
	"vinylfo/sync"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
)

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
			DiscogsID:       utils.IntPtr(discogsID),
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
			log.Printf("ApplyBatch: failed to commit transaction for album %s - %s: %v", artist, title, err)
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

	updateSyncState(func(s *sync.SyncState) {
		s.Processed += len(s.LastBatch.Albums)
		s.LastBatch = nil
	})

	ctx.JSON(200, gin.H{
		"message": "Batch skipped",
	})
}
