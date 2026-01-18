package main

import (
	"fmt"
	"log"
	"os"

	"vinylfo/discogs"
	"vinylfo/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// Connect to database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")
		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Create Discogs client - API key not needed, uses OAuth from env vars
	client := discogs.NewClient("")

	// Get albums with DiscogsID
	var albums []models.Album
	result := db.Where("discogs_id IS NOT NULL AND discogs_id > 0").Find(&albums)
	fmt.Printf("Found %d albums with Discogs IDs\n", result.RowsAffected)

	updated := 0
	for _, album := range albums {
		if album.DiscogsID == nil || *album.DiscogsID == 0 {
			continue
		}

		discogsID := *album.DiscogsID
		fmt.Printf("\nProcessing: %s - %s (DiscogsID: %d)\n", album.Artist, album.Title, discogsID)

		// Get tracks from Discogs
		tracks, err := client.GetTracksForAlbum(discogsID)
		if err != nil {
			fmt.Printf("  ERROR fetching tracks: %v\n", err)
			continue
		}

		if len(tracks) == 0 {
			fmt.Printf("  No tracks found\n")
			continue
		}

		// Delete existing tracks
		db.Where("album_id = ?", album.ID).Delete(&models.Track{})

		// Create new tracks
		for i, track := range tracks {
			title := track["title"].(string)
			position := track["position"].(string)

			var trackNumber, discNumber int
			switch tn := track["track_number"].(type) {
			case int:
				trackNumber = tn
			case int64:
				trackNumber = int(tn)
			case float64:
				trackNumber = int(tn)
			}

			switch dn := track["disc_number"].(type) {
			case int:
				discNumber = dn
			case int64:
				discNumber = int(dn)
			case float64:
				discNumber = int(dn)
			}

			duration := 0
			if d, ok := track["duration"].(int); ok {
				duration = d
			}

			newTrack := models.Track{
				AlbumID:     album.ID,
				AlbumTitle:  album.Title,
				Title:       title,
				Duration:    duration,
				TrackNumber: trackNumber,
				DiscNumber:  discNumber,
				Side:        position,
				Position:    position,
			}

			if err := db.Create(&newTrack).Error; err != nil {
				fmt.Printf("  ERROR creating track %d: %v\n", i+1, err)
			} else {
				fmt.Printf("  Track %d: %s (pos=%s) -> track_number=%d, disc_number=%d\n",
					i+1, title, position, trackNumber, discNumber)
			}
		}

		updated++
	}

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Updated %d albums\n", updated)

	// Show sample tracks
	var sampleTracks []models.Track
	db.Order("id DESC").Limit(10).Find(&sampleTracks)
	fmt.Println("\nLast 10 tracks:")
	for _, t := range sampleTracks {
		fmt.Printf("  ID=%d %s (pos=%s) track=%d disc=%d\n",
			t.ID, t.Title, t.Position, t.TrackNumber, t.DiscNumber)
	}
}
