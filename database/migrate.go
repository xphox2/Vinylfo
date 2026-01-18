package database

import (
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"vinylfo/models"
)

// DB is the global database instance
var DB *gorm.DB

// InitDB initializes the database connection
func InitDB() (*gorm.DB, error) {
	// Load environment variables
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback to individual environment variables if DATABASE_URL is not set
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")

		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
		return nil, err
	}

	// Run migrations for all models
	err = db.AutoMigrate(&models.Album{}, &models.Track{}, &models.PlaybackSession{}, &models.SessionPlaylist{}, &models.SessionSharing{}, &models.SessionNote{}, &models.AppConfig{}, &models.TrackHistory{}, &models.SyncLog{}, &models.SyncProgress{}, &models.SyncHistory{}, &models.DurationSource{}, &models.DurationResolution{}, &models.DurationResolverProgress{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
		return nil, err
	}

	// Migration: Fix album unique constraint from title-only to title+artist composite
	// This allows different artists to have albums with the same title (e.g., "Greatest Hits")
	migrator := db.Migrator()
	if migrator.HasIndex(&models.Album{}, "title") {
		// Old index exists on title only - drop it
		if err := migrator.DropIndex(&models.Album{}, "title"); err != nil {
			log.Printf("Note: Could not drop old title index (may not exist): %v", err)
		} else {
			log.Println("Dropped old title-only unique index")
		}
	}
	// The new composite index idx_title_artist will be created by AutoMigrate from the struct tags

	// Ensure exactly one AppConfig row exists
	var count int64
	db.Model(&models.AppConfig{}).Count(&count)
	if count == 0 {
		if err := db.Create(&models.AppConfig{}).Error; err != nil {
			log.Fatal("Failed to create default AppConfig:", err)
			return nil, err
		}
		log.Println("Created default AppConfig row")
	}

	DB = db
	log.Println("Database connected successfully")

	return db, nil
}

// GetDB returns the global database instance
func GetDB() *gorm.DB {
	return DB
}
