package syntax

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRequiredStaticFilesExist(t *testing.T) {
	rootDir := getProjectRoot(t)

	requiredFiles := []string{
		"static/css/style.css",
		"static/css/youtube.css",
		"static/js/app.js",
		"static/js/playlist.js",
		"static/js/sync.js",
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(rootDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Required static file missing: %s", file)
		} else {
			// Check file is not empty
			info, _ := os.Stat(filePath)
			if info.Size() == 0 {
				t.Errorf("Static file is empty: %s", file)
			}
		}
	}
}

func TestRequiredTemplatesExist(t *testing.T) {
	rootDir := getProjectRoot(t)

	requiredTemplates := []string{
		"templates/index.html",
		"templates/playback-dashboard.html",
		"templates/playlist.html",
		"templates/settings.html",
		"templates/sync.html",
		"templates/search.html",
		"templates/youtube.html",
	}

	for _, tmpl := range requiredTemplates {
		tmplPath := filepath.Join(rootDir, tmpl)
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			t.Errorf("Required template missing: %s", tmpl)
		}
	}
}

func TestCSSFilesHaveValidStructure(t *testing.T) {
	rootDir := getProjectRoot(t)

	cssFiles := []string{
		"static/css/style.css",
		"static/css/youtube.css",
	}

	for _, cssFile := range cssFiles {
		cssPath := filepath.Join(rootDir, cssFile)
		if _, err := os.Stat(cssPath); os.IsNotExist(err) {
			t.Logf("CSS file not found (skipping): %s", cssFile)
			continue
		}

		content, err := os.ReadFile(cssPath)
		if err != nil {
			t.Errorf("Could not read %s: %v", cssFile, err)
			continue
		}

		issues := validateCSSStructure(string(content))
		if len(issues) > 0 {
			t.Errorf("CSS structure issues in %s:\n  %s", cssFile, strings.Join(issues, "\n  "))
		}
	}
}

func validateCSSStructure(content string) []string {
	var issues []string

	// Remove comments for accurate brace counting
	commentPattern := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	contentNoComments := commentPattern.ReplaceAllString(content, "")

	// Check brace balance
	openBraces := strings.Count(contentNoComments, "{")
	closeBraces := strings.Count(contentNoComments, "}")
	if openBraces != closeBraces {
		issues = append(issues, fmt.Sprintf("Mismatched braces: %d '{' vs %d '}'", openBraces, closeBraces))
	}

	// Check for common syntax errors
	if strings.Contains(content, ";;") {
		issues = append(issues, "Found double semicolons ';;'")
	}

	if strings.Contains(content, "{{") && !strings.Contains(content, "{{") {
		issues = append(issues, "Found double opening braces '{{'")
	}

	// Check for empty rules (selector followed immediately by closing brace)
	emptyRule := regexp.MustCompile(`\{\s*\}`)
	if emptyRule.MatchString(contentNoComments) {
		issues = append(issues, "Found empty CSS rule(s)")
	}

	return issues
}

func TestHTMLTemplatesHaveValidStructure(t *testing.T) {
	rootDir := getProjectRoot(t)

	templateFiles := []string{
		"templates/index.html",
		"templates/playback-dashboard.html",
		"templates/playlist.html",
		"templates/settings.html",
		"templates/sync.html",
		"templates/search.html",
		"templates/youtube.html",
	}

	for _, tmplFile := range templateFiles {
		tmplPath := filepath.Join(rootDir, tmplFile)
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(tmplPath)
		if err != nil {
			t.Errorf("Could not read %s: %v", tmplFile, err)
			continue
		}

		issues := validateHTMLStructure(string(content))
		if len(issues) > 0 {
			t.Errorf("HTML structure issues in %s:\n  %s", tmplFile, strings.Join(issues, "\n  "))
		}
	}
}

func validateHTMLStructure(content string) []string {
	var issues []string

	// Check for basic HTML structure
	hasDoctype := strings.Contains(strings.ToLower(content), "<!doctype html>") ||
		strings.Contains(strings.ToLower(content), "<!doctype")

	// Only check doctype if this looks like a full HTML page
	if strings.Contains(content, "<html") && !hasDoctype {
		issues = append(issues, "Missing DOCTYPE declaration")
	}

	// Check critical tag pairs
	tagPairs := []struct {
		open  string
		close string
	}{
		{"<html", "</html>"},
		{"<head", "</head>"},
		{"<body", "</body>"},
	}

	for _, pair := range tagPairs {
		hasOpen := strings.Contains(strings.ToLower(content), pair.open)
		hasClose := strings.Contains(strings.ToLower(content), pair.close)
		if hasOpen && !hasClose {
			issues = append(issues, fmt.Sprintf("Missing closing tag for %s", pair.open))
		}
	}

	// Check script and style tag balance
	scriptOpen := len(regexp.MustCompile(`(?i)<script[^>]*>`).FindAllString(content, -1))
	scriptClose := len(regexp.MustCompile(`(?i)</script>`).FindAllString(content, -1))
	if scriptOpen != scriptClose {
		issues = append(issues, fmt.Sprintf("Mismatched <script> tags: %d opening vs %d closing", scriptOpen, scriptClose))
	}

	styleOpen := len(regexp.MustCompile(`(?i)<style[^>]*>`).FindAllString(content, -1))
	styleClose := len(regexp.MustCompile(`(?i)</style>`).FindAllString(content, -1))
	if styleOpen != styleClose {
		issues = append(issues, fmt.Sprintf("Mismatched <style> tags: %d opening vs %d closing", styleOpen, styleClose))
	}

	// Check Go template syntax balance
	templateOpen := strings.Count(content, "{{")
	templateClose := strings.Count(content, "}}")
	if templateOpen != templateClose {
		issues = append(issues, fmt.Sprintf("Mismatched Go template delimiters: %d '{{' vs %d '}}'", templateOpen, templateClose))
	}

	return issues
}

func TestJavaScriptFilesHaveValidStructure(t *testing.T) {
	rootDir := getProjectRoot(t)

	// Note: sync.js is excluded because it uses complex template literals with HTML
	// that confuse the simple regex-based brace counting. It's validated by Node.js
	// in TestJavaScriptWithNode and by TestSyncJSRateLimitHandling.
	jsFiles := []string{
		"static/js/app.js",
		"static/js/playlist.js",
		// "static/js/sync.js", // Has template literals with HTML - validated by Node.js instead
	}

	for _, jsFile := range jsFiles {
		jsPath := filepath.Join(rootDir, jsFile)
		if _, err := os.Stat(jsPath); os.IsNotExist(err) {
			t.Logf("JS file not found (skipping): %s", jsFile)
			continue
		}

		content, err := os.ReadFile(jsPath)
		if err != nil {
			t.Errorf("Could not read %s: %v", jsFile, err)
			continue
		}

		issues := validateJavaScriptStructure(string(content))
		if len(issues) > 0 {
			t.Errorf("JavaScript structure issues in %s:\n  %s", jsFile, strings.Join(issues, "\n  "))
		}
	}
}

func validateJavaScriptStructure(content string) []string {
	var issues []string

	// Remove string literals and comments for accurate counting
	// Remove template literals
	templatePattern := regexp.MustCompile("`[^`]*`")
	cleaned := templatePattern.ReplaceAllString(content, "``")

	// Remove single-quoted strings
	singleQuotePattern := regexp.MustCompile(`'[^'\\]*(?:\\.[^'\\]*)*'`)
	cleaned = singleQuotePattern.ReplaceAllString(cleaned, "''")

	// Remove double-quoted strings
	doubleQuotePattern := regexp.MustCompile(`"[^"\\]*(?:\\.[^"\\]*)*"`)
	cleaned = doubleQuotePattern.ReplaceAllString(cleaned, `""`)

	// Remove single-line comments
	singleLineComment := regexp.MustCompile(`//.*$`)
	cleaned = singleLineComment.ReplaceAllString(cleaned, "")

	// Remove multi-line comments
	multiLineComment := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	cleaned = multiLineComment.ReplaceAllString(cleaned, "")

	// Check brace balance
	openBraces := strings.Count(cleaned, "{")
	closeBraces := strings.Count(cleaned, "}")
	if openBraces != closeBraces {
		issues = append(issues, fmt.Sprintf("Mismatched braces: %d '{' vs %d '}'", openBraces, closeBraces))
	}

	// Check parentheses balance
	openParens := strings.Count(cleaned, "(")
	closeParens := strings.Count(cleaned, ")")
	if openParens != closeParens {
		issues = append(issues, fmt.Sprintf("Mismatched parentheses: %d '(' vs %d ')'", openParens, closeParens))
	}

	// Check bracket balance
	openBrackets := strings.Count(cleaned, "[")
	closeBrackets := strings.Count(cleaned, "]")
	if openBrackets != closeBrackets {
		issues = append(issues, fmt.Sprintf("Mismatched brackets: %d '[' vs %d ']'", openBrackets, closeBrackets))
	}

	return issues
}

func TestJavaScriptWithNode(t *testing.T) {
	// Check if node is available
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available - skipping JavaScript syntax validation")
	}

	rootDir := getProjectRoot(t)

	jsFiles := []string{
		"static/js/app.js",
		"static/js/playlist.js",
		"static/js/sync.js",
	}

	for _, jsFile := range jsFiles {
		jsPath := filepath.Join(rootDir, jsFile)
		if _, err := os.Stat(jsPath); os.IsNotExist(err) {
			continue
		}

		// Use Node.js to check syntax
		cmd := exec.Command("node", "--check", jsPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("JavaScript syntax error in %s:\n%s", jsFile, string(output))
		}
	}
}

func TestNoConsoleLogsInProduction(t *testing.T) {
	rootDir := getProjectRoot(t)

	jsFiles := []string{
		"static/js/app.js",
		"static/js/playlist.js",
	}

	for _, jsFile := range jsFiles {
		jsPath := filepath.Join(rootDir, jsFile)
		content, err := os.ReadFile(jsPath)
		if err != nil {
			continue
		}

		// Count console.log statements (excluding sync.js which may need debug logs)
		logCount := strings.Count(string(content), "console.log")
		if logCount > 5 {
			t.Logf("WARNING: %s has %d console.log statements - consider removing for production", jsFile, logCount)
		}
	}
}

// TestSyncJSRateLimitHandling validates critical rate limit handling logic in sync.js
func TestSyncJSRateLimitHandling(t *testing.T) {
	rootDir := getProjectRoot(t)
	syncJSPath := filepath.Join(rootDir, "static/js/sync.js")

	content, err := os.ReadFile(syncJSPath)
	if err != nil {
		t.Fatalf("Could not read sync.js: %v", err)
	}

	contentStr := string(content)

	// Test 1: Verify polling stays active during rate limit
	t.Run("PollingStaysActiveDuringRateLimit", func(t *testing.T) {
		// Check for the critical fix: keeping polling active during rate limit
		if !strings.Contains(contentStr, "pollingActive") {
			t.Error("sync.js should have pollingActive flag")
		}

		// Check that rate limit handling ensures polling stays active
		if !strings.Contains(contentStr, "is_rate_limited") {
			t.Error("sync.js should handle is_rate_limited field")
		}
	})

	// Test 2: Verify rate limit countdown mechanism exists
	t.Run("RateLimitCountdownExists", func(t *testing.T) {
		if !strings.Contains(contentStr, "rateLimitSecondsLeft") {
			t.Error("sync.js should track rateLimitSecondsLeft")
		}

		if !strings.Contains(contentStr, "startRateLimitCountdown") {
			t.Error("sync.js should have startRateLimitCountdown method")
		}

		if !strings.Contains(contentStr, "stopRateLimitCountdown") {
			t.Error("sync.js should have stopRateLimitCountdown method")
		}
	})

	// Test 3: Verify wasRateLimited tracking for auto-resume
	t.Run("WasRateLimitedTracking", func(t *testing.T) {
		if !strings.Contains(contentStr, "wasRateLimited") {
			t.Error("sync.js should track wasRateLimited for auto-resume logic")
		}
	})

	// Test 4: Verify rate limit cleared detection
	t.Run("RateLimitClearedDetection", func(t *testing.T) {
		// Check for multiple conditions to detect rate limit cleared
		if !strings.Contains(contentStr, "rate_limit_seconds_left") {
			t.Error("sync.js should check rate_limit_seconds_left from server")
		}

		// Should handle case where is_rate_limited becomes false
		if !strings.Contains(contentStr, "!progress.is_rate_limited") {
			t.Error("sync.js should detect when is_rate_limited becomes false")
		}
	})

	// Test 5: Verify resumeSync function exists for auto-resume
	t.Run("ResumeSyncExists", func(t *testing.T) {
		if !strings.Contains(contentStr, "resumeSync") {
			t.Error("sync.js should have resumeSync method for auto-resume")
		}

		// Should call the resume API endpoint
		if !strings.Contains(contentStr, "/sync/resume-pause") {
			t.Error("sync.js resumeSync should call the resume-pause endpoint")
		}
	})

	// Test 6: Verify pollInProgress flag handling
	t.Run("PollInProgressHandling", func(t *testing.T) {
		if !strings.Contains(contentStr, "pollInProgress") {
			t.Error("sync.js should have pollInProgress flag")
		}

		// Should reset pollInProgress on error
		if !strings.Contains(contentStr, "this.pollInProgress = false") {
			t.Error("sync.js should reset pollInProgress flag")
		}
	})

	// Test 7: Verify scheduleNextPoll exists for continuous polling
	t.Run("ScheduleNextPollExists", func(t *testing.T) {
		if !strings.Contains(contentStr, "scheduleNextPoll") {
			t.Error("sync.js should have scheduleNextPoll method")
		}
	})

	// Test 8: Verify stale poll detection
	t.Run("StalePollDetection", func(t *testing.T) {
		if !strings.Contains(contentStr, "stalePollTimeout") {
			t.Error("sync.js should have stalePollTimeout for detecting stuck polls")
		}
	})

	// Test 9: Verify error recovery doesn't stop polling completely
	t.Run("ErrorRecoveryKeepsPolling", func(t *testing.T) {
		// After max retries, should slow down polling instead of stopping
		if !strings.Contains(contentStr, "15000") {
			t.Error("sync.js should slow down polling on errors (15s interval)")
		}
	})

	// Test 10: Verify UI updates for rate limit state
	t.Run("RateLimitUIUpdates", func(t *testing.T) {
		// Should update button text during rate limit
		if !strings.Contains(contentStr, "pause-sync") {
			t.Error("sync.js should update pause-sync button")
		}

		// Should show rate limit message
		if !strings.Contains(contentStr, "API rate limit") {
			t.Error("sync.js should display rate limit message to user")
		}
	})
}

// TestSyncJSProgressEndpointFields validates sync.js handles all progress endpoint fields
func TestSyncJSProgressEndpointFields(t *testing.T) {
	rootDir := getProjectRoot(t)
	syncJSPath := filepath.Join(rootDir, "static/js/sync.js")

	content, err := os.ReadFile(syncJSPath)
	if err != nil {
		t.Fatalf("Could not read sync.js: %v", err)
	}

	contentStr := string(content)

	// Required fields from the progress endpoint that sync.js must handle
	requiredFields := []string{
		"is_running",
		"is_paused",
		"processed",
		"total",
		"is_rate_limited",
		"rate_limit_seconds_left",
		"is_stalled",
		"folder_name",
		"total_folders",
		"folder_index",
	}

	for _, field := range requiredFields {
		if !strings.Contains(contentStr, field) {
			t.Errorf("sync.js should handle progress field: %s", field)
		}
	}
}

// TestSyncJSStateTransitions validates sync.js handles all state transitions
func TestSyncJSStateTransitions(t *testing.T) {
	rootDir := getProjectRoot(t)
	syncJSPath := filepath.Join(rootDir, "static/js/sync.js")

	content, err := os.ReadFile(syncJSPath)
	if err != nil {
		t.Fatalf("Could not read sync.js: %v", err)
	}

	contentStr := string(content)

	// Test state transition: running -> rate limited (paused)
	t.Run("RunningToRateLimited", func(t *testing.T) {
		// Should detect when backend pauses due to rate limit
		if !strings.Contains(contentStr, "progress.is_paused && !this.isPaused") {
			t.Error("sync.js should detect transition from running to paused")
		}
	})

	// Test state transition: rate limited -> running (auto-resume)
	t.Run("RateLimitedToRunning", func(t *testing.T) {
		// Should detect when rate limit clears and backend resumes
		if !strings.Contains(contentStr, "progress.is_running") {
			t.Error("sync.js should detect when backend resumes")
		}
	})

	// Test state transition: detect missed transitions
	t.Run("DetectMissedTransitions", func(t *testing.T) {
		// Should handle case where backend state changed but frontend missed it
		if !strings.Contains(contentStr, "frontend thought paused") ||
			!strings.Contains(contentStr, "backend already resumed") {
			t.Log("WARNING: sync.js may not handle all missed state transitions")
		}
	})
}
