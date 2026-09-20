package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
	"zuri/shared/pkg/logger"

	"zuri/services/sync-service/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Client represents a WebSocket client
type Client struct {
	ID           string
	UserID       uuid.UUID
	Conn         *websocket.Conn
	Hub          *Hub
	Send         chan []byte
	DeviceID     string
	DeviceType   string
	ConnectedAt  time.Time
	LastPingAt   time.Time
}

// Hub manages WebSocket connections
type Hub struct {
	clients    map[string]*Client // connection_id -> client
	userIndex  map[uuid.UUID]map[string]bool // user_id -> set of connection_ids
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	db         *sqlx.DB
	mu         sync.RWMutex
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	UserID      *uuid.UUID
	EventType   string
	Payload     interface{}
	ExcludeConn string
}

// NewHub creates a new Hub
func NewHub(db *sqlx.DB) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		userIndex:  make(map[uuid.UUID]map[string]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
		db:         db,
	}
}

// Run starts the Hub
func (h *Hub) Run() {
	logger.Info("WebSocket Hub started")
	
	// Start cleanup routine
	go h.cleanupRoutine()
	
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

// handleRegister registers a new client
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.ID] = client

	// Add to user index
	if h.userIndex[client.UserID] == nil {
		h.userIndex[client.UserID] = make(map[string]bool)
	}
	h.userIndex[client.UserID][client.ID] = true

	logger.Info(fmt.Sprintf("Client registered: %s (user: %s, device: %s)", 
		client.ID, client.UserID, client.DeviceType))

	// Store connection in database
	h.storeConnection(client)

	// Update presence
	h.updatePresence(client.UserID, models.PresenceOnline, 1)

	// Send welcome message
	welcome := models.WebSocketMessage{
		Type:      models.MessageTypeConnect,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"connection_id": client.ID,
			"user_id":       client.UserID,
			"message":       "Connected to sync service",
		},
	}
	if data, err := json.Marshal(welcome); err == nil {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
		}
	}
}

// handleUnregister unregisters a client
func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; !ok {
		return
	}

	delete(h.clients, client.ID)

	// Remove from user index
	if userConns, ok := h.userIndex[client.UserID]; ok {
		delete(userConns, client.ID)
		if len(userConns) == 0 {
			delete(h.userIndex, client.UserID)
			// Update presence to offline
			h.updatePresence(client.UserID, models.PresenceOffline, 0)
		} else {
			// Decrement connection count
			h.updatePresenceConnectionCount(client.UserID, -1)
		}
	}

	close(client.Send)
	client.Conn.Close()

	// Mark connection as inactive in database
	h.deactivateConnection(client.ID)

	logger.Info(fmt.Sprintf("Client unregistered: %s (user: %s)", client.ID, client.UserID))
}

// handleBroadcast broadcasts a message
func (h *Hub) handleBroadcast(msg *BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(models.WebSocketMessage{
		Type:      models.MessageTypeEvent,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"event_type": msg.EventType,
			"data":       msg.Payload,
		},
	})
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to marshal broadcast message: %v", err))
		return
	}

	if msg.UserID != nil {
		// Send to specific user
		if conns, ok := h.userIndex[*msg.UserID]; ok {
			for connID := range conns {
				if connID == msg.ExcludeConn {
					continue
				}
				if client, ok := h.clients[connID]; ok {
					select {
					case client.Send <- data:
					default:
						// Client buffer full, close connection
						close(client.Send)
						delete(h.clients, connID)
					}
				}
			}
		}
	} else {
		// Broadcast to all clients
		for id, client := range h.clients {
			if id == msg.ExcludeConn {
				continue
			}
			select {
			case client.Send <- data:
			default:
				close(client.Send)
				delete(h.clients, id)
			}
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDStr, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to upgrade connection: %v", err))
		return
	}

	// Get device info from query params
	deviceID := c.Query("device_id")
	deviceType := c.Query("device_type")
	if deviceType == "" {
		deviceType = "unknown"
	}

	// Create client
	client := &Client{
		ID:          uuid.New().String(),
		UserID:      userID,
		Conn:        conn,
		Hub:         h,
		Send:        make(chan []byte, 256),
		DeviceID:    deviceID,
		DeviceType:  deviceType,
		ConnectedAt: time.Now(),
		LastPingAt:  time.Now(),
	}

	// Register client
	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.LastPingAt = time.Now()
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error(fmt.Sprintf("WebSocket error: %v", err))
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.Conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (c *Client) handleMessage(data []byte) {
	var msg models.WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error(fmt.Sprintf("Failed to unmarshal WebSocket message: %v", err))
		return
	}

	switch msg.Type {
	case models.MessageTypePing:
		c.LastPingAt = time.Now()
		response := models.WebSocketMessage{
			Type:      models.MessageTypePong,
			Timestamp: time.Now(),
			Payload:   map[string]interface{}{"server_time": time.Now().Unix()},
		}
		if respData, err := json.Marshal(response); err == nil {
			c.Send <- respData
		}

	case models.MessageTypeSync:
		c.handleSyncRequest(&msg)

	case models.MessageTypePresence:
		c.handlePresenceUpdate(&msg)
	}
}

// handleSyncRequest handles sync requests
func (c *Client) handleSyncRequest(msg *models.WebSocketMessage) {
	var req models.SyncCursorRequest
	if cursor, ok := msg.Payload["cursor"].(string); ok {
		req.Cursor = cursor
	}

	// Fetch changes since cursor
	changes, err := c.fetchChanges(req.Cursor)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch changes: %v", err))
		return
	}

	response := models.WebSocketMessage{
		Type:      models.MessageTypeSync,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"changes": changes,
			"cursor":  uuid.New().String(), // New cursor for next sync
		},
	}

	if data, err := json.Marshal(response); err == nil {
		c.Send <- data
	}
}

// handlePresenceUpdate handles presence updates
func (c *Client) handlePresenceUpdate(msg *models.WebSocketMessage) {
	status, _ := msg.Payload["status"].(string)
	if status == "" {
		return
	}

	c.Hub.updatePresence(c.UserID, status, 0)
}

// fetchChanges fetches changes from the database
func (c *Client) fetchChanges(cursor string) ([]models.ChangeLog, error) {
	var changes []models.ChangeLog

	if cursor == "" {
		// Initial sync - get recent changes
		err := c.Hub.db.Select(&changes, `
			SELECT * FROM zuri_sync.change_log 
			WHERE user_id = $1 
			AND changed_at > NOW() - INTERVAL '24 hours'
			ORDER BY changed_at ASC
			LIMIT 100`, c.UserID)
		return changes, err
	}

	// Get changes since cursor
	cursorUUID, err := uuid.Parse(cursor)
	if err != nil {
		return nil, err
	}

	err = c.Hub.db.Select(&changes, `
		SELECT * FROM zuri_sync.change_log 
		WHERE user_id = $1 
		AND id > $2
		ORDER BY changed_at ASC
		LIMIT 100`, c.UserID, cursorUUID)

	return changes, err
}

// BroadcastToUser broadcasts a message to all connections of a user
func (h *Hub) BroadcastToUser(userID uuid.UUID, eventType string, payload interface{}, excludeConn string) {
	h.broadcast <- &BroadcastMessage{
		UserID:      &userID,
		EventType:   eventType,
		Payload:     payload,
		ExcludeConn: excludeConn,
	}
}

// BroadcastToAll broadcasts a message to all connected clients
func (h *Hub) BroadcastToAll(eventType string, payload interface{}, excludeConn string) {
	h.broadcast <- &BroadcastMessage{
		UserID:      nil,
		EventType:   eventType,
		Payload:     payload,
		ExcludeConn: excludeConn,
	}
}

// storeConnection stores connection info in database
func (h *Hub) storeConnection(client *Client) {
	ip := client.Conn.RemoteAddr().String()
	
	_, err := h.db.Exec(`
		INSERT INTO zuri_sync.connections 
		(connection_id, user_id, device_id, device_type, is_active, ip_address)
		VALUES ($1, $2, $3, $4, true, $5)
		ON CONFLICT (connection_id) 
		DO UPDATE SET is_active = true, last_ping_at = CURRENT_TIMESTAMP`,
		client.ID, client.UserID, client.DeviceID, client.DeviceType, ip)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to store connection: %v", err))
	}
}

// deactivateConnection marks a connection as inactive
func (h *Hub) deactivateConnection(connectionID string) {
	_, err := h.db.Exec(`
		UPDATE zuri_sync.connections 
		SET is_active = false, disconnected_at = CURRENT_TIMESTAMP
		WHERE connection_id = $1`, connectionID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to deactivate connection: %v", err))
	}
}

// updatePresence updates user presence
func (h *Hub) updatePresence(userID uuid.UUID, status string, deltaConnections int) {
	// Get current connection count
	var count int
	if deltaConnections != 0 {
		h.db.Get(&count, `
			SELECT COALESCE(SUM(delta), 0) FROM (
				SELECT active_connections + $2 as delta 
				FROM zuri_sync.presence 
				WHERE user_id = $1
				UNION ALL
				SELECT $2
			) t`, userID, deltaConnections)
	} else {
		h.db.Get(&count, `
			SELECT active_connections FROM zuri_sync.presence WHERE user_id = $1`, userID)
	}

	if count < 0 {
		count = 0
	}

	_, err := h.db.Exec(`
		INSERT INTO zuri_sync.presence 
		(user_id, status, active_connections, last_seen_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) 
		DO UPDATE SET status = $2, active_connections = $3, last_seen_at = CURRENT_TIMESTAMP`,
		userID, status, count)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update presence: %v", err))
	}
}

// updatePresenceConnectionCount updates just the connection count
func (h *Hub) updatePresenceConnectionCount(userID uuid.UUID, delta int) {
	_, err := h.db.Exec(`
		UPDATE zuri_sync.presence 
		SET active_connections = GREATEST(0, active_connections + $2),
		    last_seen_at = CURRENT_TIMESTAMP
		WHERE user_id = $1`, userID, delta)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update presence count: %v", err))
	}
}

// cleanupRoutine periodically cleans up stale connections
func (h *Hub) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ticker.C:
			h.cleanupStaleConnections()
		}
	}
}

// cleanupStaleConnections removes connections that haven't pinged in a while
func (h *Hub) cleanupStaleConnections() {
	cutoff := time.Now().Add(-5 * time.Minute)

	var staleConns []string
	err := h.db.Select(&staleConns, `
		SELECT connection_id FROM zuri_sync.connections 
		WHERE is_active = true AND last_ping_at < $1`, cutoff)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to find stale connections: %v", err))
		return
	}

	for _, connID := range staleConns {
		h.mu.RLock()
		client, ok := h.clients[connID]
		h.mu.RUnlock()

		if ok {
			h.unregister <- client
		}
	}

	if len(staleConns) > 0 {
		logger.Info(fmt.Sprintf("Cleaned up %d stale connections", len(staleConns)))
	}
}
