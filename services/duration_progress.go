package services

import (
	"time"

	"vinylfo/duration"
	"vinylfo/models"

	"gorm.io/gorm"
)

type DurationProgressService struct {
	db *gorm.DB
}

func NewDurationProgressService(db *gorm.DB) *DurationProgressService {
	return &DurationProgressService{db: db}
}

func (s *DurationProgressService) Save(state duration.ResolverState) error {
	var progress models.DurationResolverProgress
	s.db.FirstOrCreate(&progress, models.DurationResolverProgress{ID: 1})

	progress.Status = string(state.Status)
	progress.TotalTracks = state.TotalTracks
	progress.ProcessedTracks = state.ProcessedTracks
	progress.ResolvedCount = state.ResolvedCount
	progress.NeedsReviewCount = state.NeedsReviewCount
	progress.FailedCount = state.FailedCount
	progress.SkippedCount = state.SkippedCount
	progress.CurrentTrackID = state.CurrentTrackID
	progress.LastActivityAt = time.Now()
	progress.LastError = state.LastError

	return s.db.Save(&progress).Error
}

func (s *DurationProgressService) Load() (*models.DurationResolverProgress, error) {
	var progress models.DurationResolverProgress
	err := s.db.Order("id DESC").First(&progress).Error
	if err != nil {
		return nil, err
	}

	maxAge := 30 * time.Minute
	if progress.Status == "paused" {
		maxAge = 4 * time.Hour
	}

	if time.Since(progress.LastActivityAt) > maxAge {
		progress.Status = "failed"
		progress.LastError = "Timed out"
		s.db.Save(&progress)
		return nil, nil
	}

	return &progress, nil
}

func (s *DurationProgressService) MarkComplete() error {
	now := time.Now()
	return s.db.Model(&models.DurationResolverProgress{}).
		Where("id = 1").
		Updates(map[string]interface{}{
			"status":       "completed",
			"completed_at": now,
		}).Error
}

func (s *DurationProgressService) Reset() error {
	return s.db.Where("1 = 1").Delete(&models.DurationResolverProgress{}).Error
}
