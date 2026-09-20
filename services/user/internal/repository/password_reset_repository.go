package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"zuri/services/user/internal/model"
)

// PasswordResetRepository defines the interface for password reset data access.
type PasswordResetRepository interface {
	Create(ctx context.Context, reset *model.PasswordReset) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.PasswordReset, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context, before time.Time) error
}

// passwordResetRepository implements PasswordResetRepository.
type passwordResetRepository struct {
	db *gorm.DB
}

// NewPasswordResetRepository creates a new password reset repository.
func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

// Create creates a new password reset request.
func (r *passwordResetRepository) Create(ctx context.Context, reset *model.PasswordReset) error {
	return r.db.WithContext(ctx).Create(reset).Error
}

// GetByTokenHash retrieves a password reset by token hash.
func (r *passwordResetRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.PasswordReset, error) {
	var reset model.PasswordReset
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Preload("User").
		First(&reset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get password reset by token hash: %w", err)
	}
	return &reset, nil
}

// MarkAsUsed marks a password reset as used.
func (r *passwordResetRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.PasswordReset{}).Where("id = ?", id).Updates(map[string]interface{}{
		"used":    true,
		"used_at": now,
	}).Error
}

// DeleteExpired deletes expired and unused password reset requests.
func (r *passwordResetRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? OR (used = ? AND used_at < ?)", before, true, before).
		Delete(&model.PasswordReset{}).Error
}
