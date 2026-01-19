package controllers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"vinylfo/duration"
	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DurationController struct {
	db              *gorm.DB
	resolverService *services.DurationResolverService
}

var (
	bulkWorker       *services.DurationWorker
	bulkStateManager *duration.StateManager
	bulkCancel       context.CancelFunc
	bulkMutex        sync.Mutex
)

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

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	query := c.db.Model(&models.Track{}).Where("duration = 0 OR duration IS NULL")

	if albumID > 0 {
		query = query.Where("album_id = ?", albumID)
	}

	var total int64
	query.Count(&total)

	var tracks []models.Track
	if err := query.Offset(offset).Limit(limit).
		Order("album_id, track_number").
		Find(&tracks).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	ctx.JSON(http.StatusOK, gin.H{
		"tracks":      result,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationController) GetStatistics(ctx *gin.Context) {
	var totalTracks, missingDuration, resolved, needsReview int64

	c.db.Model(&models.Track{}).Count(&totalTracks)
	c.db.Model(&models.Track{}).Where("duration = 0 OR duration IS NULL").Count(&missingDuration)
	c.db.Model(&models.Track{}).Where("duration_source = ?", "resolved").Count(&resolved)
	c.db.Model(&models.DurationResolution{}).Where("status = ?", "needs_review").Count(&needsReview)

	var recentResolutions []models.DurationResolution
	c.db.Where("status IN ('resolved', 'needs_review', 'approved')").
		Order("created_at DESC").
		Limit(10).
		Find(&recentResolutions)

	ctx.JSON(http.StatusOK, gin.H{
		"total_tracks":       totalTracks,
		"missing_duration":   missingDuration,
		"resolved":           resolved,
		"needs_review":       needsReview,
		"recent_resolutions": recentResolutions,
	})
}

func (c *DurationController) ResolveTrack(ctx *gin.Context) {
	trackID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid track ID"})
		return
	}

	force := ctx.DefaultQuery("force", "false") == "true"

	var track models.Track
	if err := c.db.First(&track, trackID).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "track not found"})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"resolution": resolution,
		"message":    "Track resolution completed",
	})
}

func (c *DurationController) ResolveAlbum(ctx *gin.Context) {
	albumID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid album ID"})
		return
	}

	var album models.Album
	if err := c.db.First(&album, albumID).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}

	tracks, err := c.resolverService.GetTracksNeedingResolutionForAlbum(uint(albumID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(tracks) == 0 {
		ctx.JSON(http.StatusOK, gin.H{
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

	ctx.JSON(http.StatusOK, gin.H{
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid track ID"})
		return
	}

	var resolution models.DurationResolution
	if err := c.db.Where("track_id = ?", trackID).First(&resolution).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "no resolution found for track"})
		return
	}

	var track models.Track
	c.db.First(&track, trackID)

	var album models.Album
	c.db.First(&album, track.AlbumID)

	ctx.JSON(http.StatusOK, gin.H{
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

func (c *DurationController) GetReviewQueue(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	c.db.Model(&models.DurationResolution{}).Where("status = ?", "needs_review").Count(&total)

	var resolutions []models.DurationResolution
	if err := c.db.
		Where("status = ?", "needs_review").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&resolutions).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type SourceDisplay struct {
		ID            uint    `json:"id"`
		SourceName    string  `json:"source_name"`
		DurationValue int     `json:"duration_value"`
		MatchScore    float64 `json:"match_score"`
		Confidence    float64 `json:"confidence"`
		ExternalURL   string  `json:"external_url"`
		ErrorMessage  string  `json:"error_message"`
	}

	type ReviewItem struct {
		Resolution models.DurationResolution `json:"resolution"`
		Sources    []SourceDisplay           `json:"sources"`
		Track      models.Track              `json:"track"`
		Album      models.Album              `json:"album"`
	}

	var items []ReviewItem
	for _, res := range resolutions {
		var track models.Track
		var album models.Album
		c.db.First(&track, res.TrackID)
		c.db.First(&album, res.AlbumID)

		var sourceModels []models.DurationSource
		c.db.Where("resolution_id = ?", res.ID).Order("id").Find(&sourceModels)

		var sources []SourceDisplay
		for _, sm := range sourceModels {
			sources = append(sources, SourceDisplay{
				ID:            sm.ID,
				SourceName:    sm.SourceName,
				DurationValue: sm.DurationValue,
				MatchScore:    sm.MatchScore,
				Confidence:    sm.Confidence,
				ExternalURL:   sm.ExternalURL,
				ErrorMessage:  sm.ErrorMessage,
			})
		}

		items = append(items, ReviewItem{
			Resolution: res,
			Sources:    sources,
			Track:      track,
			Album:      album,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationController) GetResolvedQueue(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	c.db.Model(&models.DurationResolution{}).Where("status = ?", "resolved").Count(&total)

	var resolutions []models.DurationResolution
	if err := c.db.
		Where("status = ?", "resolved").
		Order("updated_at DESC").
		Offset(offset).Limit(limit).
		Find(&resolutions).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type SourceDisplay struct {
		ID            uint    `json:"id"`
		SourceName    string  `json:"source_name"`
		DurationValue int     `json:"duration_value"`
		MatchScore    float64 `json:"match_score"`
		Confidence    float64 `json:"confidence"`
		ExternalURL   string  `json:"external_url"`
		CausedMatch   bool    `json:"caused_match"`
	}

	type ResolvedItem struct {
		Resolution models.DurationResolution `json:"resolution"`
		Sources    []SourceDisplay           `json:"sources"`
		Track      models.Track              `json:"track"`
		Album      models.Album              `json:"album"`
	}

	var items []ResolvedItem
	for _, res := range resolutions {
		var track models.Track
		var album models.Album
		c.db.First(&track, res.TrackID)
		c.db.First(&album, res.AlbumID)

		var sourceModels []models.DurationSource
		c.db.Where("resolution_id = ? AND error_message = ?", res.ID, "").Order("confidence DESC").Find(&sourceModels)

		var sources []SourceDisplay
		for _, sm := range sourceModels {
			causedMatch := false
			if res.ResolvedDuration != nil {
				diff := sm.DurationValue - *res.ResolvedDuration
				if diff < 0 {
					diff = -diff
				}
				causedMatch = diff <= 3
			}
			sources = append(sources, SourceDisplay{
				ID:            sm.ID,
				SourceName:    sm.SourceName,
				DurationValue: sm.DurationValue,
				MatchScore:    sm.MatchScore,
				Confidence:    sm.Confidence,
				ExternalURL:   sm.ExternalURL,
				CausedMatch:   causedMatch,
			})
		}

		items = append(items, ResolvedItem{
			Resolution: res,
			Sources:    sources,
			Track:      track,
			Album:      album,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationController) GetReviewDetails(ctx *gin.Context) {
	resolutionID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid resolution ID"})
		return
	}

	var resolution models.DurationResolution
	if err := c.db.First(&resolution, resolutionID).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "resolution not found"})
		return
	}

	var track models.Track
	c.db.First(&track, resolution.TrackID)

	var album models.Album
	c.db.First(&album, resolution.AlbumID)

	type SourceDisplay struct {
		ID                uint    `json:"id" gorm:"primaryKey"`
		SourceName        string  `json:"source_name" gorm:"size:50"`
		DurationValue     int     `json:"duration_value"`
		Duration          int     `json:"duration" gorm:"-"`
		DurationFormatted string  `json:"duration_formatted" gorm:"-"`
		MatchScore        float64 `json:"match_score"`
		Confidence        float64 `json:"confidence"`
		ExternalURL       string  `json:"external_url" gorm:"size:512"`
		ErrorMessage      string  `json:"error_message" gorm:"size:500"`
	}

	var sourceModels []models.DurationSource
	if err := c.db.Where("resolution_id = ?", resolutionID).Order("id").Find(&sourceModels).Error; err != nil {
		log.Printf("Error loading sources for resolution %d: %v", resolutionID, err)
	}

	var sources []SourceDisplay
	for _, sm := range sourceModels {
		sources = append(sources, SourceDisplay{
			ID:            sm.ID,
			SourceName:    sm.SourceName,
			DurationValue: sm.DurationValue,
			Duration:      sm.DurationValue,
			MatchScore:    sm.MatchScore,
			Confidence:    sm.Confidence,
			ExternalURL:   sm.ExternalURL,
			ErrorMessage:  sm.ErrorMessage,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"resolution": resolution,
		"sources":    sources,
		"track":      track,
		"album":      album,
	})
}

func (c *DurationController) SubmitReview(ctx *gin.Context) {
	resolutionID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid resolution ID"})
		return
	}

	var input struct {
		Action   string `json:"action" binding:"required"`
		Duration int    `json:"duration"`
		SourceID uint   `json:"source_id"`
		Notes    string `json:"notes"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch input.Action {
	case "apply":
		if input.SourceID > 0 {
			var source models.DurationSource
			if err := c.db.First(&source, input.SourceID).Error; err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "source not found"})
				return
			}
			input.Duration = source.DurationValue
		}

		if input.Duration <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "duration required for apply action"})
			return
		}

		err := c.resolverService.ApplyResolution(uint(resolutionID), input.Duration, input.Notes)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "reject":
		err := c.resolverService.RejectResolution(uint(resolutionID), "user", input.Notes)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "manual":
		if input.Duration <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "duration required for manual action"})
			return
		}

		var resolution models.DurationResolution
		c.db.First(&resolution, resolutionID)

		err := c.resolverService.ManuallySetDuration(resolution.TrackID, input.Duration, input.Notes)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "skip":
		c.db.Model(&models.DurationResolution{}).
			Where("id = ?", resolutionID).
			Update("review_notes", input.Notes)

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Review submitted successfully",
		"action":  input.Action,
	})
}

func (c *DurationController) BulkReview(ctx *gin.Context) {
	var input struct {
		Action        string `json:"action" binding:"required"`
		ResolutionIDs []uint `json:"resolution_ids" binding:"required"`
		Notes         string `json:"notes"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var processed, failed int

	for _, resID := range input.ResolutionIDs {
		var err error

		switch input.Action {
		case "apply_all":
			var resolution models.DurationResolution
			if err = c.db.First(&resolution, resID).Error; err != nil {
				failed++
				continue
			}

			var bestDuration int
			var bestConfidence float64 = -1
			for _, src := range resolution.Sources {
				if src.DurationValue > 0 && src.Confidence > bestConfidence {
					bestConfidence = src.Confidence
					bestDuration = src.DurationValue
				}
			}

			if bestDuration > 0 {
				err = c.resolverService.ApplyResolution(resID, bestDuration, input.Notes)
			} else {
				failed++
				continue
			}

		case "reject_all":
			err = c.resolverService.RejectResolution(resID, "system", input.Notes)

		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
			return
		}

		if err != nil {
			failed++
		} else {
			processed++
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Bulk review completed",
		"processed": processed,
		"failed":    failed,
	})
}

func (c *DurationController) StartBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager != nil && bulkStateManager.IsRunning() {
		ctx.JSON(http.StatusConflict, gin.H{"error": "Bulk resolution already running"})
		return
	}

	bulkStateManager = duration.NewStateManager()
	workerCtx, cancel := context.WithCancel(context.Background())
	bulkCancel = cancel

	bulkWorker = services.NewDurationWorker(
		c.db,
		c.resolverService,
		bulkStateManager,
		workerCtx,
		cancel,
	)

	go bulkWorker.Run()

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Bulk resolution started",
		"status":  "running",
	})
}

func (c *DurationController) PauseBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager == nil || !bulkStateManager.IsRunning() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No bulk resolution running"})
		return
	}

	bulkStateManager.RequestPause()
	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution paused"})
}

func (c *DurationController) ResumeBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkStateManager == nil || !bulkStateManager.IsPaused() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No paused bulk resolution"})
		return
	}

	bulkStateManager.RequestResume()
	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution resumed"})
}

func (c *DurationController) CancelBulkResolution(ctx *gin.Context) {
	bulkMutex.Lock()
	defer bulkMutex.Unlock()

	if bulkCancel != nil {
		bulkCancel()
	}
	if bulkStateManager != nil {
		bulkStateManager.RequestStop()
		bulkStateManager.Reset()
	}

	progressService := services.NewDurationProgressService(c.db)
	progressService.Reset()

	bulkStateManager = nil
	bulkWorker = nil
	bulkCancel = nil

	ctx.JSON(http.StatusOK, gin.H{"message": "Bulk resolution cancelled"})
}

func (c *DurationController) GetBulkProgress(ctx *gin.Context) {
	if bulkStateManager == nil {
		progressService := services.NewDurationProgressService(c.db)
		saved, err := progressService.Load()
		if err != nil || saved == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"status":           "idle",
				"total_tracks":     0,
				"processed_tracks": 0,
			})
			return
		}

		if saved.Status == "running" {
			saved.Status = "failed"
			saved.LastError = "Worker stopped unexpectedly"
			c.db.Save(saved)
			ctx.JSON(http.StatusOK, gin.H{
				"status":             "failed",
				"total_tracks":       saved.TotalTracks,
				"processed_tracks":   saved.ProcessedTracks,
				"resolved_count":     saved.ResolvedCount,
				"needs_review_count": saved.NeedsReviewCount,
				"failed_count":       saved.FailedCount,
				"last_error":         "Worker stopped unexpectedly",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status":             saved.Status,
			"total_tracks":       saved.TotalTracks,
			"processed_tracks":   saved.ProcessedTracks,
			"resolved_count":     saved.ResolvedCount,
			"needs_review_count": saved.NeedsReviewCount,
			"failed_count":       saved.FailedCount,
			"last_activity":      saved.LastActivityAt,
		})
		return
	}

	state := bulkStateManager.GetState()
	percentComplete := 0.0
	if state.TotalTracks > 0 {
		percentComplete = float64(state.ProcessedTracks) / float64(state.TotalTracks) * 100
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":             string(state.Status),
		"total_tracks":       state.TotalTracks,
		"processed_tracks":   state.ProcessedTracks,
		"resolved_count":     state.ResolvedCount,
		"needs_review_count": state.NeedsReviewCount,
		"failed_count":       state.FailedCount,
		"skipped_count":      state.SkippedCount,
		"current_track":      state.CurrentTrack,
		"last_error":         state.LastError,
		"percent_complete":   percentComplete,
	})
}
