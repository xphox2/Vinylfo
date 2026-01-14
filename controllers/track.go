package controllers

import (
	"log"

	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TrackController struct {
	db *gorm.DB
}

func NewTrackController(db *gorm.DB) *TrackController {
	return &TrackController{db: db}
}

func (c *TrackController) GetTracks(ctx *gin.Context) {
	var tracks []models.Track
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title").
		Joins("left join albums on tracks.album_id = albums.id").
		Find(&tracks)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
		return
	}
	ctx.JSON(200, tracks)
}

func (c *TrackController) GetTrackByID(ctx *gin.Context) {
	id := ctx.Param("id")

	var track models.Track
	result := c.db.First(&track, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	log.Printf("Track found: ID=%d, AlbumID=%d, Title=%s", track.ID, track.AlbumID, track.Title)

	var album models.Album
	albumResult := c.db.First(&album, track.AlbumID)
	if albumResult.Error != nil {
		log.Printf("Album lookup failed for AlbumID=%d: %v", track.AlbumID, albumResult.Error)
		album = models.Album{}
	} else {
		log.Printf("Album found: ID=%d, Title=%s, ReleaseYear=%d, Genre=%s", album.ID, album.Title, album.ReleaseYear, album.Genre)
	}

	response := map[string]interface{}{
		"id":             track.ID,
		"album_id":       track.AlbumID,
		"album_title":    album.Title,
		"title":          track.Title,
		"duration":       track.Duration,
		"track_number":   track.TrackNumber,
		"audio_file_url": track.AudioFileURL,
		"release_year":   album.ReleaseYear,
		"album_genre":    album.Genre,
		"created_at":     track.CreatedAt,
		"updated_at":     track.UpdatedAt,
	}

	ctx.JSON(200, response)
}

func (c *TrackController) CreateTrack(ctx *gin.Context) {
	var track models.Track
	if err := ctx.ShouldBindJSON(&track); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&track)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create track"})
		return
	}
	ctx.JSON(201, track)
}

func (c *TrackController) UpdateTrack(ctx *gin.Context) {
	id := ctx.Param("id")
	var track models.Track
	result := c.db.First(&track, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&track); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&track)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update track"})
		return
	}
	ctx.JSON(200, track)
}

func (c *TrackController) DeleteTrack(ctx *gin.Context) {
	id := ctx.Param("id")
	var track models.Track
	result := c.db.First(&track, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	result = c.db.Delete(&track)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete track"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Track deleted successfully"})
}
