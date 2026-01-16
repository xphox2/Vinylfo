package controllers

import (
	"strings"

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
	type TrackResult struct {
		ID           uint   `json:"id"`
		AlbumID      uint   `json:"album_id"`
		Title        string `json:"title"`
		Duration     int    `json:"duration"`
		TrackNumber  int    `json:"track_number"`
		DiscNumber   int    `json:"disc_number"`
		Side         string `json:"side"`
		Position     string `json:"position"`
		AudioFileURL string `json:"audio_file_url"`
		ReleaseYear  int    `json:"release_year"`
		AlbumGenre   string `json:"album_genre"`
		AlbumTitle   string `json:"album_title"`
		AlbumArtist  string `json:"album_artist"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	var tracks []TrackResult
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Find(&tracks)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
		return
	}
	ctx.JSON(200, tracks)
}

func (c *TrackController) SearchTracks(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	type TrackResult struct {
		ID           uint   `json:"id"`
		AlbumID      uint   `json:"album_id"`
		Title        string `json:"title"`
		Duration     int    `json:"duration"`
		TrackNumber  int    `json:"track_number"`
		DiscNumber   int    `json:"disc_number"`
		Side         string `json:"side"`
		Position     string `json:"position"`
		AudioFileURL string `json:"audio_file_url"`
		ReleaseYear  int    `json:"release_year"`
		AlbumGenre   string `json:"album_genre"`
		AlbumTitle   string `json:"album_title"`
		AlbumArtist  string `json:"album_artist"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	var tracks []TrackResult
	searchTerm := "%" + strings.ToLower(query) + "%"
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("LOWER(tracks.title) LIKE ? OR LOWER(albums.title) LIKE ? OR LOWER(albums.artist) LIKE ?", searchTerm, searchTerm, searchTerm).
		Find(&tracks)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search tracks"})
		return
	}
	ctx.JSON(200, tracks)
}

func (c *TrackController) GetTrackByID(ctx *gin.Context) {
	id := ctx.Param("id")

	var trackData struct {
		ID           uint   `json:"id"`
		AlbumID      uint   `json:"album_id"`
		Title        string `json:"title"`
		Duration     int    `json:"duration"`
		TrackNumber  int    `json:"track_number"`
		AudioFileURL string `json:"audio_file_url"`
		AlbumTitle   string `json:"album_title"`
		AlbumArtist  string `json:"album_artist"`
		ReleaseYear  int    `json:"release_year"`
		AlbumGenre   string `json:"album_genre"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist, albums.release_year as release_year, albums.genre as album_genre").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.id = ?", id).
		First(&trackData)

	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Track not found"})
		return
	}

	ctx.JSON(200, trackData)
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
