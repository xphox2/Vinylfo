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

// YouTubeSyncService handles matching tracks to YouTube videos and syncing playlists
type YouTubeSyncService struct {
	db          *gorm.DB
	matcher     *YouTubeMatcher
	webSearcher *YouTubeWebSearcher
	oauthClient *duration.YouTubeOAuthClient
	apiClient   *duration.YouTubeClient
	httpClient  *http.Client
}

// NewYouTubeSyncService creates a new sync service
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

// WebSearcher returns the web searcher instance
func (s *YouTubeSyncService) WebSearcher() *YouTubeWebSearcher {
	return s.webSearcher
}

// MatchResult represents the result of matching a track to YouTube
type MatchResult struct {
	TrackID     uint                           `json:"track_id"`
	TrackTitle  string                         `json:"track_title"`
	Artist      string                         `json:"artist"`
	BestMatch   *models.TrackYouTubeMatch      `json:"best_match,omitempty"`
	Candidates  []models.TrackYouTubeCandidate `json:"candidates,omitempty"`
	NeedsReview bool                           `json:"needs_review"`
	MatchMethod string                         `json:"match_method"` // web_search, api_search, none
	Error       string                         `json:"error,omitempty"`
}

// ScoredCandidate holds a candidate with its calculated score
type ScoredCandidate struct {
	VideoID      string
	Title        string
	ChannelName  string
	Duration     int
	ThumbnailURL string
	ViewCount    int64
	Score        YouTubeMatchScore
	Source       string // web_search or api_search
}

// MatchTrack finds the best YouTube video match for a single track
// If force is true, will re-search even if track already has a match
// If useApiFallback is true, will use YouTube API if web search doesn't find good matches
func (s *YouTubeSyncService) MatchTrack(ctx context.Context, trackID uint, force bool, useApiFallback bool) (*MatchResult, error) {
	// Load track with album info
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

	// Check if already matched (skip if not forcing re-match)
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

	// Delete existing match if force=true
	if force {
		s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeMatch{})
		s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})
		log.Printf("Force re-matching track %d: %s - %s", trackID, track.Title, album.Artist)
	}

	// Collect candidates from all sources
	var allCandidates []ScoredCandidate

	// Step 1: Try web search first (no API quota)
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

	// Step 2: Evaluate web search results
	if len(allCandidates) > 0 {
		sort.Slice(allCandidates, func(i, j int) bool {
			return allCandidates[i].Score.Composite > allCandidates[j].Score.Composite
		})

		best := allCandidates[0]

		if s.matcher.IsAutoMatch(best.Score) {
			// Auto-match: high confidence
			match := s.createMatch(&track, best, false)
			if err := s.saveMatch(match); err != nil {
				return nil, fmt.Errorf("failed to save match: %w", err)
			}
			result.BestMatch = match
			result.MatchMethod = best.Source
			return result, nil
		} else if s.matcher.IsAcceptableMatch(best.Score) {
			// Needs review: acceptable but not confident
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

	// Step 3: Fallback to YouTube API (only if enabled)
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

	// Re-evaluate with API results
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

	// No acceptable match found
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

// searchViaWeb performs web search and scores results
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

// searchViaAPI performs YouTube API search and scores results
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

// fetchVideoMetadata fetches video metadata using noembed (works without web search)
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

// createMatch creates a TrackYouTubeMatch from a scored candidate
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

// createCandidates creates TrackYouTubeCandidates from scored candidates
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

// saveMatch saves or updates a match in the database
func (s *YouTubeSyncService) saveMatch(match *models.TrackYouTubeMatch) error {
	var existing models.TrackYouTubeMatch
	if err := s.db.Where("track_id = ?", match.TrackID).First(&existing).Error; err == nil {
		match.ID = existing.ID
		return s.db.Save(match).Error
	}
	return s.db.Create(match).Error
}

// saveCandidates saves candidates, replacing any existing ones for the track
func (s *YouTubeSyncService) saveCandidates(candidates []models.TrackYouTubeCandidate) error {
	if len(candidates) == 0 {
		return nil
	}

	trackID := candidates[0].TrackID

	// Delete existing candidates
	if err := s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{}).Error; err != nil {
		return err
	}

	// Insert new candidates
	return s.db.Create(&candidates).Error
}

// MatchPlaylistResult represents the result of matching all tracks in a playlist
type MatchPlaylistResult struct {
	PlaylistID  string        `json:"playlist_id"`
	TotalTracks int           `json:"total_tracks"`
	Matched     int           `json:"matched"`
	NeedsReview int           `json:"needs_review"`
	Unavailable int           `json:"unavailable"`
	Errors      int           `json:"errors"`
	Tracks      []MatchResult `json:"tracks"`
}

// MatchPlaylist matches all tracks in a playlist to YouTube videos
// If force is true, will re-search for all tracks
// If useApiFallback is true, will use YouTube API if web search doesn't find good matches
func (s *YouTubeSyncService) MatchPlaylist(ctx context.Context, playlistID string, force bool, useApiFallback bool) (*MatchPlaylistResult, error) {
	// Get all tracks in the playlist
	var playlistTracks []models.SessionPlaylist
	if err := s.db.Where("session_id = ?", playlistID).Order("`order` ASC").Find(&playlistTracks).Error; err != nil {
		return nil, fmt.Errorf("failed to get playlist tracks: %w", err)
	}

	result := &MatchPlaylistResult{
		PlaylistID:  playlistID,
		TotalTracks: len(playlistTracks),
	}

	for _, pt := range playlistTracks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		matchResult, err := s.MatchTrack(ctx, pt.TrackID, force, useApiFallback)
		if err != nil {
			result.Errors++
			result.Tracks = append(result.Tracks, MatchResult{
				TrackID: pt.TrackID,
				Error:   err.Error(),
			})
			continue
		}

		result.Tracks = append(result.Tracks, *matchResult)

		if matchResult.BestMatch != nil {
			switch matchResult.BestMatch.Status {
			case "matched", "reviewed":
				result.Matched++
			case "needs_review":
				result.NeedsReview++
			case "unavailable":
				result.Unavailable++
			}
		}
	}

	return result, nil
}

// SyncPlaylistRequest contains parameters for syncing a playlist to YouTube
type SyncPlaylistRequest struct {
	PlaylistID         string `json:"playlist_id"`          // Local playlist ID
	YouTubePlaylistID  string `json:"youtube_playlist_id"`  // Existing YT playlist or empty for new
	PlaylistName       string `json:"playlist_name"`        // Name for new playlist
	IncludeNeedsReview bool   `json:"include_needs_review"` // Include tracks needing review
}

// SyncPlaylistResult represents the result of syncing to YouTube
type SyncPlaylistResult struct {
	YouTubePlaylistID string   `json:"youtube_playlist_id"`
	SyncedCount       int      `json:"synced_count"`
	SkippedCount      int      `json:"skipped_count"`
	ErrorCount        int      `json:"error_count"`
	Errors            []string `json:"errors,omitempty"`
}

// SyncPlaylistToYouTube syncs matched tracks from a local playlist to a YouTube playlist
func (s *YouTubeSyncService) SyncPlaylistToYouTube(ctx context.Context, req SyncPlaylistRequest) (*SyncPlaylistResult, error) {
	if !s.oauthClient.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated with YouTube")
	}

	result := &SyncPlaylistResult{}

	// Create new playlist if needed
	youtubePlaylistID := req.YouTubePlaylistID
	if youtubePlaylistID == "" {
		name := req.PlaylistName
		if name == "" {
			name = "Vinylfo Playlist"
		}
		playlist, err := s.oauthClient.CreatePlaylist(ctx, name, "Created by Vinylfo", "private")
		if err != nil {
			return nil, fmt.Errorf("failed to create YouTube playlist: %w", err)
		}
		youtubePlaylistID = playlist.ID
	}
	result.YouTubePlaylistID = youtubePlaylistID

	// Get all tracks in the playlist with their matches
	var playlistTracks []models.SessionPlaylist
	if err := s.db.Where("session_id = ?", req.PlaylistID).Order("`order` ASC").Find(&playlistTracks).Error; err != nil {
		return nil, fmt.Errorf("failed to get playlist tracks: %w", err)
	}

	position := 0
	for _, pt := range playlistTracks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Get the match for this track
		var match models.TrackYouTubeMatch
		if err := s.db.Where("track_id = ?", pt.TrackID).First(&match).Error; err != nil {
			result.SkippedCount++
			continue
		}

		// Skip based on status
		if match.Status == "unavailable" {
			result.SkippedCount++
			continue
		}
		if match.Status == "needs_review" && !req.IncludeNeedsReview {
			result.SkippedCount++
			continue
		}
		if match.YouTubeVideoID == "" {
			result.SkippedCount++
			continue
		}

		// Add video to YouTube playlist
		if err := s.oauthClient.AddVideoToPlaylist(ctx, youtubePlaylistID, match.YouTubeVideoID, position); err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Track %d: %v", pt.TrackID, err))
			position++
			continue
		}

		result.SyncedCount++
		position++
	}

	// Update the PlaybackSession with YouTube playlist info
	now := time.Now()
	var session models.PlaybackSession
	if err := s.db.Where("playlist_id = ?", req.PlaylistID).First(&session).Error; err == nil {
		session.YouTubePlaylistID = youtubePlaylistID
		if req.PlaylistName != "" {
			session.YouTubePlaylistName = req.PlaylistName
		} else {
			session.YouTubePlaylistName = session.PlaylistName
		}
		session.YouTubeSyncedAt = &now
		s.db.Save(&session)
	}

	return result, nil
}

// GetPlaylistSyncStatus returns the current sync status for a playlist
func (s *YouTubeSyncService) GetPlaylistSyncStatus(playlistID string) (*PlaylistSyncStatus, error) {
	// Get playlist info
	var session models.PlaybackSession
	if err := s.db.Where("playlist_id = ?", playlistID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("playlist not found: %w", err)
	}

	// Count tracks
	var totalTracks int64
	s.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Count(&totalTracks)

	// Get track IDs
	var trackIDs []uint
	s.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Pluck("track_id", &trackIDs)

	// Count matches by status
	var matched, needsReview, unavailable int64
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "matched").Count(&matched)
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "reviewed").Count(&matched) // Add reviewed to matched
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "needs_review").Count(&needsReview)
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "unavailable").Count(&unavailable)

	pending := int(totalTracks) - int(matched) - int(needsReview) - int(unavailable)
	if pending < 0 {
		pending = 0
	}

	var progress float64
	if totalTracks > 0 {
		progress = float64(matched+needsReview+unavailable) / float64(totalTracks)
	}

	readyToSync := pending == 0 && needsReview == 0

	return &PlaylistSyncStatus{
		PlaylistID:          playlistID,
		PlaylistName:        session.PlaylistName,
		TotalTracks:         int(totalTracks),
		Matched:             int(matched),
		NeedsReview:         int(needsReview),
		Unavailable:         int(unavailable),
		Pending:             pending,
		MatchProgress:       progress,
		ReadyToSync:         readyToSync,
		YouTubePlaylistID:   session.YouTubePlaylistID,
		YouTubePlaylistName: session.YouTubePlaylistName,
		LastSyncedAt:        session.YouTubeSyncedAt,
	}, nil
}

// PlaylistSyncStatus represents the sync status of a playlist
type PlaylistSyncStatus struct {
	PlaylistID          string     `json:"playlist_id"`
	PlaylistName        string     `json:"playlist_name"`
	TotalTracks         int        `json:"total_tracks"`
	Matched             int        `json:"matched"`
	NeedsReview         int        `json:"needs_review"`
	Unavailable         int        `json:"unavailable"`
	Pending             int        `json:"pending"`
	MatchProgress       float64    `json:"match_progress"`
	ReadyToSync         bool       `json:"ready_to_sync"`
	YouTubePlaylistID   string     `json:"youtube_playlist_id,omitempty"`
	YouTubePlaylistName string     `json:"youtube_playlist_name,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	LastSyncedCount     int        `json:"last_synced_count,omitempty"`
}

// SelectCandidate selects a candidate as the match for a track
func (s *YouTubeSyncService) SelectCandidate(trackID, candidateID uint) (*models.TrackYouTubeMatch, error) {
	var candidate models.TrackYouTubeCandidate
	if err := s.db.First(&candidate, candidateID).Error; err != nil {
		return nil, fmt.Errorf("candidate not found: %w", err)
	}

	if candidate.TrackID != trackID {
		return nil, fmt.Errorf("candidate does not belong to track")
	}

	now := time.Now()
	match := &models.TrackYouTubeMatch{
		TrackID:        trackID,
		YouTubeVideoID: candidate.YouTubeVideoID,
		VideoTitle:     candidate.VideoTitle,
		VideoDuration:  candidate.VideoDuration,
		ChannelName:    candidate.ChannelName,
		ThumbnailURL:   candidate.ThumbnailURL,
		ViewCount:      candidate.ViewCount,
		MatchScore:     candidate.MatchScore,
		TitleScore:     candidate.TitleScore,
		ArtistScore:    candidate.ArtistScore,
		DurationScore:  candidate.DurationScore,
		ChannelScore:   candidate.ChannelScore,
		MatchMethod:    "manual",
		NeedsReview:    false,
		Status:         "reviewed",
		ReviewedAt:     &now,
	}

	if err := s.saveMatch(match); err != nil {
		return nil, fmt.Errorf("failed to save match: %w", err)
	}

	// Clear candidates
	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return match, nil
}

// SetManualMatch sets a manual YouTube video match for a track
func (s *YouTubeSyncService) SetManualMatch(ctx context.Context, trackID uint, videoID string) (*models.TrackYouTubeMatch, error) {
	// Validate video ID
	if !IsValidVideoID(videoID) {
		return nil, fmt.Errorf("invalid YouTube video ID")
	}

	// Fetch video metadata
	var metadata *VideoMetadata
	var err error
	if s.webSearcher != nil {
		metadata, err = s.webSearcher.FetchVideoMetadataWithDuration(ctx, videoID)
		if err != nil {
			metadata, err = s.webSearcher.FetchVideoMetadata(ctx, videoID)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video metadata: %w", err)
	}

	// Get track for scoring
	var track models.Track
	if err := s.db.First(&track, trackID).Error; err != nil {
		return nil, fmt.Errorf("track not found: %w", err)
	}

	var album models.Album
	if err := s.db.First(&album, track.AlbumID).Error; err != nil {
		return nil, fmt.Errorf("album not found: %w", err)
	}

	// Calculate score for the manual match
	score := s.matcher.CalculateScore(
		track.Title, album.Artist, track.Duration,
		metadata.Title, metadata.ChannelName, metadata.Duration,
	)

	now := time.Now()
	match := &models.TrackYouTubeMatch{
		TrackID:        trackID,
		YouTubeVideoID: videoID,
		VideoTitle:     metadata.Title,
		VideoDuration:  metadata.Duration,
		ChannelName:    metadata.ChannelName,
		ThumbnailURL:   metadata.ThumbnailURL,
		MatchScore:     score.Composite,
		TitleScore:     score.Title,
		ArtistScore:    score.Artist,
		DurationScore:  score.Duration,
		ChannelScore:   score.Channel,
		MatchMethod:    "manual",
		NeedsReview:    false,
		Status:         "reviewed",
		ReviewedAt:     &now,
	}

	if err := s.saveMatch(match); err != nil {
		return nil, fmt.Errorf("failed to save match: %w", err)
	}

	// Clear any candidates
	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return match, nil
}

// MarkUnavailable marks a track as having no available YouTube match
func (s *YouTubeSyncService) MarkUnavailable(trackID uint) error {
	now := time.Now()
	match := &models.TrackYouTubeMatch{
		TrackID:     trackID,
		Status:      "unavailable",
		MatchMethod: "manual",
		ReviewedAt:  &now,
	}

	if err := s.saveMatch(match); err != nil {
		return fmt.Errorf("failed to mark unavailable: %w", err)
	}

	// Clear any candidates
	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return nil
}

// GetCandidates returns all candidates for a track
func (s *YouTubeSyncService) GetCandidates(trackID uint) ([]models.TrackYouTubeCandidate, error) {
	var candidates []models.TrackYouTubeCandidate
	if err := s.db.Where("track_id = ?", trackID).Order("rank").Find(&candidates).Error; err != nil {
		return nil, err
	}
	return candidates, nil
}

// GetMatch returns the current match for a track
func (s *YouTubeSyncService) GetMatch(trackID uint) (*models.TrackYouTubeMatch, error) {
	var match models.TrackYouTubeMatch
	if err := s.db.Where("track_id = ?", trackID).First(&match).Error; err != nil {
		return nil, err
	}
	return &match, nil
}
