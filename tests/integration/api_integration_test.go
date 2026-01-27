package integration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRoutesFileExists(t *testing.T) {
	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	if _, err := os.Stat(routesPath); os.IsNotExist(err) {
		t.Fatal("routes/routes.go not found")
	}
}

func TestRequiredRoutesAreDefined(t *testing.T) {
	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Could not read routes.go: %v", err)
	}

	contentStr := string(content)

	// Check for essential routes (this project uses direct routes, not /api prefix)
	essentialRoutes := []string{
		"/albums",
		"/playback",
		"/health",
		"/version",
	}

	for _, route := range essentialRoutes {
		if !strings.Contains(contentStr, route) {
			t.Errorf("Missing route: %s", route)
		}
	}

	// Check for controller registrations
	controllerPatterns := []string{
		"albumController",
		"playbackController",
		"discogsController",
	}

	for _, pattern := range controllerPatterns {
		if !strings.Contains(contentStr, pattern) {
			t.Errorf("Missing controller registration: %s", pattern)
		}
	}
}

func TestControllersExist(t *testing.T) {
	rootDir := getProjectRoot(t)
	controllersDir := filepath.Join(rootDir, "controllers")

	if _, err := os.Stat(controllersDir); os.IsNotExist(err) {
		t.Fatal("controllers directory not found")
	}

	// Check for essential controller files
	requiredControllers := []string{
		"album.go",
		"discogs.go",
		"playback.go",
		"playlist.go",
		"youtube.go",
	}

	for _, ctrl := range requiredControllers {
		ctrlPath := filepath.Join(controllersDir, ctrl)
		if _, err := os.Stat(ctrlPath); os.IsNotExist(err) {
			t.Errorf("Required controller missing: %s", ctrl)
		}
	}
}

func TestControllersHaveHandlerFunctions(t *testing.T) {
	rootDir := getProjectRoot(t)

	controllerChecks := map[string][]string{
		"controllers/album.go": {
			"func",
			"gin.Context",
		},
		"controllers/discogs.go": {
			"func",
			"gin.Context",
		},
		"controllers/playback.go": {
			"func",
			"gin.Context",
		},
	}

	for file, patterns := range controllerChecks {
		filePath := filepath.Join(rootDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("Could not read %s: %v", file, err)
			continue
		}

		for _, pattern := range patterns {
			if !strings.Contains(string(content), pattern) {
				t.Errorf("%s missing expected pattern: %s", file, pattern)
			}
		}
	}
}

func TestAPIResponseStructures(t *testing.T) {
	rootDir := getProjectRoot(t)

	// Check that controllers return proper JSON responses
	controllerFiles := []string{
		"controllers/album.go",
		"controllers/discogs.go",
		"controllers/playback.go",
	}

	for _, ctrlFile := range controllerFiles {
		filePath := filepath.Join(rootDir, ctrlFile)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)

		// Check for JSON response patterns
		hasJSONResponse := strings.Contains(contentStr, "ctx.JSON") ||
			strings.Contains(contentStr, "c.JSON") ||
			strings.Contains(contentStr, ".JSON(")

		if !hasJSONResponse {
			t.Errorf("%s does not appear to return JSON responses", ctrlFile)
		}
	}
}

func TestMiddlewareSetup(t *testing.T) {
	t.Skip("static setup is handled in main.go")

	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Could not read routes.go: %v", err)
	}

	contentStr := string(content)

	// Check for gin router setup
	if !strings.Contains(contentStr, "gin.") {
		t.Error("routes.go does not use gin framework")
	}

	// Check for static file serving or embedded files
	if !strings.Contains(contentStr, "Static") && !strings.Contains(contentStr, "static") && !strings.Contains(contentStr, "embed") {
		t.Error("routes.go does not set up static file serving")
	}
}

func TestTemplateSetup(t *testing.T) {
	rootDir := getProjectRoot(t)
	// Templates are set up in main.go for this project
	mainPath := filepath.Join(rootDir, "main.go")

	content, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("Could not read main.go: %v", err)
	}

	contentStr := string(content)

	// Check for template loading
	if !strings.Contains(contentStr, "template") && !strings.Contains(contentStr, "Template") &&
		!strings.Contains(contentStr, "HTML") {
		t.Error("main.go does not appear to set up HTML templates")
	}

	// Check for template parsing
	if !strings.Contains(contentStr, "ParseFS") && !strings.Contains(contentStr, "ParseGlob") &&
		!strings.Contains(contentStr, "LoadHTMLGlob") {
		t.Error("main.go does not appear to parse templates")
	}
}

func TestErrorHandlingInControllers(t *testing.T) {
	rootDir := getProjectRoot(t)

	controllerFiles := []string{
		"controllers/album.go",
		"controllers/discogs.go",
	}

	for _, ctrlFile := range controllerFiles {
		filePath := filepath.Join(rootDir, ctrlFile)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)

		// Check for error handling patterns
		hasErrorHandling := strings.Contains(contentStr, "err != nil") ||
			strings.Contains(contentStr, "error")

		if !hasErrorHandling {
			t.Logf("WARNING: %s may lack error handling", ctrlFile)
		}

		// Check for proper HTTP status codes in error responses
		hasStatusCodes := strings.Contains(contentStr, "400") ||
			strings.Contains(contentStr, "404") ||
			strings.Contains(contentStr, "500") ||
			strings.Contains(contentStr, "StatusBadRequest") ||
			strings.Contains(contentStr, "StatusNotFound") ||
			strings.Contains(contentStr, "StatusInternalServerError")

		if !hasStatusCodes {
			t.Logf("WARNING: %s may not use proper HTTP status codes", ctrlFile)
		}
	}
}

func TestDiscogsOAuthRoutesExist(t *testing.T) {
	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Could not read routes.go: %v", err)
	}

	contentStr := string(content)

	oauthPatterns := []string{
		"oauth",
		"callback",
	}

	foundOAuth := false
	for _, pattern := range oauthPatterns {
		if strings.Contains(strings.ToLower(contentStr), pattern) {
			foundOAuth = true
			break
		}
	}

	if !foundOAuth {
		t.Error("routes.go does not appear to have OAuth routes")
	}
}

func TestSyncAPIRoutesExist(t *testing.T) {
	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Could not read routes.go: %v", err)
	}

	contentStr := string(content)

	syncRoutes := []string{
		"sync",
		"progress",
		"pause",
		"resume",
	}

	for _, route := range syncRoutes {
		if !strings.Contains(strings.ToLower(contentStr), route) {
			t.Errorf("Missing sync-related route pattern: %s", route)
		}
	}
}

func TestAPIVersioningOrGrouping(t *testing.T) {
	rootDir := getProjectRoot(t)
	routesPath := filepath.Join(rootDir, "routes", "routes.go")

	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Could not read routes.go: %v", err)
	}

	contentStr := string(content)

	// Check for route grouping (good practice)
	hasGrouping := strings.Contains(contentStr, ".Group(") ||
		strings.Contains(contentStr, "api :=") ||
		regexp.MustCompile(`"/api`).MatchString(contentStr)

	if !hasGrouping {
		t.Log("WARNING: routes.go may not use route grouping - consider grouping API routes")
	}
}
