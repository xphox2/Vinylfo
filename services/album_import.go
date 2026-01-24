package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

// CleanupOrphanedTracks removes tracks with invalid data (album_id=0 or empty title)
// This is a safety net for edge cases where tracks may have been created without proper album association
func (i *AlbumImporter) CleanupOrphanedTracks() (int64, error) {
	// First, log how many orphaned tracks we're about to delete for diagnostics
	var countBefore int64
	i.db.Model(&models.Track{}).Where("album_id = ? OR TRIM(title) = ?", 0, "").Count(&countBefore)

	if countBefore > 0 {
		log.Printf("CleanupOrphanedTracks: Found %d orphaned tracks (album_id=0 or empty title) - investigating root cause", countBefore)

		// Log sample of orphaned tracks for debugging
		var sampleTracks []models.Track
		i.db.Where("album_id = ? OR TRIM(title) = ?", 0, "").Limit(5).Find(&sampleTracks)
		for _, track := range sampleTracks {
			log.Printf("CleanupOrphanedTracks SAMPLE: id=%d, album_id=%d, title='%s', created_at=%v",
				track.ID, track.AlbumID, track.Title, track.CreatedAt)
		}

		// Log albums that might be missing
		var orphanAlbumIDs []uint
		i.db.Model(&models.Track{}).Where("album_id = ?", 0).Pluck("id", &orphanAlbumIDs)
		if len(orphanAlbumIDs) > 0 {
			log.Printf("CleanupOrphanedTracks: Tracks with album_id=0 have IDs: %v", orphanAlbumIDs)
		}
	}

	var result *gorm.DB
	result = i.db.Where("album_id = ? OR TRIM(title) = ?", 0, "").Delete(&models.Track{})
	if result.Error != nil {
		return 0, result.Error
	}
	deletedCount := result.RowsAffected
	log.Printf("CleanupOrphanedTracks: Deleted %d orphaned tracks (album_id=0 or empty title)", deletedCount)
	return deletedCount, nil
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
	return i.DownloadCoverImageWithRetry(imageURL, 1) // Default: no retries for backward compatibility
}

// DownloadCoverImageWithRetry downloads a cover image from a URL with configurable retries
func (i *AlbumImporter) DownloadCoverImageWithRetry(imageURL string, maxAttempts int) ([]byte, string, error) {
	if imageURL == "" {
		logCoverDownload("SKIP", imageURL, "empty URL provided")
		return nil, "", nil
	}

	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, contentType, err := i.downloadCoverImageOnce(imageURL, attempt, maxAttempts)
		if err == nil {
			return data, contentType, nil
		}
		lastErr = err

		// Don't retry on certain errors
		if strings.Contains(err.Error(), "empty URL") ||
			strings.Contains(err.Error(), "invalid content type") ||
			strings.Contains(err.Error(), "status 404") ||
			strings.Contains(err.Error(), "status 403") {
			logCoverDownload("FAIL_NO_RETRY", imageURL, "error not retryable: %v", err)
			return nil, "", err
		}

		if attempt < maxAttempts {
			backoff := time.Duration(attempt) * 2 * time.Second
			logCoverDownload("RETRY_WAIT", imageURL, "attempt %d/%d failed, waiting %v before retry: %v",
				attempt, maxAttempts, backoff, err)
			time.Sleep(backoff)
		}
	}

	logCoverDownload("FAIL_EXHAUSTED", imageURL, "all %d attempts failed, last error: %v", maxAttempts, lastErr)
	return nil, "", lastErr
}

// downloadCoverImageOnce performs a single download attempt
func (i *AlbumImporter) downloadCoverImageOnce(imageURL string, attempt, maxAttempts int) ([]byte, string, error) {
	logCoverDownload("START", imageURL, "attempt %d/%d", attempt, maxAttempts)

	client := config.DefaultClient()

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		logCoverDownload("ERROR", imageURL, "failed to create request: %v", err)
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0")

	startTime := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		logCoverDownload("ERROR", imageURL, "HTTP request failed after %v: %v", elapsed, err)
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	logCoverDownload("RESPONSE", imageURL, "status=%d, content-length=%s, elapsed=%v",
		resp.StatusCode, resp.Header.Get("Content-Length"), elapsed)

	if resp.StatusCode != http.StatusOK {
		logCoverDownload("ERROR", imageURL, "bad status code: %d", resp.StatusCode)
		return nil, "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logCoverDownload("ERROR", imageURL, "failed to read response body: %v", err)
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
		logCoverDownload("WARN", imageURL, "no Content-Type header, defaulting to image/jpeg")
	}

	if !strings.HasPrefix(contentType, "image/") {
		logCoverDownload("ERROR", imageURL, "invalid content type: %s (body preview: %.100s)",
			contentType, string(data))
		return nil, "", fmt.Errorf("invalid content type: %s", contentType)
	}

	logCoverDownload("SUCCESS", imageURL, "downloaded %d bytes, type=%s", len(data), contentType)
	return data, contentType, nil
}

// logCoverDownload logs cover download events to the sync debug log
func logCoverDownload(event, imageURL, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Truncate URL for readability in logs
	shortURL := imageURL
	if len(shortURL) > 80 {
		shortURL = shortURL[:77] + "..."
	}
	fullMsg := fmt.Sprintf("COVER_DOWNLOAD [%s] url=%s: %s", event, shortURL, msg)

	// Log to sync debug file
	f, err := os.OpenFile(discogs.SyncDebugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fullMsg))
	}

	// Also log to standard log for visibility
	log.Print(fullMsg)
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
		// Skip tracks with empty titles
		if strings.TrimSpace(trackInput.Title) == "" {
			log.Printf("CreateAlbum: Skipping track with empty title for album %s - %s", album.Title, album.Artist)
			continue
		}

		// Safety check
		if album.ID == 0 {
			log.Printf("CreateAlbum ERROR: album.ID is 0, skipping track creation")
			continue
		}

		log.Printf("CreateAlbum: Creating track '%s' for album_id=%d", trackInput.Title, album.ID)
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
	// Get tracks and master_id in one API call
	tracks, masterID, err := i.client.GetTracksForAlbumWithMaster(discogsID)

	// Handle deleted/private releases or server errors - try to get from master release
	if err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "Internal Server Error")) {
		log.Printf("FetchAndSaveTracks: Release %d error (%v) for %s - %s, checking master release", discogsID, err, artist, albumTitle)

		// Try to get master release info
		masterData, masterErr := i.client.GetMasterRelease(discogsID)
		if masterErr == nil && masterData != nil {
			if mid, ok := masterData["id"].(int); ok && mid > 0 {
				masterID = mid
				log.Printf("FetchAndSaveTracks: Found master release %d for %s - %s", masterID, artist, albumTitle)

				// Get the main release from the master
				mainReleaseID, mainErr := i.client.GetMainReleaseFromMaster(masterID)
				if mainErr == nil && mainReleaseID > 0 {
					log.Printf("FetchAndSaveTracks: Fetching tracks from main release %d (from master %d)", mainReleaseID, masterID)
					tracks, _, err = i.client.GetTracksForAlbumWithMaster(mainReleaseID)
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
		log.Printf("FetchAndSaveTracks: No durations found for %s - %s (release %d, master %d), attempting cross-reference", artist, albumTitle, discogsID, masterID)
		// Use master_id to check master's main release first before falling back to search
		crossRefTracks, crossErr := i.client.CrossReferenceTimestampsWithMaster(albumTitle, artist, tracks, masterID)
		if crossErr != nil {
			if strings.Contains(crossErr.Error(), "rate limited") {
				return false, fmt.Sprintf("Failed to fetch tracks: %v", crossErr)
			}
		}
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

			imageData, imageType, imageErr := i.DownloadCoverImageWithRetry(v, 3)
			if imageErr != nil {
				updates["cover_image_failed"] = true
				log.Printf("FetchAndSaveTracks: failed to download cover image after 3 attempts: %v", imageErr)
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

		// Skip tracks with empty titles
		if strings.TrimSpace(title) == "" {
			log.Printf("FetchAndSaveTracks: Skipping track with empty title for album %s - %s", albumTitle, artist)
			continue
		}

		// Safety check: ensure albumID is valid
		if albumID == 0 {
			log.Printf("FetchAndSaveTracks ERROR: albumID is 0, skipping track creation for %s - %s", title, albumTitle)
			continue
		}

		newTrack := models.Track{
			AlbumID:     albumID,
			Title:       title,
			Duration:    duration,
			TrackNumber: trackNumber,
			DiscNumber:  discNumber,
			Side:        position,
			Position:    position,
		}

		log.Printf("FetchAndSaveTracks: Creating track '%s' for album_id=%d (album: %s - %s)",
			title, albumID, albumTitle, artist)

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
