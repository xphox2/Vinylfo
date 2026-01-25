package services

import (
	"context"
	"testing"

	"vinylfo/duration"
	"vinylfo/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type mockMusicClientNoResults struct{}

func (m mockMusicClientNoResults) Name() string { return "mock" }

func (m mockMusicClientNoResults) SearchTrack(ctx context.Context, title, artist, album string) (*duration.TrackSearchResult, error) {
	return nil, nil
}

func (m mockMusicClientNoResults) IsConfigured() bool         { return true }
func (m mockMusicClientNoResults) GetRateLimitRemaining() int { return -1 }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.Album{},
		&models.Track{},
		&models.DurationResolution{},
		&models.DurationSource{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestResolveTrackDuration_NoDurations_DoesNotInsertOrphanTrack(t *testing.T) {
	db := newTestDB(t)

	album := models.Album{Title: "Test Album", Artist: "Test Artist"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}

	original := models.Track{AlbumID: album.ID, Title: "Test Song", Duration: 0}
	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}

	var before int64
	if err := db.Model(&models.Track{}).Count(&before).Error; err != nil {
		t.Fatalf("count tracks before: %v", err)
	}

	svc := &DurationResolverService{
		db:      db,
		clients: []duration.MusicAPIClient{mockMusicClientNoResults{}},
		config:  DefaultDurationResolverConfig(),
	}

	// Re-load the track as the resolver expects a populated struct.
	var track models.Track
	if err := db.First(&track, original.ID).Error; err != nil {
		t.Fatalf("reload track: %v", err)
	}

	res, err := svc.ResolveTrackDuration(context.Background(), track)
	if err != nil {
		t.Fatalf("ResolveTrackDuration error: %v", err)
	}
	if res == nil {
		t.Fatalf("ResolveTrackDuration returned nil resolution")
	}

	var after int64
	if err := db.Model(&models.Track{}).Count(&after).Error; err != nil {
		t.Fatalf("count tracks after: %v", err)
	}
	if after != before {
		t.Fatalf("unexpected track row count change: before=%d after=%d", before, after)
	}

	// The resolver should mark the original track as needing review.
	var updated models.Track
	if err := db.First(&updated, original.ID).Error; err != nil {
		t.Fatalf("reload updated track: %v", err)
	}
	if !updated.DurationNeedsReview {
		t.Fatalf("expected DurationNeedsReview=true for track id=%d", updated.ID)
	}

	// No orphan track should be created.
	var orphanCount int64
	if err := db.Model(&models.Track{}).
		Where("album_id = 0").
		Count(&orphanCount).Error; err != nil {
		t.Fatalf("count orphan tracks: %v", err)
	}
	if orphanCount != 0 {
		t.Fatalf("expected 0 orphan tracks with album_id=0, got %d", orphanCount)
	}
}

func TestGetTracksNeedingResolution_FiltersInvalidTracks(t *testing.T) {
	db := newTestDB(t)

	album := models.Album{Title: "Test Album", Artist: "Test Artist"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}

	valid := models.Track{AlbumID: album.ID, Title: "Valid Song", Duration: 0}
	if err := db.Create(&valid).Error; err != nil {
		t.Fatalf("create valid track: %v", err)
	}

	invalid := models.Track{AlbumID: 0, Title: "", Duration: 0}
	if err := db.Create(&invalid).Error; err != nil {
		t.Fatalf("create invalid track: %v", err)
	}

	svc := &DurationResolverService{
		db:      db,
		clients: []duration.MusicAPIClient{mockMusicClientNoResults{}},
		config:  DefaultDurationResolverConfig(),
	}

	tracks, err := svc.GetTracksNeedingResolution()
	if err != nil {
		t.Fatalf("GetTracksNeedingResolution error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].ID != valid.ID {
		t.Fatalf("expected track id=%d, got id=%d", valid.ID, tracks[0].ID)
	}
}
