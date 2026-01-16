package main

import (
	"html/template"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"vinylfo/controllers"
	"vinylfo/database"
	"vinylfo/discogs"
	"vinylfo/routes"
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

	validationResult := discogs.ValidateOAuthConfig()
	if !validationResult.IsValid {
		log.Println("Warning: OAuth configuration has errors. OAuth functionality may not work correctly.")
	}
	discogs.PrintOAuthConfigSummary()

	playbackController = controllers.NewPlaybackController(db)

	go playbackController.SimulateTimer()

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
	))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index-page", nil)
	})

	r.GET("/dashboard", func(c *gin.Context) {
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

	routes.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
