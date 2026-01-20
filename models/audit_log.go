package models

import (
	"time"
)

type AuditLog struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType   string    `gorm:"size:100;index" json:"event_type"`
	EventAction string    `gorm:"size:100;index" json:"event_action"`
	UserID      uint      `gorm:"index" json:"user_id"`
	IPAddress   string    `gorm:"size:45" json:"ip_address"`
	UserAgent   string    `gorm:"size:500" json:"user_agent"`
	Resource    string    `gorm:"size:255" json:"resource"`
	Details     string    `gorm:"type:text" json:"details"`
	Status      string    `gorm:"size:50" json:"status"`
	ErrorMsg    string    `gorm:"type:text" json:"error_msg"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

type AuditEventType string

const (
	AuditEventOAuth    AuditEventType = "oauth"
	AuditEventAuth     AuditEventType = "auth"
	AuditEventAPI      AuditEventType = "api"
	AuditEventSync     AuditEventType = "sync"
	AuditEventPlayback AuditEventType = "playback"
	AuditEventPlaylist AuditEventType = "playlist"
	AuditEventSettings AuditEventType = "settings"
	AuditEventSecurity AuditEventType = "security"
)

type AuditEventAction string

const (
	AuditActionLogin        AuditEventAction = "login"
	AuditActionLogout       AuditEventAction = "logout"
	AuditActionTokenRefresh AuditEventAction = "token_refresh"
	AuditActionTokenRevoke  AuditEventAction = "token_revoke"
	AuditActionConnect      AuditEventAction = "connect"
	AuditActionDisconnect   AuditEventAction = "disconnect"
	AuditActionCreate       AuditEventAction = "create"
	AuditActionUpdate       AuditEventAction = "update"
	AuditActionDelete       AuditEventAction = "delete"
	AuditActionExport       AuditEventAction = "export"
	AuditActionImport       AuditEventAction = "import"
	AuditActionError        AuditEventAction = "error"
	AuditActionWarning      AuditEventAction = "warning"
)
