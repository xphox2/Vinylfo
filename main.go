package main

import (
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

	err = database.SeedDatabase(db)
	if err != nil {
		log.Printf("Warning: Failed to seed database: %v", err)
	}

	validationResult := discogs.ValidateOAuthConfig()
	if !validationResult.IsValid {
		log.Println("Warning: OAuth configuration has errors. OAuth functionality may not work correctly.")
	}
	discogs.PrintOAuthConfigSummary()

	playbackController = controllers.NewPlaybackController(db)

	go playbackController.SimulateTimer()

	r := gin.Default()

	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	r.GET("/dashboard", func(c *gin.Context) {
		c.File("./static/playback-dashboard.html")
	})

	r.GET("/playlist", func(c *gin.Context) {
		c.File("./static/playlist.html")
	})

	r.GET("/track/:id", func(c *gin.Context) {
		c.File("./static/track-detail.html")
	})

	r.GET("/settings", func(c *gin.Context) {
		c.File("./static/settings.html")
	})

	r.GET("/sync", func(c *gin.Context) {
		c.File("./static/sync.html")
	})

	r.GET("/search", func(c *gin.Context) {
		c.File("./static/search.html")
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
