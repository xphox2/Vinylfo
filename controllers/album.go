package controllers

import (
	"strings"

	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AlbumController struct {
	db *gorm.DB
}

func NewAlbumController(db *gorm.DB) *AlbumController {
	return &AlbumController{db: db}
}

func (c *AlbumController) GetAlbums(ctx *gin.Context) {
	var albums []models.Album
	result := c.db.Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch albums"})
		return
	}
	ctx.JSON(200, albums)
}

func (c *AlbumController) SearchAlbums(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	var albums []models.Album
	searchTerm := "%" + strings.ToLower(query) + "%"
	result := c.db.Where("LOWER(title) LIKE ? OR LOWER(artist) LIKE ?", searchTerm, searchTerm).Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search albums"})
		return
	}
	ctx.JSON(200, albums)
}

func (c *AlbumController) GetAlbumByID(ctx *gin.Context) {
	id := ctx.Param("id")
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}
	ctx.JSON(200, album)
}

func (c *AlbumController) GetTracksByAlbumID(ctx *gin.Context) {
	id := ctx.Param("id")

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
		AlbumTitle   string `json:"album_title"`
		AlbumArtist  string `json:"album_artist"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	var tracks []TrackResult
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.album_id = ?", id).
		Find(&tracks)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
		return
	}
	ctx.JSON(200, tracks)
}

func (c *AlbumController) CreateAlbum(ctx *gin.Context) {
	var album models.Album
	if err := ctx.ShouldBindJSON(&album); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&album)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create album"})
		return
	}
	ctx.JSON(201, album)
}

func (c *AlbumController) UpdateAlbum(ctx *gin.Context) {
	id := ctx.Param("id")
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&album); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&album)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update album"})
		return
	}
	ctx.JSON(200, album)
}

func (c *AlbumController) DeleteAlbum(ctx *gin.Context) {
	id := ctx.Param("id")
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	result = c.db.Delete(&album)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete album"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Album deleted successfully"})
}
