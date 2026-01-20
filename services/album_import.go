package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"vinylfo/config"
	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/utils"

	"gorm.io/gorm"
)

// AlbumImporter handles importing albums from Discogs
type AlbumImporter struct {
	db     *gorm.DB
	client *discogs.Client
}

// NewAlbumImporter creates a new AlbumImporter instance
func NewAlbumImporter(db *gorm.DB, client *discogs.Client) *AlbumImporter {
	return &AlbumImporter{
		db:     db,
		client: client,
	}
}

// AlbumInput represents the input data for creating an album
type AlbumInput struct {
	Title       string
	Artist      string
	ReleaseYear int
	Genre       string
	Label       string
	Country     string
	ReleaseDate string
	Style       string
	CoverImage  string
	DiscogsID   int
	FolderID    int
}

// TrackInput represents the input data for creating a track
type TrackInput struct {
	Title       string
	Duration    int
	TrackNumber int
	DiscNumber  int
	Side        string
	Position    string
}

// DownloadCoverImage downloads a cover image from a URL
func (i *AlbumImporter) DownloadCoverImage(imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", nil
	}

	client := config.DefaultClient()

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("invalid content type: %s", contentType)
	}

	return data, contentType, nil
}

// CreateAlbumWithTracks creates an album with its tracks in the database
func (i *AlbumImporter) CreateAlbumWithTracks(input AlbumInput, tracks []TrackInput) (*models.Album, error) {
	album := models.Album{
		Title:           input.Title,
		Artist:          input.Artist,
		ReleaseYear:     input.ReleaseYear,
		Genre:           input.Genre,
		Label:           input.Label,
		Country:         input.Country,
		ReleaseDate:     input.ReleaseDate,
		Style:           input.Style,
		CoverImageURL:   input.CoverImage,
		DiscogsID:       utils.IntPtr(input.DiscogsID),
		DiscogsFolderID: input.FolderID,
	}

	// Download cover image if URL provided
	if input.CoverImage != "" {
		imageData, imageType, err := i.DownloadCoverImage(input.CoverImage)
		if err != nil {
			album.CoverImageFailed = true
		} else {
			album.DiscogsCoverImage = imageData
			album.DiscogsCoverImageType = imageType
		}
	}

	if err := i.db.Create(&album).Error; err != nil {
		return nil, fmt.Errorf("failed to create album: %w", err)
	}

	// Create tracks
	for _, trackInput := range tracks {
		track := models.Track{
			AlbumID:     album.ID,
			Title:       trackInput.Title,
			Duration:    trackInput.Duration,
			TrackNumber: trackInput.TrackNumber,
			DiscNumber:  trackInput.DiscNumber,
			Side:        trackInput.Side,
			Position:    trackInput.Position,
		}
		if err := i.db.Create(&track).Error; err != nil {
			return nil, fmt.Errorf("failed to create track: %w", err)
		}
	}

	// Reload with tracks
	i.db.Preload("Tracks").First(&album, album.ID)
	return &album, nil
}

// FetchAndSaveTracks fetches tracks from Discogs and saves them to the database
func (i *AlbumImporter) FetchAndSaveTracks(db *gorm.DB, albumID uint, discogsID int, albumTitle, artist string) (bool, string) {
	tracks, err := i.client.GetTracksForAlbum(discogsID)

	// Handle deleted/private releases or server errors - try to get from master release
	if err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "Internal Server Error")) {
		log.Printf("FetchAndSaveTracks: Release %d error (%v) for %s - %s, checking master release", discogsID, err, artist, albumTitle)

		// Try to get master release info
		masterData, masterErr := i.client.GetMasterRelease(discogsID)
		if masterErr == nil && masterData != nil {
			if masterID, ok := masterData["id"].(int); ok && masterID > 0 {
				log.Printf("FetchAndSaveTracks: Found master release %d for %s - %s", masterID, artist, albumTitle)

				// Get the main release from the master
				mainReleaseID, mainErr := i.client.GetMainReleaseFromMaster(masterID)
				if mainErr == nil && mainReleaseID > 0 {
					log.Printf("FetchAndSaveTracks: Fetching tracks from main release %d (from master %d)", mainReleaseID, masterID)
					tracks, err = i.client.GetTracksForAlbum(mainReleaseID)
					if err == nil {
						log.Printf("FetchAndSaveTracks: Successfully fetched tracks from main release %d", mainReleaseID)
					}
				}
			}
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch tracks: %v", err)
		db.Create(&models.SyncLog{
			DiscogsID:  discogsID,
			AlbumTitle: albumTitle,
			Artist:     artist,
			ErrorType:  "tracks",
			ErrorMsg:   errMsg,
		})
		return false, errMsg
	}

	if len(tracks) == 0 {
		errMsg := "No tracks found for release"
		db.Create(&models.SyncLog{
			DiscogsID:  discogsID,
			AlbumTitle: albumTitle,
			Artist:     artist,
			ErrorType:  "tracks",
			ErrorMsg:   errMsg,
		})
		return false, errMsg
	}

	hasDurations := false
	for _, track := range tracks {
		if dur, ok := track["duration"].(int); ok && dur > 0 {
			hasDurations = true
			break
		}
	}

	if !hasDurations {
		log.Printf("FetchAndSaveTracks: No durations found for %s - %s (release %d), attempting cross-reference", artist, albumTitle, discogsID)
		crossRefTracks, crossErr := i.client.CrossReferenceTimestamps(albumTitle, artist, tracks)
		if crossErr == nil && len(crossRefTracks) > 0 {
			hasDurations = false
			for _, track := range crossRefTracks {
				if dur, ok := track["duration"].(int); ok && dur > 0 {
					hasDurations = true
					break
				}
			}
			if hasDurations {
				tracks = crossRefTracks
				log.Printf("FetchAndSaveTracks: Successfully cross-referenced durations for %s - %s", artist, albumTitle)
			}
		}
	}

	// Fetch full album metadata
	fullAlbumData, err := i.client.GetAlbum(discogsID)
	if err == nil {
		updates := make(map[string]interface{})

		if v, ok := fullAlbumData["genre"].(string); ok && v != "" {
			updates["genre"] = v
		}
		if v, ok := fullAlbumData["style"].(string); ok && v != "" {
			updates["style"] = v
		}
		if v, ok := fullAlbumData["label"].(string); ok && v != "" {
			updates["label"] = v
		}
		if v, ok := fullAlbumData["country"].(string); ok && v != "" {
			updates["country"] = v
		}
		if v, ok := fullAlbumData["cover_image"].(string); ok && v != "" {
			updates["cover_image_url"] = v

			imageData, imageType, imageErr := i.DownloadCoverImage(v)
			if imageErr != nil {
				updates["cover_image_failed"] = true
			} else if len(imageData) > 0 {
				updates["discogs_cover_image"] = imageData
				updates["discogs_cover_image_type"] = imageType
				updates["cover_image_failed"] = false
			}
		}

		if len(updates) > 0 {
			db.Model(&models.Album{}).Where("id = ?", albumID).Updates(updates)
		}
	}

	// Remove existing tracks before syncing
	db.Where("album_id = ?", albumID).Delete(&models.Track{})

	// Create new tracks
	for _, track := range tracks {
		title := ""
		if t, ok := track["title"].(string); ok {
			title = t
		}
		position := ""
		if p, ok := track["position"].(string); ok {
			position = p
		}
		duration := parseDuration(track["duration"])

		// Debug: show all keys and values in the track map
		log.Printf("FetchAndSaveTracks DEBUG: track map has %d keys", len(track))
		for k, v := range track {
			log.Printf("FetchAndSaveTracks DEBUG: key=%s value=%v type=%T", k, v, v)
		}

		// Get track_number - use default 0 if not present or wrong type
		trackNumber := 0
		switch tn := track["track_number"].(type) {
		case int:
			trackNumber = tn
		case int64:
			trackNumber = int(tn)
		case float64:
			trackNumber = int(tn)
		}

		// Get disc_number - use default 0 if not present or wrong type
		discNumber := 0
		switch dn := track["disc_number"].(type) {
		case int:
			discNumber = dn
		case int64:
			discNumber = int(dn)
		case float64:
			discNumber = int(dn)
		}

		log.Printf("FetchAndSaveTracks: title=%s, position=%s, track_number=%d, disc_number=%d",
			title, position, trackNumber, discNumber)

		newTrack := models.Track{
			AlbumID:     albumID,
			Title:       title,
			Duration:    duration,
			TrackNumber: trackNumber,
			DiscNumber:  discNumber,
			Side:        position,
			Position:    position,
		}

		if err := db.Create(&newTrack).Error; err != nil {
			errMsg := fmt.Sprintf("Failed to create track %s: %v", title, err)
			db.Create(&models.SyncLog{
				DiscogsID:  discogsID,
				AlbumTitle: albumTitle,
				Artist:     artist,
				ErrorType:  "track",
				ErrorMsg:   errMsg,
			})
			return false, errMsg
		}
	}

	return true, ""
}

// ImportFromDiscogs imports an album from Discogs by its ID
func (i *AlbumImporter) ImportFromDiscogs(discogsID int) (*models.Album, error) {
	discogsData, err := i.client.GetAlbum(discogsID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch album from Discogs: %w", err)
	}

	input := AlbumInput{
		DiscogsID: discogsID,
	}

	if v, ok := discogsData["title"].(string); ok {
		input.Title = v
	}
	if v, ok := discogsData["artist"].(string); ok {
		input.Artist = v
	}
	switch v := discogsData["year"].(type) {
	case float64:
		input.ReleaseYear = int(v)
	case int:
		input.ReleaseYear = v
	}
	if v, ok := discogsData["genre"].(string); ok {
		input.Genre = v
	}
	if v, ok := discogsData["label"].(string); ok {
		input.Label = v
	}
	if v, ok := discogsData["country"].(string); ok {
		input.Country = v
	}
	if v, ok := discogsData["release_date"].(string); ok {
		input.ReleaseDate = v
	}
	if v, ok := discogsData["style"].(string); ok {
		input.Style = v
	}
	if v, ok := discogsData["cover_image"].(string); ok {
		input.CoverImage = v
	}

	// Parse tracks
	var tracks []TrackInput
	if tracklist, ok := discogsData["tracklist"].([]map[string]interface{}); ok {
		positionInfos := make([]discogs.PositionInfo, 0, len(tracklist))
		for _, t := range tracklist {
			if pos, ok := t["position"].(string); ok {
				posInfo := discogs.ParsePosition(pos)
				positionInfos = append(positionInfos, posInfo)
			} else {
				positionInfos = append(positionInfos, discogs.PositionInfo{IsValid: false})
			}
		}

		trackCounter := 0
		for i, t := range tracklist {
			track := TrackInput{
				Duration: parseDuration(t["duration"]),
			}
			if title, ok := t["title"].(string); ok {
				track.Title = title
			}
			if position, ok := t["position"].(string); ok {
				track.Side = position
				track.Position = position
			}

			posInfo := positionInfos[i]
			trackCounter++
			track.TrackNumber = trackCounter

			if posInfo.IsValid {
				track.DiscNumber = posInfo.DiscNumber
			} else {
				track.DiscNumber = 1
			}

			tracks = append(tracks, track)
		}
	}

	return i.CreateAlbumWithTracks(input, tracks)
}

// parseDuration parses a duration from various formats
func parseDuration(v interface{}) int {
	switch d := v.(type) {
	case float64:
		return int(d)
	case int:
		return d
	case string:
		if d != "" {
			parts := strings.Split(d, ":")
			if len(parts) == 2 {
				if mins, err := strconv.Atoi(parts[0]); err == nil {
					if secs, err := strconv.Atoi(parts[1]); err == nil {
						return mins*60 + secs
					}
				}
			}
		}
	}
	return 0
}
