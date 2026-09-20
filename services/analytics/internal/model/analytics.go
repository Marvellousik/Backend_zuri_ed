// Package model contains domain models for the Analytics Service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// QuizAttemptStatus represents the status of a quiz attempt.
type QuizAttemptStatus string

const (
	AttemptStatusInProgress QuizAttemptStatus = "in_progress"
	AttemptStatusCompleted  QuizAttemptStatus = "completed"
	AttemptStatusAbandoned  QuizAttemptStatus = "abandoned"
)

// QuizAttempt represents a user's attempt at a quiz.
type QuizAttempt struct {
	ID                uuid.UUID         `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID            uuid.UUID         `gorm:"type:uuid;not null;index" json:"user_id"`
	QuizID            uuid.UUID         `gorm:"type:uuid;not null;index" json:"quiz_id"`
	StartedAt         time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"started_at"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	Score             *int              `json:"score,omitempty"`
	MaxScore          *int              `json:"max_score,omitempty"`
	Percentage        *float64          `gorm:"type:decimal(5,2)" json:"percentage,omitempty"`
	TimeTakenSeconds  *int              `json:"time_taken_seconds,omitempty"`
	Status            QuizAttemptStatus `gorm:"type:varchar(20);default:'in_progress'" json:"status"`
	CreatedAt         time.Time         `json:"created_at"`

	// Associations
	Answers []QuizAnswer `gorm:"foreignKey:AttemptID" json:"answers,omitempty"`
}

// TableName specifies the table name for QuizAttempt.
func (QuizAttempt) TableName() string {
	return "zuri_analytics.quiz_attempts"
}

// QuizAnswer represents a user's answer to a specific question.
type QuizAnswer struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	AttemptID         uuid.UUID `gorm:"type:uuid;not null;index" json:"attempt_id"`
	QuestionID        uuid.UUID `gorm:"type:uuid;not null;index" json:"question_id"`
	UserAnswer        string    `gorm:"type:text;not null" json:"user_answer"`
	IsCorrect         *bool     `json:"is_correct,omitempty"`
	SimilarityScore   *float64  `gorm:"type:decimal(5,4)" json:"similarity_score,omitempty"`
	PointsEarned      int       `gorm:"default:0" json:"points_earned"`
	TimeTakenSeconds  *int      `json:"time_taken_seconds,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// TableName specifies the table name for QuizAnswer.
func (QuizAnswer) TableName() string {
	return "zuri_analytics.quiz_answers"
}

// StudySession represents a user's study session on a specific date.
type StudySession struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID            uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	SessionDate       time.Time `gorm:"type:date;not null" json:"session_date"`
	DurationMinutes   int       `gorm:"default:0" json:"duration_minutes"`
	MaterialsReviewed int       `gorm:"default:0" json:"materials_reviewed"`
	QuizzesCompleted  int       `gorm:"default:0" json:"quizzes_completed"`
	FlashcardsReviewed int      `gorm:"default:0" json:"flashcards_reviewed"`
	CreatedAt         time.Time `json:"created_at"`
}

// TableName specifies the table name for StudySession.
func (StudySession) TableName() string {
	return "zuri_analytics.study_sessions"
}

// TopicMastery represents a user's mastery level for a specific topic.
type TopicMastery struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID              uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	CourseID            *uuid.UUID `gorm:"type:uuid;index" json:"course_id,omitempty"`
	Topic               string     `gorm:"type:varchar(255);not null" json:"topic"`
	MasteryScore        float64    `gorm:"type:decimal(5,2);default:0.00" json:"mastery_score"`
	QuestionsAttempted  int        `gorm:"default:0" json:"questions_attempted"`
	QuestionsCorrect    int        `gorm:"default:0" json:"questions_correct"`
	CurrentIntervalDays int        `gorm:"default:0" json:"current_interval_days"`
	LastStudiedAt       *time.Time `json:"last_studied_at,omitempty"`
	NextReviewAt        *time.Time `json:"next_review_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// TableName specifies the table name for TopicMastery.
func (TopicMastery) TableName() string {
	return "zuri_analytics.topic_mastery"
}

// AIInteractionType represents the type of AI interaction.
type AIInteractionType string

const (
	AIInteractionQuizGeneration  AIInteractionType = "quiz_generation"
	AIInteractionSummary         AIInteractionType = "summary"
	AIInteractionFlashcard       AIInteractionType = "flashcard"
	AIInteractionChat            AIInteractionType = "chat"
)

// AIInteraction tracks usage of AI features.
type AIInteraction struct {
	ID                uuid.UUID         `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID            uuid.UUID         `gorm:"type:uuid;not null;index" json:"user_id"`
	InteractionType   AIInteractionType `gorm:"type:varchar(50);not null" json:"interaction_type"`
	MaterialID        *uuid.UUID        `gorm:"type:uuid" json:"material_id,omitempty"`
	PromptTokens      int               `gorm:"default:0" json:"prompt_tokens"`
	CompletionTokens  int               `gorm:"default:0" json:"completion_tokens"`
	TotalTokens       int               `gorm:"default:0" json:"total_tokens"`
	LatencyMs         *int              `json:"latency_ms,omitempty"`
	Success           bool              `gorm:"default:true" json:"success"`
	ErrorMessage      *string           `gorm:"type:text" json:"error_message,omitempty"`
	Model             *string           `gorm:"type:varchar(50)" json:"model,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
}

// TableName specifies the table name for AIInteraction.
func (AIInteraction) TableName() string {
	return "zuri_analytics.ai_interactions"
}

// GoalType represents the type of learning goal.
type GoalType string

const (
	GoalTypeStudyTime        GoalType = "study_time"
	GoalTypeQuizScore        GoalType = "quiz_score"
	GoalTypeStreak           GoalType = "streak"
	GoalTypeCourseCompletion GoalType = "course_completion"
)

// LearningGoal represents a user's learning objective.
type LearningGoal struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	CourseID       *uuid.UUID `gorm:"type:uuid;index" json:"course_id,omitempty"`
	Title          string     `gorm:"type:varchar(255);not null" json:"title"`
	Description    string     `gorm:"type:text" json:"description,omitempty"`
	TargetDate     *time.Time `gorm:"type:date" json:"target_date,omitempty"`
	TargetScore    *int       `json:"target_score,omitempty"`
	CurrentValue   int        `gorm:"default:0" json:"current_value"`
	GoalType       GoalType   `gorm:"type:varchar(50);default:'study_time'" json:"goal_type"`
	IsCompleted    bool       `gorm:"default:false" json:"is_completed"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName specifies the table name for LearningGoal.
func (LearningGoal) TableName() string {
	return "zuri_analytics.learning_goals"
}

// UserStudyStats represents aggregated study statistics for a user.
type UserStudyStats struct {
	UserID                uuid.UUID `json:"user_id"`
	TotalStudyDays        int       `json:"total_study_days"`
	TotalStudyMinutes     int       `json:"total_study_minutes"`
	TotalQuizzesCompleted int       `json:"total_quizzes_completed"`
	TotalMaterialsReviewed int      `json:"total_materials_reviewed"`
	LastStudyDate         *time.Time `json:"last_study_date,omitempty"`
}

// QuizPerformanceSummary represents aggregated quiz performance.
type QuizPerformanceSummary struct {
	UserID          uuid.UUID `json:"user_id"`
	QuizID          uuid.UUID `json:"quiz_id"`
	AttemptCount    int       `json:"attempt_count"`
	AvgPercentage   float64   `json:"avg_percentage"`
	BestPercentage  float64   `json:"best_percentage"`
	BestTimeSeconds *int      `json:"best_time_seconds,omitempty"`
}
