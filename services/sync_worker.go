package services

import (
	"fmt"
	"os"
	"strings"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/sync"

	"gorm.io/gorm"
)

// SyncWorker handles the batch sync process
type SyncWorker struct {
	db              *gorm.DB
	client          *discogs.Client
	importer        *AlbumImporter
	progressService *SyncProgressService
	stateManager    *sync.LegacyStateManager
	config          SyncConfig
}

// SyncConfig holds configuration for the sync worker
type SyncConfig struct {
	Username      string
	BatchSize     int
	SyncMode      string
	CurrentFolder int
	Folders       *[]map[string]interface{}
}

// NewSyncWorker creates a new SyncWorker instance
func NewSyncWorker(db *gorm.DB, client *discogs.Client, stateManager *sync.LegacyStateManager, config SyncConfig) *SyncWorker {
	return &SyncWorker{
		db:              db,
		client:          client,
		importer:        NewAlbumImporter(db, client),
		progressService: NewSyncProgressService(db),
		stateManager:    stateManager,
		config:          config,
	}
}

// Run executes the main sync loop
func (w *SyncWorker) Run() {
	defer func() {
		if r := recover(); r != nil {
			w.logToFile("Sync: PANIC in SyncWorker.Run: %v", r)
			w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
				s.IsRunning = false
				s.IsPaused = false
				s.LastActivity = time.Time{}
			})
		}
	}()

	w.logToFile("Sync: SyncWorker.Run STARTING")

	initialState := w.stateManager.GetState()
	w.logToFile("Sync: initial state - IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d",
		initialState.IsRunning, initialState.IsPaused, initialState.Processed, initialState.Total)

	for {
		w.logToFile("Sync: ========== LOOP ITERATION START ==========")

		// Update last activity timestamp
		w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
			s.LastActivity = time.Now()
		})

		state := w.stateManager.GetState()
		lastBatchAlbums := 0
		if state.LastBatch != nil {
			lastBatchAlbums = len(state.LastBatch.Albums)
		}
		w.logToFile("Sync: loop TOP - IsRunning=%v, IsPaused=%v, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.IsRunning, state.IsPaused, state.Processed,
			state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		if !state.IsRunning {
			w.logToFile("Sync: complete (not running), Processed=%d/%d", state.Processed, state.Total)
			return
		}

		w.logToFile("Sync: NOT PAUSED - proceeding with batch processing, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.Processed, state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		// Determine if we need to fetch new data from API
		needFetch, nextPage, nextFolder, done := w.handlePagination(state)
		if done {
			return
		}

		// Check if sync was stopped during processing
		if !state.IsRunning {
			return
		}

		// Get current releases to process
		currentReleases := w.getCurrentReleases(state)

		if needFetch {
			releases, shouldReturn := w.fetchNextBatch(nextPage, nextFolder, state)
			if shouldReturn {
				return
			}
			if releases == nil {
				continue
			}
			currentReleases = releases
		} else {
			w.logToFile("processSyncBatches: processing batch %d with %d albums", state.CurrentPage, len(currentReleases))
		}

		// Re-check running state after potential API call delays
		state = w.stateManager.GetState()
		if !state.IsRunning || state.IsPaused {
			return
		}

		if len(currentReleases) == 0 {
			w.logToFile("processSyncBatches: no releases to process, continuing loop")
			time.Sleep(200 * time.Millisecond)
			continue
		}

		w.logToFile("Sync: processing batch of %d albums, Processed=%d/%d", len(currentReleases), state.Processed, state.Total)

		// Process each album
		for _, album := range currentReleases {
			currentCheck := w.stateManager.GetState()
			if !currentCheck.IsRunning || currentCheck.IsPaused {
				return
			}

			w.processAlbum(album, state)
		}

		// Check if LastBatch is empty after processing
		if w.checkPauseState() {
			continue
		}

		w.progressService.Save(w.stateManager.GetState())
		time.Sleep(200 * time.Millisecond)
	}
}

// handlePagination determines if we need to fetch new data and handles folder transitions
func (w *SyncWorker) handlePagination(state sync.LegacySyncState) (needFetch bool, nextPage int, nextFolder int, done bool) {
	page := state.CurrentPage
	folderID := state.CurrentFolder

	if state.LastBatch == nil || len(state.LastBatch.Albums) == 0 {
		needFetch = true

		if w.config.SyncMode == "all-folders" && len(*w.config.Folders) > 0 {
			if state.CurrentPage > 1 {
				// Move to next folder
				if state.FolderIndex >= len(*w.config.Folders)-1 {
					w.logToFile("processSyncBatches: all folders synced complete. Total processed: %d", state.Processed)
					w.markComplete(state)
					return false, 0, 0, true
				}
				w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
					s.FolderIndex++
					s.CurrentFolder = (*w.config.Folders)[s.FolderIndex]["id"].(int)
					s.CurrentPage = 1
				})
				updatedState := w.stateManager.GetState()
				folderID = updatedState.CurrentFolder
				page = 1
				w.logToFile("processSyncBatches: moving to folder %d (%s)", folderID, (*w.config.Folders)[updatedState.FolderIndex]["name"])
			}
		}
	}

	return needFetch, page, folderID, false
}

// fetchNextBatch fetches the next batch of albums from the API
func (w *SyncWorker) fetchNextBatch(page, folderID int, state sync.LegacySyncState) ([]map[string]interface{}, bool) {
	w.logToFile("processSyncBatches: fetching page %d from API for folder %d", page, folderID)
	time.Sleep(500 * time.Millisecond)

	// Check if sync was stopped or paused before making API call
	if !state.IsRunning || state.IsPaused {
		return nil, true
	}

	var releases []map[string]interface{}
	var err error
	var totalItems int

	if w.config.SyncMode == "all-folders" && folderID > 0 {
		releases, totalItems, err = w.client.GetUserCollectionByFolder(w.config.Username, folderID, page, w.config.BatchSize)
	} else if w.config.SyncMode == "all-folders" {
		releases, totalItems, err = w.client.GetUserCollectionByFolder(w.config.Username, 0, page, w.config.BatchSize)
	} else if w.config.SyncMode == "specific" {
		releases, totalItems, err = w.client.GetUserCollectionByFolder(w.config.Username, folderID, page, w.config.BatchSize)
	} else {
		releases, err = w.client.GetUserCollection(w.config.Username, page, w.config.BatchSize)
	}

	// Update total if API reports a different count
	if totalItems > 0 {
		currentState := w.stateManager.GetState()
		if totalItems != currentState.Total {
			w.logToFile("processSyncBatches: API reports total=%d, updating from %d", totalItems, currentState.Total)
			w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
				s.Total = totalItems
			})
		}
	}

	if err != nil {
		return w.handleFetchError(err, page, state)
	}

	if len(releases) == 0 {
		return w.handleEmptyReleases(page, state)
	}

	w.logToFile("processSyncBatches: fetched %d albums from page %d folder %d", len(releases), page, folderID)

	if len(releases) < w.config.BatchSize {
		w.logToFile("processSyncBatches: received fewer albums than page size (%d < %d), sync complete", len(releases), w.config.BatchSize)
		w.markComplete(state)
		return nil, true
	}

	apiRem := w.client.GetAPIRemaining()
	anonRem := w.client.GetAPIRemainingAnon()

	w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
		s.LastBatch = &sync.SyncBatch{
			ID:     page,
			Albums: releases,
		}
		s.APIRemaining = apiRem
		s.AnonRemaining = anonRem
		s.CurrentPage = page + 1
	})

	// Check if sync was stopped or paused after API call
	checkState := w.stateManager.GetState()
	if !checkState.IsRunning || checkState.IsPaused {
		return nil, true
	}

	return releases, false
}

// handleFetchError handles errors during fetching
func (w *SyncWorker) handleFetchError(err error, page int, state sync.LegacySyncState) ([]map[string]interface{}, bool) {
	errStr := err.Error()
	if strings.Contains(errStr, "Page") && strings.Contains(errStr, "outside of valid range") {
		w.logToFile("processSyncBatches: reached end of pagination (page %d doesn't exist)", page)

		// Handle end of current folder
		if w.config.SyncMode == "all-folders" && len(*w.config.Folders) > 0 && state.FolderIndex < len(*w.config.Folders)-1 {
			w.logToFile("processSyncBatches: more folders to process, continuing")
			w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
				s.CurrentPage = 1
				s.LastBatch = nil
			})
			return nil, false // Continue to next iteration
		}

		w.markComplete(state)
		return nil, true
	}

	w.logToFile("processSyncBatches: failed to fetch page %d: %v", page, err)
	w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
		s.IsRunning = false
		s.IsPaused = false
		s.LastBatch = nil
		s.LastActivity = time.Time{}
	})
	return nil, true
}

// handleEmptyReleases handles the case when no releases are returned
func (w *SyncWorker) handleEmptyReleases(page int, state sync.LegacySyncState) ([]map[string]interface{}, bool) {
	w.logToFile("processSyncBatches: received empty releases list at page %d", page)

	// Update total to reflect actual processed count
	checkState := w.stateManager.GetState()
	if checkState.Processed > checkState.Total {
		w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
			s.Total = checkState.Processed
		})
		w.logToFile("processSyncBatches: adjusted total to %d (was %d)", checkState.Processed, checkState.Total)
	}

	// Handle empty page - move to next folder or complete
	if w.config.SyncMode == "all-folders" && len(*w.config.Folders) > 0 {
		if state.FolderIndex < len(*w.config.Folders)-1 {
			w.logToFile("processSyncBatches: moving to next folder after empty page in folder %d", state.CurrentFolder)
			w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
				s.FolderIndex++
				s.CurrentFolder = (*w.config.Folders)[s.FolderIndex]["id"].(int)
				s.CurrentPage = 1
				s.LastBatch = nil
			})
			return nil, false // Continue to next iteration
		}
	}

	w.markComplete(state)
	return nil, true
}

// getCurrentReleases gets the current releases from state
func (w *SyncWorker) getCurrentReleases(state sync.LegacySyncState) []map[string]interface{} {
	if state.LastBatch != nil {
		return state.LastBatch.Albums
	}
	return []map[string]interface{}{}
}

// processAlbum processes a single album
func (w *SyncWorker) processAlbum(album map[string]interface{}, state sync.LegacySyncState) {
	title, _ := album["title"].(string)
	artist, _ := album["artist"].(string)
	year, _ := album["year"].(int)
	coverImage, _ := album["cover_image"].(string)
	discogsID := 0
	if v, ok := album["discogs_id"].(int); ok {
		discogsID = v
	}
	albumFolderID := 0
	if f, ok := album["folder_id"].(int); ok {
		albumFolderID = f
	}
	if albumFolderID == 0 {
		albumFolderID = state.CurrentFolder
	}

	var existingAlbum models.Album
	var result *gorm.DB

	// Check for existing album
	if discogsID > 0 {
		result = w.db.Where("discogs_id = ?", discogsID).First(&existingAlbum)
		if result.Error == gorm.ErrRecordNotFound {
			result = w.db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)
		}
	} else {
		result = w.db.Where("title = ? AND artist = ?", title, artist).First(&existingAlbum)
	}

	if result.Error == gorm.ErrRecordNotFound {
		w.createNewAlbum(title, artist, year, coverImage, discogsID, albumFolderID)
	} else {
		w.updateExistingAlbum(&existingAlbum, title, artist, year, coverImage, discogsID, albumFolderID)
	}
}

// createNewAlbum creates a new album in the database
func (w *SyncWorker) createNewAlbum(title, artist string, year int, coverImage string, discogsID, albumFolderID int) {
	maxRetries := 3
	var newAlbum models.Album
	var tx *gorm.DB
	var createErr error

	imageData, imageType, imageErr := w.importer.DownloadCoverImage(coverImage)
	imageFailed := imageErr != nil
	if imageErr != nil {
		w.logToFile("processSyncBatches: failed to download image for %s - %s: %v", artist, title, imageErr)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		newAlbum = models.Album{
			Title:                 title,
			Artist:                artist,
			ReleaseYear:           year,
			CoverImageURL:         coverImage,
			DiscogsCoverImage:     imageData,
			DiscogsCoverImageType: imageType,
			CoverImageFailed:      imageFailed,
			DiscogsID:             intPtr(discogsID),
			DiscogsFolderID:       albumFolderID,
		}
		tx = w.db.Begin()
		if tx.Error != nil {
			if attempt < maxRetries && w.isLockTimeout(tx.Error) {
				tx.Rollback()
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			w.logToFile("processSyncBatches: failed to start transaction for album: %s - %s", artist, title)
			w.db.Create(&models.SyncLog{
				DiscogsID:  discogsID,
				AlbumTitle: title,
				Artist:     artist,
				ErrorType:  "album",
				ErrorMsg:   fmt.Sprintf("Failed to start transaction: %v", tx.Error),
			})
			break
		}

		createErr = tx.Create(&newAlbum).Error
		if createErr != nil {
			tx.Rollback()
			if attempt < maxRetries && w.isLockTimeout(createErr) {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			w.logToFile("processSyncBatches: failed to create album: %s - %s: %v", artist, title, createErr)
			w.db.Create(&models.SyncLog{
				DiscogsID:  discogsID,
				AlbumTitle: title,
				Artist:     artist,
				ErrorType:  "album",
				ErrorMsg:   fmt.Sprintf("Failed to create album: %v", createErr),
			})
			w.progressService.Save(w.stateManager.GetState())
			break
		}

		w.logToFile("processSyncBatches: Created album: %s - %s (folder: %d)", artist, title, albumFolderID)

		if discogsID > 0 {
			success, errMsg := w.importer.FetchAndSaveTracks(tx, newAlbum.ID, discogsID, title, artist)
			if !success {
				w.logToFile("processSyncBatches: Failed to fetch tracks for album %s - %s: %s", artist, title, errMsg)
				tx.Rollback()
				break
			}
			w.logToFile("processSyncBatches: Successfully synced album with tracks: %s - %s", artist, title)
		}

		if err := tx.Commit().Error; err != nil {
			if attempt < maxRetries && w.isLockTimeout(err) {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			w.logToFile("processSyncBatches: failed to commit album: %s - %s: %v", artist, title, err)
			w.progressService.Save(w.stateManager.GetState())
			break
		}

		w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
			s.Processed++
		})
		w.progressService.Save(w.stateManager.GetState())
		w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
			w.removeFirstAlbumFromBatch(s)
		})
		w.progressService.Save(w.stateManager.GetState())
		w.logToFile("processSyncBatches: Album synced successfully: %s - %s, Processed=%d", artist, title, w.stateManager.GetState().Processed)
		break
	}
}

// updateExistingAlbum updates an existing album with new data
func (w *SyncWorker) updateExistingAlbum(existingAlbum *models.Album, title, artist string, year int, coverImage string, discogsID, albumFolderID int) {
	updated := false
	updates := make(map[string]interface{})

	// Update DiscogsID if it was previously missing
	if existingAlbum.DiscogsID == nil && discogsID > 0 {
		updates["discogs_id"] = discogsID
		updated = true
	}

	// Update folder ID if changed
	if albumFolderID > 0 && existingAlbum.DiscogsFolderID != albumFolderID {
		updates["discogs_folder_id"] = albumFolderID
		updated = true
	}

	// Update cover image if we have one and it's different or was missing
	if coverImage != "" && existingAlbum.CoverImageURL != coverImage {
		updates["cover_image_url"] = coverImage
		if imageData, imageType, err := w.importer.DownloadCoverImage(coverImage); err == nil && imageData != nil {
			updates["discogs_cover_image"] = imageData
			updates["discogs_cover_image_type"] = imageType
			updates["cover_image_failed"] = false
		}
		updated = true
	}

	// Update year if we have one and existing is 0
	if year > 0 && existingAlbum.ReleaseYear == 0 {
		updates["release_year"] = year
		updated = true
	}

	if updated {
		if err := w.db.Model(existingAlbum).Updates(updates).Error; err != nil {
			w.logToFile("Sync: failed to update album %s - %s: %v", artist, title, err)
		} else {
			w.logToFile("Sync: updated existing album: %s - %s", artist, title)
		}
	} else {
		w.logToFile("Sync: album exists (no updates needed): %s - %s", artist, title)
	}

	w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
		s.Processed++
	})
	w.progressService.Save(w.stateManager.GetState())
	w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
		w.removeFirstAlbumFromBatch(s)
		if s.Processed%5 == 0 {
			s.APIRemaining = w.client.GetAPIRemaining()
			s.AnonRemaining = w.client.GetAPIRemainingAnon()
		}
	})
	w.progressService.Save(w.stateManager.GetState())
	w.logToFile("Sync: processed=%d/%d", w.stateManager.GetState().Processed, w.stateManager.GetState().Total)
}

// checkPauseState checks if sync is paused and waits for resume
func (w *SyncWorker) checkPauseState() bool {
	state := w.stateManager.GetState()
	if state.LastBatch == nil || len(state.LastBatch.Albums) == 0 {
		if state.IsPaused {
			w.logToFile("Sync: PAUSED - entering wait loop (batch empty)")

			waitStart := time.Now()
			for {
				checkState := w.stateManager.GetState()
				elapsed := time.Since(waitStart)

				if elapsed.Seconds() < 1 || elapsed.Seconds() > 5 || checkState.IsPaused != state.IsPaused {
					w.logToFile("Sync: wait loop check - IsPaused=%v, IsRunning=%v, elapsed=%v",
						checkState.IsPaused, checkState.IsRunning, elapsed)
				}

				if !checkState.IsPaused {
					w.logToFile("Sync: RESUME DETECTED after %v", elapsed)
					break
				}

				if !checkState.IsRunning {
					w.logToFile("Sync: sync stopped while paused")
					return true
				}

				time.Sleep(100 * time.Millisecond)
			}

			w.logToFile("Sync: continuing loop after wait")
			return true
		}

		w.logToFile("processSyncBatches: batch empty, will fetch next page")
		time.Sleep(200 * time.Millisecond)
		return true
	}
	return false
}

// markComplete marks the sync as complete
func (w *SyncWorker) markComplete(state sync.LegacySyncState) {
	w.logToFile("processSyncBatches: sync complete. Processed=%d, Total=%d, Mode=%s", state.Processed, state.Total, w.config.SyncMode)

	w.stateManager.UpdateState(func(s *sync.LegacySyncState) {
		s.IsRunning = false
		s.IsPaused = false
		s.Total = state.Processed
		s.LastBatch = nil
		s.LastActivity = time.Time{}
	})

	w.progressService.Save(w.stateManager.GetState())

	progress := w.progressService.Load(w.stateManager.GetState())
	if progress != nil {
		w.progressService.ArchiveToHistory(progress)
		w.progressService.Delete(progress.ID)
	}

	w.db.Model(&models.AppConfig{}).Where("id = ?", 1).Update("last_sync_at", time.Now())
}

// removeFirstAlbumFromBatch removes the first album from the current batch
func (w *SyncWorker) removeFirstAlbumFromBatch(s *sync.LegacySyncState) {
	if s.LastBatch != nil && len(s.LastBatch.Albums) > 0 {
		s.LastBatch.Albums = s.LastBatch.Albums[1:]
		if len(s.LastBatch.Albums) == 0 {
			s.LastBatch = nil
		}
	}
}

// isLockTimeout checks if an error is a database lock timeout
func (w *SyncWorker) isLockTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Lock wait timeout") || strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "try restarting transaction")
}

// logToFile writes a log message to the sync debug log file
func (w *SyncWorker) logToFile(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	f, _ := os.OpenFile("sync_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}
