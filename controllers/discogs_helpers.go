package controllers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// intPtr returns a pointer to the given int, or nil if the value is 0
func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

// downloadImage downloads an image from the given URL and returns the data, content type, and any error
func downloadImage(imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

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

// logToFile writes a log message to the sync debug log file
func logToFile(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	f, _ := os.OpenFile("sync_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}

// isLockTimeout checks if an error is a database lock timeout
func isLockTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Lock wait timeout") || strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "try restarting transaction")
}

// maskValue masks a string value for logging, showing only first and last 4 characters
func maskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
