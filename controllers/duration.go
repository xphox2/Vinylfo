package controllers

import (
	"context"
	"os"
	"strconv"
	"time"

	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DurationController struct {
	db              *gorm.DB
	resolverService *services.DurationResolverService
}

func NewDurationController(db *gorm.DB) *DurationController {
	config := services.DefaultDurationResolverConfig()
	config.ContactEmail = "https://github.com/xphox2/Vinylfo"
	config.YouTubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	config.LastFMAPIKey = os.Getenv("LASTFM_API_KEY")

	return &DurationController{
		db:              db,
		resolverService: services.NewDurationResolverService(db, config),
	}
}

func (c *DurationController) GetTracksNeedingResolution(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "50"))
	albumID, _ := strconv.Atoi(ctx.Query("album_id"))
	searchQuery := ctx.Query("q")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	subQuery := "(SELECT track_id FROM duration_resolutions WHERE status != 'failed')"
	query := c.db.Model(&models.Track{}).
		Where("duration = 0 OR duration IS NULL").
		Where("album_id != 0").
		Where("title IS NOT NULL AND TRIM(title) != ''").
		Where("(duration_needs_review = ? OR tracks.id NOT IN "+subQuery+")", true)

	if albumID > 0 {
		query = query.Where("album_id = ?", albumID)
	}

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		trackSubQuery := "(SELECT id FROM tracks WHERE title LIKE ?)"
		albumSubQuery := "(SELECT id FROM albums WHERE title LIKE ? OR artist LIKE ?)"
		query = query.Where("(tracks.id IN "+trackSubQuery+" OR tracks.album_id IN "+albumSubQuery+")", searchPattern, searchPattern, searchPattern)
	}

	var total int64
	query.Count(&total)

	var tracks []models.Track
	if err := query.Offset(offset).Limit(limit).
		Order("album_id, track_number").
		Find(&tracks).Error; err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	type TrackWithAlbum struct {
		models.Track
		AlbumTitle string `json:"album_title"`
		Artist     string `json:"artist"`
	}

	var result []TrackWithAlbum
	for _, track := range tracks {
		var album models.Album
		c.db.First(&album, track.AlbumID)
		result = append(result, TrackWithAlbum{
			Track:      track,
			AlbumTitle: album.Title,
			Artist:     album.Artist,
		})
	}

	ctx.JSON(200, gin.H{
		"tracks":      result,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationController) GetStatistics(ctx *gin.Context) {
	var totalTracks, missingDuration, resolved, needsReview, unprocessed int64

	c.db.Model(&models.Track{}).Count(&totalTracks)
	c.db.Model(&models.Track{}).Where("duration = 0 OR duration IS NULL").Count(&missingDuration)
	c.db.Model(&models.Track{}).Where("duration_source = ?", "resolved").Count(&resolved)
	c.db.Model(&models.DurationResolution{}).Where("status = ?", "needs_review").Count(&needsReview)

	c.db.Model(&models.Track{}).
		Where("duration = 0 OR duration IS NULL").
		Where("(duration_needs_review = ? OR id NOT IN (SELECT track_id FROM duration_resolutions WHERE status != 'failed'))", true).
		Count(&unprocessed)

	var recentResolutions []models.DurationResolution
	c.db.Where("status IN ('resolved', 'needs_review', 'approved')").
		Order("created_at DESC").
		Limit(10).
		Find(&recentResolutions)

	ctx.JSON(200, gin.H{
		"total_tracks":       totalTracks,
		"missing_duration":   missingDuration,
		"resolved":           resolved,
		"needs_review":       needsReview,
		"unprocessed":        unprocessed,
		"recent_resolutions": recentResolutions,
	})
}

func (c *DurationController) ResolveTrack(ctx *gin.Context) {
	trackID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid track ID"})
		return
	}

	force := ctx.DefaultQuery("force", "false") == "true"

	var track models.Track
	if err := c.db.First(&track, trackID).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "track not found"})
		return
	}

	if force {
		c.db.Where("track_id = ?", trackID).Delete(&models.DurationResolution{})
		c.db.Where("resolution_id IN (SELECT id FROM duration_resolutions WHERE track_id = ?)", trackID).Delete(&models.DurationSource{})
	}

	resolveCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resolution, err := c.resolverService.ResolveTrackDuration(resolveCtx, track)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"resolution": resolution,
		"message":    "Track resolution completed",
	})
}

func (c *DurationController) RetryFailedTrack(ctx *gin.Context) {
	trackID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid track ID"})
		return
	}

	var track models.Track
	if err := c.db.First(&track, trackID).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "track not found"})
		return
	}

	var resolution models.DurationResolution
	if err := c.db.Where("track_id = ? AND status = ?", trackID, "failed").First(&resolution).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "no failed resolution found for track"})
		return
	}

	c.db.Where("resolution_id = ?", resolution.ID).Delete(&models.DurationSource{})
	c.db.Delete(&resolution)

	track.DurationNeedsReview = false
	c.db.Save(&track)

	c.ResolveTrack(ctx)
}

func (c *DurationController) SetManualDuration(ctx *gin.Context) {
	trackID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid track ID"})
		return
	}

	var input struct {
		Duration int    `json:"duration" binding:"required"`
		Notes    string `json:"notes"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if input.Duration <= 0 {
		ctx.JSON(400, gin.H{"error": "duration must be positive"})
		return
	}

	err = c.resolverService.ManuallySetDuration(uint(trackID), input.Duration, input.Notes)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"message": "Manual duration saved",
	})
}

func (c *DurationController) ResolveAlbum(ctx *gin.Context) {
	albumID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid album ID"})
		return
	}

	var album models.Album
	if err := c.db.First(&album, albumID).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "album not found"})
		return
	}

	tracks, err := c.resolverService.GetTracksNeedingResolutionForAlbum(uint(albumID))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if len(tracks) == 0 {
		ctx.JSON(200, gin.H{
			"message":      "No tracks need resolution in this album",
			"resolved":     0,
			"needs_review": 0,
			"failed":       0,
		})
		return
	}

	resolveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var resolved, needsReview, failed int
	var resolutions []models.DurationResolution

	for _, track := range tracks {
		resolution, err := c.resolverService.ResolveTrackDuration(resolveCtx, track)
		if err != nil {
			failed++
			continue
		}

		switch resolution.Status {
		case "resolved":
			resolved++
		case "needs_review":
			needsReview++
		case "failed":
			failed++
		}
		resolutions = append(resolutions, *resolution)
	}

	ctx.JSON(200, gin.H{
		"message":      "Album resolution completed",
		"total_tracks": len(tracks),
		"resolved":     resolved,
		"needs_review": needsReview,
		"failed":       failed,
		"resolutions":  resolutions,
	})
}

func (c *DurationController) GetResolutionStatus(ctx *gin.Context) {
	trackID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid track ID"})
		return
	}

	var resolution models.DurationResolution
	if err := c.db.Where("track_id = ?", trackID).First(&resolution).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "no resolution found for track"})
		return
	}

	var track models.Track
	c.db.First(&track, trackID)

	var album models.Album
	c.db.First(&album, track.AlbumID)

	ctx.JSON(200, gin.H{
		"resolution": resolution,
		"track": gin.H{
			"id":       track.ID,
			"title":    track.Title,
			"duration": track.Duration,
		},
		"album": gin.H{
			"id":     album.ID,
			"title":  album.Title,
			"artist": album.Artist,
		},
	})
}
