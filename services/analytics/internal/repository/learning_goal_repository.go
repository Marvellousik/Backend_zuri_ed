package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"zuri/services/analytics/internal/model"
	"zuri/shared/pkg/database"
)

// LearningGoalRepository defines the interface for learning goal data access.
type LearningGoalRepository interface {
	Create(ctx context.Context, goal *model.LearningGoal) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.LearningGoal, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, includeCompleted bool) ([]model.LearningGoal, error)
	Update(ctx context.Context, goal *model.LearningGoal) error
	MarkComplete(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// learningGoalRepository implements LearningGoalRepository.
type learningGoalRepository struct {
	db *database.DB
}

// NewLearningGoalRepository creates a new learning goal repository.
func NewLearningGoalRepository(db *database.DB) LearningGoalRepository {
	return &learningGoalRepository{db: db}
}

// Create creates a new learning goal.
func (r *learningGoalRepository) Create(ctx context.Context, goal *model.LearningGoal) error {
	return r.db.DB.WithContext(ctx).Create(goal).Error
}

// GetByID retrieves a learning goal by ID.
func (r *learningGoalRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.LearningGoal, error) {
	var goal model.LearningGoal
	err := r.db.DB.WithContext(ctx).First(&goal, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// GetByUserID retrieves learning goals for a user.
func (r *learningGoalRepository) GetByUserID(ctx context.Context, userID uuid.UUID, includeCompleted bool) ([]model.LearningGoal, error) {
	var goals []model.LearningGoal
	query := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID)
	
	if !includeCompleted {
		query = query.Where("is_completed = ?", false)
	}
	
	err := query.Order("created_at DESC").Find(&goals).Error
	return goals, err
}

// Update updates a learning goal.
func (r *learningGoalRepository) Update(ctx context.Context, goal *model.LearningGoal) error {
	return r.db.DB.WithContext(ctx).Save(goal).Error
}

// MarkComplete marks a goal as completed.
func (r *learningGoalRepository) MarkComplete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.DB.WithContext(ctx).
		Model(&model.LearningGoal{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_completed": true,
			"completed_at": now,
		}).Error
}

// Delete deletes a learning goal.
func (r *learningGoalRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.LearningGoal{}, "id = ?", id).Error
}
