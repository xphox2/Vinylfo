package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"vinylfo/models"
)

// DB is the global database instance
var DB *gorm.DB

// InitDB initializes the database connection
func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		missingVars := []string{}
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")

		if dbUser == "" {
			missingVars = append(missingVars, "DB_USER")
		}
		if dbPass == "" {
			missingVars = append(missingVars, "DB_PASS")
		}
		if dbHost == "" {
			missingVars = append(missingVars, "DB_HOST")
		}
		if dbPort == "" {
			missingVars = append(missingVars, "DB_PORT")
		}
		if dbName == "" {
			missingVars = append(missingVars, "DB_NAME")
		}

		if len(missingVars) > 0 {
			return nil, fmt.Errorf("missing required environment variables: %s. Either set DATABASE_URL or all of: DB_USER, DB_PASS, DB_HOST, DB_PORT, DB_NAME", strings.Join(missingVars, ", "))
		}

		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Run migrations for all models
	err = db.AutoMigrate(&models.Album{}, &models.Track{}, &models.PlaybackSession{}, &models.SessionPlaylist{}, &models.SessionSharing{}, &models.SessionNote{}, &models.AppConfig{}, &models.TrackHistory{}, &models.SyncLog{}, &models.SyncProgress{}, &models.SyncHistory{}, &models.DurationSource{}, &models.DurationResolution{}, &models.DurationResolverProgress{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
		return nil, err
	}

	// Migration: Fix album unique constraint from title-only to title+artist composite
	migrator := db.Migrator()
	if migrator.HasIndex(&models.Album{}, "title") {
		if err := migrator.DropIndex(&models.Album{}, "title"); err != nil {
			log.Printf("Note: Could not drop old title index (may not exist): %v", err)
		} else {
			log.Println("Dropped old title-only unique index")
		}
	}

	// Ensure exactly one AppConfig row exists
	var count int64
	db.Model(&models.AppConfig{}).Count(&count)
	if count == 0 {
		log.Println("Creating default AppConfig row...")
		if err := db.Create(&models.AppConfig{}).Error; err != nil {
			log.Printf("Warning: Failed to create default AppConfig: %v", err)
		} else {
			log.Println("Default AppConfig row created")
		}
	}

	// Migration: Add YouTube OAuth columns if they don't exist
	var columnCount int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'app_configs' AND column_name = 'youtube_access_token'").Scan(&columnCount)
	if columnCount == 0 {
		log.Println("Adding YouTube OAuth columns to app_configs table...")
		if err := db.Exec(`
			ALTER TABLE app_configs 
			ADD COLUMN youtube_access_token VARCHAR(500) DEFAULT NULL,
			ADD COLUMN youtube_refresh_token VARCHAR(500) DEFAULT NULL,
			ADD COLUMN youtube_token_expiry DATETIME(3) DEFAULT NULL,
			ADD COLUMN youtube_connected TINYINT(1) DEFAULT 0
		`).Error; err != nil {
			log.Printf("Warning: Failed to add YouTube OAuth columns: %v", err)
		} else {
			log.Println("YouTube OAuth columns added successfully")
		}
	}

	// Verify default row exists
	var config models.AppConfig
	if err := db.First(&config).Error; err == nil {
		if config.YouTubeConnected && (config.YouTubeAccessToken == "" || config.YouTubeRefreshToken == "") {
			log.Println("Warning: YouTubeConnected is true but tokens are missing - tokens may not have been saved properly")
		}
	}

	DB = db
	log.Println("Database connected successfully")

	return db, nil
}

// GetDB returns the global database instance
func GetDB() *gorm.DB {
	return DB
}
