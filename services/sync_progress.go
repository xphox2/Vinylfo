package services

import (
	"encoding/json"
	"time"

	"vinylfo/models"
	"vinylfo/sync"

	"gorm.io/gorm"
)

// SyncProgressService handles persistence of sync progress
type SyncProgressService struct {
	db *gorm.DB
}

// NewSyncProgressService creates a new SyncProgressService instance
func NewSyncProgressService(db *gorm.DB) *SyncProgressService {
	return &SyncProgressService{db: db}
}

// Load loads the current sync progress from the database
func (s *SyncProgressService) Load(state sync.SyncState) *models.SyncProgress {
	var progress models.SyncProgress

	// Use raw SQL with a short timeout to avoid hanging on locked tables
	err := s.db.Raw("SELECT id, folder_id, folder_name, folder_index, current_page, processed, total_albums, last_activity_at, status, last_batch_json, sync_mode, processed_ids_json FROM sync_progresses ORDER BY id DESC LIMIT 1").Scan(&progress).Error

	if err != nil || progress.ID == 0 {
		return nil
	}

	maxAge := 30 * time.Minute
	if state.IsPaused() {
		maxAge = 4 * time.Hour
	}

	if time.Since(progress.LastActivityAt) > maxAge {
		// Use raw SQL to avoid transaction issues
		s.db.Exec("DELETE FROM sync_progresses WHERE id = ?", progress.ID)
		return nil
	}

	return &progress
}

// Save saves the current sync state to the database
func (s *SyncProgressService) Save(state sync.SyncState) {
	var progress models.SyncProgress
	s.db.FirstOrCreate(&progress, models.SyncProgress{ID: 1})

	progress.SyncMode = state.SyncMode
	progress.FolderID = state.CurrentFolder
	progress.FolderIndex = state.FolderIndex
	progress.CurrentPage = state.CurrentPage
	// Save UniqueProcessed to preserve actual album count (not re-processed albums)
	progress.Processed = state.UniqueProcessed
	if progress.Processed == 0 {
		progress.Processed = state.Processed
	}
	progress.TotalAlbums = state.Total
	progress.LastActivityAt = time.Now()

	if !state.IsRunning() && !state.IsPaused() {
		progress.Status = "completed"
	} else if state.IsPaused() {
		progress.Status = "paused"
	} else {
		progress.Status = "running"
	}

	var folderNames []string
	for _, f := range state.Folders {
		if name, ok := f["name"].(string); ok {
			folderNames = append(folderNames, name)
		}
	}
	if len(folderNames) > 0 && state.FolderIndex < len(folderNames) {
		progress.FolderName = folderNames[state.FolderIndex]
	}

	if state.LastBatch != nil && len(state.LastBatch.Albums) > 0 {
		batchJSON, err := json.Marshal(state.LastBatch)
		if err == nil {
			progress.LastBatchJSON = string(batchJSON)
		}
	} else {
		progress.LastBatchJSON = ""
	}

	// Save ProcessedIDs as JSON
	if state.ProcessedIDs != nil && len(state.ProcessedIDs) > 0 {
		processedIDsJSON, err := json.Marshal(state.ProcessedIDs)
		if err == nil {
			progress.ProcessedIDsJSON = string(processedIDsJSON)
		}
	} else {
		progress.ProcessedIDsJSON = ""
	}

	s.db.Save(&progress)
}

// ArchiveToHistory moves a completed sync progress to history
func (s *SyncProgressService) ArchiveToHistory(progress *models.SyncProgress) {
	var history models.SyncHistory
	history.SyncMode = progress.SyncMode
	history.FolderID = progress.FolderID
	history.FolderName = progress.FolderName
	history.Processed = progress.Processed
	history.TotalAlbums = progress.TotalAlbums
	history.Status = progress.Status
	history.StartedAt = progress.CreatedAt
	history.CompletedAt = progress.LastActivityAt
	if progress.LastActivityAt.IsZero() {
		history.CompletedAt = time.Now()
	}
	history.DurationSecs = int(history.CompletedAt.Sub(history.StartedAt).Seconds())
	if history.DurationSecs < 1 {
		history.DurationSecs = 1
	}

	s.db.Create(&history)
}

// RestoreLastBatch restores the last batch from the database into the sync state
func (s *SyncProgressService) RestoreLastBatch(state *sync.SyncState) {
	progress := s.Load(*state)
	if progress == nil {
		return
	}

	// Restore LastBatch
	if progress.LastBatchJSON != "" {
		var batch sync.SyncBatch
		if err := json.Unmarshal([]byte(progress.LastBatchJSON), &batch); err == nil {
			state.LastBatch = &batch
		}
	}

	// Restore ProcessedIDs
	if progress.ProcessedIDsJSON != "" {
		var processedIDs map[int]bool
		if err := json.Unmarshal([]byte(progress.ProcessedIDsJSON), &processedIDs); err == nil {
			state.ProcessedIDs = processedIDs
		}
	}
}

// Clear deletes all sync progress records
func (s *SyncProgressService) Clear() {
	s.db.Delete(&models.SyncProgress{}, "1=1")
}

// Delete deletes a specific progress record by ID
func (s *SyncProgressService) Delete(progressID uint) {
	s.db.Exec("DELETE FROM sync_progresses WHERE id = ?", progressID)
}
