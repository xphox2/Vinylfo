package controllers

import (
	"vinylfo/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SessionNoteController handles session note operations
type SessionNoteController struct {
	db *gorm.DB
}

// NewSessionNoteController creates a new session note controller
func NewSessionNoteController(db *gorm.DB) *SessionNoteController {
	return &SessionNoteController{db: db}
}

// CreateSessionNote creates a new session note
func (c *SessionNoteController) CreateSessionNote(ctx *gin.Context) {
	var sessionNote models.SessionNote
	if err := ctx.ShouldBindJSON(&sessionNote); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := c.db.Create(&sessionNote)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to create session note"})
		return
	}

	ctx.JSON(201, sessionNote)
}

// GetSessionNotes retrieves all notes for a session
func (c *SessionNoteController) GetSessionNotes(ctx *gin.Context) {
	sessionID := ctx.Param("session_id")
	var sessionNotes []models.SessionNote
	result := c.db.Where("session_id = ?", sessionID).Find(&sessionNotes)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to fetch session notes"})
		return
	}
	ctx.JSON(200, sessionNotes)
}

// GetSessionNote retrieves a specific session note by ID
func (c *SessionNoteController) GetSessionNote(ctx *gin.Context) {
	id := ctx.Param("id")
	var sessionNote models.SessionNote
	result := c.db.First(&sessionNote, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session note not found"})
		return
	}
	ctx.JSON(200, sessionNote)
}

// UpdateSessionNote updates a session note
func (c *SessionNoteController) UpdateSessionNote(ctx *gin.Context) {
	id := ctx.Param("id")
	var sessionNote models.SessionNote
	result := c.db.First(&sessionNote, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session note not found"})
		return
	}

	if err := ctx.ShouldBindJSON(&sessionNote); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result = c.db.Save(&sessionNote)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to update session note"})
		return
	}
	ctx.JSON(200, sessionNote)
}

// DeleteSessionNote deletes a session note
func (c *SessionNoteController) DeleteSessionNote(ctx *gin.Context) {
	id := ctx.Param("id")
	var sessionNote models.SessionNote
	result := c.db.First(&sessionNote, id)
	if result.Error != nil {
		ctx.JSON(404, gin.H{"error": "Session note not found"})
		return
	}

	result = c.db.Delete(&sessionNote)
	if result.Error != nil {
		ctx.JSON(500, gin.H{"error": "Failed to delete session note"})
		return
	}
	ctx.JSON(200, gin.H{"message": "Session note deleted successfully"})
}
