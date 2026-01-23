package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"vinylfo/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	err = db.AutoMigrate(
		&models.PlaybackSession{},
		&models.SessionPlaylist{},
		&models.Album{},
		&models.Track{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestGetAllPlaylists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	controller := NewPlaylistController(db)

	router := gin.New()
	router.GET("/sessions/playlist", controller.GetAllPlaylists)

	t.Run("empty database", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/sessions/playlist", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body != "[]" && body != "null" {
			t.Errorf("Expected empty array or null, got: %s", body)
		}
	})

	t.Run("with playlists", func(t *testing.T) {
		db.Create(&models.SessionPlaylist{SessionID: "playlist1", TrackID: 1, Order: 1})
		db.Create(&models.SessionPlaylist{SessionID: "playlist1", TrackID: 2, Order: 2})
		db.Create(&models.SessionPlaylist{SessionID: "playlist2", TrackID: 3, Order: 1})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/sessions/playlist", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var playlists []models.SessionPlaylist
		err := json.Unmarshal(w.Body.Bytes(), &playlists)
		if err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(playlists) != 2 {
			t.Errorf("Expected 2 unique playlists, got %d. Body: %s", len(playlists), w.Body.String())
		}
	})
}

func TestCreateNewPlaylist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	controller := NewPlaylistController(db)

	router := gin.New()
	router.POST("/sessions/playlist/new", controller.CreateNewPlaylist)

	t.Run("create playlist with name", func(t *testing.T) {
		body := map[string]string{"name": "myplaylist"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sessions/playlist/new", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 201 {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var count int64
		db.Model(&models.SessionPlaylist{}).Where("session_id = ?", "myplaylist").Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 playlist entry, got %d", count)
		}
	})

	t.Run("reject duplicate name", func(t *testing.T) {
		db.Create(&models.SessionPlaylist{SessionID: "existing", TrackID: 0, Order: 0})

		body := map[string]string{"name": "existing"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sessions/playlist/new", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400 for duplicate, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("reject empty name", func(t *testing.T) {
		body := map[string]string{"name": ""}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sessions/playlist/new", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400 for empty name, got %d", w.Code)
		}
	})

	t.Run("reject missing name field", func(t *testing.T) {
		body := map[string]string{"other_field": "value"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sessions/playlist/new", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400 for missing name, got %d", w.Code)
		}
	})

	t.Run("reject session_id instead of name (wrong field)", func(t *testing.T) {
		body := map[string]string{"session_id": "wrong-field-playlist"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sessions/playlist/new", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400 for wrong field name, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestUpdateSession_DoesNotOverwriteTrackID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	controller := NewPlaylistController(db)

	router := gin.New()
	router.PUT("/sessions/:id", controller.UpdateSession)

	session := models.PlaybackSession{
		PlaylistID:   "session123",
		PlaylistName: "Test Playlist",
		TrackID:      42,
		Status:       "stopped",
	}
	db.Create(&session)

	updatedData := map[string]interface{}{
		"playlist_name": "Updated Playlist",
		"status":        "playing",
	}
	jsonBody, _ := json.Marshal(updatedData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sessions/session123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.PlaybackSession
	db.First(&result, "playlist_id = ?", "session123")

	if result.TrackID != 42 {
		t.Errorf("TrackID was overwritten! Expected 42, got %d", result.TrackID)
	}

	if result.PlaylistName != "Updated Playlist" {
		t.Errorf("PlaylistName was not updated. Expected 'Updated Playlist', got '%s'", result.PlaylistName)
	}
}

func TestGetPlaylist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	controller := NewPlaylistController(db)

	router := gin.New()
	router.GET("/sessions/playlist/:id", controller.GetPlaylist)

	db.Create(&models.SessionPlaylist{SessionID: "testplaylist", TrackID: 1, Order: 1})
	db.Create(&models.SessionPlaylist{SessionID: "testplaylist", TrackID: 2, Order: 2})

	t.Run("get existing playlist", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/sessions/playlist/testplaylist", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response["session_id"] != "testplaylist" {
			t.Errorf("Expected session_id 'testplaylist', got '%v'", response["session_id"])
		}
	})

	t.Run("get non-existent playlist returns empty array", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/sessions/playlist/nonexistent", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestDeletePlaylist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	controller := NewPlaylistController(db)

	router := gin.New()
	router.DELETE("/sessions/playlist/:id", controller.DeletePlaylist)

	db.Create(&models.SessionPlaylist{SessionID: "deleteplaylist", TrackID: 1, Order: 1})

	t.Run("delete existing playlist", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/sessions/playlist/deleteplaylist", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var count int64
		db.Model(&models.SessionPlaylist{}).Where("session_id = ?", "deleteplaylist").Count(&count)
		if count != 0 {
			t.Errorf("Expected playlist to be deleted, got %d entries", count)
		}
	})
}
