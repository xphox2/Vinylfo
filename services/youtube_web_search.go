package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"vinylfo/duration"
)

// VideoMetadata represents metadata for a YouTube video
type VideoMetadata struct {
	VideoID      string `json:"video_id"`
	Title        string `json:"title"`
	ChannelName  string `json:"channel_name"`
	ChannelURL   string `json:"channel_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Duration     int    `json:"duration"` // seconds (may be 0 if unavailable)
}

// YouTubeWebSearcher handles searching for YouTube videos via web search
type YouTubeWebSearcher struct {
	httpClient  *http.Client
	cache       *WebSearchCache
	userAgent   string
	lastRequest time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

// NewYouTubeWebSearcher creates a new web searcher instance
func NewYouTubeWebSearcher() (*YouTubeWebSearcher, error) {
	cache, err := NewWebSearchCache("")
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &YouTubeWebSearcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:       cache,
		userAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		minInterval: 1 * time.Second, // Faster rate limiting for better results
	}, nil
}

// waitForRateLimit waits until enough time has passed since the last request
func (s *YouTubeWebSearcher) waitForRateLimit() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(s.lastRequest)
	if elapsed < s.minInterval {
		sleepTime := s.minInterval - elapsed
		log.Printf("Rate limiting: waiting %.1f seconds", sleepTime.Seconds())
		time.Sleep(sleepTime)
	}
	s.lastRequest = time.Now()
}

// ClearCache removes all cached web search results
func (s *YouTubeWebSearcher) ClearCache() error {
	return s.cache.Clear()
}

// badTitlePatterns matches very low-quality YouTube videos (only exclude lyrics)
var badTitlePatterns = []string{
	"lyrics", "lyric video",
}

// isGoodTitle checks if a video title is likely to be a quality match
func isGoodTitle(title string) bool {
	lower := strings.ToLower(title)
	for _, pattern := range badTitlePatterns {
		if strings.Contains(lower, pattern) {
			return false
		}
	}
	return true
}

// SearchResult represents a single search result with extracted video info
type SearchResult struct {
	VideoID  string         `json:"video_id"`
	URL      string         `json:"url"`
	Metadata *VideoMetadata `json:"metadata,omitempty"`
}

// SearchForTrack searches for YouTube videos matching the given track
// Returns video IDs found from web search results
func (s *YouTubeWebSearcher) SearchForTrack(ctx context.Context, title, artist string) ([]SearchResult, error) {
	// Normalize title and artist for better search results
	normalizedTitle := duration.NormalizeTitle(title)
	normalizedArtist := duration.NormalizeArtistName(artist)

	// Check cache first
	cacheKey := s.generateCacheKey(normalizedTitle, normalizedArtist)
	if cached, found := s.cache.Get(cacheKey); found {
		return cached, nil
	}

	// Try multiple query variations in order of specificity
	// Using the full track title + artist as a phrase (no quotes around individual parts)
	queryVariations := []string{
		// Best: Full track title + artist (as phrase) + YouTube Music
		fmt.Sprintf(`"%s %s" YouTube Music`, normalizedTitle, normalizedArtist),
		// Full track title + artist + Official Audio
		fmt.Sprintf(`"%s %s" "Official Audio"`, normalizedTitle, normalizedArtist),
		// Full track title + artist
		fmt.Sprintf(`"%s %s"`, normalizedTitle, normalizedArtist),
		// Artist + track title (reversed)
		fmt.Sprintf(`"%s" "%s"`, normalizedArtist, normalizedTitle),
		// Just the track title (fallback)
		fmt.Sprintf(`"%s"`, normalizedTitle),
	}

	var allResults []SearchResult
	seen := make(map[string]bool)

	for i, query := range queryVariations {
		log.Printf("Query variation %d: %s", i+1, query)

		searchResults, err := s.performWebSearch(ctx, query)
		if err != nil {
			log.Printf("Query %d failed: %v", i+1, err)
			continue
		}

		for _, url := range searchResults {
			if !seen[url] {
				seen[url] = true
				videoID := ExtractVideoID(url)
				if videoID != "" {
					allResults = append(allResults, SearchResult{
						VideoID: videoID,
						URL:     url,
					})
				}
			}
		}

		// If we got good results, stop searching
		if len(allResults) >= 5 {
			log.Printf("Found %d results, stopping search", len(allResults))
			break
		}
	}

	results := allResults

	// Fetch metadata for each video (using noembed to get duration)
	for i := range results {
		metadata, err := s.FetchVideoMetadataWithDuration(ctx, results[i].VideoID)
		if err == nil {
			results[i].Metadata = metadata
		}
	}

	// Cache the results
	if len(results) > 0 {
		s.cache.Set(cacheKey, results)
	}

	return results, nil
}

// performWebSearch executes a web search query using the provided query string
func (s *YouTubeWebSearcher) performWebSearch(ctx context.Context, query string) ([]string, error) {
	log.Printf("Searching: %s", query)

	// Rate limit to avoid overwhelming the search server
	s.waitForRateLimit()

	// Try user's private SearXNG server first
	urls, err := s.searchSearXNG(ctx, "http://search.technicallabs.org:8888/search", query)
	if err == nil && len(urls) > 0 {
		log.Printf("Private SearXNG: %d results", len(urls))
		return urls, nil
	}
	log.Printf("Private SearXNG failed: %v", err)

	// Rate limit between search engines
	s.waitForRateLimit()

	// Try DuckDuckGo HTML as backup
	urls, err = s.searchDuckDuckGoHTML(ctx, query)
	if err == nil && len(urls) > 0 {
		log.Printf("DuckDuckGo: %d results", len(urls))
		return urls, nil
	}

	// Rate limit before trying public instances
	s.waitForRateLimit()

	// Try public SearXNG instances as last resort
	searchEndpoints := []string{
		"https://searx.be/search",
		"https://searxng.de/search",
	}

	for _, endpoint := range searchEndpoints {
		s.waitForRateLimit()
		urls, err := s.searchSearXNG(ctx, endpoint, query)
		if err != nil {
			continue
		}
		if len(urls) > 0 {
			return urls, nil
		}
	}

	return nil, fmt.Errorf("all search engines failed")
}

// searchSearXNG searches using a SearXNG instance
func (s *YouTubeWebSearcher) searchSearXNG(ctx context.Context, endpoint, query string) ([]string, error) {
	searchURL := fmt.Sprintf("%s?q=%s&engines=youtube&language=en&format=json", endpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SearXNG returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searxResp struct {
		Results []struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &searxResp); err != nil {
		return nil, fmt.Errorf("failed to parse SearXNG response: %w", err)
	}

	var urls []string
	seen := make(map[string]bool)
	for _, r := range searxResp.Results {
		// Filter out low-quality results based on title
		if r.Title != "" && !isGoodTitle(r.Title) {
			continue
		}
		if strings.Contains(r.URL, "youtube.com/watch?v=") && !seen[r.URL] {
			urls = append(urls, r.URL)
			seen[r.URL] = true
		}
	}

	log.Printf("SearXNG filtered results: %d of %d", len(urls), len(searxResp.Results))
	return urls, nil
}

// searchDuckDuckGoHTML searches using DuckDuckGo HTML
func (s *YouTubeWebSearcher) searchDuckDuckGoHTML(ctx context.Context, query string) ([]string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Accept 200 or 202
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("DuckDuckGo returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse YouTube links from HTML
	var urls []string
	seen := make(map[string]bool)

	// Match various YouTube URL patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`href="(https://www\.youtube\.com/watch\?v=[a-zA-Z0-9_-]{11})[^"]*"`),
		regexp.MustCompile(`href="(https://youtu\.be/[a-zA-Z0-9_-]{11})"`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(string(body), -1)
		for _, match := range matches {
			if len(match) > 1 && !seen[match[1]] {
				urls = append(urls, match[1])
				seen[match[1]] = true
			}
		}
	}

	return urls, nil
}

// FetchVideoMetadata retrieves video metadata using YouTube's oEmbed endpoint
// This does NOT count against YouTube API quota
func (s *YouTubeWebSearcher) FetchVideoMetadata(ctx context.Context, videoID string) (*VideoMetadata, error) {
	oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", videoID)

	req, err := http.NewRequestWithContext(ctx, "GET", oembedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oEmbed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var oembed OEmbedResponse
	if err := json.Unmarshal(body, &oembed); err != nil {
		return nil, fmt.Errorf("failed to parse oEmbed response: %w", err)
	}

	return &VideoMetadata{
		VideoID:      videoID,
		Title:        oembed.Title,
		ChannelName:  oembed.AuthorName,
		ChannelURL:   oembed.AuthorURL,
		ThumbnailURL: oembed.ThumbnailURL,
		Duration:     0, // oEmbed doesn't provide duration
	}, nil
}

// FetchVideoMetadataWithDuration retrieves video metadata including duration
// Uses noembed.com as it provides duration (oEmbed doesn't)
func (s *YouTubeWebSearcher) FetchVideoMetadataWithDuration(ctx context.Context, videoID string) (*VideoMetadata, error) {
	// First try noembed.com for duration
	noembedURL := fmt.Sprintf("https://noembed.com/embed?url=https://www.youtube.com/watch?v=%s", videoID)

	req, err := http.NewRequestWithContext(ctx, "GET", noembedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Fall back to regular oEmbed
		return s.FetchVideoMetadata(ctx, videoID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fall back to regular oEmbed
		return s.FetchVideoMetadata(ctx, videoID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.FetchVideoMetadata(ctx, videoID)
	}

	var noembed NoEmbedResponse
	if err := json.Unmarshal(body, &noembed); err != nil {
		return s.FetchVideoMetadata(ctx, videoID)
	}

	// Check for error response
	if noembed.Error != "" {
		return nil, fmt.Errorf("noembed error: %s", noembed.Error)
	}

	return &VideoMetadata{
		VideoID:      videoID,
		Title:        noembed.Title,
		ChannelName:  noembed.AuthorName,
		ChannelURL:   noembed.AuthorURL,
		ThumbnailURL: noembed.ThumbnailURL,
		Duration:     noembed.Duration,
	}, nil
}

// OEmbedResponse represents YouTube's oEmbed API response
type OEmbedResponse struct {
	Title           string `json:"title"`
	AuthorName      string `json:"author_name"`
	AuthorURL       string `json:"author_url"`
	Type            string `json:"type"`
	Height          int    `json:"height"`
	Width           int    `json:"width"`
	Version         string `json:"version"`
	ProviderName    string `json:"provider_name"`
	ProviderURL     string `json:"provider_url"`
	ThumbnailHeight int    `json:"thumbnail_height"`
	ThumbnailWidth  int    `json:"thumbnail_width"`
	ThumbnailURL    string `json:"thumbnail_url"`
	HTML            string `json:"html"`
}

// NoEmbedResponse represents noembed.com's response (includes duration)
type NoEmbedResponse struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Duration     int    `json:"duration"` // Duration in seconds
	Error        string `json:"error"`    // Error message if request failed
}

// generateCacheKey creates a cache key from title and artist
func (s *YouTubeWebSearcher) generateCacheKey(title, artist string) string {
	normalized := strings.ToLower(strings.TrimSpace(title + "|" + artist))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:16])
}

// =============================================================================
// Video ID Extraction
// =============================================================================

// youtubeVideoIDPatterns matches various YouTube URL formats
var youtubeVideoIDPatterns = []*regexp.Regexp{
	// Standard watch URLs
	regexp.MustCompile(`youtube\.com/watch\?(?:[^&]*&)*v=([a-zA-Z0-9_-]{11})`),
	// Short URLs
	regexp.MustCompile(`youtu\.be/([a-zA-Z0-9_-]{11})`),
	// Embed URLs
	regexp.MustCompile(`youtube\.com/embed/([a-zA-Z0-9_-]{11})`),
	// Shorts URLs
	regexp.MustCompile(`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`),
	// v/ URLs
	regexp.MustCompile(`youtube\.com/v/([a-zA-Z0-9_-]{11})`),
	// Attribution link
	regexp.MustCompile(`youtube\.com/attribution_link\?.*v%3D([a-zA-Z0-9_-]{11})`),
}

// ExtractVideoID extracts a YouTube video ID from various URL formats
func ExtractVideoID(input string) string {
	for _, pattern := range youtubeVideoIDPatterns {
		if matches := pattern.FindStringSubmatch(input); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// ExtractVideoIDs extracts all unique YouTube video IDs from a list of strings
func ExtractVideoIDs(inputs []string) []string {
	var videoIDs []string
	seen := make(map[string]bool)

	for _, input := range inputs {
		for _, pattern := range youtubeVideoIDPatterns {
			matches := pattern.FindAllStringSubmatch(input, -1)
			for _, match := range matches {
				if len(match) > 1 && !seen[match[1]] {
					videoIDs = append(videoIDs, match[1])
					seen[match[1]] = true
				}
			}
		}
	}

	return videoIDs
}

// IsValidVideoID checks if a string is a valid YouTube video ID format
func IsValidVideoID(id string) bool {
	if len(id) != 11 {
		return false
	}
	validChars := regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	return validChars.MatchString(id)
}

// =============================================================================
// Web Search Cache
// =============================================================================

// WebSearchCacheEntry represents a cached search result
type WebSearchCacheEntry struct {
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	CachedAt time.Time      `json:"cached_at"`
}

// WebSearchCache caches web search results to avoid repeated searches
type WebSearchCache struct {
	cacheDir string
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewWebSearchCache creates a new web search cache
func NewWebSearchCache(cacheDir string) (*WebSearchCache, error) {
	if cacheDir == "" {
		cacheDir = filepath.Join(".", ".youtube_web_cache")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &WebSearchCache{
		cacheDir: cacheDir,
		ttl:      30 * 24 * time.Hour, // 30 days
	}, nil
}

// Get retrieves cached search results
func (c *WebSearchCache) Get(key string) ([]SearchResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := filepath.Join(c.cacheDir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry WebSearchCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CachedAt) > c.ttl {
		os.Remove(path)
		return nil, false
	}

	return entry.Results, true
}

// Set stores search results in the cache
func (c *WebSearchCache) Set(key string, results []SearchResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := WebSearchCacheEntry{
		Query:    key,
		Results:  results,
		CachedAt: time.Now(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(c.cacheDir, key+".json")
	return os.WriteFile(path, data, 0644)
}

// Clear removes all cached entries
func (c *WebSearchCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			os.Remove(filepath.Join(c.cacheDir, entry.Name()))
		}
	}

	return nil
}
