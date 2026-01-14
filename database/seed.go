package database

import (
	"log"
	"time"

	"gorm.io/gorm"
	"vinylfo/models"
)

// SeedDatabase populates the database with sample data for testing
func SeedDatabase(db *gorm.DB) error {
	log.Println("Seeding database with sample data...")

	// Check if database is already seeded
	var albumCount int64
	db.Model(&models.Album{}).Count(&albumCount)
	if albumCount > 0 {
		log.Println("Database already seeded, skipping...")
		return nil
	}

	// Create sample albums
	albums := []models.Album{
		{
			Title:         "Abbey Road",
			Artist:        "The Beatles",
			ReleaseYear:   1969,
			Genre:         "Rock",
			CoverImageURL: "https://example.com/abbey_road.jpg",
		},
		{
			Title:         "Rumours",
			Artist:        "Fleetwood Mac",
			ReleaseYear:   1977,
			Genre:         "Rock",
			CoverImageURL: "https://example.com/rumours.jpg",
		},
		{
			Title:         "Dark Side of the Moon",
			Artist:        "Pink Floyd",
			ReleaseYear:   1973,
			Genre:         "Progressive Rock",
			CoverImageURL: "https://example.com/dark_side.jpg",
		},
		{
			Title:         "Thriller",
			Artist:        "Michael Jackson",
			ReleaseYear:   1982,
			Genre:         "Pop",
			CoverImageURL: "https://example.com/thriller.jpg",
		},
	}

	// Create albums and their tracks
	for i, album := range albums {
		// Create album
		err := db.Create(&album).Error
		if err != nil {
			log.Printf("Failed to create album %s: %v", album.Title, err)
			return err
		}

		// Create tracks for this album
		var tracks []models.Track
		switch i {
		case 0: // Abbey Road
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Come Together", Duration: 259, TrackNumber: 1, AudioFileURL: "https://example.com/come_together.mp3"},
				{AlbumID: album.ID, Title: "Something", Duration: 182, TrackNumber: 2, AudioFileURL: "https://example.com/something.mp3"},
				{AlbumID: album.ID, Title: "Maxwell's Silver Hammer", Duration: 207, TrackNumber: 3, AudioFileURL: "https://example.com/maxwells_silver_hammer.mp3"},
				{AlbumID: album.ID, Title: "Oh! Darling", Duration: 193, TrackNumber: 4, AudioFileURL: "https://example.com/oh_darling.mp3"},
			}
		case 1: // Rumours
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Monday Madonna", Duration: 247, TrackNumber: 1, AudioFileURL: "https://example.com/monday_madonna.mp3"},
				{AlbumID: album.ID, Title: "Ho Hey", Duration: 225, TrackNumber: 2, AudioFileURL: "https://example.com/ho_heys.mp3"},
				{AlbumID: album.ID, Title: "Dreams", Duration: 206, TrackNumber: 3, AudioFileURL: "https://example.com/dreams.mp3"},
				{AlbumID: album.ID, Title: "Don't Stop Me Now", Duration: 206, TrackNumber: 4, AudioFileURL: "https://example.com/dont_stop_me_now.mp3"},
			}
		case 2: // Dark Side of the Moon
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Speak to Me", Duration: 20, TrackNumber: 1, AudioFileURL: "https://example.com/speak_to_me.mp3"},
				{AlbumID: album.ID, Title: "Breathe", Duration: 161, TrackNumber: 2, AudioFileURL: "https://example.com/breathe.mp3"},
				{AlbumID: album.ID, Title: "On the Run", Duration: 220, TrackNumber: 3, AudioFileURL: "https://example.com/on_the_run.mp3"},
				{AlbumID: album.ID, Title: "Time", Duration: 237, TrackNumber: 4, AudioFileURL: "https://example.com/time.mp3"},
			}
		case 3: // Thriller
			tracks = []models.Track{
				{AlbumID: album.ID, Title: "Wanna Be Startin' Somethin'", Duration: 258, TrackNumber: 1, AudioFileURL: "https://example.com/wanna_be_startin.mp3"},
				{AlbumID: album.ID, Title: "Baby Be Mine", Duration: 225, TrackNumber: 2, AudioFileURL: "https://example.com/baby_be_mine.mp3"},
				{AlbumID: album.ID, Title: "The Girl is Mine", Duration: 192, TrackNumber: 3, AudioFileURL: "https://example.com/the_girl_is_mine.mp3"},
				{AlbumID: album.ID, Title: "Thriller", Duration: 258, TrackNumber: 4, AudioFileURL: "https://example.com/thriller.mp3"},
			}
		}

		// Create tracks
		for _, track := range tracks {
			err := db.Create(&track).Error
			if err != nil {
				log.Printf("Failed to create track %s for album %s: %v", track.Title, album.Title, err)
				return err
			}
		}
	}

	// Create some sample playback sessions
	playbackSessions := []models.PlaybackSession{
		{
			TrackID:   1, // First track
			StartTime: time.Now().Add(-10 * time.Minute),
			EndTime:   time.Now().Add(-5 * time.Minute),
			Duration:  300,
			Progress:  250,
		},
		{
			TrackID:   2, // Second track
			StartTime: time.Now().Add(-15 * time.Minute),
			EndTime:   time.Now().Add(-8 * time.Minute),
			Duration:  420,
			Progress:  180,
		},
	}

	for _, session := range playbackSessions {
		err := db.Create(&session).Error
		if err != nil {
			log.Printf("Failed to create playback session: %v", err)
			return err
		}
	}

	log.Println("Database seeding completed successfully")
	return nil
}
