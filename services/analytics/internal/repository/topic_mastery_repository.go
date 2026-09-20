package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"zuri/services/analytics/internal/model"
	"zuri/shared/pkg/database"
)

// TopicMasteryRepository defines the interface for topic mastery data access.
type TopicMasteryRepository interface {
	CreateOrUpdate(ctx context.Context, mastery *model.TopicMastery) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TopicMastery, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error)
	GetByUserAndCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.TopicMastery, error)
	GetByUserAndTopic(ctx context.Context, userID uuid.UUID, topic string) (*model.TopicMastery, error)
	GetTopicsForReview(ctx context.Context, userID uuid.UUID, before time.Time) ([]model.TopicMastery, error)
	UpdateMastery(ctx context.Context, userID uuid.UUID, courseID *uuid.UUID, topic string, isCorrect bool) error
}

// topicMasteryRepository implements TopicMasteryRepository.
type topicMasteryRepository struct {
	db *database.DB
}

// NewTopicMasteryRepository creates a new topic mastery repository.
func NewTopicMasteryRepository(db *database.DB) TopicMasteryRepository {
	return &topicMasteryRepository{db: db}
}

// CreateOrUpdate creates or updates a topic mastery record.
func (r *topicMasteryRepository) CreateOrUpdate(ctx context.Context, mastery *model.TopicMastery) error {
	var existing model.TopicMastery
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND topic = ? AND (course_id = ? OR (course_id IS NULL AND ? IS NULL))",
			mastery.UserID, mastery.Topic, mastery.CourseID, mastery.CourseID).
		First(&existing).Error
	
	if err == nil {
		// Update existing
		existing.MasteryScore = mastery.MasteryScore
		existing.QuestionsAttempted = mastery.QuestionsAttempted
		existing.QuestionsCorrect = mastery.QuestionsCorrect
		existing.CurrentIntervalDays = mastery.CurrentIntervalDays
		existing.LastStudiedAt = mastery.LastStudiedAt
		existing.NextReviewAt = mastery.NextReviewAt
		return r.db.DB.WithContext(ctx).Save(&existing).Error
	}
	
	// Create new
	return r.db.DB.WithContext(ctx).Create(mastery).Error
}

// GetByID retrieves a topic mastery by ID.
func (r *topicMasteryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TopicMastery, error) {
	var mastery model.TopicMastery
	err := r.db.DB.WithContext(ctx).First(&mastery, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &mastery, nil
}

// GetByUserID retrieves all topic mastery records for a user.
func (r *topicMasteryRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error) {
	var masteryList []model.TopicMastery
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("mastery_score DESC").
		Find(&masteryList).Error
	return masteryList, err
}

// GetByUserAndCourse retrieves topic mastery for a user in a course.
func (r *topicMasteryRepository) GetByUserAndCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.TopicMastery, error) {
	var masteryList []model.TopicMastery
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND course_id = ?", userID, courseID).
		Order("mastery_score DESC").
		Find(&masteryList).Error
	return masteryList, err
}

// GetByUserAndTopic retrieves topic mastery for a specific topic.
func (r *topicMasteryRepository) GetByUserAndTopic(ctx context.Context, userID uuid.UUID, topic string) (*model.TopicMastery, error) {
	var mastery model.TopicMastery
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND topic = ?", userID, topic).
		First(&mastery).Error
	if err != nil {
		return nil, err
	}
	return &mastery, nil
}

// GetTopicsForReview retrieves topics due for review.
func (r *topicMasteryRepository) GetTopicsForReview(ctx context.Context, userID uuid.UUID, before time.Time) ([]model.TopicMastery, error) {
	var masteryList []model.TopicMastery
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND (next_review_at IS NULL OR next_review_at <= ?)", userID, before).
		Order("mastery_score ASC").
		Find(&masteryList).Error
	return masteryList, err
}

// UpdateMastery updates mastery score based on a question result.
func (r *topicMasteryRepository) UpdateMastery(ctx context.Context, userID uuid.UUID, courseID *uuid.UUID, topic string, isCorrect bool) error {
	var mastery model.TopicMastery
	
	// Try to find existing record
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND topic = ? AND (course_id = ? OR (course_id IS NULL AND ? IS NULL))",
			userID, topic, courseID, courseID).
		First(&mastery).Error
	
	now := time.Now()
	
	if err != nil {
		// Create new record
		mastery = model.TopicMastery{
			UserID:             userID,
			CourseID:           courseID,
			Topic:              topic,
			QuestionsAttempted: 1,
			QuestionsCorrect:   0,
			MasteryScore:       0,
			CurrentIntervalDays: 0,
			LastStudiedAt:      &now,
		}
		if isCorrect {
			mastery.QuestionsCorrect = 1
			mastery.MasteryScore = 100
			mastery.CurrentIntervalDays = 1
		}
		nextReview := now.AddDate(0, 0, mastery.CurrentIntervalDays)
		mastery.NextReviewAt = &nextReview
		return r.db.DB.WithContext(ctx).Create(&mastery).Error
	}
	
	// Update existing record
	mastery.QuestionsAttempted++
	if isCorrect {
		mastery.QuestionsCorrect++
	}
	
	// Calculate new mastery score (simple percentage for now)
	if mastery.QuestionsAttempted > 0 {
		mastery.MasteryScore = float64(mastery.QuestionsCorrect) / float64(mastery.QuestionsAttempted) * 100
	}
	
	// Update spaced repetition interval
	if isCorrect {
		if mastery.CurrentIntervalDays == 0 {
			mastery.CurrentIntervalDays = 1
		} else if mastery.CurrentIntervalDays == 1 {
			mastery.CurrentIntervalDays = 3
		} else {
			mastery.CurrentIntervalDays = int(float64(mastery.CurrentIntervalDays) * 1.5)
			if mastery.CurrentIntervalDays > 30 {
				mastery.CurrentIntervalDays = 30
			}
		}
	} else {
		// Reset interval on failure
		mastery.CurrentIntervalDays = 1
	}
	
	mastery.LastStudiedAt = &now
	nextReview := now.AddDate(0, 0, mastery.CurrentIntervalDays)
	mastery.NextReviewAt = &nextReview
	
	return r.db.DB.WithContext(ctx).Save(&mastery).Error
}
