package utils

import (
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
