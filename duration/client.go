package duration

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// TrackSearchResult represents a track duration found by an external API
type TrackSearchResult struct {
	// Identification
	ExternalID  string `json:"external_id"`  // ID in external system (e.g., MusicBrainz recording ID)
	ExternalURL string `json:"external_url"` // URL to view on external service

	// Track info returned by API
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"` // Duration in SECONDS

	// Quality metrics
	MatchScore float64 `json:"match_score"` // 0.0-1.0: How well this result matches our search
	Confidence float64 `json:"confidence"`  // 0.0-1.0: How confident the source is

	// Debug info
	RawResponse string `json:"raw_response"` // Full API response JSON
}

// MusicAPIClient is the interface all external API clients must implement
type MusicAPIClient interface {
	// Name returns the source identifier (e.g., "musicbrainz", "wikipedia")
	Name() string

	// SearchTrack searches for a track and returns duration information
	// Parameters:
	//   - ctx: Context for cancellation/timeout
	//   - title: Track title (required)
	//   - artist: Artist name (required)
	//   - album: Album name (optional, improves accuracy)
	// Returns:
	//   - *TrackSearchResult: Best matching result, or nil if not found
	//   - error: Any error that occurred (network, parsing, etc.)
	SearchTrack(ctx context.Context, title, artist, album string) (*TrackSearchResult, error)

	// IsConfigured returns true if the client has valid configuration
	IsConfigured() bool

	// GetRateLimitRemaining returns remaining API calls in current window
	// Returns -1 if rate limiting is not tracked
	GetRateLimitRemaining() int
}

// BaseClient provides common HTTP functionality for all API clients
type BaseClient struct {
	HTTPClient  *http.Client
	RateLimiter *RateLimiter
	UserAgent   string
}

// NewBaseClient creates a base client with sensible defaults
func NewBaseClient(userAgent string, requestsPerMinute int) *BaseClient {
	return &BaseClient{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		RateLimiter: NewRateLimiter(requestsPerMinute),
		UserAgent:   userAgent,
	}
}

// CalculateMatchScore calculates how well a result matches the search query
// Uses Levenshtein distance normalized to 0.0-1.0
func CalculateMatchScore(searchTitle, searchArtist, resultTitle, resultArtist string) float64 {
	titleScore := stringSimilarity(searchTitle, resultTitle)
	artistScore := stringSimilarity(searchArtist, resultArtist)

	// Weight title slightly higher than artist
	return (titleScore * 0.6) + (artistScore * 0.4)
}

// stringSimilarity returns similarity between two strings (0.0-1.0)
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
