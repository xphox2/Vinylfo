package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"vinylfo/models"
)

// DB is the global database instance
var DB *gorm.DB

// DBType stores the current database type for use in other functions
var DBType string

// InitDB initializes the database connection
func InitDB() (*gorm.DB, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "sqlite" // Default to SQLite for desktop deployment
	}
	DBType = dbType

	var db *gorm.DB
	var err error

	if dbType == "sqlite" {
		db, err = initSQLite()
	} else {
		db, err = initMySQL()
	}

	if err != nil {
		return nil, err
	}

	// Configure connection pool based on database type
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
		return nil, err
	}

	if dbType == "sqlite" {
		// SQLite: allow a small pool for read concurrency
		sqlDB.SetMaxOpenConns(5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(time.Hour)

		// SQLite performance tuning
		sqlDB.Exec("PRAGMA foreign_keys = ON")
		sqlDB.Exec("PRAGMA journal_mode = WAL")
		sqlDB.Exec("PRAGMA synchronous = NORMAL")
		sqlDB.Exec("PRAGMA cache_size = -64000")   // 64MB cache
		sqlDB.Exec("PRAGMA busy_timeout = 5000")   // 5 second wait for locks
		sqlDB.Exec("PRAGMA mmap_size = 134217728") // 128MB memory-mapped I/O

		// Run integrity check on startup
		var integrityResult string
		sqlDB.QueryRow("PRAGMA integrity_check").Scan(&integrityResult)
		if integrityResult != "ok" {
			log.Printf("WARNING: Database integrity check failed: %s", integrityResult)
		}
	} else {
		// MySQL: connection pool for concurrent access
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// Run migrations for all models
	err = db.AutoMigrate(
		&models.Album{},
		&models.Track{},
		&models.PlaybackSession{},
		&models.SessionPlaylist{},
		&models.SessionSharing{},
		&models.SessionNote{},
		&models.AppConfig{},
		&models.TrackHistory{},
		&models.SyncLog{},
		&models.SyncProgress{},
		&models.SyncHistory{},
		&models.DurationSource{},
		&models.DurationResolution{},
		&models.DurationResolverProgress{},
		&models.PKCEState{},
		&models.AuditLog{},
		// YouTube Sync models
		&models.TrackYouTubeMatch{},
		&models.TrackYouTubeCandidate{},
	)
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

	// Migration: Add YouTube OAuth columns if they don't exist (MySQL only)
	// SQLite handles this through AutoMigrate; MySQL needs explicit column checks
	if dbType != "sqlite" {
		var columnCount int64
		db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'app_configs' AND column_name = 'youtube_access_token'").Scan(&columnCount)
		if columnCount == 0 {
			log.Println("Adding YouTube OAuth columns to app_configs table...")
			if err := db.Exec(`
				ALTER TABLE app_configs
				ADD COLUMN youtube_access_token TEXT DEFAULT NULL,
				ADD COLUMN youtube_refresh_token TEXT DEFAULT NULL,
				ADD COLUMN youtube_token_expiry DATETIME(3) DEFAULT NULL,
				ADD COLUMN youtube_connected TINYINT(1) DEFAULT 0
			`).Error; err != nil {
				log.Printf("Warning: Failed to add YouTube OAuth columns: %v", err)
			} else {
				log.Println("YouTube OAuth columns added successfully")
			}
		} else {
			// Ensure columns are large enough for encrypted tokens
			var existingSize int64
			db.Raw("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_name = 'app_configs' AND column_name = 'youtube_access_token'").Scan(&existingSize)
			if existingSize > 0 && existingSize < 1000 {
				log.Println("Expanding YouTube OAuth columns to support encrypted tokens...")
				db.Exec(`ALTER TABLE app_configs MODIFY COLUMN youtube_access_token TEXT DEFAULT NULL`)
				db.Exec(`ALTER TABLE app_configs MODIFY COLUMN youtube_refresh_token TEXT DEFAULT NULL`)
			}
		}
	}

	// Verify default row exists
	var config models.AppConfig
	if err := db.First(&config).Error; err == nil {
		if config.YouTubeConnected && (config.YouTubeAccessToken == "" || config.YouTubeRefreshToken == "") {
			log.Println("Warning: YouTubeConnected is true but tokens are missing - tokens may not have been saved properly")
		}
	}

	// Create TTL index for audit logs (automatically delete logs older than 90 days)
	if !migrator.HasIndex(&models.AuditLog{}, "idx_audit_logs_created_at") {
		if err := db.Exec(`
			CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at)
		`).Error; err != nil {
			log.Printf("Note: Could not create audit_logs index: %v", err)
		}
	}

	// Migration: Add youtube_video_id column to track_youtube_matches if missing
	var ytColumnCount int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='track_youtube_matches'").Scan(&ytColumnCount)
	if ytColumnCount > 0 {
		db.Raw("SELECT COUNT(*) FROM pragma_table_info('track_youtube_matches') WHERE name='youtube_video_id'").Scan(&ytColumnCount)
		if ytColumnCount == 0 {
			log.Println("Adding youtube_video_id column to track_youtube_matches table...")
			if err := db.Exec(`ALTER TABLE track_youtube_matches ADD COLUMN youtube_video_id VARCHAR(20) DEFAULT NULL`).Error; err != nil {
				log.Printf("Warning: Failed to add youtube_video_id column: %v", err)
			} else {
				log.Println("youtube_video_id column added successfully")
			}
		}
		// Fix any records that have NULL youtube_video_id - copy from you_tube_video_id if exists
		var nullCount int64
		db.Raw("SELECT COUNT(*) FROM track_youtube_matches WHERE (youtube_video_id IS NULL OR youtube_video_id = '') AND you_tube_video_id IS NOT NULL AND you_tube_video_id != ''").Scan(&nullCount)
		if nullCount > 0 {
			log.Printf("Found %d records with NULL youtube_video_id but with you_tube_video_id - copying values...", nullCount)
			db.Exec("UPDATE track_youtube_matches SET youtube_video_id = you_tube_video_id WHERE (youtube_video_id IS NULL OR youtube_video_id = '') AND you_tube_video_id IS NOT NULL AND you_tube_video_id != ''")
		}
		// Check if there are still null values
		db.Raw("SELECT COUNT(*) FROM track_youtube_matches WHERE youtube_video_id IS NULL OR youtube_video_id = ''").Scan(&nullCount)
		if nullCount > 0 {
			log.Printf("Found %d records with NULL youtube_video_id - these may need to be re-saved", nullCount)
		}
		// Drop the incorrectly named column you_tube_video_id
		var hasYouTubeVideoIDCol int
		db.Raw("SELECT COUNT(*) FROM pragma_table_info('track_youtube_matches') WHERE name='you_tube_video_id'").Scan(&hasYouTubeVideoIDCol)
		if hasYouTubeVideoIDCol > 0 {
			log.Println("Dropping incorrectly named you_tube_video_id column...")
			db.Exec("ALTER TABLE track_youtube_matches DROP COLUMN you_tube_video_id")
		}
		// Create index on youtube_video_id if not exists
		if !migrator.HasIndex(&models.TrackYouTubeMatch{}, "idx_youtube_video_id") {
			if err := db.Exec(`CREATE INDEX idx_youtube_video_id ON track_youtube_matches(youtube_video_id)`).Error; err != nil {
				log.Printf("Note: Could not create youtube_video_id index: %v", err)
			}
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

// initSQLite initializes a SQLite database connection
func initSQLite() (*gorm.DB, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/vinylfo.db"
	}

	// Ensure directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	log.Printf("Opening SQLite database at: %s", dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to SQLite database:", err)
		return nil, err
	}

	return db, nil
}

// initMySQL initializes a MySQL database connection
func initMySQL() (*gorm.DB, error) {
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

		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true&allowNativePasswords=true"
	}

	log.Println("Opening MySQL database connection...")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to MySQL database:", err)
		return nil, err
	}

	return db, nil
}

// ShutdownDB performs a clean shutdown of the database connection
func ShutdownDB() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("error getting database connection: %w", err)
	}

	// Checkpoint WAL before closing (SQLite only)
	if DBType == "sqlite" {
		log.Println("Checkpointing SQLite WAL...")
		sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("error closing database connection: %w", err)
	}

	log.Println("Database connection closed")
	return nil
}
