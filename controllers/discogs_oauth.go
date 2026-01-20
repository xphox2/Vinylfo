package controllers

import (
	"os"

	"vinylfo/discogs"
	"vinylfo/models"

	"github.com/gin-gonic/gin"
)

func (c *DiscogsController) GetOAuthURL(ctx *gin.Context) {
	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		config = models.AppConfig{ID: 1}
		c.db.FirstOrCreate(&config, models.AppConfig{ID: 1})
	}

	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")

	if consumerKey == "" || consumerSecret == "" {
		ctx.JSON(500, gin.H{"error": "DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set in .env file"})
		return
	}

	oauth := &discogs.OAuthConfig{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
	}
	client := discogs.NewClientWithOAuth("", oauth)

	token, secret, authURL, err := client.GetRequestToken()
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to get request token"})
		return
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":  token,
		"discogs_access_secret": secret,
	})

	ctx.JSON(200, gin.H{
		"auth_url": authURL,
		"token":    token,
	})
}

func (c *DiscogsController) OAuthCallback(ctx *gin.Context) {
	if ctx.Query("oauth_token") == "" || ctx.Query("oauth_verifier") == "" {
		ctx.String(400, "Missing oauth_token or oauth_verifier")
		return
	}

	var config models.AppConfig
	err := c.db.First(&config).Error
	if err != nil {
		ctx.String(500, "Failed to load config: %v", err)
		return
	}

	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")

	oauth := &discogs.OAuthConfig{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    config.DiscogsAccessToken,
		AccessSecret:   config.DiscogsAccessSecret,
	}
	client := discogs.NewClientWithOAuth("", oauth)

	accessToken, accessSecret, username, err := client.GetAccessToken(config.DiscogsAccessToken, config.DiscogsAccessSecret, ctx.Query("oauth_verifier"))
	if err != nil {
		ctx.String(500, "Failed to get access token: %v", err)
		return
	}

	if username == "" {
		username, err = client.GetUserIdentity()
		if err != nil {
			ctx.String(500, "Failed to get user identity: %v", err)
			return
		}
	}

	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":  accessToken,
		"discogs_access_secret": accessSecret,
		"discogs_username":      username,
		"is_discogs_connected":  true,
	})

	ctx.Redirect(302, "/settings?discogs_connected=true")
}

func (c *DiscogsController) Disconnect(ctx *gin.Context) {
	c.db.Model(&models.AppConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"discogs_access_token":  "",
		"discogs_access_secret": "",
		"discogs_username":      "",
		"is_discogs_connected":  false,
	})

	ctx.JSON(200, gin.H{
		"message": "Disconnected from Discogs",
		"note":    "The authorization has been removed from this application. To fully disconnect, please also revoke access at: https://www.discogs.com/settings/applications",
	})
}
