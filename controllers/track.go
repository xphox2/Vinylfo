package controllers

import (
	"strconv"
	"strings"

	"vinylfo/models"
	"vinylfo/utils"

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

	pageStr := ctx.DefaultQuery("page", "1")
	limitStr := ctx.DefaultQuery("limit", "25")
	excludeIDs := ctx.Query("exclude_track_ids")

	if !utils.ValidateRequest(ctx,
		utils.ValidatePageParams(pageStr, limitStr),
	) {
		return
	}

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

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

	var tracks []TrackResult
	var total int64

	baseQuery := c.db.Model(&models.Track{}).
		Table("tracks").
		Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id")

	if excludeIDs != "" {
		var ids []uint
		idStrings := strings.Split(excludeIDs, ",")
		for _, idStr := range idStrings {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				ids = append(ids, uint(id))
			}
		}
		if len(ids) > 0 {
			baseQuery = baseQuery.Where("tracks.id NOT IN ?", ids)
		}
	}

	baseQuery.Count(&total)

	result := baseQuery.Offset(offset).Limit(limit).Find(&tracks)

	if result.Error != nil {
		utils.InternalError(ctx, "Failed to fetch tracks")
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	utils.Success(ctx, 200, gin.H{
		"data":       tracks,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
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

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "25"))
	excludeIDs := ctx.Query("exclude_track_ids")

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

	var tracks []TrackResult
	var total int64
	searchTerm := "%" + strings.ToLower(query) + "%"

	baseQuery := c.db.Model(&models.Track{}).
		Table("tracks").
		Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("LOWER(tracks.title) LIKE ? OR LOWER(albums.title) LIKE ? OR LOWER(albums.artist) LIKE ?", searchTerm, searchTerm, searchTerm)

	if excludeIDs != "" {
		var ids []uint
		idStrings := strings.Split(excludeIDs, ",")
		for _, idStr := range idStrings {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				ids = append(ids, uint(id))
			}
		}
		if len(ids) > 0 {
			baseQuery = baseQuery.Where("tracks.id NOT IN ?", ids)
		}
	}

	baseQuery.Count(&total)

	result := baseQuery.Offset(offset).Limit(limit).Find(&tracks)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search tracks"})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"data":       tracks,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *TrackController) GetTrackByID(ctx *gin.Context) {
	id := ctx.Param("id")
	trackID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.BadRequest(ctx, "Invalid track ID")
		return
	}

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
		Where("tracks.id = ?", trackID).
		First(&trackData)

	if result.Error != nil {
		utils.NotFound(ctx, "Track not found")
		return
	}

	utils.Success(ctx, 200, trackData)
}

func (c *TrackController) CreateTrack(ctx *gin.Context) {
	var track models.Track
	if err := ctx.ShouldBindJSON(&track); err != nil {
		utils.BadRequest(ctx, err.Error())
		return
	}

	if !utils.ValidateRequest(ctx,
		utils.ValidateRequired(track.AlbumID, "album_id"),
		utils.ValidateRequired(track.Title, "title"),
		utils.ValidateStringNotEmpty(track.Title, "title"),
	) {
		return
	}

	result := c.db.Create(&track)
	if result.Error != nil {
		utils.InternalError(ctx, "Failed to create track")
		return
	}
	utils.Created(ctx, track)
}

func (c *TrackController) UpdateTrack(ctx *gin.Context) {
	id := ctx.Param("id")
	trackID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.BadRequest(ctx, "Invalid track ID")
		return
	}

	var track models.Track
	result := c.db.First(&track, trackID)
	if result.Error != nil {
		utils.NotFound(ctx, "Track not found")
		return
	}

	if err := ctx.ShouldBindJSON(&track); err != nil {
		utils.BadRequest(ctx, err.Error())
		return
	}

	if !utils.ValidateRequest(ctx,
		utils.ValidateRequired(track.Title, "title"),
		utils.ValidateStringNotEmpty(track.Title, "title"),
	) {
		return
	}

	result = c.db.Save(&track)
	if result.Error != nil {
		utils.InternalError(ctx, "Failed to update track")
		return
	}
	utils.Success(ctx, 200, track)
}

func (c *TrackController) DeleteTrack(ctx *gin.Context) {
	id := ctx.Param("id")
	trackID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.BadRequest(ctx, "Invalid track ID")
		return
	}

	var track models.Track
	result := c.db.First(&track, trackID)
	if result.Error != nil {
		utils.NotFound(ctx, "Track not found")
		return
	}

	result = c.db.Delete(&track)
	if result.Error != nil {
		utils.InternalError(ctx, "Failed to delete track")
		return
	}
	utils.Success(ctx, 200, gin.H{"message": "Track deleted successfully"})
}
