package repository

import (
	"context"

	"github.com/google/uuid"

	"zuri/services/content/internal/model"
	"zuri/shared/pkg/database"
)

// MaterialRepository defines the interface for material data access.
type MaterialRepository interface {
	Create(ctx context.Context, material *model.Material) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Material, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Material, error)
	GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Material, error)
	Update(ctx context.Context, material *model.Material) error
	UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status model.ProcessingStatus, summary string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// materialRepository implements MaterialRepository.
type materialRepository struct {
	db *database.DB
}

// NewMaterialRepository creates a new material repository.
func NewMaterialRepository(db *database.DB) MaterialRepository {
	return &materialRepository{db: db}
}

// Create creates a new material.
func (r *materialRepository) Create(ctx context.Context, material *model.Material) error {
	return r.db.DB.WithContext(ctx).Create(material).Error
}

// GetByID retrieves a material by ID.
func (r *materialRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Material, error) {
	var material model.Material
	err := r.db.WithContext(ctx).
		Preload("Course").
		First(&material, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &material, nil
}

// GetByUserID retrieves materials for a user with pagination.
func (r *materialRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Material, error) {
	var materials []model.Material
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&materials).Error
	return materials, err
}

// GetByCourseID retrieves materials for a course.
func (r *materialRepository) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Material, error) {
	var materials []model.Material
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Order("created_at DESC").
		Find(&materials).Error
	return materials, err
}

// Update updates a material.
func (r *materialRepository) Update(ctx context.Context, material *model.Material) error {
	return r.db.DB.WithContext(ctx).Save(material).Error
}

// UpdateProcessingStatus updates the processing status and summary.
func (r *materialRepository) UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status model.ProcessingStatus, summary string) error {
	updates := map[string]interface{}{
		"processing_status": status,
	}
	if summary != "" {
		updates["summary"] = summary
	}
	return r.db.DB.WithContext(ctx).
		Model(&model.Material{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Delete soft-deletes a material.
func (r *materialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.Material{}, "id = ?", id).Error
}
