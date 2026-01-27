package integration

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"vinylfo/controllers"
	"vinylfo/models"
)

func setupPlaybackController(t *testing.T) (*controllers.PlaybackController, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.PlaybackSession{}); err != nil {
		t.Fatalf("failed to migrate playback sessions: %v", err)
	}

	return controllers.NewPlaybackController(db), db
}

func TestUpdateProgressDoesNotRegressSmallDelta(t *testing.T) {
	controller, db := setupPlaybackController(t)

	session := models.PlaybackSession{
		PlaylistID:    "playlist-1",
		PlaylistName:  "Playlist 1",
		TrackID:       1,
		QueueIndex:    0,
		QueuePosition: 10,
		Status:        "playing",
	}

	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	pm := controller.GetPlaybackManager()
	pm.StartPlayback(session.PlaylistID, &session)
	pm.UpdatePosition(session.PlaylistID, 10)

	if pm.GetPosition(session.PlaylistID) != 10 {
		t.Fatalf("expected playback manager position to be 10 before update, got %d", pm.GetPosition(session.PlaylistID))
	}

	body, err := json.Marshal(map[string]interface{}{
		"playlist_id":      session.PlaylistID,
		"track_id":         session.TrackID,
		"position_seconds": 9,
		"queue_index":      session.QueueIndex,
	})
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/playback/update-progress", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.UpdateProgress(ctx)

	if recorder.Code != 200 {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response["status"] != "Progress saved" {
		t.Fatalf("expected response status 'Progress saved', got %v", response["status"])
	}

	var updated models.PlaybackSession
	if err := db.First(&updated, "playlist_id = ?", session.PlaylistID).Error; err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}

	if updated.QueuePosition != 10 {
		t.Fatalf("expected queue position to remain 10, got %d", updated.QueuePosition)
	}

	if pm.GetPosition(session.PlaylistID) != 10 {
		t.Fatalf("expected playback manager position to remain 10, got %d", pm.GetPosition(session.PlaylistID))
	}
}

func TestUpdateProgressAllowsLargeBackwardSeek(t *testing.T) {
	controller, db := setupPlaybackController(t)

	session := models.PlaybackSession{
		PlaylistID:    "playlist-2",
		PlaylistName:  "Playlist 2",
		TrackID:       2,
		QueueIndex:    0,
		QueuePosition: 10,
		Status:        "playing",
	}

	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	pm := controller.GetPlaybackManager()
	pm.StartPlayback(session.PlaylistID, &session)
	pm.UpdatePosition(session.PlaylistID, 10)

	if pm.GetPosition(session.PlaylistID) != 10 {
		t.Fatalf("expected playback manager position to be 10 before update, got %d", pm.GetPosition(session.PlaylistID))
	}

	body, err := json.Marshal(map[string]interface{}{
		"playlist_id":      session.PlaylistID,
		"track_id":         session.TrackID,
		"position_seconds": 6,
		"queue_index":      session.QueueIndex,
	})
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/playback/update-progress", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.UpdateProgress(ctx)

	if recorder.Code != 200 {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response["status"] != "Progress saved" {
		t.Fatalf("expected response status 'Progress saved', got %v", response["status"])
	}

	var updated models.PlaybackSession
	if err := db.First(&updated, "playlist_id = ?", session.PlaylistID).Error; err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}

	if updated.QueuePosition != 6 {
		t.Fatalf("expected queue position to be 6, got %d", updated.QueuePosition)
	}

	if pm.GetPosition(session.PlaylistID) != 6 {
		t.Fatalf("expected playback manager position to be 6, got %d", pm.GetPosition(session.PlaylistID))
	}
}
