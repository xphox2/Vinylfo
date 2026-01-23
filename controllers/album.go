package controllers

import (
	"log"
	"strconv"
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
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "25"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var albums []models.Album
	var total int64

	c.db.Model(&models.Album{}).Count(&total)

	result := c.db.Offset(offset).Limit(limit).Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch albums"})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"data":       albums,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *AlbumController) SearchAlbums(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "25"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var albums []models.Album
	var total int64
	searchTerm := "%" + strings.ToLower(query) + "%"

	c.db.Model(&models.Album{}).Where("LOWER(title) LIKE ? OR LOWER(artist) LIKE ?", searchTerm, searchTerm).Count(&total)

	result := c.db.Where("LOWER(title) LIKE ? OR LOWER(artist) LIKE ?", searchTerm, searchTerm).Offset(offset).Limit(limit).Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search albums"})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"data":       albums,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *AlbumController) GetAlbumByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("GetAlbumByID DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}
	ctx.JSON(200, album)
}

func (c *AlbumController) GetAlbumImage(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("GetAlbumImage DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	if len(album.DiscogsCoverImage) == 0 {
		ctx.JSON(404, gin.H{"error": "No image found for this album"})
		return
	}

	contentType := album.DiscogsCoverImageType
	if contentType == "" {
		contentType = "image/jpeg"
	}

	ctx.Data(200, contentType, album.DiscogsCoverImage)
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
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("UpdateAlbum DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&album); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&album)
	if result.Error != nil {
		log.Printf("UpdateAlbum save error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to update album"})
		return
	}
	ctx.JSON(200, album)
}

func (c *AlbumController) DeleteAlbum(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("DeleteAlbum DB error: %v", result.Error)
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
