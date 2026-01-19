package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	return &YouTubeClient{
		BaseClient: NewBaseClient(userAgent, youtubeRateLimit),
		apiKey:     apiKey,
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

	searchQuery := c.buildSearchQuery(title, artist, album)

	searchURL := fmt.Sprintf("%s/search?part=snippet&type=video&q=%s&maxResults=10&key=%s",
		youtubeBaseURL,
		url.QueryEscape(searchQuery),
		c.apiKey,
	)

	c.RateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("YouTube API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResp youtubeSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	if len(searchResp.Items) == 0 {
		return nil, nil
	}

	videoIDs := make([]string, 0, len(searchResp.Items))
	for _, item := range searchResp.Items {
		if item.ID.VideoID != "" {
			videoIDs = append(videoIDs, item.ID.VideoID)
		}
	}

	if len(videoIDs) == 0 {
		return nil, nil
	}

	videoResp, err := c.getVideoDetails(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	result := c.findBestMatch(searchResp.Items, videoResp.Items, title, artist, album)
	if result != nil {
		result.RawResponse = string(body)
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

	c.RateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create video request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("video request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("YouTube videos API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video response: %w", err)
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

	if bestResult != nil && bestResult.MatchScore < 0.4 {
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
