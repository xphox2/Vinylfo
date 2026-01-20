package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"vinylfo/duration"
	"vinylfo/models"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type YouTubeController struct {
	db    *gorm.DB
	oauth *duration.YouTubeOAuthClient
}

func NewYouTubeController(db *gorm.DB) *YouTubeController {
	return &YouTubeController{
		db:    db,
		oauth: duration.NewYouTubeOAuthClient(db),
	}
}

func (c *YouTubeController) GetOAuthURL(ctx *gin.Context) {
	state, codeVerifier, codeChallenge, err := utils.CreatePKCEState()
	if err != nil {
		utils.LogSecurityEvent("pkce_error", ctx.ClientIP(), ctx.GetHeader("User-Agent"), "oauth", err.Error())
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.SetCookie("youtube_oauth_state", state, 300, "/", "", false, true)
	ctx.SetCookie("youtube_oauth_code_verifier", codeVerifier, 300, "/", "", false, true)

	authURL, err := c.oauth.GetAuthURL(state, codeChallenge)
	if err != nil {
		utils.LogSecurityEvent("oauth_config_error", ctx.ClientIP(), ctx.GetHeader("User-Agent"), "oauth", err.Error())
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	utils.LogOAuthEvent(models.AuditActionConnect, 0, ctx.ClientIP(), ctx.GetHeader("User-Agent"), true, map[string]interface{}{
		"action": "initiate_oauth",
	})

	ctx.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

func (c *YouTubeController) OAuthCallback(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	errorParam := ctx.Query("error")

	if errorParam != "" {
		utils.LogOAuthEvent(models.AuditActionConnect, 0, ctx.ClientIP(), ctx.GetHeader("User-Agent"), false, map[string]interface{}{
			"action": "oauth_callback",
			"error":  errorParam,
		})
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, oauthErrorHTML("Authorization denied: "+errorParam))
		return
	}

	if code == "" {
		utils.LogSecurityEvent("missing_code", ctx.ClientIP(), ctx.GetHeader("User-Agent"), "oauth", "No authorization code received")
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusBadRequest, oauthErrorHTML("No authorization code received"))
		return
	}

	storedState, err := ctx.Cookie("youtube_oauth_state")
	if err != nil || storedState == "" {
		utils.LogSecurityEvent("missing_state", ctx.ClientIP(), ctx.GetHeader("User-Agent"), "oauth", "No state cookie found")
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusBadRequest, oauthErrorHTML("No state cookie found. Please try again."))
		return
	}

	if state != storedState {
		utils.LogSecurityEvent("state_mismatch", ctx.ClientIP(), ctx.GetHeader("User-Agent"), "oauth", "State mismatch")
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusBadRequest, oauthErrorHTML("State mismatch. Please try again."))
		return
	}

	codeVerifier, _ := ctx.Cookie("youtube_oauth_code_verifier")

	if err := c.oauth.ExchangeCode(code, state, codeVerifier); err != nil {
		utils.LogOAuthEvent(models.AuditActionConnect, 0, ctx.ClientIP(), ctx.GetHeader("User-Agent"), false, map[string]interface{}{
			"action": "exchange_code",
			"error":  err.Error(),
		})
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusInternalServerError, oauthErrorHTML("Failed to exchange code: "+err.Error()))
		return
	}

	utils.LogOAuthEvent(models.AuditActionConnect, 1, ctx.ClientIP(), ctx.GetHeader("User-Agent"), true, map[string]interface{}{
		"action": "oauth_success",
	})

	ctx.SetCookie("youtube_oauth_state", "", -1, "/", "", false, true)
	ctx.SetCookie("youtube_oauth_code_verifier", "", -1, "/", "", false, true)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, oauthSuccessHTML)
}

func (c *YouTubeController) Disconnect(ctx *gin.Context) {
	if err := c.oauth.RevokeToken(); err != nil {
		utils.LogOAuthEvent(models.AuditActionDisconnect, 1, ctx.ClientIP(), ctx.GetHeader("User-Agent"), false, map[string]interface{}{
			"action": "revoke_token",
			"error":  err.Error(),
		})
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect: " + err.Error()})
		return
	}

	utils.LogOAuthEvent(models.AuditActionDisconnect, 1, ctx.ClientIP(), ctx.GetHeader("User-Agent"), true, map[string]interface{}{
		"action": "disconnect",
	})

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Successfully disconnected from YouTube",
		"connected": false,
	})
}

func (c *YouTubeController) GetStatus(ctx *gin.Context) {
	var config models.AppConfig
	if err := c.db.First(&config).Error; err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch config"})
		return
	}

	status := gin.H{
		"connected":     c.oauth.IsAuthenticated(),
		"is_configured": c.oauth.IsConfigured(),
		"db_connected":  config.YouTubeConnected,
		"has_token":     config.YouTubeAccessToken != "",
	}

	ctx.JSON(http.StatusOK, status)
}

func (c *YouTubeController) CreatePlaylist(ctx *gin.Context) {
	var input struct {
		Title         string `json:"title" binding:"required"`
		Description   string `json:"description"`
		PrivacyStatus string `json:"privacy_status"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	privacyStatus := "private"
	if input.PrivacyStatus != "" {
		privacyStatus = input.PrivacyStatus
	}

	playlist, err := c.oauth.CreatePlaylist(ctx.Request.Context(), input.Title, input.Description, privacyStatus)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"id":             playlist.ID,
		"title":          playlist.Snippet.Title,
		"description":    playlist.Snippet.Description,
		"privacy_status": playlist.Status.PrivacyStatus,
	})
}

func (c *YouTubeController) UpdatePlaylist(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	var input struct {
		Title         string `json:"title"`
		Description   string `json:"description"`
		PrivacyStatus string `json:"privacy_status"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := input.Title
	description := input.Description
	privacyStatus := input.PrivacyStatus

	if title == "" && description == "" && privacyStatus == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "At least one field to update is required"})
		return
	}

	if err := c.oauth.UpdatePlaylist(ctx.Request.Context(), playlistID, title, description, privacyStatus); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Playlist updated successfully"})
}

func (c *YouTubeController) GetPlaylists(ctx *gin.Context) {
	maxResults := ctx.DefaultQuery("max_results", "50")
	maxResultsInt := 50
	if _, err := fmt.Sscanf(maxResults, "%d", &maxResultsInt); err != nil || maxResultsInt <= 0 || maxResultsInt > 50 {
		maxResultsInt = 50
	}

	playlists, err := c.oauth.GetPlaylists(ctx.Request.Context(), maxResultsInt)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]gin.H, 0, len(playlists.Items))
	for _, item := range playlists.Items {
		result = append(result, gin.H{
			"id":             item.ID,
			"title":          item.Snippet.Title,
			"description":    item.Snippet.Description,
			"privacy_status": item.Status.PrivacyStatus,
			"channel_id":     item.Snippet.ChannelID,
			"content_details": gin.H{
				"item_count": item.ContentDetails.ItemCount,
			},
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"items":       result,
		"total_count": playlists.PageInfo.TotalResults,
	})
}

func (c *YouTubeController) GetPlaylistItems(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	var input struct {
		MaxResults int `json:"max_results"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		input.MaxResults = 50
	}

	if input.MaxResults <= 0 || input.MaxResults > 50 {
		input.MaxResults = 50
	}

	items, err := c.oauth.GetPlaylistItems(ctx.Request.Context(), playlistID, input.MaxResults)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]gin.H, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, gin.H{
			"id":          item.ID,
			"title":       item.Snippet.Title,
			"description": item.Snippet.Description,
			"position":    item.Snippet.Position,
			"video_id":    item.Snippet.VideoID,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"items":         result,
		"playlist_id":   playlistID,
		"total_results": items.PageInfo.TotalResults,
	})
}

func (c *YouTubeController) AddTrackToPlaylist(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	var input struct {
		VideoID  string `json:"video_id" binding:"required"`
		Position int    `json:"position"`
		TrackID  uint   `json:"track_id"`
		AlbumID  uint   `json:"album_id"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	position := input.Position
	if position < 0 {
		position = 0
	}

	if err := c.oauth.AddVideoToPlaylist(ctx.Request.Context(), playlistID, input.VideoID, position); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if input.TrackID > 0 {
		c.updateTrackYouTubeInfo(input.TrackID, input.VideoID)
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":     "Video added to playlist successfully",
		"playlist_id": playlistID,
		"video_id":    input.VideoID,
	})
}

func (c *YouTubeController) RemoveTrackFromPlaylist(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	itemID := ctx.Param("item_id")

	if playlistID == "" || itemID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID and Item ID are required"})
		return
	}

	if err := c.oauth.RemoveVideoFromPlaylist(ctx.Request.Context(), itemID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "Video removed from playlist successfully",
		"playlist_id": playlistID,
		"item_id":     itemID,
	})
}

func (c *YouTubeController) DeletePlaylist(ctx *gin.Context) {
	playlistID := ctx.Param("id")
	if playlistID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	if err := c.oauth.DeletePlaylist(ctx.Request.Context(), playlistID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "Playlist deleted successfully",
		"playlist_id": playlistID,
	})
}

func (c *YouTubeController) SearchVideos(ctx *gin.Context) {
	var input struct {
		Query      string `json:"query" binding:"required"`
		MaxResults int    `json:"max_results"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.MaxResults <= 0 || input.MaxResults > 10 {
		input.MaxResults = 5
	}

	results, err := c.oauth.SearchVideos(ctx.Request.Context(), input.Query, input.MaxResults)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	videos := make([]gin.H, 0, len(results.Items))
	for _, item := range results.Items {
		thumbnails := item.Snippet.Thumbnails
		thumbnailURL := ""
		if thumbnails != nil {
			if thumb, ok := thumbnails["default"]; ok {
				thumbnailURL = thumb.URL
			} else if thumb, ok := thumbnails["medium"]; ok {
				thumbnailURL = thumb.URL
			} else if thumb, ok := thumbnails["high"]; ok {
				thumbnailURL = thumb.URL
			}
		}
		videos = append(videos, gin.H{
			"video_id":  item.ID.VideoID,
			"title":     item.Snippet.Title,
			"channel":   item.Snippet.ChannelTitle,
			"thumbnail": thumbnailURL,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"videos":        videos,
		"total_results": results.PageInfo.TotalResults,
	})
}

func (c *YouTubeController) ExportPlaylist(ctx *gin.Context) {
	var input struct {
		SessionID     string `json:"session_id" binding:"required"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		PrivacyStatus string `json:"privacy_status"`
	}

	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var session models.PlaybackSession
	if err := c.db.First(&session, "playlist_id = ?", input.SessionID).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	title := input.Title
	if title == "" {
		title = session.PlaylistName
	}
	if title == "" {
		title = "Vinylfo Playlist"
	}

	privacyStatus := input.PrivacyStatus
	if privacyStatus == "" {
		privacyStatus = "private"
	}

	playlist, err := c.oauth.CreatePlaylist(ctx.Request.Context(), title, input.Description, privacyStatus)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create playlist: " + err.Error()})
		return
	}

	var trackIDs []uint
	if session.Queue != "" {
		if err := json.Unmarshal([]byte(session.Queue), &trackIDs); err != nil {
			trackIDs = []uint{}
		}
	}

	successCount := 0
	failCount := 0
	for i, trackID := range trackIDs {
		var track models.Track
		if err := c.db.First(&track, trackID).Error; err != nil {
			failCount++
			continue
		}

		searchQuery := track.Title
		results, err := c.oauth.SearchVideos(ctx.Request.Context(), searchQuery, 1)
		if err != nil || len(results.Items) == 0 {
			failCount++
			continue
		}

		videoID := results.Items[0].ID.VideoID
		if err := c.oauth.AddVideoToPlaylist(ctx.Request.Context(), playlist.ID, videoID, i); err != nil {
			failCount++
			continue
		}
		successCount++
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":       "Playlist exported successfully",
		"playlist_id":   playlist.ID,
		"playlist_url":  "https://www.youtube.com/playlist?list=" + playlist.ID,
		"total_tracks":  len(trackIDs),
		"success_count": successCount,
		"fail_count":    failCount,
	})
}

func (c *YouTubeController) updateTrackYouTubeInfo(trackID uint, videoID string) {
}

// Note: generateSecureState and randomString are deprecated in favor of PKCE.
// State is now generated securely via utils.CreatePKCEState() using crypto/rand.

const oauthSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta http-equiv="refresh" content="3;url=/settings">
    <title>YouTube Connected - Vinylfo</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
               display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0;
               background-color: #f5f5f5; }
        .container { text-align: center; padding: 2rem; background: white; border-radius: 8px; 
                     box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .success { color: #28a745; font-size: 48px; margin-bottom: 1rem; }
        h1 { color: #333; margin-bottom: 0.5rem; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="success">&#10004;</div>
        <h1>YouTube Connected!</h1>
        <p>Redirecting to settings...</p>
    </div>
</body>
</html>`

func oauthErrorHTML(message string) string {
	// HTML-escape the message to prevent XSS
	escapedMessage := htmlEscapeString(message)
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta http-equiv="refresh" content="5;url=/settings">
    <title>Error - Vinylfo</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
               display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0;
               background-color: #f5f5f5; }
        .container { text-align: center; padding: 2rem; background: white; border-radius: 8px;
                     box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .error { color: #dc3545; font-size: 48px; margin-bottom: 1rem; }
        h1 { color: #333; margin-bottom: 0.5rem; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="error">&#10006;</div>
        <h1>Connection Failed</h1>
        <p>` + escapedMessage + `</p>
        <p>Redirecting to settings...</p>
    </div>
</body>
</html>`
}

// htmlEscapeString escapes special HTML characters to prevent XSS
func htmlEscapeString(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}
