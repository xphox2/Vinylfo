package routes

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"vinylfo/controllers"
	"vinylfo/database"
	"vinylfo/duration"
	"vinylfo/utils"

	"github.com/gin-gonic/gin"
)

// Version is set at build time using: go build -ldflags "-X vinylfo/routes.Version=v0.1.0-alpha"
var Version = "dev"

func CSPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/feeds/art" || c.Request.URL.Path == "/feeds/track" {
			c.Header("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"connect-src 'self'; "+
					"frame-ancestors *")
			c.Header("Cache-Control", "no-store")
		} else if c.Request.URL.Path == "/feeds/video" || c.Request.URL.Path == "/feeds/video/events" {
			c.Header("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' https://apis.google.com https://accounts.google.com https://www.youtube.com https://s.ytimg.com; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"connect-src 'self' https://www.googleapis.com https://oauth2.googleapis.com https://www.youtube.com; "+
					"frame-src https://accounts.google.com https://www.youtube.com https://www.youtube-nocookie.com; "+
					"media-src 'self' https://www.youtube.com; "+
					"frame-ancestors *")
			c.Header("Cache-Control", "no-store")
		} else {
			c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://apis.google.com https://accounts.google.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://www.googleapis.com https://oauth2.googleapis.com; frame-src https://accounts.google.com")
			c.Header("X-Frame-Options", "DENY")
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}

func SetupRoutes(r *gin.Engine) {
	db := database.GetDB()

	playbackController := controllers.NewPlaybackController(db)
	albumController := controllers.NewAlbumController(db, playbackController.BroadcastState)
	trackController := controllers.NewTrackController(db)
	playlistController := controllers.NewPlaylistController(db)
	sessionSharingController := controllers.NewSessionSharingController(db)
	sessionNoteController := controllers.NewSessionNoteController(db)
	discogsController := controllers.NewDiscogsController(db)
	settingsController := controllers.NewSettingsController(db)

	r.Use(CSPMiddleware())

	// Serve favicon
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.File("./icons/vinyl-icon.ico")
	})

	// Version endpoint for bug reports and diagnostics
	r.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":   Version,
			"db_type":   database.DBType,
			"timestamp": time.Now().Unix(),
		})
	})

	// Config endpoint for frontend (version info for footer, etc.)
	r.GET("/api/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version": Version,
		})
	})

	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{
				"status":    "unhealthy",
				"error":     "database connection error",
				"timestamp": time.Now().Unix(),
			})
			return
		}

		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(pingCtx); err != nil {
			c.JSON(503, gin.H{
				"status":    "unhealthy",
				"error":     "database ping failed",
				"timestamp": time.Now().Unix(),
			})
			return
		}

		c.JSON(200, gin.H{
			"status":    "healthy",
			"database":  "connected",
			"db_type":   database.DBType,
			"timestamp": time.Now().Unix(),
		})
	})

	r.GET("/albums", albumController.GetAlbums)
	r.GET("/albums/search", albumController.SearchAlbums)
	r.GET("/albums/:id", albumController.GetAlbumByID)
	r.GET("/albums/:id/image", albumController.GetAlbumImage)
	r.GET("/albums/:id/tracks", albumController.GetTracksByAlbumID)
	r.GET("/albums/:id/delete-preview", albumController.DeleteAlbumPreview)
	r.POST("/albums/:id/image", albumController.UpdateAlbumImage)
	r.POST("/albums", albumController.CreateAlbum)
	r.PUT("/albums/:id", albumController.UpdateAlbum)
	r.DELETE("/albums/:id", albumController.DeleteAlbum)

	r.GET("/tracks", trackController.GetTracks)
	r.GET("/tracks/search", trackController.SearchTracks)
	r.GET("/tracks/:id", trackController.GetTrackByID)
	r.POST("/tracks", trackController.CreateTrack)
	r.PUT("/tracks/:id", trackController.UpdateTrack)
	r.PUT("/tracks/:id/youtube", trackController.SetYouTubeVideo)
	r.DELETE("/tracks/:id/youtube", trackController.DeleteYouTubeVideo)
	r.DELETE("/tracks/:id", trackController.DeleteTrack)
	r.GET("/api/debug/youtube-matches", trackController.DebugYouTubeMatches)

	r.GET("/playback", playbackController.GetCurrent)
	r.GET("/playback/current", playbackController.GetPlaybackState)
	r.GET("/playback/events", playbackController.StreamEvents)
	r.POST("/playback/start", playbackController.Start)
	r.POST("/playback/start-playlist", playbackController.StartPlaylist)
	r.POST("/playback/pause", playbackController.Pause)
	r.POST("/playback/resume", playbackController.Resume)
	r.POST("/playback/skip", playbackController.Skip)
	r.POST("/playback/play-index", playbackController.PlayIndex)
	r.POST("/playback/previous", playbackController.Previous)
	r.POST("/playback/stop", playbackController.Stop)
	r.POST("/playback/restore", playbackController.RestoreSession)
	r.POST("/playback/clear", playbackController.Clear)
	r.POST("/playback/update-progress", playbackController.UpdateProgress)
	r.POST("/playback/seek", playbackController.Seek)
	r.GET("/playback/state", playbackController.GetPlaybackState)
	r.GET("/playback/history", playbackController.GetHistory)
	r.GET("/playback/history/most-played", playbackController.GetMostPlayed)
	r.GET("/playback/history/recent", playbackController.GetRecent)
	r.GET("/playback/history/:track_id", playbackController.GetTrackHistory)
	r.POST("/playback/update-history", playbackController.UpdateHistory)

	// Video Feed for OBS streaming
	videoFeedController := controllers.NewVideoFeedController(db, playbackController, duration.NewYouTubeOAuthClient(db))
	r.GET("/feeds/video", videoFeedController.GetVideoFeedPage)
	r.GET("/feeds/video/events", videoFeedController.StreamEvents)
	r.GET("/playback/current-youtube", videoFeedController.GetCurrentYouTubeVideo)
	r.GET("/playback/next-preload", videoFeedController.GetNextTrackPreload)
	r.POST("/playback/video/play", videoFeedController.Play)
	r.POST("/playback/video/pause", videoFeedController.Pause)
	r.POST("/playback/video/stop", videoFeedController.Stop)
	r.POST("/playback/video/next", videoFeedController.Next)
	r.POST("/playback/video/previous", videoFeedController.Previous)
	r.POST("/playback/video/seek", videoFeedController.Seek)
	r.GET("/playback/video/youtube-duration", videoFeedController.GetYouTubeVideoDuration)
	r.POST("/playback/video/refresh-duration", videoFeedController.RefreshYouTubeDuration)
	r.POST("/playback/video/refresh-all-durations", videoFeedController.RefreshAllYouTubeDurations)

	// Album Art Feed for OBS streaming
	albumArtFeedController := controllers.NewAlbumArtFeedController()
	r.GET("/feeds/art", albumArtFeedController.GetAlbumArtFeedPage)

	// Track Info Feed for OBS streaming
	trackFeedController := controllers.NewTrackFeedController()
	r.GET("/feeds/track", trackFeedController.GetTrackFeedPage)

	r.GET("/sessions", playlistController.GetSessions)
	r.GET("/playback-sessions/:id", playlistController.GetSessionByID)
	r.POST("/sessions", playlistController.CreateSession)
	r.PUT("/playback-sessions/:id", playlistController.UpdateSession)
	r.DELETE("/playback-sessions/:id", playlistController.DeleteSession)
	r.POST("/sessions/playlist", playlistController.CreatePlaylistSession)
	r.GET("/sessions/playlist", playlistController.GetAllPlaylists)
	r.POST("/sessions/playlist/new", playlistController.CreateNewPlaylist)
	r.GET("/sessions/playlist/:id", playlistController.GetPlaylist)
	r.PUT("/sessions/playlist/:id", playlistController.UpdatePlaylist)
	r.DELETE("/sessions/playlist/:id", playlistController.DeletePlaylist)
	r.DELETE("/sessions/playlist/:id/delete-all", playlistController.DeletePlaylistWithSessions)
	r.POST("/sessions/playlist/:id/tracks", playlistController.AddTrackToPlaylist)
	r.DELETE("/sessions/playlist/:id/tracks/:track_id", playlistController.RemoveTrackFromPlaylist)
	r.POST("/sessions/playlist/:id/shuffle", playlistController.ShufflePlaylist)

	r.POST("/sessions/:session_id/share", sessionSharingController.CreateSessionSharing)
	r.GET("/sessions/:session_id/share", sessionSharingController.GetSessionSharing)
	r.PUT("/sessions/:session_id/share", sessionSharingController.UpdateSessionSharing)
	r.DELETE("/sessions/:session_id/share", sessionSharingController.DeleteSessionSharing)
	r.GET("/share/:token", sessionSharingController.GetPublicSessionSharing)

	r.POST("/sessions/:session_id/notes", sessionNoteController.CreateSessionNote)
	r.GET("/sessions/:session_id/notes", sessionNoteController.GetSessionNotes)
	r.GET("/notes/:id", sessionNoteController.GetSessionNote)
	r.PUT("/notes/:id", sessionNoteController.UpdateSessionNote)
	r.DELETE("/notes/:id", sessionNoteController.DeleteSessionNote)

	r.GET("/api/discogs/oauth/url", discogsController.GetOAuthURL)
	r.GET("/api/discogs/oauth/callback", discogsController.OAuthCallback)
	r.POST("/api/discogs/disconnect", discogsController.Disconnect)
	r.GET("/api/discogs/status", discogsController.GetStatus)
	r.GET("/api/discogs/folders", discogsController.GetFolders)
	r.GET("/api/discogs/search", discogsController.Search)
	r.GET("/api/discogs/albums/:id", discogsController.PreviewAlbum)
	r.POST("/api/discogs/albums", discogsController.CreateAlbum)
	r.POST("/api/discogs/sync/start", discogsController.StartSync)
	r.GET("/api/discogs/sync/progress", discogsController.GetSyncProgress)
	r.GET("/api/discogs/sync/history", discogsController.GetSyncHistory)
	r.GET("/api/discogs/sync/resume", discogsController.ResumeSync)
	r.POST("/api/discogs/sync/pause", discogsController.PauseSync)
	r.POST("/api/discogs/sync/resume-pause", discogsController.ResumeSyncFromPause)
	r.GET("/api/discogs/sync/batch/:id", discogsController.GetBatchDetails)
	r.POST("/api/discogs/sync/batch/:id/confirm", discogsController.ConfirmBatch)
	r.POST("/api/discogs/sync/batch/:id/skip", discogsController.SkipBatch)
	r.POST("/api/discogs/sync/cancel", discogsController.CancelSync)
	r.POST("/api/discogs/fetch-username", discogsController.FetchUsername)
	r.POST("/api/discogs/refresh-tracks", discogsController.RefreshTracks)
	r.GET("/api/discogs/unlinked-albums", discogsController.FindUnlinkedAlbums)
	r.POST("/api/discogs/unlinked-albums/delete", discogsController.DeleteUnlinkedAlbums)
	r.POST("/api/discogs/cleanup-orphaned-tracks", discogsController.CleanupOrphanedTracks)

	r.GET("/api/settings", settingsController.Get)
	r.PUT("/api/settings", settingsController.Update)
	r.POST("/api/database/reset", settingsController.ResetDatabase)
	r.POST("/api/database/seed", settingsController.SeedDatabase)

	r.GET("/api/settings/logs", settingsController.GetLogSettings)
	r.PUT("/api/settings/logs", settingsController.UpdateLogSettings)
	r.POST("/api/settings/logs/cleanup", settingsController.CleanupLogs)

	// Feed settings endpoints
	r.GET("/api/settings/feeds", settingsController.GetFeedSettings)
	r.PUT("/api/settings/feeds", settingsController.UpdateFeedSettings)

	// Log export endpoint for bug reports
	r.GET("/api/logs/export", func(c *gin.Context) {
		zipPath, err := utils.CreateSupportZip("logs", 10)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		filename := filepath.Base(zipPath)
		c.FileAttachment(zipPath, filename)
	})

	r.GET("/api/logs/list", func(c *gin.Context) {
		logs, err := utils.GetLogFiles("logs")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"logs":  logs,
			"count": len(logs),
		})
	})

	r.GET("/api/audit/logs", settingsController.GetAuditLogs)
	r.POST("/api/audit/cleanup", settingsController.CleanupAuditLogs)

	// Database backup endpoints (SQLite only)
	r.POST("/api/database/backup", func(c *gin.Context) {
		if database.DBType != "sqlite" {
			c.JSON(400, gin.H{"error": "Backup is only available for SQLite databases"})
			return
		}

		dbPath := os.Getenv("DB_PATH")
		result, err := utils.BackupDatabase(db, dbPath)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"message":     "Backup created successfully",
			"backup_path": result.BackupPath,
			"size":        result.Size,
			"created_at":  result.CreatedAt,
		})
	})

	r.GET("/api/database/backups", func(c *gin.Context) {
		if database.DBType != "sqlite" {
			c.JSON(400, gin.H{"error": "Backup listing is only available for SQLite databases"})
			return
		}

		dbPath := os.Getenv("DB_PATH")
		backups, err := utils.ListBackups(dbPath)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		count, totalSize, oldest, newest, _ := utils.GetBackupStats(dbPath)

		c.JSON(200, gin.H{
			"backups":       backups,
			"count":         count,
			"total_size":    totalSize,
			"oldest_backup": oldest,
			"newest_backup": newest,
		})
	})

	r.POST("/api/database/backups/cleanup", func(c *gin.Context) {
		if database.DBType != "sqlite" {
			c.JSON(400, gin.H{"error": "Backup cleanup is only available for SQLite databases"})
			return
		}

		var req struct {
			Keep int `json:"keep"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Keep = 5 // Default
		}

		dbPath := os.Getenv("DB_PATH")
		deleted, err := utils.CleanupOldBackups(dbPath, req.Keep)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"message": "Backup cleanup completed",
			"deleted": deleted,
			"kept":    req.Keep,
		})
	})

	durationController := controllers.NewDurationController(db)
	durationReviewController := controllers.NewDurationReviewController(db)

	duration := r.Group("/api/duration")
	{
		duration.GET("/tracks", durationController.GetTracksNeedingResolution)
		duration.GET("/stats", durationController.GetStatistics)

		duration.POST("/track/:id/manual", durationController.SetManualDuration)
		duration.POST("/resolve/track/:id", durationController.ResolveTrack)
		duration.POST("/resolve/track/:id/retry", durationController.RetryFailedTrack)
		duration.POST("/resolve/album/:id", durationController.ResolveAlbum)
		duration.GET("/resolve/track/:id", durationController.GetResolutionStatus)

		duration.POST("/resolve/start", durationController.StartBulkResolution)
		duration.POST("/resolve/pause", durationController.PauseBulkResolution)
		duration.POST("/resolve/resume", durationController.ResumeBulkResolution)
		duration.POST("/resolve/cancel", durationController.CancelBulkResolution)
		duration.GET("/resolve/progress", durationController.GetBulkProgress)

		duration.GET("/review", durationReviewController.GetReviewQueue)
		duration.GET("/review/resolved", durationReviewController.GetResolvedQueue)
		duration.GET("/review/:id", durationReviewController.GetReviewDetails)
		duration.POST("/review/:id", durationReviewController.SubmitReview)
		duration.POST("/review/bulk", durationReviewController.BulkReview)
	}

	youtubeController := controllers.NewYouTubeController(db)
	youtubeSyncController := controllers.NewYouTubeSyncController(db)

	youtube := r.Group("/api/youtube")
	{
		// OAuth
		youtube.GET("/oauth/url", youtubeController.GetOAuthURL)
		youtube.GET("/oauth/callback", youtubeController.OAuthCallback)
		youtube.POST("/disconnect", youtubeController.Disconnect)
		youtube.GET("/status", youtubeController.GetStatus)

		// YouTube Playlists (direct management)
		youtube.POST("/playlists", youtubeController.CreatePlaylist)
		youtube.PUT("/playlists/:id", youtubeController.UpdatePlaylist)
		youtube.GET("/playlists", youtubeController.GetPlaylists)
		youtube.GET("/playlists/:id", youtubeController.GetPlaylistItems)
		youtube.DELETE("/playlists/:id", youtubeController.DeletePlaylist)

		youtube.POST("/playlists/:id/videos", youtubeController.AddTrackToPlaylist)
		youtube.DELETE("/playlists/:id/videos/:item_id", youtubeController.RemoveTrackFromPlaylist)

		youtube.POST("/search", youtubeController.SearchVideos)
		youtube.POST("/export-playlist", youtubeController.ExportPlaylist)

		// YouTube Sync (match local tracks to YouTube videos)
		youtube.POST("/match-track/:track_id", youtubeSyncController.MatchTrack)
		youtube.POST("/match-playlist/:playlist_id", youtubeSyncController.MatchPlaylist)
		youtube.GET("/matches/:playlist_id", youtubeSyncController.GetMatches)
		youtube.GET("/match/:track_id", youtubeSyncController.GetTrackMatch)
		youtube.PUT("/matches/:track_id", youtubeSyncController.UpdateMatch)
		youtube.DELETE("/matches/:track_id", youtubeSyncController.DeleteMatch)
		youtube.POST("/sync-playlist/:playlist_id", youtubeSyncController.SyncPlaylist)
		youtube.GET("/sync-status/:playlist_id", youtubeSyncController.GetSyncStatus)
		youtube.GET("/candidates/:track_id", youtubeSyncController.GetCandidates)
		youtube.POST("/candidates/:track_id/select/:candidate_id", youtubeSyncController.SelectCandidate)
		youtube.POST("/clear-cache", youtubeSyncController.ClearWebCache)
	}
}
