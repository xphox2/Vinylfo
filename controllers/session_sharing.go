package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"vinylfo/models"
)

// SessionSharingController handles session sharing operations
type SessionSharingController struct {
	db *gorm.DB
}

// NewSessionSharingController creates a new session sharing controller
func NewSessionSharingController(db *gorm.DB) *SessionSharingController {
	return &SessionSharingController{db: db}
}

// CreateSessionSharing creates a new session sharing entry
func (c *SessionSharingController) CreateSessionSharing(ctx *gin.Context) {
	var sessionSharing models.SessionSharing
	if err := ctx.ShouldBindJSON(&sessionSharing); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&sessionSharing)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create session sharing"})
		return
	}

	ctx.JSON(201, sessionSharing)
}

// GetSessionSharing retrieves a session sharing entry by ID
func (c *SessionSharingController) GetSessionSharing(ctx *gin.Context) {
	id := ctx.Param("session_id")
	var sessionSharing models.SessionSharing
	result := c.db.First(&sessionSharing, "session_id = ?", id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session sharing not found"})
		return
	}
	ctx.JSON(200, sessionSharing)
}

// UpdateSessionSharing updates a session sharing entry
func (c *SessionSharingController) UpdateSessionSharing(ctx *gin.Context) {
	id := ctx.Param("session_id")
	var sessionSharing models.SessionSharing
	result := c.db.First(&sessionSharing, "session_id = ?", id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session sharing not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&sessionSharing); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&sessionSharing)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update session sharing"})
		return
	}
	ctx.JSON(200, sessionSharing)
}

// DeleteSessionSharing deletes a session sharing entry
func (c *SessionSharingController) DeleteSessionSharing(ctx *gin.Context) {
	id := ctx.Param("session_id")
	var sessionSharing models.SessionSharing
	result := c.db.First(&sessionSharing, "session_id = ?", id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session sharing not found"})
		return
	}

	result = c.db.Delete(&sessionSharing)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete session sharing"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Session sharing deleted successfully"})
}

// GetPublicSessionSharing retrieves public session sharing information
func (c *SessionSharingController) GetPublicSessionSharing(ctx *gin.Context) {
	token := ctx.Param("token")
	var sessionSharing models.SessionSharing
	result := c.db.First(&sessionSharing, "sharing_token = ?", token)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session sharing not found"})
		return
	}
	ctx.JSON(200, sessionSharing)
}
