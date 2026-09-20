package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"zuri/services/notification-service/models"
	"zuri/services/notification-service/services"

	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/middleware"
)

// Handler handles HTTP requests
type Handler struct {
	db           *sqlx.DB
	fcmService   *services.FCMService
	emailService *services.EmailService
}

// NewHandler creates a new handler
func NewHandler(db *sqlx.DB, fcmService *services.FCMService, emailService *services.EmailService) *Handler {
	return &Handler{
		db:           db,
		fcmService:   fcmService,
		emailService: emailService,
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

		// Protected routes — under /notifications to match gateway proxy paths
		notifications := api.Group("/notifications")
		notifications.Use(middleware.AuthRequired())
		{
			// Preferences
			notifications.GET("/preferences", h.GetPreferences)
			notifications.PUT("/preferences", h.UpdatePreferences)

			// Device management
			notifications.POST("/devices/register", h.RegisterDevice)
			notifications.DELETE("/devices/:token", h.UnregisterDevice)

			// Reminders
			notifications.GET("/reminders", h.GetReminders)
			notifications.POST("/reminders", h.CreateReminder)
			notifications.DELETE("/reminders/:id", h.CancelReminder)

			// Notification history
			notifications.GET("/history", h.GetNotificationHistory)
		}

		// Internal routes (for other services to send notifications)
		internal := api.Group("/internal")
		{
			internal.POST("/send", h.SendNotification)
			internal.POST("/events", h.HandleEvent)
		}
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "notification-service",
		"time":    time.Now().UTC(),
	})
}

// GetPreferences gets user notification preferences
func (h *Handler) GetPreferences(c *gin.Context) {
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

	var prefs models.NotificationPreferences
	err = h.db.Get(&prefs, `
		SELECT id, user_id, push_enabled, email_enabled, email_frequency,
		       quiet_hours_start, quiet_hours_end, timezone,
		       notify_on_quiz_completion, notify_on_streak_achievement,
		       notify_on_goal_completion, notify_on_material_processed,
		       notify_on_study_reminder, updated_at, created_at
		FROM zuri_notification.preferences
		WHERE user_id = $1`, uid)

	if err != nil {
		// Return default preferences if not found
		prefs = models.NotificationPreferences{
			UserID:                    uid,
			PushEnabled:               true,
			EmailEnabled:              true,
			EmailFrequency:            "immediate",
			QuietHoursStart:           22,
			QuietHoursEnd:             8,
			Timezone:                  "UTC",
			NotifyOnQuizCompletion:    true,
			NotifyOnStreakAchievement: true,
			NotifyOnGoalCompletion:    true,
			NotifyOnMaterialProcessed: true,
			NotifyOnStudyReminder:     true,
		}
		
		// Insert default preferences
		_, err = h.db.NamedExec(`
			INSERT INTO zuri_notification.preferences 
			(user_id, push_enabled, email_enabled, email_frequency, 
			 quiet_hours_start, quiet_hours_end, timezone,
			 notify_on_quiz_completion, notify_on_streak_achievement,
			 notify_on_goal_completion, notify_on_material_processed, notify_on_study_reminder)
			VALUES (:user_id, :push_enabled, :email_enabled, :email_frequency,
				:quiet_hours_start, :quiet_hours_end, :timezone,
				:notify_on_quiz_completion, :notify_on_streak_achievement,
				:notify_on_goal_completion, :notify_on_material_processed, :notify_on_study_reminder)`,
			prefs)
			
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to create preferences: %v", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create preferences"})
			return
		}
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdatePreferences updates user notification preferences
func (h *Handler) UpdatePreferences(c *gin.Context) {
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

	var req models.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	query := "UPDATE zuri_notification.preferences SET "
	params := []interface{}{}
	paramCount := 0

	if req.PushEnabled != nil {
		paramCount++
		query += "push_enabled = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.PushEnabled)
	}
	if req.EmailEnabled != nil {
		paramCount++
		query += "email_enabled = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.EmailEnabled)
	}
	if req.EmailFrequency != nil {
		paramCount++
		query += "email_frequency = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.EmailFrequency)
	}
	if req.QuietHoursStart != nil {
		paramCount++
		query += "quiet_hours_start = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.QuietHoursStart)
	}
	if req.QuietHoursEnd != nil {
		paramCount++
		query += "quiet_hours_end = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.QuietHoursEnd)
	}
	if req.Timezone != nil {
		paramCount++
		query += "timezone = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.Timezone)
	}
	if req.NotifyOnQuizCompletion != nil {
		paramCount++
		query += "notify_on_quiz_completion = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.NotifyOnQuizCompletion)
	}
	if req.NotifyOnStreakAchievement != nil {
		paramCount++
		query += "notify_on_streak_achievement = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.NotifyOnStreakAchievement)
	}
	if req.NotifyOnGoalCompletion != nil {
		paramCount++
		query += "notify_on_goal_completion = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.NotifyOnGoalCompletion)
	}
	if req.NotifyOnMaterialProcessed != nil {
		paramCount++
		query += "notify_on_material_processed = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.NotifyOnMaterialProcessed)
	}
	if req.NotifyOnStudyReminder != nil {
		paramCount++
		query += "notify_on_study_reminder = $" + string(rune('0'+paramCount)) + ", "
		params = append(params, *req.NotifyOnStudyReminder)
	}

	if paramCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	paramCount++
	query = query[:len(query)-2] + " WHERE user_id = $" + string(rune('0'+paramCount))
	params = append(params, uid)

	_, err = h.db.Exec(query, params...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update preferences: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Preferences updated"})
}

// RegisterDevice registers a device token for push notifications
func (h *Handler) RegisterDevice(c *gin.Context) {
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

	var req models.RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add token to user's device tokens
	_, err = h.db.Exec(`
		UPDATE zuri_notification.preferences 
		SET push_device_tokens = array_append(push_device_tokens, $1),
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2 
		AND NOT ($1 = ANY(push_device_tokens))`,
		req.DeviceToken, uid)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to register device: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device registered"})
}

// UnregisterDevice removes a device token
func (h *Handler) UnregisterDevice(c *gin.Context) {
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
	token := c.Param("token")

	_, err = h.db.Exec(`
		UPDATE zuri_notification.preferences 
		SET push_device_tokens = array_remove(push_device_tokens, $1),
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2`,
		token, uid)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to unregister device: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device unregistered"})
}

// GetReminders gets user's scheduled reminders
func (h *Handler) GetReminders(c *gin.Context) {
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

	var reminders []models.ScheduledReminder
	err = h.db.Select(&reminders, `
		SELECT * FROM zuri_notification.scheduled_reminders 
		WHERE user_id = $1 AND is_active = true AND cancelled_at IS NULL
		ORDER BY scheduled_for ASC`, uid)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get reminders: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get reminders"})
		return
	}

	c.JSON(http.StatusOK, reminders)
}

// CreateReminder creates a new reminder
func (h *Handler) CreateReminder(c *gin.Context) {
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

	var req models.CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reminder := models.ScheduledReminder{
		UserID:            uid,
		Type:              req.Type,
		Title:             req.Title,
		Body:              req.Body,
		ScheduledFor:      req.ScheduledFor,
		Recurrence:        req.Recurrence,
		RecurrenceEndDate: req.RecurrenceEndDate,
		EntityType:        req.EntityType,
		EntityID:          req.EntityID,
		IsActive:          true,
	}

	_, err = h.db.NamedExec(`
		INSERT INTO zuri_notification.scheduled_reminders 
		(user_id, reminder_type, title, body, scheduled_for, timezone,
		 recurrence, recurrence_end_date, entity_type, entity_id, is_active)
		VALUES (:user_id, :reminder_type, :title, :body, :scheduled_for, :timezone,
			:recurrence, :recurrence_end_date, :entity_type, :entity_id, :is_active)`,
		reminder)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create reminder: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reminder"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Reminder created"})
}

// CancelReminder cancels a reminder
func (h *Handler) CancelReminder(c *gin.Context) {
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
	reminderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reminder ID"})
		return
	}

	_, err = h.db.Exec(`
		UPDATE zuri_notification.scheduled_reminders 
		SET is_active = false, cancelled_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2`,
		reminderID, uid)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to cancel reminder: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reminder cancelled"})
}

// GetNotificationHistory gets user's notification history
func (h *Handler) GetNotificationHistory(c *gin.Context) {
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

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var history []models.NotificationHistory
	err = h.db.Select(&history, `
		SELECT * FROM zuri_notification.history 
		WHERE user_id = $1
		ORDER BY sent_at DESC
		LIMIT $2`, uid, limit)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get history: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification history"})
		return
	}

	c.JSON(http.StatusOK, history)
}

// SendNotification sends a notification (internal API)
func (h *Handler) SendNotification(c *gin.Context) {
	var req models.SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Queue the notification
	queue := models.NotificationQueue{
		UserID:      req.UserID,
		Type:        req.Type,
		Channel:     h.getChannelForType(req.Type),
		Title:       req.Title,
		Body:        req.Body,
		Data:        req.Data,
		Status:      models.StatusPending,
	}

	if req.Scheduled != nil {
		queue.ScheduledAt = req.Scheduled
	} else {
		now := time.Now()
		queue.ScheduledAt = &now
	}

	_, err := h.db.NamedExec(`
		INSERT INTO zuri_notification.queue 
		(user_id, notification_type, channel, title, body, data, scheduled_at, status)
		VALUES (:user_id, :notification_type, :channel, :title, :body, :data, :scheduled_at, :status)`,
		queue)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to queue notification: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue notification"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Notification queued"})
}

// HandleEvent processes notification events from other services
func (h *Handler) HandleEvent(c *gin.Context) {
	var event models.NotificationEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check user preferences before queueing
	var prefs models.NotificationPreferences
	err := h.db.Get(&prefs, `
		SELECT id, user_id, push_enabled, email_enabled, email_frequency,
		       quiet_hours_start, quiet_hours_end, timezone,
		       notify_on_quiz_completion, notify_on_streak_achievement,
		       notify_on_goal_completion, notify_on_material_processed,
		       notify_on_study_reminder, updated_at, created_at
		FROM zuri_notification.preferences
		WHERE user_id = $1`, event.UserID)

	if err != nil {
		logger.Warn(fmt.Sprintf("No preferences found for user %s, using defaults", event.UserID))
		// Continue with default preferences (send notification)
	}

	// Check if user wants this notification type
	if !h.shouldNotifyForEvent(&prefs, event.EventType) {
		c.JSON(http.StatusOK, gin.H{"message": "Notification skipped due to preferences"})
		return
	}

	// Check quiet hours
	if h.isInQuietHours(&prefs) {
		// Schedule for after quiet hours
		nextAvailable := h.getNextAvailableTime(&prefs)
		event.Data["quiet_hours_delayed"] = true
		
		queue := models.NotificationQueue{
			UserID:      event.UserID,
			Type:        models.NotificationTypePush,
			Channel:     models.ChannelFCM,
			Title:       event.Title,
			Body:        event.Body,
			Data:        event.Data,
			ScheduledAt: &nextAvailable,
			Status:      models.StatusPending,
		}

		_, err := h.db.NamedExec(`
			INSERT INTO zuri_notification.queue 
			(user_id, notification_type, channel, title, body, data, scheduled_at, status)
			VALUES (:user_id, :notification_type, :channel, :title, :body, :data, :scheduled_at, :status)`,
			queue)

		if err != nil {
			logger.Error(fmt.Sprintf("Failed to queue delayed notification: %v", err))
		}

		c.JSON(http.StatusAccepted, gin.H{"message": "Notification scheduled after quiet hours"})
		return
	}

	// Queue immediate notification
	now := time.Now()
	queue := models.NotificationQueue{
		UserID:      event.UserID,
		Type:        models.NotificationTypePush,
		Channel:     models.ChannelFCM,
		Title:       event.Title,
		Body:        event.Body,
		Data:        event.Data,
		ScheduledAt: &now,
		Status:      models.StatusPending,
	}

	_, err = h.db.NamedExec(`
		INSERT INTO zuri_notification.queue 
		(user_id, notification_type, channel, title, body, data, scheduled_at, status)
		VALUES (:user_id, :notification_type, :channel, :title, :body, :data, :scheduled_at, :status)`,
		queue)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to queue notification: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue notification"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Notification queued"})
}

// Helper functions
func (h *Handler) getChannelForType(notificationType string) string {
	switch notificationType {
	case models.NotificationTypePush:
		return models.ChannelFCM
	case models.NotificationTypeEmail:
		return models.ChannelSMTP
	default:
		return models.ChannelFCM
	}
}

func (h *Handler) shouldNotifyForEvent(prefs *models.NotificationPreferences, eventType string) bool {
	// If preferences are nil or new, default to true
	if prefs == nil || prefs.ID == uuid.Nil {
		return true
	}

	switch eventType {
	case models.EventQuizCompleted:
		return prefs.NotifyOnQuizCompletion
	case models.EventStreakAchieved:
		return prefs.NotifyOnStreakAchievement
	case models.EventGoalCompleted:
		return prefs.NotifyOnGoalCompletion
	case models.EventMaterialProcessed:
		return prefs.NotifyOnMaterialProcessed
	case models.EventStudyReminder:
		return prefs.NotifyOnStudyReminder
	default:
		return true
	}
}

func (h *Handler) isInQuietHours(prefs *models.NotificationPreferences) bool {
	if prefs == nil {
		return false
	}

	loc, err := time.LoadLocation(prefs.Timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	hour := now.Hour()

	if prefs.QuietHoursStart > prefs.QuietHoursEnd {
		// Overnight quiet hours (e.g., 22:00 - 08:00)
		return hour >= prefs.QuietHoursStart || hour < prefs.QuietHoursEnd
	}

	return hour >= prefs.QuietHoursStart && hour < prefs.QuietHoursEnd
}

func (h *Handler) getNextAvailableTime(prefs *models.NotificationPreferences) time.Time {
	loc, err := time.LoadLocation(prefs.Timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), prefs.QuietHoursEnd, 0, 0, 0, loc)

	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}

	return next.UTC()
}


