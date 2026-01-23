package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func CleanupOldLogs(retentionCount int, logDir string) (int, error) {
	if retentionCount <= 0 {
		retentionCount = 10
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	var logFiles []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "vinylfo_") || strings.HasPrefix(name, "sync_debug_") {
			if strings.HasSuffix(name, ".log") {
				info, _ := entry.Info()
				logFiles = append(logFiles, info)
			}
		}
	}

	if len(logFiles) <= retentionCount {
		return 0, nil
	}

	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime().After(logFiles[j].ModTime())
	})

	deleted := 0
	for i := retentionCount; i < len(logFiles); i++ {
		path := filepath.Join(logDir, logFiles[i].Name())
		if err := os.Remove(path); err == nil {
			deleted++
		}
	}

	return deleted, nil
}

func GetLogFileCount(logDir string) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "vinylfo_") || strings.HasPrefix(name, "sync_debug_") {
			if strings.HasSuffix(name, ".log") {
				count++
			}
		}
	}

	return count, nil
}

func GetTimestampedLogPath(logDir string, prefix string) string {
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(logDir, prefix+"_"+timestamp+".log")
}

// LogFileInfo contains information about a log file
type LogFileInfo struct {
	Filename string    `json:"filename"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
}

// GetLogFiles returns a list of log files sorted by modification time (newest first)
func GetLogFiles(logDir string) ([]LogFileInfo, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var logs []LogFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if (strings.HasPrefix(name, "vinylfo_") || strings.HasPrefix(name, "sync_debug_")) &&
			strings.HasSuffix(name, ".log") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			logs = append(logs, LogFileInfo{
				Filename: name,
				Path:     filepath.Join(logDir, name),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
			})
		}
	}

	// Sort by modification time, newest first
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ModTime.After(logs[j].ModTime)
	})

	return logs, nil
}

// ExportLogsToZip creates a zip file containing the most recent log files.
// Returns the path to the created zip file.
func ExportLogsToZip(logDir string, maxFiles int) (string, error) {
	if maxFiles <= 0 {
		maxFiles = 5
	}

	logs, err := GetLogFiles(logDir)
	if err != nil {
		return "", fmt.Errorf("failed to list log files: %w", err)
	}

	if len(logs) == 0 {
		return "", fmt.Errorf("no log files found")
	}

	// Limit to maxFiles
	if len(logs) > maxFiles {
		logs = logs[:maxFiles]
	}

	// Create zip file in temp directory
	timestamp := time.Now().Format("20060102_150405")
	zipPath := filepath.Join(os.TempDir(), fmt.Sprintf("vinylfo_logs_%s.zip", timestamp))

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, log := range logs {
		if err := addFileToZip(zipWriter, log.Path, log.Filename); err != nil {
			continue // Skip files that can't be added
		}
	}

	return zipPath, nil
}

func addFileToZip(zipWriter *zip.Writer, filePath, filename string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filename
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
