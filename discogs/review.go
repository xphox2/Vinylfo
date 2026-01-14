package discogs

import (
	"vinylfo/models"
)

type ReviewSeverity string

const (
	SeverityInfo     ReviewSeverity = "info"
	SeverityWarning  ReviewSeverity = "warning"
	SeverityConflict ReviewSeverity = "conflict"
)

type FieldChange struct {
	Field        string         `json:"field"`
	CurrentValue interface{}    `json:"current_value"`
	NewValue     interface{}    `json:"new_value"`
	Severity     ReviewSeverity `json:"severity"`
}

type AlbumReview struct {
	AlbumID      uint          `json:"album_id"`
	DiscogsID    int           `json:"discogs_id"`
	Title        string        `json:"title"`
	Artist       string        `json:"artist"`
	Changes      []FieldChange `json:"changes"`
	HasConflicts bool          `json:"has_conflicts"`
	CanAutoApply bool          `json:"can_auto_apply"`
	Summary      string        `json:"summary"`
}

type BatchReview struct {
	BatchID       int           `json:"batch_id"`
	TotalAlbums   int           `json:"total_albums"`
	NewAlbums     int           `json:"new_albums"`
	UpdatedAlbums int           `json:"updated_albums"`
	ConflictCount int           `json:"conflict_count"`
	Reviews       []AlbumReview `json:"reviews"`
}

type DataReviewService struct {
	autoApplySafe bool
}

func NewDataReviewService(autoApplySafe bool) *DataReviewService {
	return &DataReviewService{
		autoApplySafe: autoApplySafe,
	}
}

func (s *DataReviewService) ReviewAlbum(local *models.Album, discogsData map[string]interface{}) *AlbumReview {
	review := &AlbumReview{
		AlbumID: local.ID,
		Title:   local.Title,
		Artist:  local.Artist,
		Changes: make([]FieldChange, 0),
	}

	if discogsID, ok := discogsData["discogs_id"].(int); ok {
		review.DiscogsID = discogsID
	}

	changes := s.compareFields(local, discogsData)

	for _, change := range changes {
		review.Changes = append(review.Changes, change)
		if change.Severity == SeverityConflict {
			review.HasConflicts = true
		}
	}

	review.CanAutoApply = !review.HasConflicts || s.autoApplySafe
	review.Summary = s.generateSummary(review)

	return review
}

func (s *DataReviewService) compareFields(local *models.Album, discogsData map[string]interface{}) []FieldChange {
	changes := make([]FieldChange, 0)

	if year, ok := discogsData["year"].(int); ok && year != local.ReleaseYear {
		if year > 0 {
			changes = append(changes, FieldChange{
				Field:        "release_year",
				CurrentValue: local.ReleaseYear,
				NewValue:     year,
				Severity:     s.determineYearSeverity(local.ReleaseYear, year),
			})
		}
	}

	if genre, ok := discogsData["genre"].(string); ok && genre != "" && genre != local.Genre {
		changes = append(changes, FieldChange{
			Field:        "genre",
			CurrentValue: local.Genre,
			NewValue:     genre,
			Severity:     SeverityInfo,
		})
	}

	if coverImage, ok := discogsData["cover_image"].(string); ok && coverImage != "" && coverImage != local.CoverImageURL {
		changes = append(changes, FieldChange{
			Field:        "cover_image",
			CurrentValue: local.CoverImageURL,
			NewValue:     coverImage,
			Severity:     SeverityInfo,
		})
	}

	if styles, ok := discogsData["styles"].([]string); ok && len(styles) > 0 {
		changes = append(changes, FieldChange{
			Field:        "styles",
			CurrentValue: nil,
			NewValue:     styles,
			Severity:     SeverityInfo,
		})
	}

	return changes
}

func (s *DataReviewService) determineYearSeverity(current, new int) ReviewSeverity {
	if current == 0 {
		return SeverityInfo
	}

	diff := new - current
	if diff < 0 {
		diff = -diff
	}

	if diff <= 1 {
		return SeverityInfo
	}
	if diff <= 5 {
		return SeverityWarning
	}
	return SeverityConflict
}

func (s *DataReviewService) generateSummary(review *AlbumReview) string {
	if len(review.Changes) == 0 {
		return "No changes needed"
	}

	infoCount := 0
	warningCount := 0
	conflictCount := 0

	for _, change := range review.Changes {
		switch change.Severity {
		case SeverityInfo:
			infoCount++
		case SeverityWarning:
			warningCount++
		case SeverityConflict:
			conflictCount++
		}
	}

	parts := make([]string, 0)
	if infoCount > 0 {
		parts = append(parts, s.pluralize(infoCount, "new field"))
	}
	if warningCount > 0 {
		parts = append(parts, s.pluralize(warningCount, "minor update"))
	}
	if conflictCount > 0 {
		parts = append(parts, s.pluralize(conflictCount, "conflict"))
	}

	return s.joinParts(parts)
}

func (s *DataReviewService) pluralize(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	result := ""
	n := count
	for n > 0 {
		n = n / 10
		result += " "
	}
	return result + singular + "s"
}

func (s *DataReviewService) joinParts(parts []string) string {
	if len(parts) == 0 {
		return "No changes"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for i := 1; i < len(parts)-1; i++ {
		result += ", " + parts[i]
	}
	result += " and " + parts[len(parts)-1]
	return result
}

func (s *DataReviewService) ReviewBatch(localAlbums []models.Album, discogsReleases []map[string]interface{}) *BatchReview {
	review := &BatchReview{
		Reviews: make([]AlbumReview, 0),
	}

	albumMap := make(map[string]*models.Album)
	for i := range localAlbums {
		key := s.albumKey(&localAlbums[i])
		albumMap[key] = &localAlbums[i]
	}

	for i, release := range discogsReleases {
		title, _ := release["title"].(string)
		artist, _ := release["artist"].(string)

		albumReview := &AlbumReview{
			DiscogsID: i + 1,
			Title:     title,
			Artist:    artist,
			Changes:   make([]FieldChange, 0),
		}

		key := s.releaseKey(release)
		if local, exists := albumMap[key]; exists {
			albumReview.AlbumID = local.ID
			albumReview.Changes = s.compareFields(local, release)
			albumReview.HasConflicts = s.hasConflicts(albumReview.Changes)
			albumReview.CanAutoApply = !albumReview.HasConflicts || s.autoApplySafe
			albumReview.Summary = s.generateSummary(albumReview)
			review.UpdatedAlbums++
		} else {
			albumReview.Summary = "New album"
			albumReview.CanAutoApply = true
			albumReview.Changes = s.getNewFields(release)
			review.NewAlbums++
		}

		if albumReview.HasConflicts {
			review.ConflictCount++
		}

		review.Reviews = append(review.Reviews, *albumReview)
	}

	review.TotalAlbums = len(discogsReleases)

	return review
}

func (s *DataReviewService) albumKey(album *models.Album) string {
	return album.Title + "|" + album.Artist
}

func (s *DataReviewService) releaseKey(release map[string]interface{}) string {
	title, _ := release["title"].(string)
	artist, _ := release["artist"].(string)
	return title + "|" + artist
}

func (s *DataReviewService) hasConflicts(changes []FieldChange) bool {
	for _, change := range changes {
		if change.Severity == SeverityConflict {
			return true
		}
	}
	return false
}

func (s *DataReviewService) getNewFields(release map[string]interface{}) []FieldChange {
	changes := make([]FieldChange, 0)

	if year, ok := release["year"].(int); ok && year > 0 {
		changes = append(changes, FieldChange{
			Field:        "release_year",
			CurrentValue: nil,
			NewValue:     year,
			Severity:     SeverityInfo,
		})
	}

	if genre, ok := release["genre"].(string); ok && genre != "" {
		changes = append(changes, FieldChange{
			Field:        "genre",
			CurrentValue: nil,
			NewValue:     genre,
			Severity:     SeverityInfo,
		})
	}

	if coverImage, ok := release["cover_image"].(string); ok && coverImage != "" {
		changes = append(changes, FieldChange{
			Field:        "cover_image",
			CurrentValue: nil,
			NewValue:     coverImage,
			Severity:     SeverityInfo,
		})
	}

	return changes
}

func (s *DataReviewService) ShouldAutoApply(review *AlbumReview) bool {
	return review.CanAutoApply
}
