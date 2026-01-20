package controllers

import (
	"context"
	"testing"
	"time"

	"vinylfo/models"
)

func TestNewPlaybackManager(t *testing.T) {
	pm := NewPlaybackManager()
	if pm == nil {
		t.Fatal("NewPlaybackManager returned nil")
	}
	if pm.sessions == nil {
		t.Error("sessions map is nil")
	}
	if pm.currentTrack != nil {
		t.Error("currentTrack should be nil")
	}
	if pm.playlistID != "" {
		t.Error("playlistID should be empty")
	}
}

func TestPlaybackManager_StartPlayback(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID:   "test-playlist",
		PlaylistName: "Test Playlist",
		Status:       "playing",
	}

	pm.StartPlayback("test-playlist", session)

	if !pm.IsPlaying("test-playlist") {
		t.Error("IsPlaying should return true after StartPlayback")
	}
	if pm.IsPaused("test-playlist") {
		t.Error("IsPaused should return false after StartPlayback")
	}
	if pm.GetPosition("test-playlist") != 0 {
		t.Error("Position should be 0 after StartPlayback")
	}
	if pm.GetCurrentPlaylistID() != "test-playlist" {
		t.Error("GetCurrentPlaylistID should return the playlist ID")
	}
	if pm.GetCurrentPlaylistName() != "Test Playlist" {
		t.Error("GetCurrentPlaylistName should return the playlist name")
	}
}

func TestPlaybackManager_PausePlayback(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID: "test-playlist",
		Status:     "playing",
	}
	pm.StartPlayback("test-playlist", session)

	pm.PausePlayback("test-playlist")

	if pm.IsPlaying("test-playlist") {
		t.Error("IsPlaying should return false after PausePlayback")
	}
	if !pm.IsPaused("test-playlist") {
		t.Error("IsPaused should return true after PausePlayback")
	}
}

func TestPlaybackManager_ResumePlayback(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID: "test-playlist",
		Status:     "paused",
	}
	pm.StartPlayback("test-playlist", session)
	pm.PausePlayback("test-playlist")

	pm.ResumePlayback("test-playlist")

	if !pm.IsPlaying("test-playlist") {
		t.Error("IsPlaying should return true after ResumePlayback")
	}
	if pm.IsPaused("test-playlist") {
		t.Error("IsPaused should return false after ResumePlayback")
	}
}

func TestPlaybackManager_StopPlayback(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID: "test-playlist",
		Status:     "playing",
	}
	pm.StartPlayback("test-playlist", session)

	pm.StopPlayback("test-playlist")

	if pm.IsPlaying("test-playlist") {
		t.Error("IsPlaying should return false after StopPlayback")
	}
	if pm.HasSession("test-playlist") {
		t.Error("HasSession should return false after StopPlayback")
	}
	if pm.GetCurrentPlaylistID() != "" {
		t.Error("GetCurrentPlaylistID should be empty after StopPlayback")
	}
}

func TestPlaybackManager_UpdatePosition(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID:    "test-playlist",
		QueuePosition: 0,
	}
	pm.StartPlayback("test-playlist", session)

	pm.UpdatePosition("test-playlist", 30)

	if pm.GetPosition("test-playlist") != 30 {
		t.Error("Position should be 30 after UpdatePosition")
	}
}

func TestPlaybackManager_SetCurrentTrack(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID: "test-playlist",
	}
	pm.StartPlayback("test-playlist", session)

	track := &models.Track{
		ID:       1,
		Title:    "Test Track",
		Duration: 180,
	}
	pm.SetCurrentTrack("test-playlist", track)

	if pm.GetCurrentTrack() != track {
		t.Error("GetCurrentTrack should return the set track")
	}
	if pm.GetCurrentPlaylistID() != "test-playlist" {
		t.Error("GetCurrentPlaylistID should still be the playlist ID")
	}
}

func TestPlaybackManager_MultipleSessions(t *testing.T) {
	pm := NewPlaybackManager()

	session1 := &models.PlaybackSession{PlaylistID: "playlist-1", Status: "playing"}
	session2 := &models.PlaybackSession{PlaylistID: "playlist-2", Status: "playing"}

	pm.StartPlayback("playlist-1", session1)
	pm.StartPlayback("playlist-2", session2)

	if !pm.HasSession("playlist-1") {
		t.Error("HasSession should return true for playlist-1")
	}
	if !pm.HasSession("playlist-2") {
		t.Error("HasSession should return true for playlist-2")
	}

	pm.PausePlayback("playlist-1")

	if pm.IsPlaying("playlist-1") {
		t.Error("playlist-1 should not be playing after PausePlayback")
	}
	if !pm.IsPlaying("playlist-2") {
		t.Error("playlist-2 should still be playing")
	}

	pm.StopPlayback("playlist-1")

	if pm.HasSession("playlist-1") {
		t.Error("playlist-1 session should be removed")
	}
	if !pm.HasSession("playlist-2") {
		t.Error("playlist-2 session should still exist")
	}
}

func TestPlaybackManager_GetSession(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID:   "test-playlist",
		PlaylistName: "Test",
	}
	pm.StartPlayback("test-playlist", session)

	retrieved := pm.GetSession("test-playlist")
	if retrieved == nil {
		t.Fatal("GetSession returned nil")
	}
	if retrieved.PlaylistID != "test-playlist" {
		t.Error("Retrieved session has wrong PlaylistID")
	}
}

func TestPlaybackManager_GetNonexistentSession(t *testing.T) {
	pm := NewPlaybackManager()

	retrieved := pm.GetSession("nonexistent")
	if retrieved != nil {
		t.Error("GetSession should return nil for nonexistent playlist")
	}
	if pm.HasSession("nonexistent") {
		t.Error("HasSession should return false for nonexistent playlist")
	}
}

func TestPlaybackManager_UpdateSessionState(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID:    "test-playlist",
		QueuePosition: 0,
	}
	pm.StartPlayback("test-playlist", session)

	pm.UpdateSessionState("test-playlist", func(s *PlaybackSessionState) {
		s.Position = 45
	})

	if pm.GetPosition("test-playlist") != 45 {
		t.Error("Position should be updated to 45")
	}
}

func TestPlaybackManager_Concurrency(t *testing.T) {
	pm := NewPlaybackManager()
	session := &models.PlaybackSession{
		PlaylistID: "test-playlist",
		Status:     "playing",
	}
	pm.StartPlayback("test-playlist", session)

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			pm.StartPlayback("test-playlist", session)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			pm.GetPosition("test-playlist")
			pm.IsPlaying("test-playlist")
			pm.HasSession("test-playlist")
		}
		done <- true
	}()

	<-done
	<-done

	if !pm.HasSession("test-playlist") {
		t.Error("Session should still exist after concurrent access")
	}
}

func TestNewPlaybackController(t *testing.T) {
	pm := NewPlaybackManager()
	controller := &PlaybackController{playbackManager: pm}
	if controller == nil {
		t.Fatal("NewPlaybackController returned nil")
	}
	if controller.playbackManager == nil {
		t.Error("playbackManager should not be nil")
	}
}

func TestSimulateTimer_ContextCancellation(t *testing.T) {
	pm := NewPlaybackManager()
	controller := &PlaybackController{playbackManager: pm}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)

	go func() {
		controller.SimulateTimer(ctx)
		done <- true
	}()

	session := &models.PlaybackSession{
		PlaylistID:    "test-playlist",
		Status:        "playing",
		QueuePosition: 0,
	}
	pm.StartPlayback("test-playlist", session)

	track := &models.Track{
		ID:       1,
		Title:    "Test Track",
		Duration: 180,
	}
	pm.SetCurrentTrack("test-playlist", track)

	time.Sleep(100 * time.Millisecond)

	initialPosition := pm.GetPosition("test-playlist")

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SimulateTimer did not exit after context cancellation")
	}

	finalPosition := pm.GetPosition("test-playlist")
	if finalPosition < initialPosition {
		t.Error("Position should not decrease after cancellation")
	}
}
