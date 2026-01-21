package services

import (
	"context"
	"fmt"
	"time"

	"vinylfo/models"
)

type MatchPlaylistResult struct {
	PlaylistID  string        `json:"playlist_id"`
	TotalTracks int           `json:"total_tracks"`
	Matched     int           `json:"matched"`
	NeedsReview int           `json:"needs_review"`
	Unavailable int           `json:"unavailable"`
	Errors      int           `json:"errors"`
	Tracks      []MatchResult `json:"tracks"`
}

func (s *YouTubeSyncService) MatchPlaylist(ctx context.Context, playlistID string, force bool, useApiFallback bool) (*MatchPlaylistResult, error) {
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

type SyncPlaylistRequest struct {
	PlaylistID         string `json:"playlist_id"`
	YouTubePlaylistID  string `json:"youtube_playlist_id"`
	PlaylistName       string `json:"playlist_name"`
	IncludeNeedsReview bool   `json:"include_needs_review"`
}

type SyncPlaylistResult struct {
	YouTubePlaylistID string   `json:"youtube_playlist_id"`
	SyncedCount       int      `json:"synced_count"`
	SkippedCount      int      `json:"skipped_count"`
	ErrorCount        int      `json:"error_count"`
	Errors            []string `json:"errors,omitempty"`
}

func (s *YouTubeSyncService) SyncPlaylistToYouTube(ctx context.Context, req SyncPlaylistRequest) (*SyncPlaylistResult, error) {
	if !s.oauthClient.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated with YouTube")
	}

	result := &SyncPlaylistResult{}

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

		var match models.TrackYouTubeMatch
		if err := s.db.Where("track_id = ?", pt.TrackID).First(&match).Error; err != nil {
			result.SkippedCount++
			continue
		}

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

		if err := s.oauthClient.AddVideoToPlaylist(ctx, youtubePlaylistID, match.YouTubeVideoID, position); err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Track %d: %v", pt.TrackID, err))
			position++
			continue
		}

		result.SyncedCount++
		position++
	}

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

func (s *YouTubeSyncService) GetPlaylistSyncStatus(playlistID string) (*PlaylistSyncStatus, error) {
	var session models.PlaybackSession
	if err := s.db.Where("playlist_id = ?", playlistID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("playlist not found: %w", err)
	}

	var totalTracks int64
	s.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Count(&totalTracks)

	var trackIDs []uint
	s.db.Model(&models.SessionPlaylist{}).Where("session_id = ?", playlistID).Pluck("track_id", &trackIDs)

	var matched, needsReview, unavailable int64
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "matched").Count(&matched)
	s.db.Model(&models.TrackYouTubeMatch{}).Where("track_id IN ? AND status = ?", trackIDs, "reviewed").Count(&matched)
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

	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return match, nil
}

func (s *YouTubeSyncService) SetManualMatch(ctx context.Context, trackID uint, videoID string) (*models.TrackYouTubeMatch, error) {
	if !IsValidVideoID(videoID) {
		return nil, fmt.Errorf("invalid YouTube video ID")
	}

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

	var track models.Track
	if err := s.db.First(&track, trackID).Error; err != nil {
		return nil, fmt.Errorf("track not found: %w", err)
	}

	var album models.Album
	if err := s.db.First(&album, track.AlbumID).Error; err != nil {
		return nil, fmt.Errorf("album not found: %w", err)
	}

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

	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return match, nil
}

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

	s.db.Where("track_id = ?", trackID).Delete(&models.TrackYouTubeCandidate{})

	return nil
}

func (s *YouTubeSyncService) GetCandidates(trackID uint) ([]models.TrackYouTubeCandidate, error) {
	var candidates []models.TrackYouTubeCandidate
	if err := s.db.Where("track_id = ?", trackID).Order("rank").Find(&candidates).Error; err != nil {
		return nil, err
	}
	return candidates, nil
}

func (s *YouTubeSyncService) GetMatch(trackID uint) (*models.TrackYouTubeMatch, error) {
	var match models.TrackYouTubeMatch
	if err := s.db.Where("track_id = ?", trackID).First(&match).Error; err != nil {
		return nil, err
	}
	return &match, nil
}
