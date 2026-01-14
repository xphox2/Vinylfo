package controllers

import (
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
	var tracks []models.Track
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title").
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
