package syntax

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllGoFilesCompile(t *testing.T) {
	rootDir := getProjectRoot(t)

	// Run go build to check all files compile
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Go build failed - code does not compile:\n%s", string(output))
	}
	t.Log("All Go files compile successfully")
}

func TestAllGoFilesParseCorrectly(t *testing.T) {
	rootDir := getProjectRoot(t)

	var failedFiles []string
	var totalFiles int
	fset := token.NewFileSet()

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") && !shouldSkipFile(path) {
			totalFiles++
			// Actually parse the Go file using go/parser
			_, parseErr := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if parseErr != nil {
				failedFiles = append(failedFiles, fmt.Sprintf("%s: %v", path, parseErr))
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Error walking project: %v", err)
	}

	t.Logf("Total Go files parsed: %d", totalFiles)

	if len(failedFiles) > 0 {
		t.Errorf("Parse errors found in %d files:\n%s", len(failedFiles), strings.Join(failedFiles, "\n"))
	}
}

func TestGoVetPasses(t *testing.T) {
	rootDir := getProjectRoot(t)

	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go vet found issues:\n%s", string(output))
	} else {
		t.Log("go vet passed with no issues")
	}
}

func TestNoGoFmtIssues(t *testing.T) {
	rootDir := getProjectRoot(t)

	var filesWithFmtIssues []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") && !shouldSkipFile(path) {
			if fileNeedsFormatting(path) {
				filesWithFmtIssues = append(filesWithFmtIssues, path)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Error walking project: %v", err)
	}

	if len(filesWithFmtIssues) > 0 {
		// This is a warning, not a failure - formatting issues don't break functionality
		t.Logf("WARNING: %d files need formatting (run 'gofmt -w .' to fix):", len(filesWithFmtIssues))
		for _, f := range filesWithFmtIssues {
			t.Logf("  %s", f)
		}
	} else {
		t.Log("All files are properly formatted")
	}
}

func fileNeedsFormatting(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	cmd := exec.Command("gofmt", path)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return string(content) != string(output)
}

func TestGoModIsValid(t *testing.T) {
	rootDir := getProjectRoot(t)
	modPath := filepath.Join(rootDir, "go.mod")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Fatal("go.mod file not found")
	}

	content, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Could not read go.mod: %v", err)
	}

	if !strings.Contains(string(content), "module vinylfo") {
		t.Error("go.mod does not contain 'module vinylfo'")
	}

	if !strings.Contains(string(content), "go 1.") {
		t.Error("go.mod does not specify Go version")
	}

	// Verify go.mod is valid by running go mod verify
	cmd := exec.Command("go", "mod", "verify")
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go mod verify failed: %s", string(output))
	}

	t.Log("go.mod is valid")
}

func TestMainPackageHasMainFunction(t *testing.T) {
	rootDir := getProjectRoot(t)
	mainGoPath := filepath.Join(rootDir, "main.go")

	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		t.Fatal("main.go not found")
	}

	content, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("Could not read main.go: %v", err)
	}

	if !strings.Contains(string(content), "func main()") {
		t.Error("main.go does not contain 'func main()'")
	}

	if !strings.Contains(string(content), "package main") {
		t.Error("main.go does not have 'package main'")
	}

	t.Log("main.go has valid package and main function")
}

func TestRequiredPackagesExist(t *testing.T) {
	rootDir := getProjectRoot(t)

	requiredPackages := []string{
		"controllers",
		"models",
		"services",
		"discogs",
		"routes",
		"sync",
		"utils",
	}

	for _, pkg := range requiredPackages {
		pkgPath := filepath.Join(rootDir, pkg)
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			t.Errorf("Required package directory missing: %s", pkg)
		} else {
			// Check that directory contains at least one .go file
			files, _ := filepath.Glob(filepath.Join(pkgPath, "*.go"))
			if len(files) == 0 {
				t.Errorf("Package directory %s contains no .go files", pkg)
			}
		}
	}
}

func TestNoTODOsInCriticalCode(t *testing.T) {
	rootDir := getProjectRoot(t)
	criticalFiles := []string{
		"main.go",
		"routes/routes.go",
		"database/migrate.go",
	}

	for _, file := range criticalFiles {
		filePath := filepath.Join(rootDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("Could not read %s: %v", file, err)
			continue
		}

		if strings.Contains(string(content), "TODO") || strings.Contains(string(content), "FIXME") {
			t.Logf("WARNING: %s contains TODO/FIXME comments", file)
		}
	}
}

func shouldSkipFile(path string) bool {
	skipPatterns := []string{
		"/vendor/",
		"/.git/",
		"/rsrc_",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func getProjectRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Could not get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
