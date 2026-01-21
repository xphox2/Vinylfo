package services

import (
	"math"
	"regexp"
	"strings"

	"vinylfo/duration"
)

// YouTubeMatchScore holds the breakdown of all scoring components
type YouTubeMatchScore struct {
	Composite float64 `json:"composite"` // Final weighted score
	Title     float64 `json:"title"`     // Title similarity (0.0-1.0)
	Artist    float64 `json:"artist"`    // Artist similarity (0.0-1.0)
	Duration  float64 `json:"duration"`  // Duration proximity (0.0-1.0)
	Channel   float64 `json:"channel"`   // Channel name match (0.0-1.0)
}

// YouTubeMatchConfig holds configurable thresholds for matching
type YouTubeMatchConfig struct {
	// Score thresholds
	AutoMatchThreshold float64 // Score >= this auto-matches (default: 0.85)
	MinMatchThreshold  float64 // Score >= this is acceptable (default: 0.6)

	// Scoring weights (must sum to 1.0)
	TitleWeight    float64 // Weight for title similarity (default: 0.40)
	ArtistWeight   float64 // Weight for artist similarity (default: 0.30)
	DurationWeight float64 // Weight for duration proximity (default: 0.20)
	ChannelWeight  float64 // Weight for channel match (default: 0.10)

	// Duration scoring tolerances (in seconds)
	DurationPerfect    int // Perfect match threshold (default: 3)
	DurationExcellent  int // Excellent match threshold (default: 10)
	DurationGood       int // Good match threshold (default: 30)
	DurationAcceptable int // Acceptable match threshold (default: 60)
	DurationPoor       int // Poor match threshold (default: 120)
}

// DefaultYouTubeMatchConfig returns the default configuration
func DefaultYouTubeMatchConfig() YouTubeMatchConfig {
	return YouTubeMatchConfig{
		AutoMatchThreshold: 0.85,
		MinMatchThreshold:  0.6,
		TitleWeight:        0.40,
		ArtistWeight:       0.30,
		DurationWeight:     0.20,
		ChannelWeight:      0.10,
		DurationPerfect:    3,
		DurationExcellent:  10,
		DurationGood:       30,
		DurationAcceptable: 60,
		DurationPoor:       120,
	}
}

// YouTubeMatcher handles scoring YouTube video matches against tracks
type YouTubeMatcher struct {
	Config YouTubeMatchConfig
}

// NewYouTubeMatcher creates a new matcher with default configuration
func NewYouTubeMatcher() *YouTubeMatcher {
	return &YouTubeMatcher{
		Config: DefaultYouTubeMatchConfig(),
	}
}

// NewYouTubeMatcherWithConfig creates a new matcher with custom configuration
func NewYouTubeMatcherWithConfig(config YouTubeMatchConfig) *YouTubeMatcher {
	return &YouTubeMatcher{
		Config: config,
	}
}

// CalculateScore calculates the composite match score for a YouTube video
// Parameters:
//   - trackTitle: The original track title
//   - trackArtist: The original track artist
//   - trackDuration: The expected track duration in seconds (0 if unknown)
//   - videoTitle: The YouTube video title
//   - channelName: The YouTube channel name
//   - videoDuration: The YouTube video duration in seconds
//
// Returns a YouTubeMatchScore with all component scores and the weighted composite
func (m *YouTubeMatcher) CalculateScore(
	trackTitle, trackArtist string,
	trackDuration int,
	videoTitle, channelName string,
	videoDuration int,
) YouTubeMatchScore {
	// Calculate individual component scores
	titleScore := m.calculateTitleScore(trackTitle, videoTitle)
	artistScore := m.calculateArtistScore(trackArtist, videoTitle, channelName)
	durationScore := m.calculateDurationScore(trackDuration, videoDuration)
	channelScore := m.calculateChannelScore(trackArtist, channelName)

	// Check for "Official Music Video" in title for bonus
	officialBonus := m.calculateOfficialVideoBonus(videoTitle)

	// Calculate weighted composite
	composite := (titleScore * m.Config.TitleWeight) +
		(artistScore * m.Config.ArtistWeight) +
		(durationScore * m.Config.DurationWeight) +
		(channelScore * m.Config.ChannelWeight)

	// Apply official video bonus (up to 0.15 boost)
	composite = math.Min(1.0, composite+officialBonus)

	return YouTubeMatchScore{
		Composite: composite,
		Title:     titleScore,
		Artist:    artistScore,
		Duration:  durationScore,
		Channel:   channelScore,
	}
}

// officialVideoPatterns matches "Official Music Video" variations
var officialVideoPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bofficial\s+(?:music\s+)?video\b`),
	regexp.MustCompile(`(?i)\bofficial\s+(?:music\s+)?audio\b`),
	regexp.MustCompile(`(?i)\bofficial\s+release\b`),
}

// calculateOfficialVideoBonus returns a bonus score if video title contains "Official [Music] Video/Audio"
func (m *YouTubeMatcher) calculateOfficialVideoBonus(videoTitle string) float64 {
	for _, pattern := range officialVideoPatterns {
		if pattern.MatchString(videoTitle) {
			return 0.15 // Full bonus for official releases
		}
	}
	return 0.0
}

// calculateTitleScore calculates similarity between track title and video title
func (m *YouTubeMatcher) calculateTitleScore(trackTitle, videoTitle string) float64 {
	// Normalize both titles
	normalizedTrack := duration.NormalizeTitle(trackTitle)
	normalizedVideo := normalizeVideoTitle(videoTitle)

	// Use the existing string similarity function
	return stringSimilarity(normalizedTrack, normalizedVideo)
}

// calculateArtistScore checks if the artist name appears in video title or matches channel
func (m *YouTubeMatcher) calculateArtistScore(trackArtist, videoTitle, channelName string) float64 {
	normalizedArtist := duration.NormalizeArtistName(trackArtist)
	normalizedArtistLower := strings.ToLower(normalizedArtist)

	// Check if artist appears in video title (common format: "Artist - Song Title")
	videoTitleLower := strings.ToLower(videoTitle)
	if strings.Contains(videoTitleLower, normalizedArtistLower) {
		return 1.0
	}

	// Check similarity with channel name
	channelSimilarity := stringSimilarity(normalizedArtist, channelName)
	if channelSimilarity > 0.7 {
		return channelSimilarity
	}

	// Check if artist is part of channel name (e.g., "ArtistVEVO", "ArtistOfficial")
	channelLower := strings.ToLower(channelName)
	if strings.Contains(channelLower, normalizedArtistLower) {
		return 0.9
	}

	// Partial match - check for common variations
	// e.g., "The Beatles" vs "Beatles"
	artistWords := strings.Fields(normalizedArtistLower)
	matchedWords := 0
	for _, word := range artistWords {
		if len(word) > 2 && strings.Contains(videoTitleLower, word) {
			matchedWords++
		}
	}
	if len(artistWords) > 0 {
		wordMatchRatio := float64(matchedWords) / float64(len(artistWords))
		if wordMatchRatio > 0.5 {
			return 0.5 + (wordMatchRatio * 0.3)
		}
	}

	return 0.3 // Baseline - artist not clearly identified
}

// calculateDurationScore calculates how close the video duration is to expected track duration
func (m *YouTubeMatcher) calculateDurationScore(expectedDuration, actualDuration int) float64 {
	// If we don't have expected duration, return neutral score
	if expectedDuration <= 0 {
		return 0.5
	}

	// If actual duration is 0 (unknown), return low score
	if actualDuration <= 0 {
		return 0.3
	}

	diff := abs(expectedDuration - actualDuration)

	switch {
	case diff <= m.Config.DurationPerfect:
		return 1.0 // Perfect match
	case diff <= m.Config.DurationExcellent:
		return 0.9 // Excellent
	case diff <= m.Config.DurationGood:
		return 0.7 // Good
	case diff <= m.Config.DurationAcceptable:
		return 0.5 // Acceptable
	case diff <= m.Config.DurationPoor:
		return 0.3 // Poor
	default:
		return 0.1 // Very poor - likely wrong video or extended version
	}
}

// calculateChannelScore evaluates how well the channel name matches the artist
func (m *YouTubeMatcher) calculateChannelScore(artistName, channelName string) float64 {
	normalizedArtist := strings.ToLower(duration.NormalizeArtistName(artistName))
	normalizedChannel := strings.ToLower(strings.TrimSpace(channelName))

	// Check for official channel indicators
	isOfficial := isOfficialChannel(normalizedChannel)

	// Calculate base similarity
	similarity := stringSimilarity(normalizedArtist, normalizedChannel)

	// Check if artist name is contained in channel name
	if strings.Contains(normalizedChannel, normalizedArtist) {
		similarity = math.Max(similarity, 0.8)
	}

	// Check for VEVO channel - highest priority official source
	if strings.HasSuffix(normalizedChannel, "vevo") {
		// If VEVO channel contains artist name, give high score
		if strings.Contains(normalizedChannel, normalizedArtist) {
			return math.Min(1.0, similarity+0.2)
		}
		// Even without name match, VEVO is a trusted source
		if similarity > 0.5 {
			return math.Min(1.0, similarity+0.15)
		}
	}

	// Boost score for official channels with decent similarity
	if isOfficial && similarity > 0.4 {
		return math.Min(1.0, similarity+0.2)
	}

	// Check for "- Topic" channels (YouTube auto-generated)
	if strings.HasSuffix(normalizedChannel, " - topic") {
		channelWithoutTopic := strings.TrimSuffix(normalizedChannel, " - topic")
		topicSimilarity := stringSimilarity(normalizedArtist, channelWithoutTopic)
		if topicSimilarity > similarity {
			return math.Min(1.0, topicSimilarity+0.1) // Small bonus for Topic channels
		}
	}

	return similarity
}

// IsAutoMatch returns true if the score is high enough for automatic matching
func (m *YouTubeMatcher) IsAutoMatch(score YouTubeMatchScore) bool {
	return score.Composite >= m.Config.AutoMatchThreshold
}

// IsAcceptableMatch returns true if the score meets the minimum threshold
func (m *YouTubeMatcher) IsAcceptableMatch(score YouTubeMatchScore) bool {
	return score.Composite >= m.Config.MinMatchThreshold
}

// NeedsReview returns true if the match is acceptable but not auto-matchable
func (m *YouTubeMatcher) NeedsReview(score YouTubeMatchScore) bool {
	return m.IsAcceptableMatch(score) && !m.IsAutoMatch(score)
}

// =============================================================================
// Helper functions
// =============================================================================

// officialChannelIndicators are strings that suggest an official artist channel
var officialChannelIndicators = []string{
	"vevo",
	"official",
	" - topic",
	"records",
	"music",
}

// isOfficialChannel checks if the channel name suggests it's an official source
func isOfficialChannel(channelName string) bool {
	channelLower := strings.ToLower(channelName)
	for _, indicator := range officialChannelIndicators {
		if strings.Contains(channelLower, indicator) {
			return true
		}
	}
	return false
}

// videoTitleCleanupPattern removes common video title additions
var videoTitleCleanupPattern = regexp.MustCompile(`(?i)\s*[\[\(]?\s*(?:official\s*)?(?:music\s*)?(?:video|audio|lyric(?:s)?|hd|hq|4k|1080p|720p|visualizer|visualiser)\s*[\]\)]?\s*$`)

// artistSeparatorPattern matches common artist-title separators
var artistSeparatorPattern = regexp.MustCompile(`^(.+?)\s*[-–—:]\s*(.+)$`)

// normalizeVideoTitle cleans up YouTube video titles for better matching
func normalizeVideoTitle(title string) string {
	// Remove common video suffixes like [Official Video], (Lyric Video), etc.
	normalized := videoTitleCleanupPattern.ReplaceAllString(title, "")

	// If title follows "Artist - Song" format, extract just the song part
	if matches := artistSeparatorPattern.FindStringSubmatch(normalized); len(matches) == 3 {
		// Return just the song title part
		normalized = strings.TrimSpace(matches[2])
	}

	// Apply standard title normalization
	normalized = duration.NormalizeTitle(normalized)

	return strings.TrimSpace(normalized)
}

// stringSimilarity calculates similarity between two strings (0.0-1.0)
// Uses case-insensitive comparison with Levenshtein distance
func stringSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))

	return 1.0 - (float64(distance) / float64(maxLen))
}

// levenshteinDistance calculates edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// Note: abs() is defined in duration_resolver.go within the same package
