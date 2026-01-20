package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"vinylfo/config"
	"vinylfo/controllers"
	"vinylfo/database"
	"vinylfo/discogs"
	"vinylfo/routes"
	"vinylfo/utils"
)

var db *gorm.DB
var playbackController *controllers.PlaybackController

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found. Using default configuration.")
	}

	db, err = database.InitDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	db.Logger.LogMode(logger.Info)

	validationResult := discogs.ValidateOAuthConfig()
	if !validationResult.IsValid {
		log.Println("Warning: OAuth configuration has errors. OAuth functionality may not work correctly.")
	}
	discogs.PrintOAuthConfigSummary()

	playbackController = controllers.NewPlaybackController(db)

	utils.InitPKCE(db)
	utils.InitAuditLog(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go playbackController.SimulateTimer(ctx)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	r.Static("/static", "./static")

	tmpl := template.Must(template.ParseFiles(
		"templates/header.html",
		"templates/index.html",
		"templates/search.html",
		"templates/sync.html",
		"templates/settings.html",
		"templates/playback-dashboard.html",
		"templates/playlist.html",
		"templates/track-detail.html",
		"templates/album-detail.html",
		"templates/resolution-center.html",
		"templates/youtube.html",
	))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index-page", nil)
	})

	r.GET("/player", func(c *gin.Context) {
		c.HTML(200, "playback-dashboard-page", nil)
	})

	r.GET("/playlist", func(c *gin.Context) {
		c.HTML(200, "playlist-page", nil)
	})

	r.GET("/track/:id", func(c *gin.Context) {
		c.HTML(200, "track-detail-page", nil)
	})

	r.GET("/album/:id", func(c *gin.Context) {
		c.HTML(200, "album-detail-page", nil)
	})

	r.GET("/settings", func(c *gin.Context) {
		c.HTML(200, "settings-page", nil)
	})

	r.GET("/sync", func(c *gin.Context) {
		c.HTML(200, "sync-page", nil)
	})

	r.GET("/search", func(c *gin.Context) {
		c.HTML(200, "search-page", nil)
	})

	r.GET("/resolution-center", func(c *gin.Context) {
		c.HTML(200, "resolution-center-page", nil)
	})

	r.GET("/youtube", func(c *gin.Context) {
		c.HTML(200, "youtube-page", nil)
	})

	routes.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("Error getting database connection: %v", err)
	} else {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}

	log.Println("Server exited")
}
