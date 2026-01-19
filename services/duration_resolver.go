package services

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"vinylfo/duration"
	"vinylfo/models"

	"gorm.io/gorm"
)

type DurationResolverConfig struct {
	ConsensusThreshold   int
	ToleranceSeconds     int
	AutoApplyOnConsensus bool
	MinMatchScore        float64
	ContactEmail         string
	YouTubeAPIKey        string
	LastFMAPIKey         string
}

func DefaultDurationResolverConfig() DurationResolverConfig {
	return DurationResolverConfig{
		ConsensusThreshold:   2,
		ToleranceSeconds:     3,
		AutoApplyOnConsensus: true,
		MinMatchScore:        0.6,
		ContactEmail:         "https://github.com/xphox2/Vinylfo",
	}
}

type DurationResolverService struct {
	db      *gorm.DB
	clients []duration.MusicAPIClient
	config  DurationResolverConfig
}

func NewDurationResolverService(db *gorm.DB, config DurationResolverConfig) *DurationResolverService {
	clients := []duration.MusicAPIClient{
		duration.NewMusicBrainzClient(config.ContactEmail),
		duration.NewWikipediaClient(),
		duration.NewLastFMClient(config.LastFMAPIKey),
		duration.NewYouTubeClient(config.YouTubeAPIKey),
	}

	return &DurationResolverService{
		db:      db,
		clients: clients,
		config:  config,
	}
}

type DurationResolverTrack struct {
	models.Track
	AlbumTitle  string `json:"album_title"`
	AlbumArtist string `json:"album_artist"`
}

func (s *DurationResolverService) GetTracksNeedingResolution() ([]models.Track, error) {
	var tracks []models.Track
	err := s.db.Where("duration = 0 OR duration IS NULL").Find(&tracks).Error
	return tracks, err
}

func (s *DurationResolverService) GetTracksNeedingResolutionForAlbum(albumID uint) ([]models.Track, error) {
	var tracks []models.Track
	err := s.db.Where("album_id = ? AND (duration = 0 OR duration IS NULL)", albumID).
		Order("track_number").
		Find(&tracks).Error
	return tracks, err
}

func (s *DurationResolverService) getAlbumInfo(albumID uint) (title, artist string) {
	var album models.Album
	s.db.First(&album, albumID)
	return album.Title, album.Artist
}

func (s *DurationResolverService) ResolveTrackDuration(ctx context.Context, track models.Track) (*models.DurationResolution, error) {
	var existing models.DurationResolution
	err := s.db.Where("track_id = ?", track.ID).First(&existing).Error
	if err == nil {
		// Only skip if resolution was successful (resolved, approved) - allow retrying failed/needs_review
		if existing.Status == "resolved" || existing.Status == "approved" {
			log.Printf("Found existing successful resolution for track %d: %s", track.ID, existing.Status)
			return &existing, nil
		}
		// Delete the failed/needs_review resolution so we can retry
		log.Printf("Retrying resolution for track %d (previous status: %s)", track.ID, existing.Status)
		s.db.Where("resolution_id = ?", existing.ID).Delete(&models.DurationSource{})
		s.db.Delete(&existing)
	}

	albumTitle, artist := s.getAlbumInfo(track.AlbumID)
	log.Printf("DEBUG: Resolving track %d: Title='%s', Album='%s', Artist='%s'", track.ID, track.Title, albumTitle, artist)

	resolution := &models.DurationResolution{
		TrackID:             track.ID,
		AlbumID:             track.AlbumID,
		Status:              "in_progress",
		OriginalDuration:    track.Duration,
		TotalSourcesQueried: len(s.clients),
	}

	if err := s.db.Create(resolution).Error; err != nil {
		if s.db.Where("track_id = ?", track.ID).First(&existing).Error == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("failed to create resolution record: %w", err)
	}

	var successfulQueries int
	var allDurations []int
	var skippedExpensiveSources []string

	for _, client := range s.clients {
		if !client.IsConfigured() {
			resolution.TotalSourcesQueried--
			continue
		}

		// Skip expensive sources (YouTube) if we already have consensus from free sources
		if client.Name() == "youtube" && len(allDurations) >= s.config.ConsensusThreshold {
			_, consensusCount := s.findConsensus(allDurations)
			if consensusCount >= s.config.ConsensusThreshold {
				log.Printf("Skipping YouTube API - consensus already reached with %d sources", consensusCount)
				skippedExpensiveSources = append(skippedExpensiveSources, client.Name())
				resolution.TotalSourcesQueried--
				continue
			}
		}

		result, err := client.SearchTrack(ctx, track.Title, artist, albumTitle)
		if err != nil {
			source := models.DurationSource{
				ResolutionID: resolution.ID,
				SourceName:   client.Name(),
				ErrorMessage: err.Error(),
				QueriedAt:    time.Now(),
			}
			s.db.Create(&source)
			continue
		}

		successfulQueries++

		// Always create a source record so the UI can show what was queried
		source := models.DurationSource{
			ResolutionID: resolution.ID,
			SourceName:   client.Name(),
			QueriedAt:    time.Now(),
		}

		if result != nil {
			source.DurationValue = result.Duration
			source.Confidence = result.Confidence
			source.MatchScore = result.MatchScore
			source.ExternalID = result.ExternalID
			source.ExternalURL = result.ExternalURL
			source.RawResponse = result.RawResponse

			// Only count toward consensus if match score meets threshold
			if result.MatchScore >= s.config.MinMatchScore && result.Duration > 0 {
				allDurations = append(allDurations, result.Duration)
			}
		} else {
			// No result found - still save the source record to show it was queried
			source.ErrorMessage = "No matching track found"
		}

		s.db.Create(&source)
	}

	// Log if we saved API calls by skipping expensive sources
	if len(skippedExpensiveSources) > 0 {
		log.Printf("Saved API quota by skipping: %v", skippedExpensiveSources)
	}

	resolution.SuccessfulQueries = successfulQueries

	if len(allDurations) == 0 {
		resolution.Status = "failed"
	} else {
		resolvedDuration, consensusCount := s.findConsensus(allDurations)
		resolution.ConsensusCount = consensusCount

		if consensusCount >= s.config.ConsensusThreshold {
			resolution.Status = "resolved"
			resolution.ResolvedDuration = &resolvedDuration

			if s.config.AutoApplyOnConsensus {
				s.applyResolution(resolution, track)
			}
		} else if successfulQueries > 0 {
			resolution.Status = "needs_review"
		} else {
			resolution.Status = "failed"
		}
	}

	if err := s.db.Save(resolution).Error; err != nil {
		log.Printf("Failed to save resolution: %v", err)
	}

	return resolution, nil
}

func (s *DurationResolverService) findConsensus(durations []int) (int, int) {
	if len(durations) == 0 {
		return 0, 0
	}

	sort.Ints(durations)

	durationCounts := make(map[int]int)
	for _, d := range durations {
		durationCounts[d]++
	}

	var bestDuration int
	var bestCount int

	for duration, count := range durationCounts {
		if count > bestCount {
			bestCount = count
			bestDuration = duration
		}
	}

	if bestCount >= s.config.ConsensusThreshold {
		return bestDuration, bestCount
	}

	for i := 0; i < len(durations)-1; i++ {
		if abs(durations[i]-durations[i+1]) <= s.config.ToleranceSeconds {
			avg := (durations[i] + durations[i+1]) / 2
			return avg, 2
		}
	}

	return durations[0], 1
}

func (s *DurationResolverService) applyResolution(resolution *models.DurationResolution, track models.Track) {
	duration := *resolution.ResolvedDuration

	now := time.Now()
	track.Duration = duration
	track.DurationSource = "resolved"
	track.DurationResolvedAt = &now

	if err := s.db.Save(&track).Error; err != nil {
		log.Printf("Failed to update track duration: %v", err)
		return
	}

	resolution.AutoApplied = true
	resolution.AppliedAt = &now
	s.db.Save(resolution)
}

func (s *DurationResolverService) BulkResolve(ctx context.Context, tracks []models.Track) (*models.DurationResolverProgress, error) {
	progress := &models.DurationResolverProgress{
		Status:         "running",
		TotalTracks:    len(tracks),
		StartedAt:      timePtr(time.Now()),
		LastActivityAt: time.Now(),
	}

	if err := s.db.Create(progress).Error; err != nil {
		return nil, fmt.Errorf("failed to create progress record: %w", err)
	}

	resolvedCount := 0
	needsReviewCount := 0
	failedCount := 0
	skippedCount := 0

	for i, track := range tracks {
		progress.CurrentTrackID = track.ID
		progress.CurrentAlbumID = track.AlbumID
		progress.ProcessedTracks = i + 1
		progress.LastActivityAt = time.Now()

		if track.Duration > 0 {
			skippedCount++
			progress.SkippedCount = skippedCount
			s.db.Save(progress)
			continue
		}

		resolution, err := s.ResolveTrackDuration(ctx, track)
		if err != nil {
			log.Printf("Failed to resolve track %d: %v", track.ID, err)
			failedCount++
			progress.LastError = err.Error()
			progress.FailedCount = failedCount
			s.db.Save(progress)
			continue
		}

		switch resolution.Status {
		case "resolved":
			resolvedCount++
			progress.ResolvedCount = resolvedCount
		case "needs_review":
			needsReviewCount++
			progress.NeedsReviewCount = needsReviewCount
		case "failed":
			failedCount++
			progress.FailedCount = failedCount
		}

		s.db.Save(progress)
	}

	progress.Status = "completed"
	progress.CompletedAt = timePtr(time.Now())
	s.db.Save(progress)

	return progress, nil
}

func (s *DurationResolverService) GetResolutionByTrackID(trackID uint) (*models.DurationResolution, error) {
	var resolution models.DurationResolution
	err := s.db.Where("track_id = ?", trackID).First(&resolution).Error
	if err != nil {
		return nil, err
	}
	return &resolution, nil
}

func (s *DurationResolverService) GetPendingReviews() ([]models.DurationResolution, error) {
	var resolutions []models.DurationResolution
	err := s.db.Where("status = ?", "needs_review").Find(&resolutions).Error
	return resolutions, err
}

func (s *DurationResolverService) ApproveResolution(resolutionID uint, userID string, notes string) error {
	var resolution models.DurationResolution
	if err := s.db.First(&resolution, resolutionID).Error; err != nil {
		return err
	}

	if resolution.Status != "needs_review" {
		return fmt.Errorf("resolution is not pending review")
	}

	if resolution.ResolvedDuration == nil {
		return fmt.Errorf("no resolved duration to approve")
	}

	now := time.Now()
	resolution.Status = "approved"
	resolution.ReviewedAt = &now
	resolution.ReviewedBy = userID
	resolution.ReviewNotes = notes
	resolution.ReviewAction = "apply"
	resolution.ManuallyReviewed = true

	if err := s.db.Save(&resolution).Error; err != nil {
		return err
	}

	var track models.Track
	if err := s.db.First(&track, resolution.TrackID).Error; err != nil {
		return err
	}

	track.Duration = *resolution.ResolvedDuration
	track.DurationSource = "manual"
	track.DurationResolvedAt = &now
	track.DurationNeedsReview = false

	return s.db.Save(&track).Error
}

func (s *DurationResolverService) RejectResolution(resolutionID uint, userID string, notes string) error {
	var resolution models.DurationResolution
	if err := s.db.First(&resolution, resolutionID).Error; err != nil {
		return err
	}

	if resolution.Status == "rejected" {
		return fmt.Errorf("resolution is already rejected")
	}

	now := time.Now()

	if resolution.SuccessfulQueries > 0 {
		resolution.Status = "needs_review"
		resolution.ResolvedDuration = nil
		resolution.ReviewedAt = &now
		resolution.ReviewedBy = userID
		resolution.ReviewNotes = notes
		resolution.ReviewAction = "reject"
		resolution.ManuallyReviewed = true
		s.db.Save(&resolution)

		var track models.Track
		s.db.First(&track, resolution.TrackID)
		track.Duration = 0
		track.DurationSource = ""
		track.DurationResolvedAt = nil
		track.DurationNeedsReview = true
		s.db.Save(&track)
	} else {
		resolution.Status = "rejected"
		resolution.ResolvedDuration = nil
		resolution.ReviewedAt = &now
		resolution.ReviewedBy = userID
		resolution.ReviewNotes = notes
		resolution.ReviewAction = "reject"
		resolution.ManuallyReviewed = true
		s.db.Save(&resolution)

		var track models.Track
		s.db.First(&track, resolution.TrackID)
		track.Duration = 0
		track.DurationSource = ""
		track.DurationResolvedAt = nil
		track.DurationNeedsReview = false
		s.db.Save(&track)
	}

	return nil
}

func (s *DurationResolverService) ApplyResolution(resolutionID uint, duration int, notes string) error {
	var resolution models.DurationResolution
	if err := s.db.First(&resolution, resolutionID).Error; err != nil {
		return err
	}

	if resolution.Status != "needs_review" && resolution.Status != "resolved" {
		return fmt.Errorf("resolution cannot be applied in current status: %s", resolution.Status)
	}

	now := time.Now()
	resolution.Status = "approved"
	resolution.ResolvedDuration = &duration
	resolution.ReviewedAt = &now
	resolution.ReviewAction = "apply"
	resolution.ManuallyReviewed = true
	resolution.ReviewNotes = notes

	if err := s.db.Save(&resolution).Error; err != nil {
		return err
	}

	var track models.Track
	if err := s.db.First(&track, resolution.TrackID).Error; err != nil {
		return err
	}

	track.Duration = duration
	track.DurationSource = "manual"
	track.DurationResolvedAt = &now
	track.DurationNeedsReview = false

	return s.db.Save(&track).Error
}

func (s *DurationResolverService) ManuallySetDuration(trackID uint, duration int, notes string) error {
	var track models.Track
	if err := s.db.First(&track, trackID).Error; err != nil {
		return err
	}

	now := time.Now()
	track.Duration = duration
	track.DurationSource = "manual"
	track.DurationResolvedAt = &now
	track.DurationNeedsReview = false

	if err := s.db.Save(&track).Error; err != nil {
		return err
	}

	var resolution models.DurationResolution
	if err := s.db.Where("track_id = ?", trackID).First(&resolution).Error; err == nil {
		resolution.Status = "approved"
		resolution.ResolvedDuration = &duration
		resolution.ReviewedAt = &now
		resolution.ReviewAction = "manual"
		resolution.ManuallyReviewed = true
		resolution.ReviewNotes = notes
		s.db.Save(&resolution)
	} else {
		resolution = models.DurationResolution{
			TrackID:             trackID,
			AlbumID:             track.AlbumID,
			Status:              "approved",
			ResolvedDuration:    &duration,
			ReviewedAt:          &now,
			ReviewAction:        "manual",
			ManuallyReviewed:    true,
			ReviewNotes:         notes,
			TotalSourcesQueried: 0,
			SuccessfulQueries:   0,
		}
		s.db.Create(&resolution)
	}

	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func timePtr(t time.Time) *time.Time {
	return &t
}
