package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")
		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	type Track struct {
		ID          uint
		Title       string
		Position    string
		TrackNumber int
		DiscNumber  int
	}

	var tracks []Track
	result := db.Order("id DESC").Limit(20).Find(&tracks)

	fmt.Printf("Found %d tracks\n", result.RowsAffected)
	fmt.Println("\n=== LAST 20 TRACKS ===")
	for _, track := range tracks {
		fmt.Printf("ID=%d Title='%s' Position='%s' TrackNumber=%d DiscNumber=%d\n",
			track.ID, track.Title, track.Position, track.TrackNumber, track.DiscNumber)
	}
}
