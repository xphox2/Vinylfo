package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BackupResult contains information about a backup operation
type BackupResult struct {
	BackupPath string    `json:"backup_path"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
}

// BackupInfo contains information about an existing backup file
type BackupInfo struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupDatabase creates a backup of the SQLite database using VACUUM INTO.
// This creates a consistent, defragmented copy of the database.
func BackupDatabase(db *gorm.DB, dbPath string) (*BackupResult, error) {
	if dbPath == "" {
		dbPath = os.Getenv("DB_PATH")
		if dbPath == "" {
			dbPath = "data/vinylfo.db"
		}
	}

	// Create backups directory
	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("2006-01-02_150405")
	backupFilename := fmt.Sprintf("vinylfo_%s.db", timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	// Use VACUUM INTO to create a consistent backup
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// VACUUM INTO creates a new database file with the contents of the current database
	// This is safe to run while the database is in use (with WAL mode)
	_, err = sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}

	// Get backup file info
	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	return &BackupResult{
		BackupPath: backupPath,
		Size:       info.Size(),
		CreatedAt:  time.Now(),
	}, nil
}

// ListBackups returns a list of available backup files, sorted by creation time (newest first)
func ListBackups(dbPath string) ([]BackupInfo, error) {
	if dbPath == "" {
		dbPath = os.Getenv("DB_PATH")
		if dbPath == "" {
			dbPath = "data/vinylfo.db"
		}
	}

	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")

	// Check if backup directory exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "vinylfo_") || !strings.HasSuffix(name, ".db") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:  name,
			Path:      filepath.Join(backupDir, name),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// CleanupOldBackups removes backup files older than the specified retention count.
// Keeps the most recent 'keep' backups and deletes the rest.
func CleanupOldBackups(dbPath string, keep int) (int, error) {
	if keep <= 0 {
		keep = 5 // Default retention
	}

	backups, err := ListBackups(dbPath)
	if err != nil {
		return 0, err
	}

	if len(backups) <= keep {
		return 0, nil
	}

	deleted := 0
	for i := keep; i < len(backups); i++ {
		if err := os.Remove(backups[i].Path); err != nil {
			continue
		}
		deleted++
	}

	return deleted, nil
}

// GetBackupStats returns statistics about the backup directory
func GetBackupStats(dbPath string) (count int, totalSize int64, oldestBackup, newestBackup *time.Time, err error) {
	backups, err := ListBackups(dbPath)
	if err != nil {
		return 0, 0, nil, nil, err
	}

	if len(backups) == 0 {
		return 0, 0, nil, nil, nil
	}

	count = len(backups)
	for _, b := range backups {
		totalSize += b.Size
	}

	oldest := backups[len(backups)-1].CreatedAt
	newest := backups[0].CreatedAt

	return count, totalSize, &oldest, &newest, nil
}
