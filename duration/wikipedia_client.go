package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	wikipediaBaseURL   = "https://en.wikipedia.org/w/api.php"
	wikipediaRateLimit = 50
)

type WikipediaClient struct {
	*BaseClient
}

type wpSearchResponse struct {
	Query struct {
		Search []wpSearchResult `json:"search"`
	} `json:"query"`
}

type wpSearchResult struct {
	Title   string `json:"title"`
	PageID  int    `json:"pageid"`
	Snippet string `json:"snippet"`
}

type wpPageResponse struct {
	Parse struct {
		Title    string `json:"title"`
		PageID   int    `json:"pageid"`
		Wikitext struct {
			Content string `json:"*"`
		} `json:"wikitext"`
	} `json:"parse"`
}

type TrackInfo struct {
	Title    string
	Duration int
	Position int
}

func NewWikipediaClient() *WikipediaClient {
	return &WikipediaClient{
		BaseClient: NewBaseClient("Vinylfo/1.0 (Music Collection Manager)", wikipediaRateLimit),
	}
}

func (c *WikipediaClient) Name() string {
	return "wikipedia"
}

func (c *WikipediaClient) IsConfigured() bool {
	return true
}

func (c *WikipediaClient) GetRateLimitRemaining() int {
	return c.RateLimiter.GetRemaining()
}

func (c *WikipediaClient) SearchTrack(ctx context.Context, title, artist, album string) (*TrackSearchResult, error) {
	if title == "" || artist == "" {
		return nil, fmt.Errorf("title and artist are required")
	}

	if album == "" {
		return nil, nil
	}

	log.Printf("WIKI: Searching for album '%s' by '%s'", album, artist)

	pageTitle, err := c.searchAlbumPage(ctx, album, artist)
	if err != nil {
		return nil, fmt.Errorf("album search failed: %w", err)
	}
	if pageTitle == "" {
		log.Printf("WIKI: No page found for album '%s'", album)
		return nil, nil
	}

	log.Printf("WIKI: Found page '%s' for album '%s'", pageTitle, album)

	content, err := c.getPageContent(ctx, pageTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}
	if content == "" {
		log.Printf("WIKI: Empty content for page '%s'", pageTitle)
		return nil, nil
	}

	log.Printf("WIKI: Content length %d chars for page '%s'", len(content), pageTitle)

	tracks := c.parseTrackListing(content)
	log.Printf("WIKI: Parsed %d tracks from page '%s'", len(tracks), pageTitle)

	if len(tracks) == 0 {
		return nil, nil
	}

	track := c.findMatchingTrack(tracks, title)
	if track == nil {
		log.Printf("WIKI: No matching track found for '%s' (searched %d tracks)", title, len(tracks))
		return nil, nil
	}

	log.Printf("WIKI: Found track '%s' with duration %d seconds", track.Title, track.Duration)

	return &TrackSearchResult{
		ExternalID:  fmt.Sprintf("wikipedia:%s", pageTitle),
		ExternalURL: fmt.Sprintf("https://en.wikipedia.org/wiki/%s", url.PathEscape(pageTitle)),
		Title:       track.Title,
		Artist:      artist,
		Album:       album,
		Duration:    track.Duration,
		MatchScore:  stringSimilarity(title, track.Title),
		Confidence:  0.7,
	}, nil
}

func (c *WikipediaClient) searchAlbumPage(ctx context.Context, album, artist string) (string, error) {
	c.RateLimiter.Wait()

	searchQuery := fmt.Sprintf("%s %s", album, artist)

	reqURL := fmt.Sprintf("%s?action=query&list=search&srsearch=%s&format=json&srlimit=10",
		wikipediaBaseURL,
		url.QueryEscape(searchQuery),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var searchResp wpSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", err
	}

	albumLower := strings.ToLower(album)
	albumWords := strings.Fields(albumLower)
	artistLower := strings.ToLower(artist)

	var bestMatch string
	var bestScore float64 = 0

	for _, result := range searchResp.Query.Search {
		titleLower := strings.ToLower(result.Title)

		score := 0.0

		isAlbumPage := strings.Contains(titleLower, "(album)") ||
			strings.Contains(titleLower, " album)") ||
			strings.HasSuffix(titleLower, " (album)")

		exactMatch := titleLower == albumLower ||
			strings.HasPrefix(titleLower, albumLower+" (") ||
			strings.HasPrefix(titleLower, albumLower+" - ")

		hasAllAlbumWords := true
		for _, word := range albumWords {
			if len(word) > 2 && !strings.Contains(titleLower, word) {
				hasAllAlbumWords = false
				break
			}
		}

		if exactMatch && isAlbumPage {
			score = 300
		} else if exactMatch {
			score = 200
		} else if hasAllAlbumWords && isAlbumPage {
			score = 200
		} else if isAlbumPage {
			score = 100
		} else if hasAllAlbumWords {
			score = 50
		} else {
			for _, word := range albumWords {
				if len(word) > 3 && strings.Contains(titleLower, word) {
					score += float64(len(word))
				}
			}
		}

		if strings.Contains(titleLower, artistLower) && isAlbumPage {
			score += 20
		}

		if score > bestScore {
			bestScore = score
			bestMatch = result.Title
		}
	}

	log.Printf("WIKI: Best match for '%s' is '%s' with score %.0f", album, bestMatch, bestScore)

	if bestScore >= 50 {
		return bestMatch, nil
	}

	return "", nil
}

func (c *WikipediaClient) getPageContent(ctx context.Context, pageTitle string) (string, error) {
	c.RateLimiter.Wait()

	reqURL := fmt.Sprintf("%s?action=parse&page=%s&prop=wikitext&format=json",
		wikipediaBaseURL,
		url.QueryEscape(pageTitle),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var pageResp wpPageResponse
	if err := json.Unmarshal(body, &pageResp); err != nil {
		return "", err
	}

	return pageResp.Parse.Wikitext.Content, nil
}

func (c *WikipediaClient) parseTrackListing(content string) []TrackInfo {
	var tracks []TrackInfo

	trackListRe := regexp.MustCompile(`(?i)\{\{Track( ?listing|list)`)
	templates := []string{}

	log.Printf("WIKI: DEBUG parseTrackListing called with %d chars", len(content))
	first100 := content
	if len(content) > 100 {
		first100 = content[:100]
	}
	log.Printf("WIKI: DEBUG First 100 chars: %s", first100)

	startIdx := 0
	for {
		idx := trackListRe.FindStringIndex(content[startIdx:])
		if idx == nil {
			break
		}
		actualStart := startIdx + idx[0]

		depth := 0
		inTemplate := false
		templateStart := -1

		for i := actualStart; i < len(content); i++ {
			ch := content[i]
			if ch == '{' {
				if !inTemplate && i+1 < len(content) && content[i+1] == '{' {
					inTemplate = true
					templateStart = i
					depth = 2
					i++
					continue
				}
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 && inTemplate {
					templates = append(templates, content[templateStart:i+1])
					startIdx = i + 1
					break
				}
			}
			if templateStart != -1 && i-actualStart > 10000 {
				startIdx = actualStart + 1
				break
			}
		}
		if templateStart == -1 {
			break
		}
	}

	log.Printf("WIKI: Found %d Track listing templates", len(templates))

	for i, listing := range templates {
		log.Printf("WIKI: Processing listing %d, content length: %d", i, len(listing))
		if len(listing) < 500 {
			log.Printf("WIKI: Listing content: %s", listing)
		}
		titles := make(map[int]string)
		lengths := make(map[int]string)

		lines := strings.Split(listing, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "{{") || strings.HasPrefix(line, "}}") {
				continue
			}

			if strings.HasPrefix(line, "| title") {
				log.Printf("WIKI: DEBUG: Found title line: %s", line)
				line = strings.TrimPrefix(line, "| title")
				line = strings.TrimPrefix(line, "|")
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "=") {
					line = strings.TrimPrefix(line, "=")
					line = strings.TrimSpace(line)
				}
				parts := strings.SplitN(line, " ", 2)
				if len(parts) > 0 {
					titleNumStr := ""
					for _, c := range parts[0] {
						if c >= '0' && c <= '9' {
							titleNumStr += string(c)
						} else {
							break
						}
					}
					if titleNumStr != "" {
						titleNum, err := strconv.Atoi(titleNumStr)
						if err == nil {
							titleValue := line[len(titleNumStr):]
							titleValue = strings.TrimSpace(titleValue)
							// Remove the = sign that separates key from value
							if strings.HasPrefix(titleValue, "=") {
								titleValue = strings.TrimPrefix(titleValue, "=")
								titleValue = strings.TrimSpace(titleValue)
							}
							titleValue = regexp.MustCompile(`\{\{[^}]+\}\}`).ReplaceAllString(titleValue, "")
							titleValue = strings.TrimSpace(titleValue)
							titles[titleNum] = titleValue
							log.Printf("WIKI: Found title[%d] = %s", titleNum, titleValue)
						}
					}
				}
			} else if strings.HasPrefix(line, "| length") {
				line = strings.TrimPrefix(line, "| length")
				line = strings.TrimPrefix(line, "|")
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "=") {
					line = strings.TrimPrefix(line, "=")
					line = strings.TrimSpace(line)
				}
				parts := strings.SplitN(line, " ", 2)
				if len(parts) > 0 {
					lengthNumStr := ""
					for _, c := range parts[0] {
						if c >= '0' && c <= '9' {
							lengthNumStr += string(c)
						} else {
							break
						}
					}
					if lengthNumStr != "" {
						lengthNum, err := strconv.Atoi(lengthNumStr)
						if err == nil {
							lengthValue := line[len(lengthNumStr):]
							lengthValue = strings.TrimSpace(lengthValue)
							// Remove the = sign that separates key from value
							if strings.HasPrefix(lengthValue, "=") {
								lengthValue = strings.TrimPrefix(lengthValue, "=")
								lengthValue = strings.TrimSpace(lengthValue)
							}
							lengths[lengthNum] = lengthValue
							log.Printf("WIKI: Found length[%d] = %s", lengthNum, lengthValue)
						}
					}
				}
			}
		}

		log.Printf("WIKI: Parsed %d titles and %d lengths", len(titles), len(lengths))

		for pos, title := range titles {
			if length, ok := lengths[pos]; ok {
				duration := c.parseDurationString(length)
				if duration > 0 {
					cleanedTitle := c.cleanWikiMarkup(title)
					log.Printf("WIKI: DEBUG cleaned title[%d]: '%s' -> '%s'", pos, title, cleanedTitle)
					tracks = append(tracks, TrackInfo{
						Title:    cleanedTitle,
						Duration: duration,
						Position: pos,
					})
				}
			}
		}
	}

	log.Printf("WIKI: Parsed %d tracks total from templates", len(tracks))

	if len(tracks) == 0 {
		tableRe := regexp.MustCompile(`(?m)^\|\s*\d+\.?\s*\|\|?\s*"?([^"|]+)"?\s*\|\|?\s*(\d+:\d+)`)
		for _, match := range tableRe.FindAllStringSubmatch(content, -1) {
			if len(match) >= 3 {
				title := strings.TrimSpace(match[1])
				title = c.cleanWikiMarkup(title)
				duration := c.parseDurationString(match[2])
				if duration > 0 {
					tracks = append(tracks, TrackInfo{
						Title:    title,
						Duration: duration,
						Position: len(tracks) + 1,
					})
				}
			}
		}
	}

	return tracks
}

func (c *WikipediaClient) parseDurationString(s string) int {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0
	}

	mins, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	secs, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil || err2 != nil {
		return 0
	}

	return mins*60 + secs
}

func (c *WikipediaClient) cleanWikiMarkup(s string) string {
	// Process complete wiki links first: [[target|display]] or [[target]]
	for strings.Contains(s, "[[") && strings.Contains(s, "]]") {
		start := strings.Index(s, "[[")
		end := strings.Index(s, "]]")
		if start == -1 || end == -1 || end <= start {
			break
		}
		linkContent := s[start+2 : end]
		if idx := strings.Index(linkContent, "|"); idx != -1 {
			linkContent = linkContent[idx+1:] // Use display text after |
		}
		s = s[:start] + linkContent + s[end+2:]
	}

	// Handle incomplete wiki links (no closing ]]) - strip [[ prefix
	if strings.HasPrefix(s, "[[") {
		s = s[2:]
		// If there's a pipe from an incomplete link, take text after it
		if idx := strings.Index(s, "|"); idx != -1 && !strings.Contains(s[:idx], "]]") {
			s = s[idx+1:]
		}
	}

	// Handle [[ anywhere in the middle of the string (incomplete links)
	for strings.Contains(s, "[[") {
		start := strings.Index(s, "[[")
		// Check if there's a matching ]]
		remaining := s[start:]
		if !strings.Contains(remaining, "]]") {
			// Incomplete link - extract content
			content := s[start+2:]
			if pipeIdx := strings.Index(content, "|"); pipeIdx != -1 {
				content = content[pipeIdx+1:]
			}
			s = s[:start] + content
			break
		} else {
			// There's a ]] but the earlier loop didn't process it - shouldn't happen
			break
		}
	}

	s = strings.ReplaceAll(s, "'''", "")
	s = strings.ReplaceAll(s, "''", "")

	templateRe := regexp.MustCompile(`\{\{[^}]+\}\}`)
	s = templateRe.ReplaceAllString(s, "")

	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	s = brRe.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

func (c *WikipediaClient) findMatchingTrack(tracks []TrackInfo, searchTitle string) *TrackInfo {
	var bestMatch *TrackInfo
	var bestScore float64 = 0
	searchTitleLower := strings.ToLower(searchTitle)

	for i := range tracks {
		trackTitle := tracks[i].Title
		trackTitleLower := strings.ToLower(trackTitle)
		score := stringSimilarity(searchTitle, trackTitle)

		if strings.HasPrefix(trackTitleLower, searchTitleLower) {
			score = 1.0
		} else if strings.Contains(trackTitleLower, searchTitleLower) {
			score = 0.9
		}

		if score > bestScore && score >= 0.6 {
			bestScore = score
			bestMatch = &tracks[i]
		}
	}

	return bestMatch
}
