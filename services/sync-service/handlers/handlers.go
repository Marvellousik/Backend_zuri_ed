package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"zuri/services/sync-service/models"
	"zuri/services/sync-service/websocket"

	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/middleware"
)

// Handler handles HTTP requests
type Handler struct {
	db  *sqlx.DB
	hub *websocket.Hub
}

// NewHandler creates a new handler
func NewHandler(db *sqlx.DB, hub *websocket.Hub) *Handler {
	return &Handler{
		db:  db,
		hub: hub,
	}
}

// RegisterRoutes registers all routes
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Root health check (used by gateway)
	r.GET("/health", h.HealthCheck)

	api := r.Group("/api/v1")
	{
		// API-level health check
		api.GET("/health", h.HealthCheck)

		// WebSocket endpoint (requires auth)
		ws := api.Group("")
		ws.Use(middleware.AuthRequired())
		{
			ws.GET("/sync", h.hub.HandleWebSocket)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// Presence
			protected.GET("/presence", h.GetPresence)
			protected.PUT("/presence", h.UpdatePresence)
			protected.GET("/presence/online", h.GetOnlineUsers)

			// Sync state
			protected.GET("/sync/state", h.GetSyncState)
			protected.POST("/sync/ack", h.AckSync)

			// Events (supporting both /events and /sync/events for gateway proxy compatibility)
			protected.GET("/events", h.GetEvents)
			protected.POST("/events", h.CreateEvent)
			protected.GET("/sync/events", h.GetEvents)
			protected.POST("/sync/events", h.CreateEvent)
		}

		// Internal routes (for other services)
		internal := api.Group("/internal")
		{
			internal.POST("/broadcast", h.BroadcastEvent)
			internal.POST("/changes", h.RecordChange)
		}
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "sync-service",
		"time":    time.Now().UTC(),
	})
}

// GetPresence gets the current user's presence
func (h *Handler) GetPresence(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var presence models.Presence
	err = h.db.Get(&presence, `
		SELECT * FROM zuri_sync.presence 
		WHERE user_id = $1`, uid)

	if err != nil {
		// Return default presence
		presence = models.Presence{
			UserID:            uid,
			Status:            models.PresenceOffline,
			ActiveConnections: 0,
			LastSeenAt:        time.Now(),
		}
	}

	c.JSON(http.StatusOK, presence)
}

// UpdatePresence updates the current user's presence
func (h *Handler) UpdatePresence(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req models.PresenceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current connection count
	var connCount int
	h.db.Get(&connCount, `
		SELECT COALESCE(active_connections, 0) FROM zuri_sync.presence 
		WHERE user_id = $1`, uid)

	// Build query
	query := `
		INSERT INTO zuri_sync.presence 
		(user_id, status, status_message, last_activity_type, last_activity_data, active_connections, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) 
		DO UPDATE SET status = $2, status_message = $3, last_activity_type = $4, 
		              last_activity_data = $5, last_seen_at = CURRENT_TIMESTAMP`

	_, err = h.db.Exec(query, uid, req.Status, req.StatusMessage, 
		req.ActivityType, req.ActivityData, connCount)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update presence: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update presence"})
		return
	}

	// Broadcast presence update to other devices
	h.hub.BroadcastToUser(uid, "presence.updated", map[string]interface{}{
		"user_id":         uid,
		"status":          req.Status,
		"status_message":  req.StatusMessage,
		"activity_type":   req.ActivityType,
		"activity_data":   req.ActivityData,
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Presence updated"})
}

// GetOnlineUsers gets list of online users
func (h *Handler) GetOnlineUsers(c *gin.Context) {
	var users []struct {
		UserID           uuid.UUID `db:"user_id" json:"user_id"`
		Status           string    `db:"status" json:"status"`
		StatusMessage    *string   `db:"status_message" json:"status_message,omitempty"`
		LastSeenAt       time.Time `db:"last_seen_at" json:"last_seen_at"`
		LastActivityType *string   `db:"last_activity_type" json:"last_activity_type,omitempty"`
		ConnectionCount  int       `db:"connection_count" json:"connection_count"`
	}

	err := h.db.Select(&users, `
		SELECT * FROM zuri_sync.online_users`)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get online users: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get online users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

// GetSyncState gets the sync state for the current device
func (h *Handler) GetSyncState(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		// Return empty state when no device_id provided
		c.JSON(http.StatusOK, gin.H{
			"user_id":    uid,
			"is_syncing": false,
			"message":    "Provide device_id query param for device-specific state",
		})
		return
	}

	var state models.DeviceState
	err = h.db.Get(&state, `
		SELECT * FROM zuri_sync.device_state 
		WHERE user_id = $1 AND device_id = $2`, uid, deviceID)

	if err != nil {
		// Return empty state
		state = models.DeviceState{
			UserID:   uid,
			DeviceID: deviceID,
			IsSyncing: false,
		}
	}

	c.JSON(http.StatusOK, state)
}

// AckSync acknowledges sync completion
func (h *Handler) AckSync(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		DeviceID   string    `json:"device_id" binding:"required"`
		LastEventID uuid.UUID `json:"last_event_id" binding:"required"`
		SyncCursor string    `json:"sync_cursor" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	_, err = h.db.Exec(`
		INSERT INTO zuri_sync.device_state 
		(user_id, device_id, last_event_id, last_event_timestamp, sync_cursor, is_syncing, last_sync_at)
		VALUES ($1, $2, $3, $4, $5, false, $6)
		ON CONFLICT (user_id, device_id) 
		DO UPDATE SET last_event_id = $3, last_event_timestamp = $4, 
		              sync_cursor = $5, is_syncing = false, last_sync_at = $6`,
		uid, req.DeviceID, req.LastEventID, now, req.SyncCursor, now)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to ack sync: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to acknowledge sync"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sync acknowledged"})
}

// GetEvents gets events for the current user
func (h *Handler) GetEvents(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse query params
	since := c.Query("since")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var events []models.SyncEvent
	var dbErr error

	if since != "" {
		sinceTime, parseErr := time.Parse(time.RFC3339, since)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid since format"})
			return
		}
		dbErr = h.db.Select(&events, `
			SELECT * FROM zuri_sync.events 
			WHERE (user_id = $1 OR user_id IS NULL)
			AND created_at > $2
			ORDER BY created_at ASC
			LIMIT $3`, uid, sinceTime, limit)
	} else {
		dbErr = h.db.Select(&events, `
			SELECT * FROM zuri_sync.events 
			WHERE (user_id = $1 OR user_id IS NULL)
			ORDER BY created_at DESC
			LIMIT $2`, uid, limit)
	}

	if dbErr != nil {
		logger.Error(fmt.Sprintf("Failed to get events: %v", dbErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

// CreateEvent creates a new sync event
func (h *Handler) CreateEvent(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		EventType   string                 `json:"event_type" binding:"required"`
		EventName   string                 `json:"event_name" binding:"required"`
		CourseID    *uuid.UUID             `json:"course_id,omitempty"`
		Payload     map[string]interface{} `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := models.SyncEvent{
		EventType:     req.EventType,
		EventName:     req.EventName,
		UserID:        &uid,
		CourseID:      req.CourseID,
		Payload:       req.Payload,
		SourceService: "sync-service",
	}

	_, err = h.db.NamedExec(`
		INSERT INTO zuri_sync.events 
		(event_type, event_name, user_id, course_id, payload, source_service)
		VALUES (:event_type, :event_name, :user_id, :course_id, :payload, :source_service)`,
		event)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create event: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	// Broadcast to user's other devices
	h.hub.BroadcastToUser(uid, req.EventType, req.Payload, "")

	c.JSON(http.StatusCreated, gin.H{"message": "Event created"})
}

// BroadcastEvent broadcasts an event to users (internal API)
func (h *Handler) BroadcastEvent(c *gin.Context) {
	var req struct {
		UserID    *uuid.UUID             `json:"user_id,omitempty"`
		EventType string                 `json:"event_type" binding:"required"`
		Payload   map[string]interface{} `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store event in database
	event := models.SyncEvent{
		EventType:     req.EventType,
		EventName:     req.EventType,
		UserID:        req.UserID,
		Payload:       req.Payload,
		SourceService: c.GetHeader("X-Source-Service"),
		IsBroadcast:   true,
	}

	_, err := h.db.NamedExec(`
		INSERT INTO zuri_sync.events 
		(event_type, event_name, user_id, payload, source_service, is_broadcast)
		VALUES (:event_type, :event_name, :user_id, :payload, :source_service, :is_broadcast)`,
		event)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to store broadcast event: %v", err))
		// Continue to broadcast anyway
	}

	// Broadcast via WebSocket
	if req.UserID != nil {
		h.hub.BroadcastToUser(*req.UserID, req.EventType, req.Payload, "")
	} else {
		h.hub.BroadcastToAll(req.EventType, req.Payload, "")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event broadcasted"})
}

// RecordChange records a database change for CDC (internal API)
func (h *Handler) RecordChange(c *gin.Context) {
	var req struct {
		TableName  string                 `json:"table_name" binding:"required"`
		Operation  string                 `json:"operation" binding:"required"`
		RecordID   uuid.UUID              `json:"record_id" binding:"required"`
		UserID     *uuid.UUID             `json:"user_id,omitempty"`
		OldData    map[string]interface{} `json:"old_data,omitempty"`
		NewData    map[string]interface{} `json:"new_data,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	change := models.ChangeLog{
		TableName: req.TableName,
		Operation: req.Operation,
		RecordID:  req.RecordID,
		UserID:    req.UserID,
		OldData:   req.OldData,
		NewData:   req.NewData,
	}

	_, err := h.db.NamedExec(`
		INSERT INTO zuri_sync.change_log 
		(table_name, operation, record_id, user_id, old_data, new_data)
		VALUES (:table_name, :operation, :record_id, :user_id, :old_data, :new_data)`,
		change)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to record change: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record change"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Change recorded"})
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}


