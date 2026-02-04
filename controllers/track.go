package controllers

import (
	"fmt"
	"log"
	"regexp"
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
		ID             uint   `json:"id"`
		AlbumID        uint   `json:"album_id"`
		Title          string `json:"title"`
		Duration       int    `json:"duration"`
		TrackNumber    int    `json:"track_number"`
		DiscNumber     int    `json:"disc_number"`
		Side           string `json:"side"`
		Position       string `json:"position"`
		AudioFileURL   string `json:"audio_file_url"`
		ReleaseYear    int    `json:"release_year"`
		AlbumGenre     string `json:"album_genre"`
		AlbumTitle     string `json:"album_title"`
		AlbumArtist    string `json:"album_artist"`
		YouTubeVideoID string `json:"youtube_video_id"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	pageStr := ctx.DefaultQuery("page", "1")
	limitStr := ctx.DefaultQuery("limit", "25")
	excludeIDs := ctx.Query("exclude_track_ids")
	sortBy := ctx.DefaultQuery("sort", "title")
	order := ctx.DefaultQuery("order", "asc")

	allowedSorts := map[string]bool{
		"title":        true,
		"album_title":  true,
		"album_artist": true,
		"track_number": true,
		"duration":     true,
		"created_at":   true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "title"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}

	sortColumn := sortBy
	switch sortBy {
	case "album_title":
		sortColumn = "albums.title"
	case "album_artist":
		sortColumn = "albums.artist"
	case "track_number":
		sortColumn = "tracks.track_number"
	case "duration":
		sortColumn = "tracks.duration"
	case "created_at":
		sortColumn = "tracks.created_at"
	default:
		sortColumn = "tracks.title"
	}

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
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.title IS NOT NULL AND tracks.title != ''")

	if excludeIDs != "" {
		idStrings := strings.Split(excludeIDs, ",")
		for _, idStr := range idStrings {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				baseQuery = baseQuery.Where("tracks.id != ?", id)
			}
		}
	}

	baseQuery.Count(&total)
	result := baseQuery.Order(fmt.Sprintf("LOWER(%s) %s", sortColumn, order)).Offset(offset).Limit(limit).Find(&tracks)

	if result.Error != nil {
		utils.InternalError(ctx, "Failed to fetch tracks")
		return
	}

	if len(tracks) > 0 {
		trackIDs := make([]uint, len(tracks))
		for i, t := range tracks {
			trackIDs[i] = t.ID
		}

		var rawRows []map[string]interface{}
		c.db.Raw(`SELECT * FROM track_youtube_matches WHERE track_id IN ?`, trackIDs).Scan(&rawRows)

		ytMap := make(map[uint]string, len(rawRows))
		for _, row := range rawRows {
			trackID := uint(row["track_id"].(int64))
			if v, ok := row["youtube_video_id"].(string); ok && v != "" {
				ytMap[trackID] = v
			}
		}

		for i := range tracks {
			if vid, ok := ytMap[tracks[i].ID]; ok {
				tracks[i].YouTubeVideoID = vid
			}
		}
	}

	utils.Success(ctx, 200, gin.H{
		"data":       tracks,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": (int(total) + limit - 1) / limit,
		"sort":       sortBy,
		"order":      order,
	})
}

func (c *TrackController) SearchTracks(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	type TrackResult struct {
		ID             uint   `json:"id"`
		AlbumID        uint   `json:"album_id"`
		Title          string `json:"title"`
		Duration       int    `json:"duration"`
		TrackNumber    int    `json:"track_number"`
		DiscNumber     int    `json:"disc_number"`
		Side           string `json:"side"`
		Position       string `json:"position"`
		AudioFileURL   string `json:"audio_file_url"`
		ReleaseYear    int    `json:"release_year"`
		AlbumGenre     string `json:"album_genre"`
		AlbumTitle     string `json:"album_title"`
		AlbumArtist    string `json:"album_artist"`
		YouTubeVideoID string `json:"youtube_video_id"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	sortBy := ctx.DefaultQuery("sort", "title")
	order := ctx.DefaultQuery("order", "asc")

	allowedSorts := map[string]bool{
		"title":        true,
		"album_title":  true,
		"album_artist": true,
		"track_number": true,
		"duration":     true,
		"created_at":   true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "title"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}

	sortColumn := sortBy
	switch sortBy {
	case "album_title":
		sortColumn = "albums.title"
	case "album_artist":
		sortColumn = "albums.artist"
	case "track_number":
		sortColumn = "tracks.track_number"
	case "duration":
		sortColumn = "tracks.duration"
	case "created_at":
		sortColumn = "tracks.created_at"
	default:
		sortColumn = "tracks.title"
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
	searchTerm := "%" + strings.ToLower(query) + "%"

	var tracks []TrackResult
	var total int64

	baseQuery := c.db.Model(&models.Track{}).
		Table("tracks").
		Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.title IS NOT NULL AND tracks.title != '' AND (LOWER(tracks.title) LIKE ? OR LOWER(albums.title) LIKE ? OR LOWER(albums.artist) LIKE ?)", searchTerm, searchTerm, searchTerm)

	if excludeIDs != "" {
		idStrings := strings.Split(excludeIDs, ",")
		for _, idStr := range idStrings {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				baseQuery = baseQuery.Where("tracks.id != ?", id)
			}
		}
	}

	baseQuery.Count(&total)
	result := baseQuery.Order(fmt.Sprintf("LOWER(%s) %s", sortColumn, order)).Offset(offset).Limit(limit).Find(&tracks)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search tracks"})
		return
	}

	if len(tracks) > 0 {
		trackIDs := make([]uint, len(tracks))
		for i, t := range tracks {
			trackIDs[i] = t.ID
		}

		var rawRows []map[string]interface{}
		c.db.Raw(`SELECT * FROM track_youtube_matches WHERE track_id IN ?`, trackIDs).Scan(&rawRows)

		ytMap := make(map[uint]string, len(rawRows))
		for _, row := range rawRows {
			trackID := uint(row["track_id"].(int64))
			if v, ok := row["youtube_video_id"].(string); ok && v != "" {
				ytMap[trackID] = v
			}
		}

		for i := range tracks {
			if vid, ok := ytMap[tracks[i].ID]; ok {
				tracks[i].YouTubeVideoID = vid
			}
		}
	}

	ctx.JSON(200, gin.H{
		"data":       tracks,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": (int(total) + limit - 1) / limit,
		"sort":       sortBy,
		"order":      order,
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
		ID             uint   `json:"id"`
		AlbumID        uint   `json:"album_id"`
		Title          string `json:"title"`
		Duration       int    `json:"duration"`
		TrackNumber    int    `json:"track_number"`
		AudioFileURL   string `json:"audio_file_url"`
		AlbumTitle     string `json:"album_title"`
		AlbumArtist    string `json:"album_artist"`
		AlbumCover     string `json:"album_cover"`
		YouTubeVideoID string `json:"youtube_video_id"`
		VideoTitle     string `json:"video_title"`
		VideoDuration  int    `json:"video_duration"`
		ReleaseYear    int    `json:"release_year"`
		AlbumGenre     string `json:"album_genre"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	// First get track with album info
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist, albums.cover_image_url as album_cover, albums.release_year as release_year, albums.genre as album_genre").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.id = ?", trackID).
		First(&trackData)

	if result.Error != nil {
		utils.NotFound(ctx, "Track not found")
		return
	}

	// Then get YouTube match if it exists
	var youTubeMatch models.TrackYouTubeMatch
	c.db.Where("track_id = ? AND is_manual = ?", trackID, true).Or("track_id = ? AND match_score >= ?", trackID, 80).First(&youTubeMatch)
	if youTubeMatch.YouTubeVideoID != "" {
		trackData.YouTubeVideoID = youTubeMatch.YouTubeVideoID
		trackData.VideoTitle = youTubeMatch.VideoTitle
		trackData.VideoDuration = youTubeMatch.VideoDuration
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

func (c *TrackController) SetYouTubeVideo(ctx *gin.Context) {
	id := ctx.Param("id")
	trackID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.BadRequest(ctx, "Invalid track ID")
		return
	}

	var input struct {
		YouTubeURL string `json:"youtube_url"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(ctx, "Invalid request body")
		return
	}

	videoID := extractYouTubeVideoID(input.YouTubeURL)
	if videoID == "" {
		utils.BadRequest(ctx, "Invalid YouTube URL")
		return
	}

	var existingMatch models.TrackYouTubeMatch
	result := c.db.Where("track_id = ?", trackID).First(&existingMatch)

	if result.Error == gorm.ErrRecordNotFound {
		newMatch := models.TrackYouTubeMatch{
			TrackID:        uint(trackID),
			YouTubeVideoID: videoID,
			Status:         "matched",
			MatchMethod:    "manual",
		}
		log.Printf("[DEBUG] SetYouTubeVideo: Creating new match for track %d with videoID=%s, status=%s", trackID, videoID, newMatch.Status)
		if err := c.db.Create(&newMatch).Error; err != nil {
			utils.InternalError(ctx, "Failed to create YouTube match")
			return
		}
		existingMatch = newMatch
	} else {
		existingMatch.YouTubeVideoID = videoID
		existingMatch.Status = "matched"
		log.Printf("[DEBUG] SetYouTubeVideo: Updating existing match for track %d with videoID=%s, status=%s", trackID, videoID, existingMatch.Status)
		if err := c.db.Save(&existingMatch).Error; err != nil {
			utils.InternalError(ctx, "Failed to update YouTube match")
			return
		}
	}

	if duration, err := fetchYouTubeVideoDuration(videoID); err == nil && duration > 0 {
		existingMatch.VideoDuration = duration
		c.db.Save(&existingMatch)
		log.Printf("[DEBUG] SetYouTubeVideo: Cached duration for video %s: %d seconds", videoID, duration)
	} else {
		log.Printf("[DEBUG] SetYouTubeVideo: Could not fetch duration for video %s: %v", videoID, err)
	}

	utils.Success(ctx, 200, gin.H{
		"message":          "YouTube video set successfully",
		"youtube_video_id": videoID,
	})
}

func (c *TrackController) DeleteYouTubeVideo(ctx *gin.Context) {
	id := ctx.Param("id")
	trackID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.BadRequest(ctx, "Invalid track ID")
		return
	}

	var existingMatch models.TrackYouTubeMatch
	result := c.db.Where("track_id = ?", trackID).First(&existingMatch)

	if result.Error != nil {
		utils.NotFound(ctx, "YouTube match not found")
		return
	}

	if err := c.db.Model(&existingMatch).Updates(map[string]interface{}{
		"youtube_video_id": "",
		"status":           "unavailable",
	}).Error; err != nil {
		utils.InternalError(ctx, "Failed to clear YouTube video")
		return
	}

	utils.Success(ctx, 200, gin.H{
		"message": "YouTube video cleared successfully",
	})
}

func extractYouTubeVideoID(url string) string {
	if url == "" {
		return ""
	}

	regExp := regexp.MustCompile(`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/)([^"&?\/\s]{11})`)
	matches := regExp.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Debug endpoint to check YouTube matches in database
func (c *TrackController) DebugYouTubeMatches(ctx *gin.Context) {
	var matches []models.TrackYouTubeMatch
	result := c.db.Find(&matches)

	log.Printf("[DEBUG] DebugYouTubeMatches: Total records: %d", result.RowsAffected)

	type MatchInfo struct {
		ID             uint   `json:"id"`
		TrackID        uint   `json:"track_id"`
		YouTubeVideoID string `json:"youtube_video_id"`
		VideoTitle     string `json:"video_title"`
		MatchMethod    string `json:"match_method"`
		Status         string `json:"status"`
	}

	var matchInfo []MatchInfo
	for _, m := range matches {
		log.Printf("[DEBUG] DB record: id=%d, track_id=%d, youtube_video_id='%s', title='%s', method='%s', status='%s'",
			m.ID, m.TrackID, m.YouTubeVideoID, m.VideoTitle, m.MatchMethod, m.Status)
		matchInfo = append(matchInfo, MatchInfo{
			ID:             m.ID,
			TrackID:        m.TrackID,
			YouTubeVideoID: m.YouTubeVideoID,
			VideoTitle:     m.VideoTitle,
			MatchMethod:    m.MatchMethod,
			Status:         m.Status,
		})
	}

	ctx.JSON(200, gin.H{
		"total_matches": result.RowsAffected,
		"matches":       matchInfo,
	})
}
