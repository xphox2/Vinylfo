package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	youtubeBaseURL   = "https://www.googleapis.com/youtube/v3"
	youtubeRateLimit = 100
)

type YouTubeClient struct {
	*BaseClient
	apiKey string
	cache  *YouTubeCache
}

type youtubeSearchResponse struct {
	Kind     string `json:"kind"`
	ETag     string `json:"etag"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
	Items []youtubeSearchItem `json:"items"`
}

type youtubeSearchItem struct {
	Kind string `json:"kind"`
	ETag string `json:"etag"`
	ID   struct {
		Kind    string `json:"kind"`
		VideoID string `json:"videoId"`
	} `json:"id"`
	Snippet struct {
		PublishedAt time.Time `json:"publishedAt"`
		ChannelID   string    `json:"channelId"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Thumbnails  map[string]struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"thumbnails"`
		ChannelTitle         string    `json:"channelTitle"`
		LiveBroadcastContent string    `json:"liveBroadcastContent"`
		PublishTime          time.Time `json:"publishTime"`
	} `json:"snippet"`
}

type youtubeVideoResponse struct {
	Kind     string `json:"kind"`
	ETag     string `json:"etag"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
	Items []youtubeVideoItem `json:"items"`
}

type youtubeVideoItem struct {
	Kind    string `json:"kind"`
	ETag    string `json:"etag"`
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration        string `json:"duration"`
		Dimension       string `json:"dimension"`
		Definition      string `json:"definition"`
		Caption         string `json:"caption"`
		LicensedContent bool   `json:"licensedContent"`
		Projection      string `json:"projection"`
	} `json:"contentDetails"`
}

func NewYouTubeClient(apiKey string) *YouTubeClient {
	userAgent := "Vinylfo/1.0 (github.com/xphox2/Vinylfo)"

	cache, err := NewYouTubeCache("")
	if err != nil {
		// Log but don't fail - cache is optional
		cache = nil
	}

	return &YouTubeClient{
		BaseClient: NewBaseClient(userAgent, youtubeRateLimit),
		apiKey:     apiKey,
		cache:      cache,
	}
}

func (c *YouTubeClient) Name() string {
	return "youtube"
}

func (c *YouTubeClient) IsConfigured() bool {
	return c.apiKey != ""
}

func (c *YouTubeClient) GetRateLimitRemaining() int {
	return c.RateLimiter.GetRemaining()
}

func (c *YouTubeClient) SearchTrack(ctx context.Context, title, artist, album string) (*TrackSearchResult, error) {
	if title == "" || artist == "" {
		return nil, fmt.Errorf("title and artist are required")
	}

	// Check cache first
	if c.cache != nil {
		if entry, found := c.cache.Get(title, artist, album); found {
			if entry.Duration == -1 {
				// Cached "not found" result
				log.Printf("YT: Cache shows '%s' by '%s' not found", title, artist)
				return nil, nil
			}
			log.Printf("YT: Cache hit for '%s' by '%s' - duration %ds", title, artist, entry.Duration)
			return &TrackSearchResult{
				ExternalID:  entry.VideoID,
				ExternalURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.VideoID),
				Title:       entry.VideoTitle,
				Artist:      artist,
				Duration:    entry.Duration,
				MatchScore:  entry.MatchScore,
				Confidence:  entry.MatchScore * 0.6,
			}, nil
		}
	}

	if c.apiKey == "" {
		log.Printf("YT: YouTube API key not configured, skipping")
		return nil, nil
	}

	searchQuery := c.buildSearchQuery(title, artist, album)

	log.Printf("YT: Searching YouTube for '%s' by '%s' (album: '%s')", title, artist, album)
	log.Printf("YT: Query: %s", searchQuery)

	searchURL := fmt.Sprintf("%s/search?part=snippet&type=video&q=%s&maxResults=10&key=%s",
		youtubeBaseURL,
		url.QueryEscape(searchQuery),
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, body, err := c.DoWithRetry(ctx, req)
	if err != nil {
		log.Printf("YT: Request failed for '%s': %v", title, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("YT: YouTube API error for '%s': %d - %s", title, resp.StatusCode, string(body))
		return nil, fmt.Errorf("YouTube API error: %d - %s", resp.StatusCode, string(body))
	}

	var searchResp youtubeSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	log.Printf("YT: Got %d search results for '%s'", len(searchResp.Items), title)

	if len(searchResp.Items) == 0 {
		log.Printf("YT: No results found on YouTube for '%s'", title)
		return nil, nil
	}

	videoIDs := make([]string, 0, len(searchResp.Items))
	for _, item := range searchResp.Items {
		if item.ID.VideoID != "" {
			videoIDs = append(videoIDs, item.ID.VideoID)
		}
	}

	if len(videoIDs) == 0 {
		log.Printf("YT: No valid video IDs for '%s'", title)
		// Cache the "not found" result
		if c.cache != nil {
			c.cache.SetNotFound(title, artist, album)
		}
		return nil, nil
	}

	videoResp, err := c.getVideoDetails(ctx, videoIDs)
	if err != nil {
		log.Printf("YT: Failed to get video details for '%s': %v", title, err)
		return nil, err
	}

	result := c.findBestMatch(searchResp.Items, videoResp.Items, title, artist, album)
	if result != nil {
		log.Printf("YT: Best match for '%s': '%s' - duration %ds, match score %.2f",
			title, result.Title, result.Duration, result.MatchScore)
		result.RawResponse = string(body)
		// Cache the successful result
		if c.cache != nil {
			c.cache.Set(title, artist, album, result)
		}
	} else {
		log.Printf("YT: No suitable match found for '%s' on YouTube", title)
		// Cache the "not found" result
		if c.cache != nil {
			c.cache.SetNotFound(title, artist, album)
		}
	}

	return result, nil
}

func (c *YouTubeClient) buildSearchQuery(title, artist, album string) string {
	normalizedTitle := NormalizeTitle(title)
	normalizedArtist := NormalizeArtistName(artist)
	normalizedAlbum := NormalizeTitle(album)

	words := []string{normalizedTitle, normalizedArtist}

	if normalizedAlbum != "" && normalizedAlbum != normalizedArtist {
		words = append(words, normalizedAlbum)
	}

	query := strings.Join(words, " ")

	return query
}

func (c *YouTubeClient) getVideoDetails(ctx context.Context, videoIDs []string) (*youtubeVideoResponse, error) {
	idsParam := strings.Join(videoIDs, ",")

	videoURL := fmt.Sprintf("%s/videos?part=contentDetails&id=%s&key=%s",
		youtubeBaseURL,
		idsParam,
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create video request: %w", err)
	}

	resp, body, err := c.DoWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("video request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YouTube videos API error: %d - %s", resp.StatusCode, string(body))
	}

	var videoResp youtubeVideoResponse
	if err := json.Unmarshal(body, &videoResp); err != nil {
		return nil, fmt.Errorf("failed to parse video response: %w", err)
	}

	return &videoResp, nil
}

func (c *YouTubeClient) findBestMatch(searchItems []youtubeSearchItem, videoItems []youtubeVideoItem, searchTitle, searchArtist, searchAlbum string) *TrackSearchResult {
	if len(searchItems) == 0 || len(videoItems) == 0 {
		return nil
	}

	videoDurationMap := make(map[string]int)
	for _, item := range videoItems {
		duration := parseYouTubeDuration(item.ContentDetails.Duration)
		videoDurationMap[item.ID] = duration
	}

	var bestResult *TrackSearchResult
	var bestScore float64 = 0

	for _, searchItem := range searchItems {
		if searchItem.ID.VideoID == "" {
			continue
		}

		duration := videoDurationMap[searchItem.ID.VideoID]
		if duration == 0 {
			continue
		}

		title := searchItem.Snippet.Title

		artistName := searchItem.Snippet.ChannelTitle

		matchScore := CalculateMatchScore(searchTitle, searchArtist, title, artistName)

		if matchScore > bestScore {
			bestScore = matchScore

			bestResult = &TrackSearchResult{
				ExternalID:  searchItem.ID.VideoID,
				ExternalURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", searchItem.ID.VideoID),
				Title:       title,
				Artist:      artistName,
				Duration:    duration,
				MatchScore:  matchScore,
				Confidence:  matchScore * 0.6,
			}
		}
	}

	if bestResult != nil && bestResult.MatchScore < 0.3 {
		return nil
	}

	return bestResult
}

var youtubeDurationRegex = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

func parseYouTubeDuration(duration string) int {
	matches := youtubeDurationRegex.FindStringSubmatch(duration)
	if matches == nil {
		return 0
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])

	return hours*3600 + minutes*60 + seconds
}
