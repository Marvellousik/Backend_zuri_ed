// Package repository provides data access layer for the User Service.
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

// UserRepository defines the interface for user data access.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	SetVerificationCode(ctx context.Context, userID uuid.UUID, code string, expiresAt time.Time) error
	VerifyEmail(ctx context.Context, userID uuid.UUID) error
	UpdateProfile(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error
}

// userRepository implements UserRepository.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user.
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID retrieves a user by ID.
func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// Update updates a user.
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete permanently deletes a user.
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.User{}, id).Error
}

// SoftDelete soft deletes a user.
func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

// ExistsByEmail checks if a user with the given email exists.
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return count > 0, nil
}

// UpdatePassword updates a user's password.
func (r *userRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error
}

// SetVerificationCode sets the email verification code for a user.
func (r *userRepository) SetVerificationCode(ctx context.Context, userID uuid.UUID, code string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"verification_code":            code,
		"verification_code_expires_at": expiresAt,
	}).Error
}

// VerifyEmail marks a user's email as verified.
func (r *userRepository) VerifyEmail(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"email_verified":               true,
		"verification_code":            nil,
		"verification_code_expires_at": nil,
	}).Error
}

// UpdateProfile updates specific user profile fields.
func (r *userRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	// Remove sensitive fields that shouldn't be updated directly
	delete(updates, "id")
	delete(updates, "password_hash")
	delete(updates, "email")
	delete(updates, "created_at")

	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error
}
