package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"vinylfo/duration"
	"vinylfo/models"

	"gorm.io/gorm"
)

type YouTubeSyncService struct {
	db          *gorm.DB
	matcher     *YouTubeMatcher
	webSearcher *YouTubeWebSearcher
	oauthClient *duration.YouTubeOAuthClient
	apiClient   *duration.YouTubeClient
	httpClient  *http.Client
}

func NewYouTubeSyncService(db *gorm.DB) (*YouTubeSyncService, error) {
	webSearcher, err := NewYouTubeWebSearcher()
	if err != nil {
		log.Printf("Warning: Web search disabled: %v", err)
		webSearcher = nil
	} else {
		log.Println("Web search initialized successfully")
	}

	return &YouTubeSyncService{
		db:          db,
		matcher:     NewYouTubeMatcher(),
		webSearcher: webSearcher,
		oauthClient: duration.NewYouTubeOAuthClient(db),
		apiClient:   duration.NewYouTubeClient(""),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (s *YouTubeSyncService) WebSearcher() *YouTubeWebSearcher {
	return s.webSearcher
}

type MatchResult struct {
	TrackID     uint                           `json:"track_id"`
	TrackTitle  string                         `json:"track_title"`
	Artist      string                         `json:"artist"`
	BestMatch   *models.TrackYouTubeMatch      `json:"best_match,omitempty"`
	Candidates  []models.TrackYouTubeCandidate `json:"candidates,omitempty"`
	NeedsReview bool                           `json:"needs_review"`
	MatchMethod string                         `json:"match_method"`
	Error       string                         `json:"error,omitempty"`
}

type ScoredCandidate struct {
	VideoID      string
	Title        string
	ChannelName  string
	Duration     int
	ThumbnailURL string
	ViewCount    int64
	Score        YouTubeMatchScore
	Source       string
}

func (s *YouTubeSyncService) MatchTrack(ctx context.Context, trackID uint, force bool, useApiFallback bool) (*MatchResult, error) {
	var track models.Track
	if err := s.db.First(&track, trackID).Error; err != nil {
		return nil, fmt.Errorf("track not found: %w", err)
	}

	var album models.Album
	if err := s.db.First(&album, track.AlbumID).Error; err != nil {
		return nil, fmt.Errorf("album not found: %w", err)
	}

	result := &MatchResult{
		TrackID:    track.ID,
		TrackTitle: track.Title,
		Artist:     album.Artist,
	}

	if !force {
		var existingMatch models.TrackYouTubeMatch
		if err := s.db.Where("track_id = ?", trackID).First(&existingMatch).Error; err == nil {
			if existingMatch.Status == "matched" || existingMatch.Status == "reviewed" {
				result.BestMatch = &existingMatch
				result.MatchMethod = existingMatch.MatchMethod
				log.Printf("Track %d already matched (score: %.2f), skipping", trackID, existingMatch.MatchScore)
				return result, nil
			}
		}
	}

	if force {
		s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeMatch{})
		s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})
		log.Printf("Force re-matching track %d: %s - %s", trackID, track.Title, album.Artist)
	}

	var allCandidates []ScoredCandidate

	if s.webSearcher != nil {
		log.Printf("Attempting web search for track %d: %s - %s", trackID, track.Title, album.Artist)
		webCandidates, err := s.searchViaWeb(ctx, track.Title, album.Artist, track.Duration)
		if err != nil {
			log.Printf("Web search failed for track %d: %v", trackID, err)
		} else {
			log.Printf("Web search found %d candidates for track %d", len(webCandidates), trackID)
			allCandidates = append(allCandidates, webCandidates...)
		}
	} else {
		log.Printf("Web search not available for track %d", trackID)
	}

	if len(allCandidates) > 0 {
		sort.Slice(allCandidates, func(i, j int) bool {
			return allCandidates[i].Score.Composite > allCandidates[j].Score.Composite
		})

		best := allCandidates[0]

		if s.matcher.IsAutoMatch(best.Score) {
			match := s.createMatch(&track, best, false)
			if err := s.saveMatch(match); err != nil {
				return nil, fmt.Errorf("failed to save match: %w", err)
			}
			result.BestMatch = match
			result.MatchMethod = best.Source
			return result, nil
		} else if s.matcher.IsAcceptableMatch(best.Score) {
			match := s.createMatch(&track, best, true)
			if err := s.saveMatch(match); err != nil {
				return nil, fmt.Errorf("failed to save match: %w", err)
			}
			candidates := s.createCandidates(trackID, allCandidates)
			if err := s.saveCandidates(candidates); err != nil {
				log.Printf("Warning: failed to save candidates: %v", err)
			}
			result.BestMatch = match
			result.Candidates = candidates
			result.NeedsReview = true
			result.MatchMethod = best.Source
			return result, nil
		}
	}

	if useApiFallback && s.oauthClient.IsAuthenticated() {
		log.Printf("Using YouTube API as fallback for track %d", trackID)
		apiCandidates, err := s.searchViaAPI(ctx, track.Title, album.Artist, album.Title, track.Duration)
		if err != nil {
			log.Printf("API search failed for track %d: %v", trackID, err)
		} else {
			allCandidates = append(allCandidates, apiCandidates...)
		}
	} else if s.oauthClient.IsAuthenticated() {
		log.Printf("YouTube API fallback disabled, skipping for track %d", trackID)
	}

	if len(allCandidates) > 0 {
		sort.Slice(allCandidates, func(i, j int) bool {
			return allCandidates[i].Score.Composite > allCandidates[j].Score.Composite
		})

		best := allCandidates[0]

		if s.matcher.IsAutoMatch(best.Score) {
			match := s.createMatch(&track, best, false)
			if err := s.saveMatch(match); err != nil {
				return nil, fmt.Errorf("failed to save match: %w", err)
			}
			result.BestMatch = match
			result.MatchMethod = best.Source
			return result, nil
		} else if s.matcher.IsAcceptableMatch(best.Score) {
			match := s.createMatch(&track, best, true)
			if err := s.saveMatch(match); err != nil {
				return nil, fmt.Errorf("failed to save match: %w", err)
			}
			candidates := s.createCandidates(trackID, allCandidates)
			if err := s.saveCandidates(candidates); err != nil {
				log.Printf("Warning: failed to save candidates: %v", err)
			}
			result.BestMatch = match
			result.Candidates = candidates
			result.NeedsReview = true
			result.MatchMethod = best.Source
			return result, nil
		}
	}

	match := &models.TrackYouTubeMatch{
		TrackID:     trackID,
		Status:      "unavailable",
		MatchMethod: "none",
	}
	if err := s.saveMatch(match); err != nil {
		return nil, fmt.Errorf("failed to save unavailable status: %w", err)
	}
	result.BestMatch = match
	result.MatchMethod = "none"

	return result, nil
}

func (s *YouTubeSyncService) searchViaWeb(ctx context.Context, title, artist string, expectedDuration int) ([]ScoredCandidate, error) {
	if s.webSearcher == nil {
		return nil, fmt.Errorf("web searcher not available")
	}

	log.Printf("Searching web for: %s - %s", title, artist)
	results, err := s.webSearcher.SearchForTrack(ctx, title, artist)
	if err != nil {
		log.Printf("Web search error: %v", err)
		return nil, err
	}

	log.Printf("Web search returned %d results", len(results))

	var candidates []ScoredCandidate
	for _, r := range results {
		if r.Metadata == nil {
			continue
		}

		score := s.matcher.CalculateScore(
			title, artist, expectedDuration,
			r.Metadata.Title, r.Metadata.ChannelName, r.Metadata.Duration,
		)

		log.Printf("  Candidate: %s (score: %.2f)", r.Metadata.Title, score.Composite)

		candidates = append(candidates, ScoredCandidate{
			VideoID:      r.VideoID,
			Title:        r.Metadata.Title,
			ChannelName:  r.Metadata.ChannelName,
			Duration:     r.Metadata.Duration,
			ThumbnailURL: r.Metadata.ThumbnailURL,
			Score:        score,
			Source:       "web_search",
		})
	}

	return candidates, nil
}

func (s *YouTubeSyncService) searchViaAPI(ctx context.Context, title, artist, album string, expectedDuration int) ([]ScoredCandidate, error) {
	query := fmt.Sprintf("%s %s", title, artist)
	searchResp, err := s.oauthClient.SearchVideos(ctx, query, 10)
	if err != nil {
		return nil, err
	}

	var candidates []ScoredCandidate
	for _, item := range searchResp.Items {
		if item.ID.VideoID == "" {
			continue
		}

		metadata, err := s.fetchVideoMetadata(ctx, item.ID.VideoID)
		if err != nil {
			log.Printf("Failed to fetch metadata for %s: %v", item.ID.VideoID, err)
			metadata = &VideoMetadata{
				VideoID:     item.ID.VideoID,
				Title:       item.Snippet.Title,
				ChannelName: item.Snippet.ChannelTitle,
				Duration:    0,
			}
		}

		score := s.matcher.CalculateScore(
			title, artist, expectedDuration,
			metadata.Title, metadata.ChannelName, metadata.Duration,
		)

		var thumbnailURL string
		if thumb, ok := item.Snippet.Thumbnails["medium"]; ok {
			thumbnailURL = thumb.URL
		} else if thumb, ok := item.Snippet.Thumbnails["default"]; ok {
			thumbnailURL = thumb.URL
		}

		candidates = append(candidates, ScoredCandidate{
			VideoID:      item.ID.VideoID,
			Title:        item.Snippet.Title,
			ChannelName:  item.Snippet.ChannelTitle,
			Duration:     metadata.Duration,
			ThumbnailURL: thumbnailURL,
			Score:        score,
			Source:       "api_search",
		})
	}

	return candidates, nil
}

func (s *YouTubeSyncService) fetchVideoMetadata(ctx context.Context, videoID string) (*VideoMetadata, error) {
	url := fmt.Sprintf("https://noembed.com/embed?url=https://www.youtube.com/watch?v=%s", videoID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("noembed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var noembedResp struct {
		Title        string `json:"title"`
		AuthorName   string `json:"author_name"`
		AuthorURL    string `json:"author_url"`
		ThumbnailURL string `json:"thumbnail_url"`
		Duration     int    `json:"duration"`
		Error        string `json:"error"`
	}

	if err := json.Unmarshal(body, &noembedResp); err != nil {
		return nil, err
	}

	if noembedResp.Error != "" {
		return nil, fmt.Errorf("noembed error: %s", noembedResp.Error)
	}

	return &VideoMetadata{
		VideoID:      videoID,
		Title:        noembedResp.Title,
		ChannelName:  noembedResp.AuthorName,
		ChannelURL:   noembedResp.AuthorURL,
		ThumbnailURL: noembedResp.ThumbnailURL,
		Duration:     noembedResp.Duration,
	}, nil
}

func (s *YouTubeSyncService) createMatch(track *models.Track, candidate ScoredCandidate, needsReview bool) *models.TrackYouTubeMatch {
	now := time.Now()
	status := "matched"
	if needsReview {
		status = "needs_review"
	}

	return &models.TrackYouTubeMatch{
		TrackID:        track.ID,
		YouTubeVideoID: candidate.VideoID,
		VideoTitle:     candidate.Title,
		VideoDuration:  candidate.Duration,
		ChannelName:    candidate.ChannelName,
		ThumbnailURL:   candidate.ThumbnailURL,
		ViewCount:      candidate.ViewCount,
		MatchScore:     candidate.Score.Composite,
		TitleScore:     candidate.Score.Title,
		ArtistScore:    candidate.Score.Artist,
		DurationScore:  candidate.Score.Duration,
		ChannelScore:   candidate.Score.Channel,
		MatchMethod:    candidate.Source,
		NeedsReview:    needsReview,
		Status:         status,
		MatchedAt:      &now,
	}
}

func (s *YouTubeSyncService) createCandidates(trackID uint, candidates []ScoredCandidate) []models.TrackYouTubeCandidate {
	var result []models.TrackYouTubeCandidate
	maxCandidates := 5
	if len(candidates) < maxCandidates {
		maxCandidates = len(candidates)
	}

	for i := 0; i < maxCandidates; i++ {
		c := candidates[i]
		result = append(result, models.TrackYouTubeCandidate{
			TrackID:        trackID,
			YouTubeVideoID: c.VideoID,
			VideoTitle:     c.Title,
			VideoDuration:  c.Duration,
			ChannelName:    c.ChannelName,
			ThumbnailURL:   c.ThumbnailURL,
			ViewCount:      c.ViewCount,
			MatchScore:     c.Score.Composite,
			TitleScore:     c.Score.Title,
			ArtistScore:    c.Score.Artist,
			DurationScore:  c.Score.Duration,
			ChannelScore:   c.Score.Channel,
			Rank:           i + 1,
			SourceMethod:   c.Source,
		})
	}

	return result
}

func (s *YouTubeSyncService) saveMatch(match *models.TrackYouTubeMatch) error {
	var existing models.TrackYouTubeMatch
	if err := s.db.Where("track_id = ?", match.TrackID).First(&existing).Error; err == nil {
		match.ID = existing.ID
		return s.db.Save(match).Error
	}
	return s.db.Create(match).Error
}

func (s *YouTubeSyncService) saveCandidates(candidates []models.TrackYouTubeCandidate) error {
	if len(candidates) == 0 {
		return nil
	}

	trackID := candidates[0].TrackID

	if err := s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{}).Error; err != nil {
		return err
	}

	return s.db.Create(&candidates).Error
}
