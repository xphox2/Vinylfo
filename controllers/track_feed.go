package controllers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type TrackFeedController struct{}

type TrackFeedParams struct {
	Theme        string
	Speed        int
	Separator    string
	ShowDuration bool
	ShowAlbum    bool
	ShowArtist   bool
	Direction    string
	Prefix       string
}

func NewTrackFeedController() *TrackFeedController {
	return &TrackFeedController{}
}

func (c *TrackFeedController) GetTrackFeedPage(ctx *gin.Context) {
	theme := ctx.DefaultQuery("theme", "dark")
	if theme != "dark" && theme != "light" && theme != "transparent" {
		theme = "dark"
	}

	speedStr := ctx.DefaultQuery("speed", "5")
	speed, _ := strconv.Atoi(speedStr)
	if speed < 1 {
		speed = 1
	} else if speed > 10 {
		speed = 10
	}

	separator := ctx.DefaultQuery("separator", "*")

	showDurStr := ctx.DefaultQuery("showDuration", "true")
	showDuration := showDurStr == "true"

	showAlbumStr := ctx.DefaultQuery("showAlbum", "true")
	showAlbum := showAlbumStr == "true"

	showArtistStr := ctx.DefaultQuery("showArtist", "true")
	showArtist := showArtistStr == "true"

	direction := ctx.DefaultQuery("direction", "rtl")
	if direction != "rtl" && direction != "ltr" {
		direction = "rtl"
	}

	prefix := ctx.DefaultQuery("prefix", "Now Playing:")
	prefix = strings.TrimSpace(prefix)

	ctx.Header("Cache-Control", "no-store")
	ctx.HTML(200, "track-feed.html", gin.H{
		"theme":        theme,
		"speed":        speed,
		"separator":    separator,
		"showDuration": showDuration,
		"showAlbum":    showAlbum,
		"showArtist":   showArtist,
		"direction":    direction,
		"prefix":       prefix,
	})
}
