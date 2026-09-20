// Package repository provides data access layer for content entities.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"zuri/services/content/internal/model"
	"zuri/shared/pkg/database"
)

// CourseRepository defines the interface for course data access.
type CourseRepository interface {
	Create(ctx context.Context, course *model.Course) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Course, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Course, error)
	Update(ctx context.Context, course *model.Course) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// courseRepository implements CourseRepository.
type courseRepository struct {
	db *database.DB
}

// NewCourseRepository creates a new course repository.
func NewCourseRepository(db *database.DB) CourseRepository {
	return &courseRepository{db: db}
}

// Create creates a new course.
func (r *courseRepository) Create(ctx context.Context, course *model.Course) error {
	return r.db.DB.WithContext(ctx).Create(course).Error
}

// GetByID retrieves a course by ID with associations.
func (r *courseRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Course, error) {
	var course model.Course
	err := r.db.DB.WithContext(ctx).
		Preload("Materials", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("created_at DESC")
		}).
		Preload("Quizzes", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("created_at DESC")
		}).
		Preload("FlashcardDecks", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("created_at DESC")
		}).
		First(&course, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

// GetByUserID retrieves all courses for a user.
func (r *courseRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Course, error) {
	var courses []model.Course
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&courses).Error
	return courses, err
}

// Update updates a course.
func (r *courseRepository) Update(ctx context.Context, course *model.Course) error {
	return r.db.WithContext(ctx).Save(course).Error
}

// Delete soft-deletes a course.
func (r *courseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Course{}, "id = ?", id).Error
}
