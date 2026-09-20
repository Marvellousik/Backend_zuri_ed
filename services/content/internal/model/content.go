// Package model contains domain models for the Content Service.
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Course represents a course/study subject and institutional intelligence hub.
type Course struct {
	ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	InstitutionID    string         `gorm:"type:varchar(255);index" json:"institution_id,omitempty"`
	Code             string         `gorm:"type:varchar(50);index" json:"code,omitempty"`
	CourseOfferingID string         `gorm:"type:varchar(255);index" json:"course_offering_id,omitempty"`
	Name             string         `gorm:"type:varchar(255);not null" json:"name"`
	Description      string         `gorm:"type:text" json:"description"`
	Color            string         `gorm:"type:varchar(7);default:'#3B82F6'" json:"color"`
	Semester         string         `gorm:"type:varchar(20)" json:"semester"`
	Year             int            `json:"year"`
	CreditUnits      int            `gorm:"type:int;default:3" json:"credit_units"`
	Syllabus         string         `gorm:"type:text" json:"syllabus,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	Materials      []Material      `gorm:"foreignKey:CourseID" json:"materials,omitempty"`
	Quizzes        []Quiz          `gorm:"foreignKey:CourseID" json:"quizzes,omitempty"`
	FlashcardDecks []FlashcardDeck `gorm:"foreignKey:CourseID" json:"flashcard_decks,omitempty"`
}

// TableName specifies the table name for Course.
func (Course) TableName() string {
	return "zuri_content.courses"
}

// ProcessingStatus represents the material processing status.
type ProcessingStatus string

const (
	ProcessingStatusPending    ProcessingStatus = "pending"
	ProcessingStatusProcessing ProcessingStatus = "processing"
	ProcessingStatusCompleted  ProcessingStatus = "completed"
	ProcessingStatusFailed     ProcessingStatus = "failed"
)

// TranscriptionStatus represents audio transcription status.
type TranscriptionStatus string

const (
	TranscriptionStatusNone       = "none"
	TranscriptionStatusPending    = "pending"
	TranscriptionStatusProcessing = "processing"
	TranscriptionStatusCompleted  = "completed"
	TranscriptionStatusFailed     = "failed"
)

// Material represents a study material/document.
type Material struct {
	ID                  uuid.UUID        `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID              uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	CourseID            *uuid.UUID       `gorm:"type:uuid;index" json:"course_id,omitempty"`
	CourseOfferingID    string           `gorm:"type:varchar(255);index" json:"course_offering_id,omitempty"`
	Title               string           `gorm:"type:varchar(255);not null" json:"title"`
	FileURL             string           `gorm:"type:varchar(500)" json:"file_url,omitempty"`
	FileSize            int64            `json:"file_size,omitempty"`
	MimeType            string           `gorm:"type:varchar(100)" json:"mime_type,omitempty"`
	Sha256Checksum      string           `gorm:"type:varchar(64);index" json:"sha256_checksum,omitempty"`
	Version             int              `gorm:"type:int;default:1" json:"version"`
	TrackingID          string           `gorm:"type:varchar(100);index" json:"tracking_id,omitempty"`
	DurationSeconds     int              `gorm:"type:int" json:"duration_seconds,omitempty"`
	ProcessingStatus    ProcessingStatus `gorm:"type:varchar(20);default:'pending'" json:"processing_status"`
	TranscriptionStatus string           `gorm:"type:varchar(20);default:'none'" json:"transcription_status"`
	TranscriptionText   string           `gorm:"type:text" json:"transcription_text,omitempty"`
	Summary             string           `gorm:"type:text" json:"summary,omitempty"`
	AudioURL            string           `gorm:"type:varchar(500)" json:"audio_url,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
	DeletedAt           gorm.DeletedAt   `gorm:"index" json:"-"`

	// Associations
	Course  *Course         `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Quizzes []Quiz          `gorm:"foreignKey:MaterialID" json:"quizzes,omitempty"`
	Decks   []FlashcardDeck `gorm:"foreignKey:MaterialID" json:"decks,omitempty"`
}

// TableName specifies the table name for Material.
func (Material) TableName() string {
	return "zuri_content.materials"
}

// DifficultyLevel represents quiz/question difficulty.
type DifficultyLevel string

const (
	DifficultyEasy   DifficultyLevel = "easy"
	DifficultyMedium DifficultyLevel = "medium"
	DifficultyHard   DifficultyLevel = "hard"
)

// Quiz represents a quiz/test.
type Quiz struct {
	ID               uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID           uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	CourseID         *uuid.UUID      `gorm:"type:uuid;index" json:"course_id,omitempty"`
	MaterialID       *uuid.UUID      `gorm:"type:uuid;index" json:"material_id,omitempty"`
	Title            string          `gorm:"type:varchar(255);not null" json:"title"`
	Description      string          `gorm:"type:text" json:"description,omitempty"`
	TimeLimitMinutes int             `json:"time_limit_minutes,omitempty"`
	Difficulty       DifficultyLevel `gorm:"type:varchar(20)" json:"difficulty,omitempty"`
	ShuffleQuestions bool            `gorm:"default:false" json:"shuffle_questions"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"-"`

	// Associations
	Course    *Course        `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Material  *Material      `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
	Questions []QuizQuestion `gorm:"foreignKey:QuizID;order:order_index" json:"questions,omitempty"`
}

// TableName specifies the table name for Quiz.
func (Quiz) TableName() string {
	return "zuri_content.quizzes"
}

// QuestionType represents the type of quiz question.
type QuestionType string

const (
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
	QuestionTypeShortAnswer    QuestionType = "short_answer"
	QuestionTypeTrueFalse      QuestionType = "true_false"
)

// QuizOption represents a multiple choice option.
type QuizOption struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	IsCorrect  bool   `json:"is_correct"`
	OrderIndex int    `json:"order_index"`
}

// QuizOptions is a slice of QuizOption for JSONB storage.
type QuizOptions []QuizOption

// Value implements the driver.Valuer interface.
func (o QuizOptions) Value() (driver.Value, error) {
	if o == nil {
		return nil, nil
	}
	return json.Marshal(o)
}

// Scan implements the sql.Scanner interface.
func (o *QuizOptions) Scan(value interface{}) error {
	if value == nil {
		*o = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, o)
}

// QuizQuestion represents a single question in a quiz.
type QuizQuestion struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	QuizID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"quiz_id"`
	QuestionText  string          `gorm:"type:text;not null" json:"question_text"`
	QuestionType  QuestionType    `gorm:"type:varchar(20)" json:"question_type"`
	Options       QuizOptions     `gorm:"type:jsonb" json:"options,omitempty"`
	CorrectAnswer string          `gorm:"type:text" json:"correct_answer,omitempty"`
	Explanation   string          `gorm:"type:text" json:"explanation,omitempty"`
	Points        int             `gorm:"default:1" json:"points"`
	OrderIndex    int             `gorm:"index:idx_quiz_questions_order,priority:2" json:"order_index"`
	Difficulty    DifficultyLevel `gorm:"type:varchar(20)" json:"difficulty,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`

	// Associations
	Quiz *Quiz `gorm:"foreignKey:QuizID" json:"-"`
}

// TableName specifies the table name for QuizQuestion.
func (QuizQuestion) TableName() string {
	return "zuri_content.quiz_questions"
}

// FlashcardDeck represents a deck of flashcards.
type FlashcardDeck struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	CourseID    *uuid.UUID     `gorm:"type:uuid;index" json:"course_id,omitempty"`
	MaterialID  *uuid.UUID     `gorm:"type:uuid;index" json:"material_id,omitempty"`
	Title       string         `gorm:"type:varchar(255);not null" json:"title"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	Course     *Course     `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Material   *Material   `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
	Flashcards []Flashcard `gorm:"foreignKey:DeckID;order:order_index" json:"flashcards,omitempty"`
}

// TableName specifies the table name for FlashcardDeck.
func (FlashcardDeck) TableName() string {
	return "zuri_content.flashcard_decks"
}

// Flashcard represents a single flashcard.
type Flashcard struct {
	ID         uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	DeckID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"deck_id"`
	FrontText  string          `gorm:"type:text;not null" json:"front_text"`
	BackText   string          `gorm:"type:text;not null" json:"back_text"`
	Difficulty DifficultyLevel `gorm:"type:varchar(20)" json:"difficulty,omitempty"`
	OrderIndex int             `gorm:"index:idx_flashcards_order,priority:2" json:"order_index"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`

	// Associations
	Deck *FlashcardDeck `gorm:"foreignKey:DeckID" json:"-"`
}

// TableName specifies the table name for Flashcard.
func (Flashcard) TableName() string {
	return "zuri_content.flashcards"
}
