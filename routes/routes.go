package routes

import (
	"context"
	"time"

	"vinylfo/controllers"
	"vinylfo/database"

	"github.com/gin-gonic/gin"
)

func CSPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://apis.google.com https://accounts.google.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://www.googleapis.com https://oauth2.googleapis.com; frame-src https://accounts.google.com")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}

func SetupRoutes(r *gin.Engine) {
	db := database.GetDB()

	albumController := controllers.NewAlbumController(db)
	trackController := controllers.NewTrackController(db)
	playbackController := controllers.NewPlaybackController(db)
	playlistController := controllers.NewPlaylistController(db)
	sessionSharingController := controllers.NewSessionSharingController(db)
	sessionNoteController := controllers.NewSessionNoteController(db)
	discogsController := controllers.NewDiscogsController(db)
	settingsController := controllers.NewSettingsController(db)

	r.Use(CSPMiddleware())
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
			"timestamp": time.Now().Unix(),
		})
	})

	r.GET("/albums", albumController.GetAlbums)
	r.GET("/albums/search", albumController.SearchAlbums)
	r.GET("/albums/:id", albumController.GetAlbumByID)
	r.GET("/albums/:id/image", albumController.GetAlbumImage)
	r.GET("/albums/:id/tracks", albumController.GetTracksByAlbumID)
	r.POST("/albums", albumController.CreateAlbum)
	r.PUT("/albums/:id", albumController.UpdateAlbum)
	r.DELETE("/albums/:id", albumController.DeleteAlbum)

	r.GET("/tracks", trackController.GetTracks)
	r.GET("/tracks/search", trackController.SearchTracks)
	r.GET("/tracks/:id", trackController.GetTrackByID)
	r.POST("/tracks", trackController.CreateTrack)
	r.PUT("/tracks/:id", trackController.UpdateTrack)
	r.DELETE("/tracks/:id", trackController.DeleteTrack)

	r.GET("/playback", playbackController.GetCurrent)
	r.GET("/playback/current", playbackController.GetPlaybackState)
	r.POST("/playback/start", playbackController.Start)
	r.POST("/playback/start-playlist", playbackController.StartPlaylist)
	r.POST("/playback/pause", playbackController.Pause)
	r.POST("/playback/resume", playbackController.Resume)
	r.POST("/playback/skip", playbackController.Skip)
	r.POST("/playback/previous", playbackController.Previous)
	r.POST("/playback/stop", playbackController.Stop)
	r.POST("/playback/restore", playbackController.RestoreSession)
	r.POST("/playback/clear", playbackController.Clear)
	r.POST("/playback/update-progress", playbackController.UpdateProgress)
	r.GET("/playback/state", playbackController.GetState)
	r.GET("/playback/history", playbackController.GetHistory)
	r.GET("/playback/history/most-played", playbackController.GetMostPlayed)
	r.GET("/playback/history/recent", playbackController.GetRecent)
	r.GET("/playback/history/:track_id", playbackController.GetTrackHistory)
	r.POST("/playback/update-history", playbackController.UpdateHistory)

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

	r.GET("/api/settings", settingsController.Get)
	r.PUT("/api/settings", settingsController.Update)
	r.POST("/api/database/reset", settingsController.ResetDatabase)
	r.POST("/api/database/seed", settingsController.SeedDatabase)

	r.GET("/api/audit/logs", settingsController.GetAuditLogs)
	r.POST("/api/audit/cleanup", settingsController.CleanupAuditLogs)

	durationController := controllers.NewDurationController(db)

	duration := r.Group("/api/duration")
	{
		duration.GET("/tracks", durationController.GetTracksNeedingResolution)
		duration.GET("/stats", durationController.GetStatistics)

		duration.POST("/track/:id/manual", durationController.SetManualDuration)
		duration.POST("/resolve/track/:id", durationController.ResolveTrack)
		duration.POST("/resolve/album/:id", durationController.ResolveAlbum)
		duration.GET("/resolve/track/:id", durationController.GetResolutionStatus)

		duration.POST("/resolve/start", durationController.StartBulkResolution)
		duration.POST("/resolve/pause", durationController.PauseBulkResolution)
		duration.POST("/resolve/resume", durationController.ResumeBulkResolution)
		duration.POST("/resolve/cancel", durationController.CancelBulkResolution)
		duration.GET("/resolve/progress", durationController.GetBulkProgress)

		duration.GET("/review", durationController.GetReviewQueue)
		duration.GET("/review/resolved", durationController.GetResolvedQueue)
		duration.GET("/review/:id", durationController.GetReviewDetails)
		duration.POST("/review/:id", durationController.SubmitReview)
		duration.POST("/review/bulk", durationController.BulkReview)
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
	}
}
