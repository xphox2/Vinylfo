package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	musicBrainzBaseURL   = "https://musicbrainz.org/ws/2"
	musicBrainzRateLimit = 50 // requests per minute (~1/sec, MusicBrainz allows 1/sec for unregistered apps)
)

type MusicBrainzClient struct {
	*BaseClient
	contactEmail string
}

type mbSearchResponse struct {
	Created    string        `json:"created"`
	Count      int           `json:"count"`
	Offset     int           `json:"offset"`
	Recordings []mbRecording `json:"recordings"`
}

type mbRecording struct {
	ID           string           `json:"id"`
	Score        int              `json:"score"`
	Title        string           `json:"title"`
	Length       *int             `json:"length"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	Releases     []mbRelease      `json:"releases"`
	ISRCs        []string         `json:"isrcs"`
}

type mbArtistCredit struct {
	Name   string   `json:"name"`
	Artist mbArtist `json:"artist"`
}

type mbArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mbRelease struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

func NewMusicBrainzClient(contactEmail string) *MusicBrainzClient {
	userAgent := fmt.Sprintf("Vinylfo/1.0 (%s)", contactEmail)

	return &MusicBrainzClient{
		BaseClient:   NewBaseClient(userAgent, musicBrainzRateLimit),
		contactEmail: contactEmail,
	}
}

func (c *MusicBrainzClient) Name() string {
	return "musicbrainz"
}

func (c *MusicBrainzClient) IsConfigured() bool {
	return c.contactEmail != ""
}

func (c *MusicBrainzClient) GetRateLimitRemaining() int {
	return c.RateLimiter.GetRemaining()
}

func (c *MusicBrainzClient) SearchTrack(ctx context.Context, title, artist, album string) (*TrackSearchResult, error) {
	if title == "" || artist == "" {
		return nil, fmt.Errorf("title and artist are required")
	}

	query := c.buildQuery(title, artist, album)

	reqURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=5",
		musicBrainzBaseURL,
		url.QueryEscape(query),
	)

	log.Printf("MB: Querying MusicBrainz for '%s' by '%s' (album: '%s')", title, artist, album)
	log.Printf("MB: Query string: %s", query)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	startTime := time.Now()
	resp, body, err := c.DoWithRetry(ctx, req)
	queryDuration := time.Since(startTime)

	if err != nil {
		log.Printf("MB: Request failed for '%s': %v", title, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var searchResp mbSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("MB: Got %d recordings for '%s'", len(searchResp.Recordings), title)

	result := c.findBestMatch(searchResp.Recordings, title, artist, album)
	if result == nil {
		log.Printf("MB: No suitable match found for '%s' (all recordings had low match scores)", title)
		return nil, nil
	}

	log.Printf("MB: Best match for '%s': '%s' by '%s' - duration %ds, match score %.2f, MB score %.2f",
		title, result.Title, result.Artist, result.Duration, result.MatchScore, result.Confidence)

	result.RawResponse = string(body)
	_ = queryDuration

	return result, nil
}

func (c *MusicBrainzClient) buildQuery(title, artist, album string) string {
	escape := func(s string) string {
		replacer := strings.NewReplacer(
			`+`, `\+`,
			`-`, `\-`,
			`&&`, `\&&`,
			`||`, `\||`,
			`!`, `\!`,
			`(`, `\(`,
			`)`, `\)`,
			`{`, `\{`,
			`}`, `\}`,
			`[`, `\[`,
			`]`, `\]`,
			`^`, `\^`,
			`"`, `\"`,
			`~`, `\~`,
			`*`, `\*`,
			`?`, `\?`,
			`:`, `\:`,
			`\`, `\\`,
			`/`, `\/`,
		)
		return replacer.Replace(s)
	}

	// Normalize artist name to remove disambiguation suffixes like "(2)" before querying
	normalizedArtist := NormalizeArtistName(artist)
	// Normalize title and album to remove edition suffixes like "(Remastered)"
	normalizedTitle := NormalizeTitle(title)
	normalizedAlbum := NormalizeTitle(album)

	parts := []string{
		fmt.Sprintf(`recording:"%s"`, escape(normalizedTitle)),
		fmt.Sprintf(`artist:"%s"`, escape(normalizedArtist)),
	}

	if normalizedAlbum != "" {
		parts = append(parts, fmt.Sprintf(`release:"%s"`, escape(normalizedAlbum)))
	}

	return strings.Join(parts, " AND ")
}

func (c *MusicBrainzClient) findBestMatch(recordings []mbRecording, searchTitle, searchArtist, searchAlbum string) *TrackSearchResult {
	if len(recordings) == 0 {
		return nil
	}

	var bestResult *TrackSearchResult
	var bestScore float64 = 0

	for _, rec := range recordings {
		if rec.Length == nil || *rec.Length == 0 {
			continue
		}

		artistName := ""
		if len(rec.ArtistCredit) > 0 {
			artistName = rec.ArtistCredit[0].Name
		}

		albumName := ""
		if len(rec.Releases) > 0 {
			albumName = rec.Releases[0].Title
		}

		matchScore := CalculateMatchScore(searchTitle, searchArtist, rec.Title, artistName)

		if searchAlbum != "" && albumName != "" {
			albumScore := stringSimilarity(searchAlbum, albumName)
			matchScore = (matchScore * 0.7) + (albumScore * 0.3)
		}

		mbScore := float64(rec.Score) / 100.0
		combinedScore := (matchScore * 0.6) + (mbScore * 0.4)

		if combinedScore > bestScore {
			bestScore = combinedScore

			durationSecs := *rec.Length / 1000

			bestResult = &TrackSearchResult{
				ExternalID:  rec.ID,
				ExternalURL: fmt.Sprintf("https://musicbrainz.org/recording/%s", rec.ID),
				Title:       rec.Title,
				Artist:      artistName,
				Album:       albumName,
				Duration:    durationSecs,
				MatchScore:  matchScore,
				Confidence:  mbScore,
			}
		}
	}

	if bestResult != nil && bestResult.MatchScore < 0.3 {
		return nil
	}

	return bestResult
}
