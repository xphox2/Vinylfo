package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDatabaseMigrationsExist(t *testing.T) {
	rootDir := getProjectRoot(t)
	migratePath := filepath.Join(rootDir, "database", "migrate.go")

	if _, err := os.Stat(migratePath); os.IsNotExist(err) {
		t.Fatal("database/migrate.go not found")
	}

	content, err := os.ReadFile(migratePath)
	if err != nil {
		t.Fatalf("Could not read migrate.go: %v", err)
	}

	// Check that AutoMigrate is called for essential models
	requiredModels := []string{
		"Album",
		"AppConfig",
	}

	for _, model := range requiredModels {
		if !strings.Contains(string(content), model) {
			t.Errorf("migrate.go does not reference model: %s", model)
		}
	}

	if !strings.Contains(string(content), "AutoMigrate") {
		t.Error("migrate.go does not call AutoMigrate")
	}
}

func TestModelsExist(t *testing.T) {
	rootDir := getProjectRoot(t)
	modelsDir := filepath.Join(rootDir, "models")

	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		t.Fatal("models directory not found")
	}

	// Check for required model files
	requiredModels := []string{
		"models.go",
		"app_config.go",
	}

	for _, model := range requiredModels {
		modelPath := filepath.Join(modelsDir, model)
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			t.Errorf("Required model file missing: %s", model)
		}
	}
}

func TestModelStructuresAreValid(t *testing.T) {
	rootDir := getProjectRoot(t)

	// Check models.go has Album struct with required fields
	modelsPath := filepath.Join(rootDir, "models", "models.go")
	if content, err := os.ReadFile(modelsPath); err == nil {
		requiredPatterns := []string{
			"type Album struct",
			"ID",
			"Artist",
			"Title",
		}
		for _, pattern := range requiredPatterns {
			if !strings.Contains(string(content), pattern) {
				t.Errorf("models.go missing expected pattern: %s", pattern)
			}
		}
	}

	// Check AppConfig model has required fields
	configPath := filepath.Join(rootDir, "models", "app_config.go")
	if content, err := os.ReadFile(configPath); err == nil {
		requiredFields := []string{
			"ID",
			"DiscogsUsername",
		}
		for _, field := range requiredFields {
			if !strings.Contains(string(content), field) {
				t.Errorf("AppConfig model missing required field: %s", field)
			}
		}
	}
}

func TestDatabasePackageImportsGorm(t *testing.T) {
	rootDir := getProjectRoot(t)
	migratePath := filepath.Join(rootDir, "database", "migrate.go")

	content, err := os.ReadFile(migratePath)
	if err != nil {
		t.Fatalf("Could not read migrate.go: %v", err)
	}

	if !strings.Contains(string(content), "gorm.io/gorm") {
		t.Error("database/migrate.go does not import gorm")
	}
}

func TestSQLiteDriverAvailable(t *testing.T) {
	rootDir := getProjectRoot(t)
	goModPath := filepath.Join(rootDir, "go.mod")

	content, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Could not read go.mod: %v", err)
	}

	// Check for SQLite driver (project uses glebarez/sqlite)
	if !strings.Contains(string(content), "glebarez/sqlite") && !strings.Contains(string(content), "gorm.io/driver/sqlite") {
		t.Error("go.mod does not include SQLite driver")
	}
}
