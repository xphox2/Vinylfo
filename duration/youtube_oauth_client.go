package duration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"vinylfo/models"
	"vinylfo/utils"

	"gorm.io/gorm"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleRevokeURL   = "https://oauth2.googleapis.com/revoke"
	youtubeAPIBaseURL = "https://www.googleapis.com/youtube/v3"
	oauthRateLimit    = 100
	oauthScopes       = "https://www.googleapis.com/auth/youtube https://www.googleapis.com/auth/youtube.force-ssl"
)

type YouTubeOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type YouTubeOAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	CreatedAt    int64  `json:"created_at"`
}

type YouTubeOAuthClient struct {
	*BaseClient
	db     *gorm.DB
	config *YouTubeOAuthConfig
}

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
	Title       string `json:"title"`
	Description string `json:"description"`
	Position    int    `json:"position"`
	VideoID     string `json:"resourceId"`
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

func NewYouTubeOAuthClient(db *gorm.DB) *YouTubeOAuthClient {
	userAgent := "Vinylfo/1.0 (github.com/xphox2/Vinylfo)"

	clientID := os.Getenv("YOUTUBE_CLIENT_ID")
	clientSecret := os.Getenv("YOUTUBE_CLIENT_SECRET")
	redirectURL := os.Getenv("YOUTUBE_REDIRECT_URL")

	var config *YouTubeOAuthConfig
	if clientID != "" && clientSecret != "" {
		if redirectURL == "" {
			redirectURL = "http://localhost:8080/api/youtube/oauth/callback"
		}
		config = &YouTubeOAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
		}
	}

	return &YouTubeOAuthClient{
		BaseClient: NewBaseClient(userAgent, oauthRateLimit),
		db:         db,
		config:     config,
	}
}

func (c *YouTubeOAuthClient) IsConfigured() bool {
	return c.config != nil && c.config.ClientID != "" && c.config.ClientSecret != "" && c.config.RedirectURL != ""
}

func (c *YouTubeOAuthClient) IsAuthenticated() bool {
	if c.db == nil {
		return false
	}

	type YouTubeConfig struct {
		Connected   bool
		AccessToken string
	}
	var result YouTubeConfig
	err := c.db.Raw("SELECT youtube_connected as connected, youtube_access_token as access_token FROM app_configs WHERE id = 1").Scan(&result).Error
	if err != nil {
		return false
	}

	return result.AccessToken != "" && result.Connected
}

func (c *YouTubeOAuthClient) getToken() (*YouTubeOAuthToken, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get app config: %w", err)
	}

	if config.YouTubeAccessToken == "" {
		return nil, fmt.Errorf("no OAuth token available")
	}

	encryptedAccessToken := config.YouTubeAccessToken
	encryptedRefreshToken := config.YouTubeRefreshToken

	accessToken, err := utils.Decrypt(encryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	var refreshToken string
	if encryptedRefreshToken != "" {
		refreshToken, err = utils.Decrypt(encryptedRefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
		}
	}

	token := &YouTubeOAuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(time.Until(config.YouTubeTokenExpiry).Seconds()),
		CreatedAt:    config.YouTubeTokenExpiry.Add(-time.Duration(tokenExpiryWindow) * time.Second).Unix(),
	}

	return token, nil
}

func (c *YouTubeOAuthClient) saveToken(token *YouTubeOAuthToken) error {
	if c.db == nil {
		return fmt.Errorf("database not available")
	}

	encryptedAccessToken, err := utils.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	var encryptedRefreshToken string
	if token.RefreshToken != "" {
		encryptedRefreshToken, err = utils.Encrypt(token.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	expiry := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	result := c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"youtube_access_token":  encryptedAccessToken,
		"youtube_refresh_token": encryptedRefreshToken,
		"youtube_token_expiry":  expiry,
		"youtube_connected":     true,
	})

	if result.Error != nil {
		return fmt.Errorf("failed to save token: %w", result.Error)
	}

	return nil
}

func (c *YouTubeOAuthClient) deleteToken() error {
	if c.db == nil {
		return fmt.Errorf("database not available")
	}

	result := c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"youtube_access_token":  "",
		"youtube_refresh_token": "",
		"youtube_token_expiry":  time.Time{},
		"youtube_connected":     false,
	})

	if result.Error != nil {
		return fmt.Errorf("failed to delete token: %w", result.Error)
	}

	return nil
}

const tokenExpiryWindow = 300 // Refresh token 5 minutes before expiry

func (c *YouTubeOAuthClient) ensureValidToken() error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	if time.Now().Add(time.Duration(tokenExpiryWindow) * time.Second).After(time.Unix(token.CreatedAt, 0).Add(time.Duration(token.ExpiresIn) * time.Second)) {
		if err := c.RefreshToken(); err != nil {
			return fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	return nil
}

func (c *YouTubeOAuthClient) GetAuthURL(state, codeChallenge string) (string, error) {
	if !c.IsConfigured() {
		return "", fmt.Errorf("OAuth not configured - missing client ID, secret, or redirect URL")
	}

	params := url.Values{}
	params.Set("client_id", c.config.ClientID)
	params.Set("redirect_uri", c.config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", oauthScopes)
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	return googleAuthURL + "?" + params.Encode(), nil
}

func (c *YouTubeOAuthClient) ExchangeCode(code, state, codeVerifier string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("OAuth not configured")
	}

	if codeVerifier != "" {
		valid, err := utils.ValidatePKCEState(state, codeVerifier)
		if err != nil || !valid {
			return fmt.Errorf("PKCE validation failed: %w", err)
		}
	}

	data := url.Values{}
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", c.config.RedirectURL)
	data.Set("grant_type", "authorization_code")
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequest("POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token exchange error: %d - %s", resp.StatusCode, string(body))
	}

	var token YouTubeOAuthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	return c.saveToken(&token)
}

func (c *YouTubeOAuthClient) RefreshToken() error {
	token, err := c.getToken()
	if err != nil {
		return err
	}

	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("refresh_token", token.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh error: %d - %s", resp.StatusCode, string(body))
	}

	var newToken YouTubeOAuthToken
	if err := json.Unmarshal(body, &newToken); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	newToken.RefreshToken = token.RefreshToken
	return c.saveToken(&newToken)
}

func (c *YouTubeOAuthClient) RevokeToken() error {
	token, err := c.getToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", googleRevokeURL+"?token="+url.QueryEscape(token.AccessToken), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("token revocation failed: %w", err)
	}
	defer resp.Body.Close()

	c.deleteToken()

	return nil
}

func (c *YouTubeOAuthClient) makeAuthenticatedRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	token, _ := c.getToken()

	c.RateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.RefreshToken(); err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
		return c.makeAuthenticatedRequest(ctx, method, url, body)
	}

	return resp, nil
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
	if err := c.ensureValidToken(); err != nil {
		return err
	}

	url := youtubeAPIBaseURL + "/playlists?id=" + url.QueryEscape(playlistID)
	resp, err := c.makeAuthenticatedRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
