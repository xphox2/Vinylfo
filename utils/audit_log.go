package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"vinylfo/models"

	"gorm.io/gorm"
)

var auditDB *gorm.DB

func InitAuditLog(dbInstance *gorm.DB) {
	auditDB = dbInstance
}

type AuditLogEntry struct {
	EventType   models.AuditEventType   `json:"event_type"`
	EventAction models.AuditEventAction `json:"event_action"`
	UserID      uint                    `json:"user_id"`
	IPAddress   string                  `json:"ip_address"`
	UserAgent   string                  `json:"user_agent"`
	Resource    string                  `json:"resource"`
	Details     map[string]interface{}  `json:"details"`
	Status      string                  `json:"status"`
	ErrorMsg    string                  `json:"error_msg"`
}

func LogAuditEvent(entry AuditLogEntry) error {
	if auditDB == nil {
		return fmt.Errorf("audit log database not initialized")
	}

	detailsJSON, _ := json.Marshal(entry.Details)

	log := &models.AuditLog{
		EventType:   string(entry.EventType),
		EventAction: string(entry.EventAction),
		UserID:      entry.UserID,
		IPAddress:   entry.IPAddress,
		UserAgent:   entry.UserAgent,
		Resource:    entry.Resource,
		Details:     string(detailsJSON),
		Status:      entry.Status,
		ErrorMsg:    entry.ErrorMsg,
		CreatedAt:   time.Now(),
	}

	return auditDB.Create(log).Error
}

func LogOAuthEvent(action models.AuditEventAction, userID uint, ipAddress, userAgent string, success bool, details map[string]interface{}) error {
	status := "success"
	if !success {
		status = "error"
	}

	return LogAuditEvent(AuditLogEntry{
		EventType:   models.AuditEventOAuth,
		EventAction: action,
		UserID:      userID,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Resource:    "youtube_oauth",
		Details:     details,
		Status:      status,
	})
}

func LogAuthEvent(action models.AuditEventAction, ipAddress, userAgent string, success bool, errorMsg string) error {
	status := "success"
	if !success {
		status = "error"
	}

	return LogAuditEvent(AuditLogEntry{
		EventType:   models.AuditEventAuth,
		EventAction: action,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Resource:    "authentication",
		Status:      status,
		ErrorMsg:    errorMsg,
	})
}

func LogAPIEvent(action models.AuditEventAction, userID uint, ipAddress, userAgent, resource string, success bool, details map[string]interface{}) error {
	status := "success"
	if !success {
		status = "error"
	}

	return LogAuditEvent(AuditLogEntry{
		EventType:   models.AuditEventAPI,
		EventAction: action,
		UserID:      userID,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Resource:    resource,
		Details:     details,
		Status:      status,
	})
}

func LogSecurityEvent(action models.AuditEventAction, ipAddress, userAgent, resource, errorMsg string) error {
	return LogAuditEvent(AuditLogEntry{
		EventType:   models.AuditEventSecurity,
		EventAction: action,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Resource:    resource,
		Status:      "warning",
		ErrorMsg:    errorMsg,
	})
}

func GetAuditLogs(eventType string, limit, offset int) ([]models.AuditLog, int64, error) {
	if auditDB == nil {
		return nil, 0, fmt.Errorf("audit log database not initialized")
	}

	var logs []models.AuditLog
	var total int64

	query := auditDB.Model(&models.AuditLog{})
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	query.Count(&total)
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error

	return logs, total, err
}

func CleanupOldAuditLogs(daysRetained int) (int64, error) {
	if auditDB == nil {
		return 0, fmt.Errorf("audit log database not initialized")
	}

	cutoff := time.Now().AddDate(0, 0, -daysRetained)
	result := auditDB.Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}
