package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type AlbumArtFeedController struct{}

type AlbumArtFeedParams struct {
	Theme        string
	Animation    bool
	AnimDuration int
	Fit          string
}

func NewAlbumArtFeedController() *AlbumArtFeedController {
	return &AlbumArtFeedController{}
}

func (c *AlbumArtFeedController) GetAlbumArtFeedPage(ctx *gin.Context) {
	theme := ctx.DefaultQuery("theme", "dark")
	if theme != "dark" && theme != "light" && theme != "transparent" {
		theme = "dark"
	}

	animStr := ctx.DefaultQuery("animation", "true")
	animation := animStr == "true"

	animDurStr := ctx.DefaultQuery("animDuration", "20")
	animDur, _ := strconv.Atoi(animDurStr)
	if animDur < 5 {
		animDur = 5
	} else if animDur > 120 {
		animDur = 120
	}

	fit := ctx.DefaultQuery("fit", "cover")
	if fit != "contain" && fit != "cover" {
		fit = "cover"
	}

	ctx.Header("Cache-Control", "no-store")
	ctx.HTML(200, "album-art-feed.html", gin.H{
		"theme":        theme,
		"animation":    animation,
		"animDuration": animDur,
		"fit":          fit,
	})
}
