package controllers

import (
	"log"
	"strconv"

	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DurationReviewController struct {
	db              *gorm.DB
	resolverService *services.DurationResolverService
}

func NewDurationReviewController(db *gorm.DB) *DurationReviewController {
	config := services.DefaultDurationResolverConfig()
	config.ContactEmail = "https://github.com/xphox2/Vinylfo"

	return &DurationReviewController{
		db:              db,
		resolverService: services.NewDurationResolverService(db, config),
	}
}

func (c *DurationReviewController) GetReviewQueue(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	searchQuery := ctx.Query("q")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	baseQuery := c.db.Model(&models.DurationResolution{}).Where("status = ?", "needs_review")

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		trackSubQuery := "(SELECT id FROM tracks WHERE title LIKE ?)"
		albumSubQuery := "(SELECT id FROM albums WHERE title LIKE ? OR artist LIKE ?)"
		baseQuery = baseQuery.Where("(duration_resolutions.track_id IN "+trackSubQuery+" OR duration_resolutions.album_id IN "+albumSubQuery+")", searchPattern, searchPattern, searchPattern)
	}

	var total int64
	baseQuery.Count(&total)

	var resolutions []models.DurationResolution
	query := baseQuery.
		Order("created_at DESC").
		Offset(offset).Limit(limit)

	if err := query.Find(&resolutions).Error; err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
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

	ctx.JSON(200, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationReviewController) GetResolvedQueue(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	searchQuery := ctx.Query("q")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	baseQuery := c.db.Model(&models.DurationResolution{}).Where("status IN ?", []string{"resolved", "approved"})

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		trackSubQuery := "(SELECT id FROM tracks WHERE title LIKE ?)"
		albumSubQuery := "(SELECT id FROM albums WHERE title LIKE ? OR artist LIKE ?)"
		baseQuery = baseQuery.Where("(duration_resolutions.track_id IN "+trackSubQuery+" OR duration_resolutions.album_id IN "+albumSubQuery+")", searchPattern, searchPattern, searchPattern)
	}

	var total int64
	baseQuery.Count(&total)

	var resolutions []models.DurationResolution
	query := baseQuery.
		Order("updated_at DESC").
		Offset(offset).Limit(limit)

	if err := query.Find(&resolutions).Error; err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
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

	ctx.JSON(200, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	})
}

func (c *DurationReviewController) GetReviewDetails(ctx *gin.Context) {
	resolutionID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid resolution ID"})
		return
	}

	var resolution models.DurationResolution
	if err := c.db.First(&resolution, resolutionID).Error; err != nil {
		ctx.JSON(404, gin.H{"error": "resolution not found"})
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

	ctx.JSON(200, gin.H{
		"resolution": resolution,
		"sources":    sources,
		"track":      track,
		"album":      album,
	})
}

func (c *DurationReviewController) SubmitReview(ctx *gin.Context) {
	resolutionID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid resolution ID"})
		return
	}

	var input struct {
		Action   string `json:"action" binding:"required"`
		Duration int    `json:"duration"`
		SourceID uint   `json:"source_id"`
		Notes    string `json:"notes"`
		TrackID  int    `json:"track_id"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	switch input.Action {
	case "apply":
		if input.SourceID > 0 {
			var source models.DurationSource
			if err := c.db.First(&source, input.SourceID).Error; err != nil {
				ctx.JSON(400, gin.H{"error": "source not found"})
				return
			}
			input.Duration = source.DurationValue
		}

		if input.Duration <= 0 {
			ctx.JSON(400, gin.H{"error": "duration required for apply action"})
			return
		}

		err := c.resolverService.ApplyResolution(uint(resolutionID), input.Duration, input.Notes)
		if err != nil {
			ctx.JSON(500, gin.H{"error": err.Error()})
			return
		}

	case "reject":
		err := c.resolverService.RejectResolution(uint(resolutionID), "user", input.Notes)
		if err != nil {
			ctx.JSON(500, gin.H{"error": err.Error()})
			return
		}

	case "manual":
		if input.Duration <= 0 {
			ctx.JSON(400, gin.H{"error": "duration required for manual action"})
			return
		}

		if input.TrackID > 0 {
			err := c.resolverService.ManuallySetDuration(uint(input.TrackID), input.Duration, input.Notes)
			if err != nil {
				ctx.JSON(500, gin.H{"error": err.Error()})
				return
			}
		} else {
			var resolution models.DurationResolution
			c.db.First(&resolution, resolutionID)

			err := c.resolverService.ManuallySetDuration(resolution.TrackID, input.Duration, input.Notes)
			if err != nil {
				ctx.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}

	case "skip":
		c.db.Model(&models.DurationResolution{}).
			Where("id = ?", resolutionID).
			Update("review_notes", input.Notes)

	default:
		ctx.JSON(400, gin.H{"error": "invalid action"})
		return
	}

	ctx.JSON(200, gin.H{
		"message": "Review submitted successfully",
		"action":  input.Action,
	})
}

func (c *DurationReviewController) BulkReview(ctx *gin.Context) {
	var input struct {
		Action        string `json:"action" binding:"required"`
		ResolutionIDs []uint `json:"resolution_ids" binding:"required"`
		Notes         string `json:"notes"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
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
			ctx.JSON(400, gin.H{"error": "invalid action"})
			return
		}

		if err != nil {
			failed++
		} else {
			processed++
		}
	}

	ctx.JSON(200, gin.H{
		"message":   "Bulk review completed",
		"processed": processed,
		"failed":    failed,
	})
}
