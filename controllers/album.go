package controllers

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"vinylfo/models"
	"vinylfo/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AlbumController struct {
	db        *gorm.DB
	broadcast func(playlistID string)
}

func NewAlbumController(db *gorm.DB, broadcast func(playlistID string)) *AlbumController {
	return &AlbumController{
		db:        db,
		broadcast: broadcast,
	}
}

func (c *AlbumController) GetAlbums(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "25"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var albums []models.Album
	var total int64

	c.db.Model(&models.Album{}).Count(&total)

	result := c.db.Offset(offset).Limit(limit).Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch albums"})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"data":       albums,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *AlbumController) SearchAlbums(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "25"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var albums []models.Album
	var total int64
	searchTerm := "%" + strings.ToLower(query) + "%"

	c.db.Model(&models.Album{}).Where("LOWER(title) LIKE ? OR LOWER(artist) LIKE ?", searchTerm, searchTerm).Count(&total)

	result := c.db.Where("LOWER(title) LIKE ? OR LOWER(artist) LIKE ?", searchTerm, searchTerm).Offset(offset).Limit(limit).Find(&albums)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to search albums"})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	ctx.JSON(200, gin.H{
		"data":       albums,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *AlbumController) GetAlbumByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("GetAlbumByID DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}
	ctx.JSON(200, album)
}

func (c *AlbumController) GetAlbumImage(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("GetAlbumImage DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	if len(album.DiscogsCoverImage) == 0 {
		ctx.JSON(404, gin.H{"error": "No image found for this album"})
		return
	}

	contentType := album.DiscogsCoverImageType
	if contentType == "" {
		contentType = "image/jpeg"
	}

	ctx.Data(200, contentType, album.DiscogsCoverImage)
}

// UpdateAlbumImage updates the album cover image from a provided URL
func (c *AlbumController) UpdateAlbumImage(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}

	var req struct {
		ImageURL string `json:"image_url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": "Image URL is required"})
		return
	}

	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("UpdateAlbumImage DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	// Download the image from the provided URL
	importer := services.NewAlbumImporter(c.db, nil)
	imageData, imageType, err := importer.DownloadCoverImage(req.ImageURL)
	if err != nil {
		log.Printf("UpdateAlbumImage download error: %v", err)
		ctx.JSON(400, gin.H{"error": "Failed to download image: " + err.Error()})
		return
	}

	if len(imageData) == 0 {
		ctx.JSON(400, gin.H{"error": "No image data received from URL"})
		return
	}

	// Update the album with the new image
	album.DiscogsCoverImage = imageData
	album.DiscogsCoverImageType = imageType
	album.CoverImageURL = req.ImageURL
	album.CoverImageFailed = false

	result = c.db.Save(&album)
	if result.Error != nil {
		log.Printf("UpdateAlbumImage save error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to save album image"})
		return
	}

	ctx.JSON(200, gin.H{
		"message":      "Album image updated successfully",
		"album_id":     id,
		"content_type": imageType,
		"size":         len(imageData),
	})
}

func (c *AlbumController) GetTracksByAlbumID(ctx *gin.Context) {
	id := ctx.Param("id")

	type TrackResult struct {
		ID           uint   `json:"id"`
		AlbumID      uint   `json:"album_id"`
		Title        string `json:"title"`
		Duration     int    `json:"duration"`
		TrackNumber  int    `json:"track_number"`
		DiscNumber   int    `json:"disc_number"`
		Side         string `json:"side"`
		Position     string `json:"position"`
		AudioFileURL string `json:"audio_file_url"`
		AlbumTitle   string `json:"album_title"`
		AlbumArtist  string `json:"album_artist"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	var tracks []TrackResult
	result := c.db.Table("tracks").Select("tracks.*, albums.title as album_title, albums.artist as album_artist").
		Joins("left join albums on tracks.album_id = albums.id").
		Where("tracks.album_id = ?", id).
		Find(&tracks)

	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch tracks"})
		return
	}
	ctx.JSON(200, tracks)
}

func (c *AlbumController) CreateAlbum(ctx *gin.Context) {
	var album models.Album
	if err := ctx.ShouldBindJSON(&album); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&album)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create album"})
		return
	}
	ctx.JSON(201, album)
}

func (c *AlbumController) UpdateAlbum(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("UpdateAlbum DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&album); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&album)
	if result.Error != nil {
		log.Printf("UpdateAlbum save error: %v", result.Error)
		ctx.JSON(500, gin.H{"error": "Failed to update album"})
		return
	}
	ctx.JSON(200, album)
}

// DeleteAlbumPreview returns information about what will be affected when deleting an album
func (c *AlbumController) DeleteAlbumPreview(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}

	// Check if album exists
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	// Get tracks for this album
	var tracks []models.Track
	c.db.Where("album_id = ?", id).Find(&tracks)
	trackCount := len(tracks)

	var trackIDs []uint
	for _, track := range tracks {
		trackIDs = append(trackIDs, track.ID)
	}

	// Find playlists that contain these tracks
	type PlaylistImpact struct {
		SessionID    string `json:"session_id"`
		PlaylistName string `json:"playlist_name"`
		TrackCount   int    `json:"track_count"`
	}

	var impactedPlaylists []PlaylistImpact
	if len(trackIDs) > 0 {
		// Query session playlists to find which playlists contain these tracks
		var sessionPlaylists []models.SessionPlaylist
		c.db.Where("track_id IN ?", trackIDs).Find(&sessionPlaylists)

		// Group by session and count tracks
		sessionTrackCounts := make(map[string]int)
		sessionIDs := make([]string, 0)
		for _, sp := range sessionPlaylists {
			if _, exists := sessionTrackCounts[sp.SessionID]; !exists {
				sessionIDs = append(sessionIDs, sp.SessionID)
			}
			sessionTrackCounts[sp.SessionID]++
		}

		// Get playlist names for affected sessions
		if len(sessionIDs) > 0 {
			var sessions []models.PlaybackSession
			c.db.Where("playlist_id IN ?", sessionIDs).Find(&sessions)

			for _, session := range sessions {
				impactedPlaylists = append(impactedPlaylists, PlaylistImpact{
					SessionID:    session.PlaylistID,
					PlaylistName: session.PlaylistName,
					TrackCount:   sessionTrackCounts[session.PlaylistID],
				})
			}
		}
	}

	ctx.JSON(200, gin.H{
		"album": gin.H{
			"id":     album.ID,
			"title":  album.Title,
			"artist": album.Artist,
		},
		"track_count":        trackCount,
		"impacted_playlists": impactedPlaylists,
		"total_playlists":    len(impactedPlaylists),
	})
}

// DeleteAlbum deletes an album and all associated data (tracks, youtube matches, playlist entries)
func (c *AlbumController) DeleteAlbum(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid album ID"})
		return
	}

	confirmed := ctx.Query("confirmed") == "true"
	if !confirmed {
		ctx.JSON(400, gin.H{"error": "Deletion not confirmed. Use ?confirmed=true or call /delete-preview first"})
		return
	}

	// Check if album exists
	var album models.Album
	result := c.db.First(&album, id)
	if result.Error != nil {
		log.Printf("DeleteAlbum DB error: %v", result.Error)
		ctx.JSON(404, gin.H{"error": "Album not found"})
		return
	}

	// Get all track IDs for this album
	var tracks []models.Track
	c.db.Where("album_id = ?", id).Find(&tracks)

	var trackIDs []uint
	for _, track := range tracks {
		trackIDs = append(trackIDs, track.ID)
	}

	// Get DurationResolution IDs for these tracks first (needed for DurationSource cleanup)
	var resolutionIDs []uint
	if len(trackIDs) > 0 {
		var resolutions []models.DurationResolution
		c.db.Where("track_id IN ?", trackIDs).Find(&resolutions)
		for _, res := range resolutions {
			resolutionIDs = append(resolutionIDs, res.ID)
		}
	}

	// Find all playback sessions that have these tracks in their queue
	type SessionUpdate struct {
		Session    models.PlaybackSession
		NewQueue   []uint
		NewTrackID uint
		NewIndex   int
	}
	var sessionsToUpdate []SessionUpdate

	if len(trackIDs) > 0 {
		var allSessions []models.PlaybackSession
		c.db.Find(&allSessions)

		for _, session := range allSessions {
			if session.Queue == "" {
				continue
			}

			var queue []uint
			if err := json.Unmarshal([]byte(session.Queue), &queue); err != nil {
				continue
			}

			// Check if any deleted track is in this session's queue
			trackIDMap := make(map[uint]bool)
			for _, tid := range trackIDs {
				trackIDMap[tid] = true
			}

			var newQueue []uint
			currentTrackDeleted := false
			for _, qTrackID := range queue {
				if trackIDMap[qTrackID] {
					if qTrackID == session.TrackID {
						currentTrackDeleted = true
					}
				} else {
					newQueue = append(newQueue, qTrackID)
				}
			}

			// If tracks were removed from this session's queue
			if len(newQueue) != len(queue) {
				newTrackID := session.TrackID
				newIndex := session.QueueIndex

				if currentTrackDeleted {
					// Current track was deleted, find next track
					if len(newQueue) == 0 {
						// No tracks left, will stop session
						newTrackID = 0
						newIndex = 0
					} else if session.QueueIndex < len(newQueue) {
						// Next track is at same index
						newTrackID = newQueue[session.QueueIndex]
					} else {
						// Moved past end, go to last track
						newIndex = len(newQueue) - 1
						newTrackID = newQueue[newIndex]
					}
				} else {
					// Current track not deleted, adjust index if needed
					if session.QueueIndex >= len(newQueue) && len(newQueue) > 0 {
						newIndex = len(newQueue) - 1
					}
				}

				sessionsToUpdate = append(sessionsToUpdate, SessionUpdate{
					Session:    session,
					NewQueue:   newQueue,
					NewTrackID: newTrackID,
					NewIndex:   newIndex,
				})
			}
		}
	}

	// Execute all deletions in a transaction
	err = c.db.Transaction(func(tx *gorm.DB) error {
		// 1. Delete YouTube candidates for album's tracks
		if len(trackIDs) > 0 {
			if err := tx.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeCandidate{}).Error; err != nil {
				return err
			}
		}

		// 2. Delete YouTube matches for album's tracks
		if len(trackIDs) > 0 {
			if err := tx.Where("track_id IN ?", trackIDs).Delete(&models.TrackYouTubeMatch{}).Error; err != nil {
				return err
			}
		}

		// 3. Delete duration sources for album's tracks (via resolution_id)
		if len(resolutionIDs) > 0 {
			if err := tx.Where("resolution_id IN ?", resolutionIDs).Delete(&models.DurationSource{}).Error; err != nil {
				return err
			}
		}

		// 4. Delete duration resolutions for album's tracks
		if len(trackIDs) > 0 {
			if err := tx.Where("track_id IN ?", trackIDs).Delete(&models.DurationResolution{}).Error; err != nil {
				return err
			}
		}

		// 5. Delete track history entries
		if len(trackIDs) > 0 {
			if err := tx.Where("track_id IN ?", trackIDs).Delete(&models.TrackHistory{}).Error; err != nil {
				return err
			}
		}

		// 6. Delete session playlist entries (removes tracks from all playlists)
		var affectedSessionIDs []string
		if len(trackIDs) > 0 {
			// First, get the list of affected sessions before deleting
			var sessionPlaylists []models.SessionPlaylist
			tx.Where("track_id IN ?", trackIDs).Find(&sessionPlaylists)
			sessionIDMap := make(map[string]bool)
			for _, sp := range sessionPlaylists {
				sessionIDMap[sp.SessionID] = true
			}
			for sid := range sessionIDMap {
				affectedSessionIDs = append(affectedSessionIDs, sid)
			}

			// Delete the entries
			if err := tx.Where("track_id IN ?", trackIDs).Delete(&models.SessionPlaylist{}).Error; err != nil {
				return err
			}
		}

		// 6b. Renumber remaining SessionPlaylist entries to eliminate gaps
		for _, sessionID := range affectedSessionIDs {
			var remainingEntries []models.SessionPlaylist
			tx.Where("session_id = ?", sessionID).Order("`order` ASC").Find(&remainingEntries)

			for i, entry := range remainingEntries {
				newOrder := i + 1 // 1-indexed
				if entry.Order != newOrder {
					entry.Order = newOrder
					if err := tx.Save(&entry).Error; err != nil {
						return err
					}
				}
			}
		}

		// 7. Update playback sessions
		for _, update := range sessionsToUpdate {
			if len(update.NewQueue) == 0 {
				// No tracks left, stop the session
				update.Session.Queue = "[]"
				update.Session.TrackID = 0
				update.Session.QueueIndex = 0
				update.Session.Status = "stopped"
				update.Session.QueuePosition = 0
			} else {
				queueJSON, _ := json.Marshal(update.NewQueue)
				update.Session.Queue = string(queueJSON)
				update.Session.TrackID = update.NewTrackID
				update.Session.QueueIndex = update.NewIndex
				if update.NewTrackID == 0 {
					update.Session.Status = "stopped"
				}
			}
			update.Session.Revision++
			if err := tx.Save(&update.Session).Error; err != nil {
				return err
			}
		}

		// 8. Delete the tracks themselves
		if err := tx.Where("album_id = ?", id).Delete(&models.Track{}).Error; err != nil {
			return err
		}

		// 9. Finally, delete the album
		if err := tx.Delete(&album).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("DeleteAlbum transaction error: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to delete album: " + err.Error()})
		return
	}

	// Broadcast state updates for all affected sessions so UI refreshes automatically
	if c.broadcast != nil {
		for _, update := range sessionsToUpdate {
			c.broadcast(update.Session.PlaylistID)
		}
	}

	ctx.JSON(200, gin.H{
		"message":           "Album deleted successfully",
		"album_id":          id,
		"tracks_deleted":    len(trackIDs),
		"affected_sessions": len(sessionsToUpdate),
	})
}
