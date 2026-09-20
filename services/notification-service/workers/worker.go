package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"zuri/shared/pkg/logger"

	"zuri/services/notification-service/models"
	"zuri/services/notification-service/services"
)

// Worker processes notifications in the background
type Worker struct {
	db           *sqlx.DB
	fcmService   *services.FCMService
	emailService *services.EmailService
	quit         chan bool
}

// NewWorker creates a new notification worker
func NewWorker(db *sqlx.DB, fcmService *services.FCMService, emailService *services.EmailService) *Worker {
	return &Worker{
		db:           db,
		fcmService:   fcmService,
		emailService: emailService,
		quit:         make(chan bool),
	}
}

// Start starts the worker
func (w *Worker) Start() {
	logger.Info("Starting notification worker")
	
	// Process notifications every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	
	// Process reminders every minute
	reminderTicker := time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-ticker.C:
				w.processPendingNotifications()
			case <-reminderTicker.C:
				w.processDueReminders()
			case <-w.quit:
				ticker.Stop()
				reminderTicker.Stop()
				return
			}
		}
	}()
}

// Stop stops the worker
func (w *Worker) Stop() {
	logger.Info("Stopping notification worker")
	close(w.quit)
}

// processPendingNotifications processes pending notifications from the queue
func (w *Worker) processPendingNotifications() {
	ctx := context.Background()

	// Get pending notifications
	var notifications []models.NotificationQueue
	err := w.db.Select(&notifications, `
		SELECT * FROM zuri_notification.queue 
		WHERE status = $1 
		AND scheduled_at <= CURRENT_TIMESTAMP
		AND retry_count < 3
		ORDER BY scheduled_at ASC
		LIMIT 100`, models.StatusPending)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch pending notifications: %v", err))
		return
	}

	if len(notifications) == 0 {
		return
	}

	logger.Debug(fmt.Sprintf("Processing %d pending notifications", len(notifications)))

	for _, n := range notifications {
		w.processNotification(ctx, &n)
	}
}

// processNotification processes a single notification
func (w *Worker) processNotification(ctx context.Context, n *models.NotificationQueue) {
	// Get user preferences and email
	var prefs struct {
		PushEnabled  bool           `db:"push_enabled"`
		EmailEnabled bool           `db:"email_enabled"`
		UserEmail    string         `db:"user_email"`
		Tokens       pq.StringArray `db:"push_device_tokens"`
	}

	err := w.db.Get(&prefs, `
		SELECT COALESCE(p.push_enabled, true) as push_enabled,
		       COALESCE(p.email_enabled, true) as email_enabled,
		       u.email as user_email,
		       COALESCE(p.push_device_tokens, ARRAY[]::text[]) as push_device_tokens
		FROM zuri_auth.users u
		LEFT JOIN zuri_notification.preferences p ON p.user_id = u.id
		WHERE u.id = $1`, n.UserID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get user preferences for notification %s: %v", n.ID, err))
		w.markFailed(n.ID, "User preferences not found")
		return
	}

	var success bool

	switch n.Type {
	case models.NotificationTypePush:
		if prefs.PushEnabled && len(prefs.Tokens) > 0 {
			success = w.sendPushNotification(ctx, n, prefs.Tokens)
		} else {
			logger.Debug(fmt.Sprintf("Push disabled or no tokens for user %s", n.UserID))
			success = true // Mark as success since user doesn't want it
		}

	case models.NotificationTypeEmail:
		if prefs.EmailEnabled && prefs.UserEmail != "" {
			success = w.sendEmailNotification(n, prefs.UserEmail)
		} else {
			logger.Debug(fmt.Sprintf("Email disabled for user %s", n.UserID))
			success = true
		}
	}

	if success {
		w.markSent(n.ID)
	} else {
		w.retryOrFail(n.ID, "Failed to send")
	}
}

// sendPushNotification sends a push notification via FCM
func (w *Worker) sendPushNotification(ctx context.Context, n *models.NotificationQueue, tokens []string) bool {
	if !w.fcmService.IsEnabled() {
		logger.Warn("FCM service disabled, skipping push notification")
		return false
	}

	// Send to all device tokens
	for _, token := range tokens {
		err := w.fcmService.SendNotification(ctx, token, n.Title, n.Body, map[string]interface{}(n.Data))
		if err != nil {
			if err.Error() == "invalid_token" {
				// Remove invalid token
				w.removeInvalidToken(n.UserID, token)
			}
			logger.Error(fmt.Sprintf("Failed to send push notification to token %s: %v", token[:20], err))
		}
	}

	return true
}

// sendEmailNotification sends an email notification
func (w *Worker) sendEmailNotification(n *models.NotificationQueue, email string) bool {
	if !w.emailService.IsEnabled() {
		logger.Warn("Email service disabled, skipping email notification")
		return false
	}

	// Determine template based on notification data
	templateName := "default"
	if n.Data != nil {
		if t, ok := n.Data["template"].(string); ok {
			templateName = t
		}
	}

	var err error
	if templateName != "default" {
		err = w.emailService.SendTemplateEmail(email, n.Title, templateName, n.Data)
	} else {
		err = w.emailService.SendEmail(email, n.Title, n.Body)
	}

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email: %v", err))
		return false
	}

	return true
}

// removeInvalidToken removes an invalid FCM token from user's devices
func (w *Worker) removeInvalidToken(userID interface{}, token string) {
	_, err := w.db.Exec(`
		UPDATE zuri_notification.preferences 
		SET push_device_tokens = array_remove(push_device_tokens, $1),
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2`, token, userID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to remove invalid token: %v", err))
	} else {
		logger.Info(fmt.Sprintf("Removed invalid FCM token for user %s", userID))
	}
}

// markSent marks a notification as sent
func (w *Worker) markSent(notificationID interface{}) {
	now := time.Now()
	_, err := w.db.Exec(`
		UPDATE zuri_notification.queue 
		SET status = $1, sent_at = $2
		WHERE id = $3`, models.StatusSent, now, notificationID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to mark notification as sent: %v", err))
	}
}

// markFailed marks a notification as failed
func (w *Worker) markFailed(notificationID interface{}, reason string) {
	_, err := w.db.Exec(`
		UPDATE zuri_notification.queue 
		SET status = $1, error_message = $2
		WHERE id = $3`, models.StatusFailed, reason, notificationID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to mark notification as failed: %v", err))
	}
}

// retryOrFail increments retry count or marks as failed
func (w *Worker) retryOrFail(notificationID interface{}, reason string) {
	_, err := w.db.Exec(`
		UPDATE zuri_notification.queue 
		SET retry_count = retry_count + 1, error_message = $1
		WHERE id = $2`, reason, notificationID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update retry count: %v", err))
	}
}

// processDueReminders processes reminders that are due
func (w *Worker) processDueReminders() {
	// Get due reminders
	var reminders []models.ScheduledReminder
	err := w.db.Select(&reminders, `
		SELECT * FROM zuri_notification.scheduled_reminders 
		WHERE is_active = true 
		AND sent_at IS NULL 
		AND scheduled_for <= CURRENT_TIMESTAMP
		LIMIT 50`)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch due reminders: %v", err))
		return
	}

	if len(reminders) == 0 {
		return
	}

	logger.Debug(fmt.Sprintf("Processing %d due reminders", len(reminders)))

	for _, r := range reminders {
		w.processReminder(&r)
	}
}

// processReminder processes a single reminder
func (w *Worker) processReminder(r *models.ScheduledReminder) {
	// Queue notification
	now := time.Now()
	queue := models.NotificationQueue{
		UserID:      r.UserID,
		Type:        models.NotificationTypePush,
		Channel:     models.ChannelFCM,
		Title:       r.Title,
		Body:        r.Body,
		Data: models.JSONB{
			"reminder_id": r.ID.String(),
			"reminder_type": r.Type,
			"entity_type": r.EntityType,
			"entity_id": r.EntityID,
		},
		ScheduledAt: &now,
		Status:      models.StatusPending,
	}

	_, err := w.db.NamedExec(`
		INSERT INTO zuri_notification.queue 
		(user_id, notification_type, channel, title, body, data, scheduled_at, status)
		VALUES (:user_id, :notification_type, :channel, :title, :body, :data, :scheduled_at, :status)`,
		queue)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to queue reminder notification: %v", err))
		return
	}

	// Mark reminder as sent
	_, err = w.db.Exec(`
		UPDATE zuri_notification.scheduled_reminders 
		SET sent_at = CURRENT_TIMESTAMP
		WHERE id = $1`, r.ID)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to mark reminder as sent: %v", err))
	}

	// Handle recurrence
	if r.Recurrence != nil && *r.Recurrence != "" {
		w.scheduleNextRecurrence(r)
	}
}

// scheduleNextRecurrence schedules the next occurrence of a recurring reminder
func (w *Worker) scheduleNextRecurrence(r *models.ScheduledReminder) {
	var nextTime time.Time
	loc, _ := time.LoadLocation(r.Timezone)
	if loc == nil {
		loc = time.UTC
	}

	scheduled := r.ScheduledFor.In(loc)

	switch *r.Recurrence {
	case "daily":
		nextTime = scheduled.AddDate(0, 0, 1)
	case "weekly":
		nextTime = scheduled.AddDate(0, 0, 7)
	default:
		return
	}

	// Check if we've passed the end date
	if r.RecurrenceEndDate != nil {
		endDate := time.Date(
			r.RecurrenceEndDate.Year(),
			r.RecurrenceEndDate.Month(),
			r.RecurrenceEndDate.Day(),
			23, 59, 59, 0,
			loc,
		)
		if nextTime.After(endDate) {
			return
		}
	}

	// Create next reminder
	nextReminder := models.ScheduledReminder{
		UserID:            r.UserID,
		Type:              r.Type,
		Title:             r.Title,
		Body:              r.Body,
		ScheduledFor:      nextTime,
		Timezone:          r.Timezone,
		Recurrence:        r.Recurrence,
		RecurrenceEndDate: r.RecurrenceEndDate,
		EntityType:        r.EntityType,
		EntityID:          r.EntityID,
		IsActive:          true,
	}

	_, err := w.db.NamedExec(`
		INSERT INTO zuri_notification.scheduled_reminders 
		(user_id, reminder_type, title, body, scheduled_for, timezone,
		 recurrence, recurrence_end_date, entity_type, entity_id, is_active)
		VALUES (:user_id, :reminder_type, :title, :body, :scheduled_for, :timezone,
			:recurrence, :recurrence_end_date, :entity_type, :entity_id, :is_active)`,
		nextReminder)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to schedule next recurrence: %v", err))
	}
}

// HandleRedisEvent processes events from Redis pub/sub
func (w *Worker) HandleRedisEvent(eventType string, payload []byte) {
	logger.Debug(fmt.Sprintf("Handling Redis event: %s", eventType))

	switch eventType {
	case "quiz.completed":
		var event struct {
			UserID    string `json:"user_id"`
			QuizID    string `json:"quiz_id"`
			Score     float64 `json:"score"`
			QuizTitle string `json:"quiz_title"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			logger.Error(fmt.Sprintf("Failed to unmarshal quiz.completed event: %v", err))
			return
		}
		w.handleQuizCompleted(event)

	case "streak.achieved":
		var event struct {
			UserID      string `json:"user_id"`
			StreakCount int    `json:"streak_count"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			logger.Error(fmt.Sprintf("Failed to unmarshal streak.achieved event: %v", err))
			return
		}
		w.handleStreakAchieved(event)

	case "goal.completed":
		var event struct {
			UserID    string `json:"user_id"`
			GoalID    string `json:"goal_id"`
			GoalTitle string `json:"goal_title"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			logger.Error(fmt.Sprintf("Failed to unmarshal goal.completed event: %v", err))
			return
		}
		w.handleGoalCompleted(event)
	}
}

func (w *Worker) handleQuizCompleted(event struct {
	UserID    string `json:"user_id"`
	QuizID    string `json:"quiz_id"`
	Score     float64 `json:"score"`
	QuizTitle string `json:"quiz_title"`
}) {
	// This would create a notification based on user preferences
	// For now, just log it
	logger.Info(fmt.Sprintf("Quiz completed event: user=%s, quiz=%s, score=%.2f", 
		event.UserID, event.QuizID, event.Score))
}

func (w *Worker) handleStreakAchieved(event struct {
	UserID      string `json:"user_id"`
	StreakCount int    `json:"streak_count"`
}) {
	logger.Info(fmt.Sprintf("Streak achieved event: user=%s, streak=%d", 
		event.UserID, event.StreakCount))
}

func (w *Worker) handleGoalCompleted(event struct {
	UserID    string `json:"user_id"`
	GoalID    string `json:"goal_id"`
	GoalTitle string `json:"goal_title"`
}) {
	logger.Info(fmt.Sprintf("Goal completed event: user=%s, goal=%s", 
		event.UserID, event.GoalTitle))
}
