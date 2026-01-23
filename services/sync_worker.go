package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"vinylfo/discogs"
	"vinylfo/models"
	"vinylfo/sync"
	"vinylfo/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SyncWorker handles the batch sync process
type SyncWorker struct {
	db              *gorm.DB
	client          *discogs.Client
	importer        *AlbumImporter
	progressService *SyncProgressService
	stateManager    *sync.StateManager
	config          SyncConfig
	ctx             context.Context
	cancel          context.CancelFunc
	workerID        string
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
func NewSyncWorker(db *gorm.DB, client *discogs.Client, stateManager *sync.StateManager, config SyncConfig, ctx context.Context, cancel context.CancelFunc) *SyncWorker {
	w := &SyncWorker{
		db:              db,
		client:          client,
		importer:        NewAlbumImporter(db, client),
		progressService: NewSyncProgressService(db),
		stateManager:    stateManager,
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		workerID:        uuid.New().String(),
	}

	w.client.RateLimiter.SetRateLimitCallback(func(retryAfter int) {
		retryAt := time.Now().Add(time.Duration(retryAfter) * time.Second)
		w.stateManager.UpdateState(func(s *sync.SyncState) {
			s.RateLimitRetryAt = &retryAt
			s.RateLimitMessage = fmt.Sprintf("API rate limit - retry after %d seconds", retryAfter)
		})
		w.logToFile("Sync: RATE LIMITED - pausing sync, will retry at %v", retryAt)
		// Only set to paused if currently running - avoid state conflicts
		state := w.stateManager.GetState()
		if state.IsRunning() {
			w.stateManager.SetStatus(sync.SyncStatusPaused)
		}
	})

	w.client.RateLimiter.SetRateLimitClearedCallback(func() {
		w.logToFile("Sync: RATE LIMIT CLEARED - resuming sync")
		w.stateManager.ClearRateLimitState()
		// Only resume if currently paused (not if user manually cancelled)
		state := w.stateManager.GetState()
		if state.IsPaused() {
			w.stateManager.RequestResume()
			w.logToFile("Sync: RATE LIMIT CLEARED - RequestResume() called successfully")
		} else {
			w.logToFile("Sync: RATE LIMIT CLEARED - not resuming, state is %s", state.Status)
		}
	})

	return w
}

// Run executes the main sync loop with context support for cancellation
func (w *SyncWorker) Run() {
	defer func() {
		if r := recover(); r != nil {
			w.logToFile("Sync: PANIC in SyncWorker.Run: %v", r)
			w.stateManager.UpdateState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
				s.WorkerID = ""
			})
		}
		sync.UnregisterWorker(w.workerID)
		w.logToFile("Sync: SyncWorker.Run ENDED, workerID=%s", w.workerID)
	}()

	w.logToFile("Sync: SyncWorker.Run STARTING with workerID=%s", w.workerID)
	sync.RegisterWorker(w.workerID)

	go w.monitorContextCancellation()

	w.stateManager.UpdateState(func(s *sync.SyncState) {
		s.WorkerID = w.workerID
	})

	initialState := w.stateManager.GetState()
	w.logToFile("Sync: initial state - IsRunning=%v, IsPaused=%v, Processed=%d, Total=%d",
		initialState.IsRunning(), initialState.IsPaused(), initialState.Processed, initialState.Total)

	for {
		select {
		case <-w.ctx.Done():
			w.logToFile("Sync: context cancelled, stopping gracefully")
			w.stateManager.UpdateState(func(s *sync.SyncState) {
				s.Status = sync.SyncStatusIdle
			})
			return
		default:
		}

		w.logToFile("Sync: ========== LOOP ITERATION START ==========")

		// Check for expired rate limit state and auto-recover
		if w.client.RateLimiter.IsRateLimited() {
			secondsLeft := w.client.RateLimiter.GetSecondsUntilReset()
			if secondsLeft <= 0 {
				w.logToFile("Sync: detected expired rate limit, clearing and resuming")
				w.client.RateLimiter.ClearRateLimit()
				w.stateManager.ClearRateLimitState()
				// If we're paused due to rate limit, resume
				state := w.stateManager.GetState()
				if state.IsPaused() {
					w.stateManager.RequestResume()
				}
			}
		}

		// Update last activity timestamp
		w.stateManager.UpdateState(func(s *sync.SyncState) {
			s.LastActivity = time.Now()
		})

		state := w.stateManager.GetState()
		lastBatchAlbums := 0
		if state.LastBatch != nil {
			lastBatchAlbums = len(state.LastBatch.Albums)
		}
		w.logToFile("Sync: loop TOP - IsRunning=%v, IsPaused=%v, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.IsRunning(), state.IsPaused(), state.Processed,
			state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		if !state.IsActive() {
			w.logToFile("Sync: complete (idle), Processed=%d/%d", state.Processed, state.Total)
			return
		}

		// If paused, wait for resume or cancellation
		if state.IsPaused() {
			// Check if this is a rate limit pause - get fresh rate limit state
			isRateLimited := w.client.RateLimiter.IsRateLimited()
			secondsLeft := w.client.RateLimiter.GetSecondsUntilReset()
			w.logToFile("Sync: PAUSED - waiting for resume, Processed=%d/%d, isRateLimited=%v, secondsLeft=%d",
				state.Processed, state.Total, isRateLimited, secondsLeft)

			if err := w.stateManager.WaitForResume(w.ctx); err != nil {
				w.logToFile("Sync: context cancelled while paused: %v", err)
				return
			}
			w.logToFile("Sync: RESUMED - continuing processing, new state=%s", w.stateManager.GetState().Status)
			continue
		}

		w.logToFile("Sync: NOT PAUSED - proceeding with batch processing, Processed=%d, LastBatch=%v, albums_in_batch=%d",
			state.Processed, state.LastBatch != nil && lastBatchAlbums > 0, lastBatchAlbums)

		// Determine if we need to fetch new data from API
		needFetch, nextPage, nextFolder, done := w.handlePagination(state)
		if done {
			return
		}

		// Check if sync was stopped during processing
		state = w.stateManager.GetState()
		if !state.IsActive() {
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
		if !state.IsActive() {
			return
		}
		if state.IsPaused() {
			continue // Go back to top of loop to wait for resume
		}

		if len(currentReleases) == 0 {
			w.logToFile("processSyncBatches: no releases to process, continuing loop")
			select {
			case <-w.ctx.Done():
				w.logToFile("Sync: context cancelled during empty batch wait")
				return
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}

		w.logToFile("Sync: processing batch of %d albums, Processed=%d/%d", len(currentReleases), state.Processed, state.Total)

		// Process each album with context cancellation support
		for _, album := range currentReleases {
			select {
			case <-w.ctx.Done():
				w.logToFile("Sync: context cancelled during album processing")
				return
			default:
			}

			currentCheck := w.stateManager.GetState()
			if !currentCheck.IsActive() {
				return
			}
			if currentCheck.IsPaused() {
				w.logToFile("Sync: paused during album processing, waiting for resume")
				break // Exit album loop, go back to main loop to wait
			}

			w.processAlbum(album, state)

			w.stateManager.UpdateState(func(s *sync.SyncState) {
				s.LastActivity = time.Now()
			})
		}

		// Check if LastBatch is empty after processing
		if w.checkPauseState() {
			continue
		}

		w.progressService.Save(w.stateManager.GetState())
		select {
		case <-w.ctx.Done():
			w.logToFile("Sync: context cancelled after batch processing")
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// monitorContextCancellation watches for context cancellation and stops the sync
func (w *SyncWorker) monitorContextCancellation() {
	<-w.ctx.Done()
	w.logToFile("Sync: context cancellation detected via monitor")
	w.stateManager.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
	})
}

// handlePagination determines if we need to fetch new data and handles folder transitions
func (w *SyncWorker) handlePagination(state sync.SyncState) (needFetch bool, nextPage int, nextFolder int, done bool) {
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
				w.stateManager.UpdateState(func(s *sync.SyncState) {
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

// fetchNextBatch fetches the next batch of albums from the API with context-aware rate limiting
func (w *SyncWorker) fetchNextBatch(page, folderID int, state sync.SyncState) ([]map[string]interface{}, bool) {
	w.logToFile("processSyncBatches: fetching page %d from API for folder %d", page, folderID)

	// Context-aware rate limiting delay
	select {
	case <-w.ctx.Done():
		return nil, true
	case <-time.After(500 * time.Millisecond):
	}

	// Check if sync was stopped or paused before making API call
	state = w.stateManager.GetState()
	if !state.IsRunning() || state.IsPaused() {
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
			w.stateManager.UpdateState(func(s *sync.SyncState) {
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

	w.stateManager.UpdateState(func(s *sync.SyncState) {
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
	if !checkState.IsRunning() || checkState.IsPaused() {
		return nil, true
	}

	return releases, false
}

// handleFetchError handles errors during fetching
func (w *SyncWorker) handleFetchError(err error, page int, state sync.SyncState) ([]map[string]interface{}, bool) {
	// Check if this is a rate limit error - pause instead of stopping
	if errors.Is(err, discogs.ErrRateLimited) {
		w.logToFile("processSyncBatches: RATE LIMITED on page %d, pausing sync", page)
		// The rate limiter callback already set the state to paused
		// and started the countdown. Just return false to continue the loop
		// which will wait for resume in the main loop.
		return nil, false
	}

	errStr := err.Error()
	if strings.Contains(errStr, "Page") && strings.Contains(errStr, "outside of valid range") {
		w.logToFile("processSyncBatches: reached end of pagination (page %d doesn't exist)", page)

		// Handle end of current folder
		if w.config.SyncMode == "all-folders" && len(*w.config.Folders) > 0 && state.FolderIndex < len(*w.config.Folders)-1 {
			w.logToFile("processSyncBatches: more folders to process, continuing")
			w.stateManager.UpdateState(func(s *sync.SyncState) {
				s.CurrentPage = 1
				s.LastBatch = nil
			})
			return nil, false // Continue to next iteration
		}

		w.markComplete(state)
		return nil, true
	}

	w.logToFile("processSyncBatches: failed to fetch page %d: %v", page, err)
	w.stateManager.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
		s.LastBatch = nil
		s.LastActivity = time.Time{}
	})
	return nil, true
}

// handleEmptyReleases handles the case when no releases are returned
func (w *SyncWorker) handleEmptyReleases(page int, state sync.SyncState) ([]map[string]interface{}, bool) {
	w.logToFile("processSyncBatches: received empty releases list at page %d", page)

	// Update total to reflect actual processed count
	checkState := w.stateManager.GetState()
	if checkState.Processed > checkState.Total {
		w.stateManager.UpdateState(func(s *sync.SyncState) {
			s.Total = checkState.Processed
		})
		w.logToFile("processSyncBatches: adjusted total to %d (was %d)", checkState.Processed, checkState.Total)
	}

	// Handle empty page - move to next folder or complete
	if w.config.SyncMode == "all-folders" && len(*w.config.Folders) > 0 {
		if state.FolderIndex < len(*w.config.Folders)-1 {
			w.logToFile("processSyncBatches: moving to next folder after empty page in folder %d", state.CurrentFolder)
			w.stateManager.UpdateState(func(s *sync.SyncState) {
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
func (w *SyncWorker) getCurrentReleases(state sync.SyncState) []map[string]interface{} {
	if state.LastBatch != nil {
		return state.LastBatch.Albums
	}
	return []map[string]interface{}{}
}

// processAlbum processes a single album with context support
func (w *SyncWorker) processAlbum(album map[string]interface{}, state sync.SyncState) {
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

func (w *SyncWorker) fetchTracksWithRetry(albumID uint, discogsID int, title, artist string) (bool, string) {
	const maxAttempts = 3
	var lastErr string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-w.ctx.Done():
			return false, "context cancelled"
		default:
		}

		success, errMsg := w.importer.FetchAndSaveTracks(w.db, albumID, discogsID, title, artist)
		if success {
			return true, ""
		}
		lastErr = errMsg
		if attempt < maxAttempts {
			backoff := time.Duration(attempt) * time.Second
			w.logToFile("processSyncBatches: retrying track fetch for %s - %s after error: %s (attempt %d/%d)", artist, title, errMsg, attempt, maxAttempts)
			select {
			case <-w.ctx.Done():
				return false, lastErr
			case <-time.After(backoff):
			}
		}
	}

	return false, lastErr
}

// createNewAlbum creates a new album in the database with context-aware retry backoff
func (w *SyncWorker) createNewAlbum(title, artist string, year int, coverImage string, discogsID, albumFolderID int) {
	maxRetries := 3
	var newAlbum models.Album
	var tx *gorm.DB
	var createErr error

	imageData, imageType, imageErr := w.importer.DownloadCoverImageWithRetry(coverImage, 3)
	imageFailed := imageErr != nil
	if imageErr != nil {
		w.logToFile("processSyncBatches: failed to download image for %s - %s after 3 attempts: %v", artist, title, imageErr)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before retry
		select {
		case <-w.ctx.Done():
			w.logToFile("processSyncBatches: context cancelled, stopping album creation for %s - %s", artist, title)
			return
		default:
		}

		newAlbum = models.Album{
			Title:                 title,
			Artist:                artist,
			ReleaseYear:           year,
			CoverImageURL:         coverImage,
			DiscogsCoverImage:     imageData,
			DiscogsCoverImageType: imageType,
			CoverImageFailed:      imageFailed,
			DiscogsID:             utils.IntPtr(discogsID),
			DiscogsFolderID:       albumFolderID,
		}
		tx = w.db.Begin()
		if tx.Error != nil {
			if attempt < maxRetries && w.isLockTimeout(tx.Error) {
				tx.Rollback()
				// Context-aware backoff
				backoffTime := time.Duration(attempt+1) * 500 * time.Millisecond
				select {
				case <-w.ctx.Done():
					w.logToFile("processSyncBatches: context cancelled during backoff for %s - %s", artist, title)
					return
				case <-time.After(backoffTime):
				}
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
				backoffTime := time.Duration(attempt+1) * 500 * time.Millisecond
				select {
				case <-w.ctx.Done():
					w.logToFile("processSyncBatches: context cancelled during backoff for %s - %s", artist, title)
					return
				case <-time.After(backoffTime):
				}
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

		if err := tx.Commit().Error; err != nil {
			if attempt < maxRetries && w.isLockTimeout(err) {
				backoffTime := time.Duration(attempt+1) * 500 * time.Millisecond
				select {
				case <-w.ctx.Done():
					w.logToFile("processSyncBatches: context cancelled during backoff for %s - %s", artist, title)
					return
				case <-time.After(backoffTime):
				}
				continue
			}
			w.logToFile("processSyncBatches: failed to commit album: %s - %s: %v", artist, title, err)
			w.progressService.Save(w.stateManager.GetState())
			break
		}

		if discogsID > 0 {
			success, errMsg := w.fetchTracksWithRetry(newAlbum.ID, discogsID, title, artist)
			if !success {
				w.logToFile("processSyncBatches: Failed to fetch tracks for album %s - %s: %s", artist, title, errMsg)
				if deleteErr := w.db.Delete(&models.Album{}, newAlbum.ID).Error; deleteErr != nil {
					w.logToFile("processSyncBatches: Failed to delete album after track fetch failure for %s - %s: %v", artist, title, deleteErr)
				} else {
					w.logToFile("processSyncBatches: Deleted album after track fetch failure for %s - %s", artist, title)
					w.db.Create(&models.SyncLog{
						DiscogsID:  discogsID,
						AlbumTitle: title,
						Artist:     artist,
						ErrorType:  "tracks",
						ErrorMsg:   "Deleted album after track fetch failure",
					})
				}
			} else {
				w.logToFile("processSyncBatches: Successfully synced album with tracks: %s - %s", artist, title)
			}
		}

		w.stateManager.UpdateState(func(s *sync.SyncState) {
			s.Processed++
		})
		w.progressService.Save(w.stateManager.GetState())
		w.stateManager.UpdateState(func(s *sync.SyncState) {
			w.removeFirstAlbumFromBatch(s)
		})
		w.progressService.Save(w.stateManager.GetState())
		w.logToFile("processSyncBatches: Album synced successfully: %s - %s, Processed=%d", artist, title, w.stateManager.GetState().Processed)
		break
	}
}

// updateExistingAlbum updates an existing album with new data and context support
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
		if imageData, imageType, err := w.importer.DownloadCoverImageWithRetry(coverImage, 3); err == nil && imageData != nil {
			updates["discogs_cover_image"] = imageData
			updates["discogs_cover_image_type"] = imageType
			updates["cover_image_failed"] = false
		} else if err != nil {
			updates["cover_image_failed"] = true
			w.logToFile("Sync: failed to download cover image for %s - %s after 3 attempts: %v", artist, title, err)
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

	w.stateManager.UpdateState(func(s *sync.SyncState) {
		s.Processed++
	})
	w.progressService.Save(w.stateManager.GetState())
	w.stateManager.UpdateState(func(s *sync.SyncState) {
		w.removeFirstAlbumFromBatch(s)
		if s.Processed%5 == 0 {
			s.APIRemaining = w.client.GetAPIRemaining()
			s.AnonRemaining = w.client.GetAPIRemainingAnon()
		}
	})
	w.progressService.Save(w.stateManager.GetState())
	w.logToFile("Sync: processed=%d/%d", w.stateManager.GetState().Processed, w.stateManager.GetState().Total)
}

// checkPauseState checks if sync is paused and waits for resume using context-aware waiting
func (w *SyncWorker) checkPauseState() bool {
	state := w.stateManager.GetState()
	if state.LastBatch == nil || len(state.LastBatch.Albums) == 0 {
		if state.IsPaused() {
			w.logToFile("Sync: PAUSED - using context-aware wait for resume")

			// Use WaitForPause with context for cooperative pause/resume
			err := w.stateManager.WaitForPause(w.ctx)
			if err != nil {
				w.logToFile("Sync: wait for pause cancelled: %v", err)
				return true
			}

			// Check if we're still paused after wait
			checkState := w.stateManager.GetState()
			if !checkState.IsPaused() {
				w.logToFile("Sync: RESUME DETECTED via channel")
				return true
			}

			// Check if sync stopped while paused
			if !checkState.IsRunning() {
				w.logToFile("Sync: sync stopped while paused")
				return true
			}

			w.logToFile("Sync: continuing loop after wait")
			return true
		}

		w.logToFile("processSyncBatches: batch empty, will fetch next page")
		select {
		case <-w.ctx.Done():
			w.logToFile("Sync: context cancelled during empty batch wait")
			return true
		case <-time.After(200 * time.Millisecond):
		}
		return true
	}
	return false
}

// markComplete marks the sync as complete
func (w *SyncWorker) markComplete(state sync.SyncState) {
	w.logToFile("processSyncBatches: sync complete. Processed=%d, Total=%d, Mode=%s", state.Processed, state.Total, w.config.SyncMode)

	w.stateManager.UpdateState(func(s *sync.SyncState) {
		s.Status = sync.SyncStatusIdle
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
func (w *SyncWorker) removeFirstAlbumFromBatch(s *sync.SyncState) {
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
	f, _ := os.OpenFile(discogs.SyncDebugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}
