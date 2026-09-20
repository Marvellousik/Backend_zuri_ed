package models

import (
	"time"

	"github.com/google/uuid"
)

// Connection represents a WebSocket connection
type Connection struct {
	ID             uuid.UUID `db:"id" json:"id"`
	UserID         uuid.UUID `db:"user_id" json:"user_id"`
	ConnectionID   string    `db:"connection_id" json:"connection_id"`
	DeviceID       *string   `db:"device_id" json:"device_id,omitempty"`
	DeviceType     *string   `db:"device_type" json:"device_type,omitempty"`
	IsActive       bool      `db:"is_active" json:"is_active"`
	ConnectedAt    time.Time `db:"connected_at" json:"connected_at"`
	LastPingAt     time.Time `db:"last_ping_at" json:"last_ping_at"`
	DisconnectedAt *time.Time `db:"disconnected_at" json:"disconnected_at,omitempty"`
	IPAddress      *string   `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent      *string   `db:"user_agent" json:"user_agent,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// SyncEvent represents a sync event
type SyncEvent struct {
	ID               uuid.UUID              `db:"id" json:"id"`
	EventType        string                 `db:"event_type" json:"event_type"`
	EventName        string                 `db:"event_name" json:"event_name"`
	UserID           *uuid.UUID             `db:"user_id" json:"user_id,omitempty"`
	CourseID         *uuid.UUID             `db:"course_id" json:"course_id,omitempty"`
	Payload          map[string]interface{} `db:"payload" json:"payload"`
	SourceService    string                 `db:"source_service" json:"source_service"`
	SourceConnectionID *string              `db:"source_connection_id" json:"source_connection_id,omitempty"`
	IsBroadcast      bool                   `db:"is_broadcast" json:"is_broadcast"`
	ProcessedAt      *time.Time             `db:"processed_at" json:"processed_at,omitempty"`
	CreatedAt        time.Time              `db:"created_at" json:"created_at"`
}

// DeviceState represents sync state for a device
type DeviceState struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	DeviceID          string     `db:"device_id" json:"device_id"`
	LastEventID       *uuid.UUID `db:"last_event_id" json:"last_event_id,omitempty"`
	LastEventTimestamp *time.Time `db:"last_event_timestamp" json:"last_event_timestamp,omitempty"`
	SyncCursor        *string    `db:"sync_cursor" json:"sync_cursor,omitempty"`
	IsSyncing         bool       `db:"is_syncing" json:"is_syncing"`
	LastSyncAt        *time.Time `db:"last_sync_at" json:"last_sync_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// ChangeLog represents a database change for CDC
type ChangeLog struct {
	ID            uuid.UUID              `db:"id" json:"id"`
	TableName     string                 `db:"table_name" json:"table_name"`
	Operation     string                 `db:"operation" json:"operation"`
	RecordID      uuid.UUID              `db:"record_id" json:"record_id"`
	UserID        *uuid.UUID             `db:"user_id" json:"user_id,omitempty"`
	OldData       map[string]interface{} `db:"old_data" json:"old_data,omitempty"`
	NewData       map[string]interface{} `db:"new_data" json:"new_data,omitempty"`
	ChangedAt     time.Time              `db:"changed_at" json:"changed_at"`
	Processed     bool                   `db:"processed" json:"processed"`
	ProcessedAt   *time.Time             `db:"processed_at" json:"processed_at,omitempty"`
	BroadcastTo   []string               `db:"broadcast_to" json:"broadcast_to,omitempty"`
}

// Presence represents user presence status
type Presence struct {
	ID                 uuid.UUID              `db:"id" json:"id"`
	UserID             uuid.UUID              `db:"user_id" json:"user_id"`
	Status             string                 `db:"status" json:"status"`
	StatusMessage      *string                `db:"status_message" json:"status_message,omitempty"`
	LastSeenAt         time.Time              `db:"last_seen_at" json:"last_seen_at"`
	LastActivityType   *string                `db:"last_activity_type" json:"last_activity_type,omitempty"`
	LastActivityData   map[string]interface{} `db:"last_activity_data" json:"last_activity_data,omitempty"`
	ActiveConnections  int                    `db:"active_connections" json:"active_connections"`
	CreatedAt          time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time              `db:"updated_at" json:"updated_at"`
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// ConnectionRequest represents a connection request
type ConnectionRequest struct {
	DeviceID   string `json:"device_id,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
}

// SyncCursorRequest represents a sync request with cursor
type SyncCursorRequest struct {
	Cursor    string     `json:"cursor,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	EntityTypes []string `json:"entity_types,omitempty"`
}

// PresenceUpdateRequest represents a presence update
type PresenceUpdateRequest struct {
	Status           string                 `json:"status" binding:"required"` // online, away, busy, offline
	StatusMessage    string                 `json:"status_message,omitempty"`
	ActivityType     string                 `json:"activity_type,omitempty"`
	ActivityData     map[string]interface{} `json:"activity_data,omitempty"`
}

// Constants for event types
const (
	EventMaterialCreated = "material.created"
	EventMaterialUpdated = "material.updated"
	EventMaterialDeleted = "material.deleted"
	EventQuizCompleted   = "quiz.completed"
	EventCourseUpdated   = "course.updated"
	EventProgressUpdated = "progress.updated"
	EventStreakUpdated   = "streak.updated"
	EventGoalUpdated     = "goal.updated"
)

// Constants for WebSocket message types
const (
	MessageTypeConnect     = "connect"
	MessageTypeDisconnect  = "disconnect"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
	MessageTypeSync        = "sync"
	MessageTypeSyncAck     = "sync_ack"
	MessageTypeEvent       = "event"
	MessageTypePresence    = "presence"
	MessageTypeError       = "error"
)

// Constants for presence status
const (
	PresenceOnline  = "online"
	PresenceAway    = "away"
	PresenceBusy    = "busy"
	PresenceOffline = "offline"
)

// Constants for database operations
const (
	OperationInsert = "INSERT"
	OperationUpdate = "UPDATE"
	OperationDelete = "DELETE"
)
