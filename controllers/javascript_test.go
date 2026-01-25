package controllers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaScriptSyntaxValidation(t *testing.T) {
	// This test validates JavaScript files using Node.js
	// It runs as part of the main test suite so `go test ./...` catches JS syntax errors
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available - skipping JavaScript syntax validation")
	}

	jsFiles := []string{
		"static/js/app.js",
		"static/js/playlist.js",
		"static/js/sync.js",
	}

	for _, jsFile := range jsFiles {
		jsPath := filepath.Join("..", jsFile)
		if _, err := exec.LookPath("node"); err != nil {
			continue
		}

		cmd := exec.Command("node", "--check", jsPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("JavaScript syntax error in %s:\n%s", jsFile, string(output))
		}
	}
}

func TestSyncJSCriticalFixesExist(t *testing.T) {
	content, err := os.ReadFile("../static/js/sync.js")
	if err != nil {
		t.Fatalf("Could not read sync.js: %v", err)
	}

	contentStr := string(content)

	tests := []struct {
		name    string
		pattern string
		errMsg  string
	}{
		{"PollingActiveFlag", "pollingActive", "sync.js should have pollingActive flag"},
		{"RateLimitHandling", "is_rate_limited", "sync.js should handle is_rate_limited"},
		{"RateLimitSecondsLeft", "rateLimitSecondsLeft", "sync.js should track rateLimitSecondsLeft"},
		{"CountdownMethods", "startRateLimitCountdown", "sync.js should have startRateLimitCountdown"},
		{"WasRateLimited", "wasRateLimited", "sync.js should track wasRateLimited"},
		{"ResumeSync", "resumeSync", "sync.js should have resumeSync method"},
		{"PauseSyncButton", "pause-sync", "sync.js should update pause-sync button"},
		{"RateLimitMessage", "API rate limit", "sync.js should display rate limit message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(contentStr, tt.pattern) {
				t.Error(tt.errMsg)
			}
		})
	}
}
