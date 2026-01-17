package routes

import (
	"time"

	"vinylfo/controllers"
	"vinylfo/database"

	"github.com/gin-gonic/gin"
)

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

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
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
}
