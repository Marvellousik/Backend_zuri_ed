package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// JSONB is a PostgreSQL JSONB-compatible type that implements driver.Valuer
// and sql.Scanner so sqlx can bind it to JSONB columns.
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface.
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface.
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into JSONB", value)
	}
	return json.Unmarshal(bytes, j)
}

// NotificationPreferences represents user notification settings
type NotificationPreferences struct {
	ID                       uuid.UUID      `db:"id" json:"id"`
	UserID                   uuid.UUID      `db:"user_id" json:"user_id"`
	PushEnabled              bool           `db:"push_enabled" json:"push_enabled"`
	PushDeviceTokens         pq.StringArray `db:"push_device_tokens" json:"push_device_tokens"`
	EmailEnabled             bool           `db:"email_enabled" json:"email_enabled"`
	EmailFrequency           string    `db:"email_frequency" json:"email_frequency"`
	QuietHoursStart          int       `db:"quiet_hours_start" json:"quiet_hours_start"`
	QuietHoursEnd            int       `db:"quiet_hours_end" json:"quiet_hours_end"`
	Timezone                 string    `db:"timezone" json:"timezone"`
	NotifyOnQuizCompletion   bool      `db:"notify_on_quiz_completion" json:"notify_on_quiz_completion"`
	NotifyOnStreakAchievement bool     `db:"notify_on_streak_achievement" json:"notify_on_streak_achievement"`
	NotifyOnGoalCompletion   bool      `db:"notify_on_goal_completion" json:"notify_on_goal_completion"`
	NotifyOnMaterialProcessed bool     `db:"notify_on_material_processed" json:"notify_on_material_processed"`
	NotifyOnStudyReminder    bool      `db:"notify_on_study_reminder" json:"notify_on_study_reminder"`
	UpdatedAt                time.Time `db:"updated_at" json:"updated_at"`
	CreatedAt                time.Time `db:"created_at" json:"created_at"`
}

// NotificationQueue represents a pending notification
type NotificationQueue struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	UserID       uuid.UUID       `db:"user_id" json:"user_id"`
	Type         string          `db:"notification_type" json:"notification_type"`
	Channel      string          `db:"channel" json:"channel"`
	Title        string          `db:"title" json:"title"`
	Body         string          `db:"body" json:"body"`
	Data         JSONB `db:"data" json:"data"`
	DeviceToken  *string         `db:"device_token" json:"device_token,omitempty"`
	EmailAddress *string         `db:"email_address" json:"email_address,omitempty"`
	ScheduledAt  *time.Time      `db:"scheduled_at" json:"scheduled_at,omitempty"`
	SentAt       *time.Time      `db:"sent_at" json:"sent_at,omitempty"`
	Status       string          `db:"status" json:"status"`
	ErrorMessage *string         `db:"error_message" json:"error_message,omitempty"`
	RetryCount   int             `db:"retry_count" json:"retry_count"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}

// NotificationHistory represents sent notifications
type NotificationHistory struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	UserID       uuid.UUID       `db:"user_id" json:"user_id"`
	Type         string          `db:"notification_type" json:"notification_type"`
	Channel      string          `db:"channel" json:"channel"`
	Title        string          `db:"title" json:"title"`
	Body         string          `db:"body" json:"body"`
	Data         JSONB `db:"data" json:"data"`
	SentAt       time.Time       `db:"sent_at" json:"sent_at"`
	DeliveredAt  *time.Time      `db:"delivered_at" json:"delivered_at,omitempty"`
	OpenedAt     *time.Time      `db:"opened_at" json:"opened_at,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}

// ScheduledReminder represents a scheduled reminder
type ScheduledReminder struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	Type              string     `db:"reminder_type" json:"reminder_type"`
	Title             string     `db:"title" json:"title"`
	Body              string     `db:"body" json:"body"`
	ScheduledFor      time.Time  `db:"scheduled_for" json:"scheduled_for"`
	Timezone          string     `db:"timezone" json:"timezone"`
	Recurrence        *string    `db:"recurrence" json:"recurrence,omitempty"`
	RecurrenceEndDate *time.Time `db:"recurrence_end_date" json:"recurrence_end_date,omitempty"`
	EntityType        *string    `db:"entity_type" json:"entity_type,omitempty"`
	EntityID          *uuid.UUID `db:"entity_id" json:"entity_id,omitempty"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	SentAt            *time.Time `db:"sent_at" json:"sent_at,omitempty"`
	CancelledAt       *time.Time `db:"cancelled_at" json:"cancelled_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// UpdatePreferencesRequest represents update request
type UpdatePreferencesRequest struct {
	PushEnabled               *bool   `json:"push_enabled,omitempty"`
	EmailEnabled              *bool   `json:"email_enabled,omitempty"`
	EmailFrequency            *string `json:"email_frequency,omitempty"`
	QuietHoursStart           *int    `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd             *int    `json:"quiet_hours_end,omitempty"`
	Timezone                  *string `json:"timezone,omitempty"`
	NotifyOnQuizCompletion    *bool   `json:"notify_on_quiz_completion,omitempty"`
	NotifyOnStreakAchievement *bool   `json:"notify_on_streak_achievement,omitempty"`
	NotifyOnGoalCompletion    *bool   `json:"notify_on_goal_completion,omitempty"`
	NotifyOnMaterialProcessed *bool   `json:"notify_on_material_processed,omitempty"`
	NotifyOnStudyReminder     *bool   `json:"notify_on_study_reminder,omitempty"`
}

// RegisterDeviceRequest represents device registration
type RegisterDeviceRequest struct {
	DeviceToken string `json:"device_token" binding:"required"`
	DeviceType  string `json:"device_type,omitempty"`
}

// SendNotificationRequest represents a notification send request
type SendNotificationRequest struct {
	UserID    uuid.UUID              `json:"user_id" binding:"required"`
	Type      string                 `json:"type" binding:"required"` // push, email
	Title     string                 `json:"title" binding:"required"`
	Body      string                 `json:"body" binding:"required"`
	Data      JSONB `json:"data,omitempty"`
	Scheduled *time.Time             `json:"scheduled,omitempty"`
}

// CreateReminderRequest represents a reminder creation request
type CreateReminderRequest struct {
	Type              string     `json:"type" binding:"required"`
	Title             string     `json:"title" binding:"required"`
	Body              string     `json:"body" binding:"required"`
	ScheduledFor      time.Time  `json:"scheduled_for" binding:"required"`
	Recurrence        *string    `json:"recurrence,omitempty"`
	RecurrenceEndDate *time.Time `json:"recurrence_end_date,omitempty"`
	EntityType        *string    `json:"entity_type,omitempty"`
	EntityID          *uuid.UUID `json:"entity_id,omitempty"`
}

// NotificationEvent represents an event that triggers notifications
type NotificationEvent struct {
	EventType   string                 `json:"event_type"`
	UserID      uuid.UUID              `json:"user_id"`
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	Data        JSONB `json:"data,omitempty"`
}

// Constants for notification types
const (
	NotificationTypePush  = "push"
	NotificationTypeEmail = "email"
)

// Constants for channels
const (
	ChannelFCM  = "fcm"
	ChannelSMTP = "smtp"
)

// Constants for notification status
const (
	StatusPending   = "pending"
	StatusSent      = "sent"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Constants for reminder types
const (
	ReminderTypeStudy      = "study_reminder"
	ReminderTypeQuiz       = "quiz_reminder"
	ReminderTypeReview     = "review_reminder"
	ReminderTypeGoal       = "goal_reminder"
)

// Constants for event types
const (
	EventQuizCompleted      = "quiz.completed"
	EventStreakAchieved     = "streak.achieved"
	EventGoalCompleted      = "goal.completed"
	EventMaterialProcessed  = "material.processed"
	EventStudyReminder      = "study.reminder"
)
