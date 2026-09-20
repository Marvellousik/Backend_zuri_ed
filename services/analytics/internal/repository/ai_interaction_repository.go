package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"zuri/services/analytics/internal/model"
	"zuri/shared/pkg/database"
)

// AIInteractionRepository defines the interface for AI interaction data access.
type AIInteractionRepository interface {
	Create(ctx context.Context, interaction *model.AIInteraction) error
	GetByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.AIInteraction, error)
	GetByUserAndType(ctx context.Context, userID uuid.UUID, interactionType model.AIInteractionType, limit int) ([]model.AIInteraction, error)
	GetUsageStats(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*AIUsageStats, error)
}

// AIUsageStats represents aggregated AI usage statistics.
type AIUsageStats struct {
	TotalInteractions   int64
	TotalTokens         int64
	TotalPromptTokens   int64
	TotalCompletionTokens int64
	SuccessfulCalls     int64
	FailedCalls         int64
}

// aiInteractionRepository implements AIInteractionRepository.
type aiInteractionRepository struct {
	db *database.DB
}

// NewAIInteractionRepository creates a new AI interaction repository.
func NewAIInteractionRepository(db *database.DB) AIInteractionRepository {
	return &aiInteractionRepository{db: db}
}

// Create creates a new AI interaction record.
func (r *aiInteractionRepository) Create(ctx context.Context, interaction *model.AIInteraction) error {
	return r.db.DB.WithContext(ctx).Create(interaction).Error
}

// GetByUserID retrieves AI interactions for a user.
func (r *aiInteractionRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.AIInteraction, error) {
	var interactions []model.AIInteraction
	query := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&interactions).Error
	return interactions, err
}

// GetByUserAndType retrieves AI interactions by type.
func (r *aiInteractionRepository) GetByUserAndType(ctx context.Context, userID uuid.UUID, interactionType model.AIInteractionType, limit int) ([]model.AIInteraction, error) {
	var interactions []model.AIInteraction
	query := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND interaction_type = ?", userID, interactionType).
		Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&interactions).Error
	return interactions, err
}

// GetUsageStats retrieves aggregated usage statistics.
func (r *aiInteractionRepository) GetUsageStats(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*AIUsageStats, error) {
	var stats AIUsageStats
	
	err := r.db.DB.WithContext(ctx).Model(&model.AIInteraction{}).
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, startDate, endDate).
		Select(`
			COUNT(*) as total_interactions,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as total_completion_tokens,
			COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) as successful_calls,
			COALESCE(SUM(CASE WHEN NOT success THEN 1 ELSE 0 END), 0) as failed_calls
		`).
		Scan(&stats).Error
	
	if err != nil {
		return nil, err
	}
	
	return &stats, nil
}
