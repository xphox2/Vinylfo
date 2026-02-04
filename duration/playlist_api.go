package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const youtubeAPIBaseURL = "https://www.googleapis.com/youtube/v3"

type playlistResponse struct {
	Kind           string                 `json:"kind"`
	ETag           string                 `json:"etag"`
	ID             string                 `json:"id"`
	Snippet        playlistSnippet        `json:"snippet"`
	Status         playlistStatus         `json:"status"`
	ContentDetails playlistContentDetails `json:"contentDetails"`
}

type playlistContentDetails struct {
	ItemCount int `json:"itemCount"`
}

type playlistSnippet struct {
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	PublishedAt  time.Time `json:"publishedAt"`
	ChannelID    string    `json:"channelId"`
	ChannelTitle string    `json:"channelTitle"`
}

type playlistStatus struct {
	PrivacyStatus string `json:"privacyStatus"`
}

type playlistListResponse struct {
	Kind     string `json:"kind"`
	ETag     string `json:"etag"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
	Items []playlistResponse `json:"items"`
}

type playlistItemResponse struct {
	Kind    string              `json:"kind"`
	ETag    string              `json:"etag"`
	ID      string              `json:"id"`
	Snippet playlistItemSnippet `json:"snippet"`
}

type playlistItemSnippet struct {
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Position     int        `json:"position"`
	ResourceID   resourceId `json:"resourceId"`
	ChannelTitle string     `json:"videoOwnerChannelTitle"`
}

type resourceId struct {
	Kind    string `json:"kind"`
	VideoID string `json:"videoId"`
}

type playlistItemListResponse struct {
	Kind     string `json:"kind"`
	ETag     string `json:"etag"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
	Items []playlistItemResponse `json:"items"`
}

func (c *YouTubeOAuthClient) CreatePlaylist(ctx context.Context, title, description string, privacyStatus string) (*playlistResponse, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"snippet": map[string]string{
			"title":       title,
			"description": description,
		},
		"status": map[string]string{
			"privacyStatus": privacyStatus,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := youtubeAPIBaseURL + "/playlists?part=snippet,status"
	resp, err := c.makeAuthenticatedRequest(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create playlist: %d - %s", resp.StatusCode, string(respBody))
	}

	var playlist playlistResponse
	if err := json.NewDecoder(resp.Body).Decode(&playlist); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &playlist, nil
}

func (c *YouTubeOAuthClient) UpdatePlaylist(ctx context.Context, playlistID string, title, description string, privacyStatus string) error {
	if err := c.ensureValidToken(); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"id": playlistID,
		"snippet": map[string]string{
			"title":       title,
			"description": description,
		},
		"status": map[string]string{
			"privacyStatus": privacyStatus,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := youtubeAPIBaseURL + "/playlists?part=snippet,status"
	resp, err := c.makeAuthenticatedRequest(ctx, "PUT", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update playlist: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *YouTubeOAuthClient) GetPlaylists(ctx context.Context, maxResults int) (*playlistListResponse, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/playlists?part=snippet,status&mine=true&maxResults=%d", youtubeAPIBaseURL, maxResults)
	resp, err := c.makeAuthenticatedRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get playlists: %d - %s", resp.StatusCode, string(respBody))
	}

	var playlists playlistListResponse
	if err := json.NewDecoder(resp.Body).Decode(&playlists); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &playlists, nil
}

func (c *YouTubeOAuthClient) GetPlaylistItems(ctx context.Context, playlistID string, maxResults int) (*playlistItemListResponse, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/playlistItems?part=snippet&playlistId=%s&maxResults=%d", youtubeAPIBaseURL, url.QueryEscape(playlistID), maxResults)
	resp, err := c.makeAuthenticatedRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get playlist items: %d - %s", resp.StatusCode, string(respBody))
	}

	var items playlistItemListResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &items, nil
}

func (c *YouTubeOAuthClient) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string, position int) error {
	if err := c.ensureValidToken(); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"snippet": map[string]interface{}{
			"playlistId": playlistID,
			"position":   position,
			"resourceId": map[string]string{
				"kind":    "youtube#video",
				"videoId": videoID,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := youtubeAPIBaseURL + "/playlistItems?part=snippet"
	resp, err := c.makeAuthenticatedRequest(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add video to playlist: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *YouTubeOAuthClient) RemoveVideoFromPlaylist(ctx context.Context, playlistItemID string) error {
	if err := c.ensureValidToken(); err != nil {
		return err
	}

	url := youtubeAPIBaseURL + "/playlistItems?id=" + url.QueryEscape(playlistItemID)
	resp, err := c.makeAuthenticatedRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove video from playlist: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *YouTubeOAuthClient) DeletePlaylist(ctx context.Context, playlistID string) error {
	fmt.Printf("DEBUG: DeletePlaylist called with playlistID: %s\n", playlistID)

	if err := c.ensureValidToken(); err != nil {
		fmt.Printf("DEBUG: ensureValidToken failed: %v\n", err)
		return err
	}
	fmt.Printf("DEBUG: Token is valid\n")

	url := youtubeAPIBaseURL + "/playlists?id=" + url.QueryEscape(playlistID)
	fmt.Printf("DEBUG: DELETE URL: %s\n", url)

	resp, err := c.makeAuthenticatedRequest(ctx, "DELETE", url, nil)
	if err != nil {
		fmt.Printf("DEBUG: makeAuthenticatedRequest failed: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("DEBUG: DELETE response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete playlist: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *YouTubeOAuthClient) SearchVideos(ctx context.Context, query string, maxResults int) (*youtubeSearchResponse, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/search?part=snippet&type=video&q=%s&maxResults=%d", youtubeAPIBaseURL, url.QueryEscape(query), maxResults)

	c.RateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	token, _ := c.getToken()
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var searchResp youtubeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &searchResp, nil
}
