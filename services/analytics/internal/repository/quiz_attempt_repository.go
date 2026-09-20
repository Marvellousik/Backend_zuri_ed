// Package repository provides data access layer for analytics entities.
package repository

import (
	"context"

	"github.com/google/uuid"

	"zuri/services/analytics/internal/model"
	"zuri/shared/pkg/database"
)

// QuizAttemptRepository defines the interface for quiz attempt data access.
type QuizAttemptRepository interface {
	Create(ctx context.Context, attempt *model.QuizAttempt) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.QuizAttempt, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.QuizAttempt, error)
	GetByQuizID(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) ([]model.QuizAttempt, error)
	Update(ctx context.Context, attempt *model.QuizAttempt) error
	GetAttemptCount(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) (int64, error)
	GetBestAttempt(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) (*model.QuizAttempt, error)
	
	// Answer methods
	CreateAnswer(ctx context.Context, answer *model.QuizAnswer) error
	GetAnswersByAttemptID(ctx context.Context, attemptID uuid.UUID) ([]model.QuizAnswer, error)
}

// quizAttemptRepository implements QuizAttemptRepository.
type quizAttemptRepository struct {
	db *database.DB
}

// NewQuizAttemptRepository creates a new quiz attempt repository.
func NewQuizAttemptRepository(db *database.DB) QuizAttemptRepository {
	return &quizAttemptRepository{db: db}
}

// Create creates a new quiz attempt.
func (r *quizAttemptRepository) Create(ctx context.Context, attempt *model.QuizAttempt) error {
	return r.db.DB.WithContext(ctx).Create(attempt).Error
}

// GetByID retrieves a quiz attempt by ID with answers.
func (r *quizAttemptRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.QuizAttempt, error) {
	var attempt model.QuizAttempt
	err := r.db.DB.WithContext(ctx).
		Preload("Answers").
		First(&attempt, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// GetByUserID retrieves quiz attempts for a user.
func (r *quizAttemptRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.QuizAttempt, error) {
	var attempts []model.QuizAttempt
	query := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	
	err := query.Find(&attempts).Error
	return attempts, err
}

// GetByQuizID retrieves attempts for a specific quiz.
func (r *quizAttemptRepository) GetByQuizID(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) ([]model.QuizAttempt, error) {
	var attempts []model.QuizAttempt
	err := r.db.DB.WithContext(ctx).
		Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Order("created_at DESC").
		Find(&attempts).Error
	return attempts, err
}

// Update updates a quiz attempt.
func (r *quizAttemptRepository) Update(ctx context.Context, attempt *model.QuizAttempt) error {
	return r.db.DB.WithContext(ctx).Save(attempt).Error
}

// GetAttemptCount returns the number of attempts for a quiz.
func (r *quizAttemptRepository) GetAttemptCount(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.DB.WithContext(ctx).
		Model(&model.QuizAttempt{}).
		Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Count(&count).Error
	return count, err
}

// GetBestAttempt returns the best attempt for a quiz (highest percentage).
func (r *quizAttemptRepository) GetBestAttempt(ctx context.Context, quizID uuid.UUID, userID uuid.UUID) (*model.QuizAttempt, error) {
	var attempt model.QuizAttempt
	err := r.db.DB.WithContext(ctx).
		Where("quiz_id = ? AND user_id = ? AND status = ?", quizID, userID, model.AttemptStatusCompleted).
		Order("percentage DESC").
		First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// CreateAnswer creates a new quiz answer.
func (r *quizAttemptRepository) CreateAnswer(ctx context.Context, answer *model.QuizAnswer) error {
	return r.db.DB.WithContext(ctx).Create(answer).Error
}

// GetAnswersByAttemptID retrieves answers for an attempt.
func (r *quizAttemptRepository) GetAnswersByAttemptID(ctx context.Context, attemptID uuid.UUID) ([]model.QuizAnswer, error) {
	var answers []model.QuizAnswer
	err := r.db.DB.WithContext(ctx).
		Where("attempt_id = ?", attemptID).
		Find(&answers).Error
	return answers, err
}
