package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const (
	lastfmBaseURL   = "http://ws.audioscrobbler.com/2.0"
	lastfmRateLimit = 300
)

type LastFMClient struct {
	*BaseClient
	APIKey string
}

type lastfmTrackResponse struct {
	Track struct {
		Name   string `json:"name"`
		Artist struct {
			Name string `json:"name"`
		} `json:"artist"`
		Duration interface{} `json:"duration"`
		URL      string      `json:"url"`
		Album    struct {
			Title string `json:"title"`
		} `json:"album"`
	} `json:"track"`
}

type lastfmErrorResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func NewLastFMClient(apiKey string) *LastFMClient {
	return &LastFMClient{
		BaseClient: NewBaseClient("Vinylfo/1.0 (Music Collection Manager)", lastfmRateLimit),
		APIKey:     apiKey,
	}
}

func (c *LastFMClient) Name() string {
	return "Last.fm"
}

func (c *LastFMClient) IsConfigured() bool {
	return c.APIKey != ""
}

func (c *LastFMClient) GetRateLimitRemaining() int {
	return c.RateLimiter.GetRemaining()
}

func parseLastFMDuration(d interface{}) int {
	if d == nil {
		return 0
	}
	switch v := d.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if val, err := strconv.Atoi(v); err == nil {
			return val
		}
		return 0
	default:
		return 0
	}
}

func (c *LastFMClient) SearchTrack(ctx context.Context, title, artist, album string) (*TrackSearchResult, error) {
	if title == "" || artist == "" {
		return nil, fmt.Errorf("title and artist are required")
	}

	if c.APIKey == "" {
		log.Printf("LF: LastFM API key not configured, skipping")
		return nil, nil
	}

	log.Printf("LF: Searching LastFM for '%s' by '%s'", title, artist)

	trackInfo, err := c.getTrackInfo(ctx, title, artist)
	if err != nil {
		log.Printf("LF: LastFM lookup failed for '%s': %v", title, err)
		return nil, fmt.Errorf("lastfm lookup failed: %w", err)
	}

	if trackInfo == nil {
		log.Printf("LF: No results found on LastFM for '%s'", title)
		return nil, nil
	}

	durationMs := parseLastFMDuration(trackInfo.Track.Duration)
	durationSeconds := durationMs / 1000

	normalizedTitle := NormalizeTitle(title)
	normalizedResultTitle := NormalizeTitle(trackInfo.Track.Name)

	matchScore := stringSimilarity(normalizedTitle, normalizedResultTitle)

	confidence := 0.6
	if matchScore > 0.8 {
		confidence = 0.8
	}

	log.Printf("LF: Found '%s' by '%s' on LastFM - duration %ds, match score %.2f",
		trackInfo.Track.Name, trackInfo.Track.Artist.Name, durationSeconds, matchScore)

	return &TrackSearchResult{
		ExternalID:  fmt.Sprintf("lastfm:%s:%s", trackInfo.Track.Artist.Name, url.QueryEscape(trackInfo.Track.Name)),
		ExternalURL: trackInfo.Track.URL,
		Title:       trackInfo.Track.Name,
		Artist:      trackInfo.Track.Artist.Name,
		Album:       trackInfo.Track.Album.Title,
		Duration:    durationSeconds,
		MatchScore:  matchScore,
		Confidence:  confidence,
	}, nil
}

func (c *LastFMClient) getTrackInfo(ctx context.Context, title, artist string) (*lastfmTrackResponse, error) {
	searchURL := fmt.Sprintf("%s/?method=track.search&track=%s&artist=%s&api_key=%s&format=json",
		lastfmBaseURL,
		url.QueryEscape(title),
		url.QueryEscape(artist),
		c.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, body, err := c.DoWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	// Check if response is actually JSON (not HTML error page)
	if resp.StatusCode != 200 {
		log.Printf("LF: LastFM API returned status %d for '%s', response: %.200s", resp.StatusCode, title, string(body))
		return nil, fmt.Errorf("LastFM API error: status %d", resp.StatusCode)
	}

	// Check if response starts with '{' (valid JSON)
	if len(body) == 0 || body[0] != '{' {
		log.Printf("LF: LastFM returned non-JSON response for '%s': %.200s", title, string(body))
		return nil, nil
	}

	var searchResults struct {
		Results struct {
			TrackMatches struct {
				Track []struct {
					Name     string      `json:"name"`
					Artist   string      `json:"artist"`
					Duration interface{} `json:"duration"`
					URL      string      `json:"url"`
				} `json:"track"`
			} `json:"trackmatches"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &searchResults); err != nil {
		log.Printf("LF: Failed to parse LastFM response for '%s': %v, response: %.200s", title, err, string(body))
		return nil, nil
	}

	if len(searchResults.Results.TrackMatches.Track) == 0 {
		return nil, nil
	}

	bestMatch := searchResults.Results.TrackMatches.Track[0]
	durationMs := parseLastFMDuration(bestMatch.Duration)

	if durationMs == 0 {
		return c.getTrackInfoByTitleArtist(ctx, title, artist)
	}

	return &lastfmTrackResponse{
		Track: struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
			Duration interface{} `json:"duration"`
			URL      string      `json:"url"`
			Album    struct {
				Title string `json:"title"`
			} `json:"album"`
		}{
			Name: bestMatch.Name,
			Artist: struct {
				Name string `json:"name"`
			}{
				Name: bestMatch.Artist,
			},
			Duration: durationMs,
			URL:      bestMatch.URL,
			Album: struct {
				Title string `json:"title"`
			}{
				Title: "",
			},
		},
	}, nil
}

func (c *LastFMClient) getTrackInfoByTitleArtist(ctx context.Context, title, artist string) (*lastfmTrackResponse, error) {
	searchURL := fmt.Sprintf("%s/?method=track.getInfo&track=%s&artist=%s&api_key=%s&format=json",
		lastfmBaseURL,
		url.QueryEscape(title),
		url.QueryEscape(artist),
		c.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, body, err := c.DoWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	// Check for non-200 status codes
	if resp.StatusCode != 200 {
		log.Printf("LF: LastFM API returned status %d for track.getInfo '%s', response: %.200s", resp.StatusCode, title, string(body))
		return nil, fmt.Errorf("LastFM API error: status %d", resp.StatusCode)
	}

	var errorResp lastfmErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil {
		if errorResp.Error == 6 {
			return nil, nil
		}
	}

	var trackResp lastfmTrackResponse
	if err := json.Unmarshal(body, &trackResp); err != nil {
		return nil, err
	}

	if trackResp.Track.Name == "" {
		return nil, nil
	}

	durationMs := parseLastFMDuration(trackResp.Track.Duration)
	trackResp.Track.Duration = durationMs

	return &trackResp, nil
}
