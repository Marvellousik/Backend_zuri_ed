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

// RefreshTokenRepository defines the interface for refresh token data access.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID, replacedByID *uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context, before time.Time) error
}

// refreshTokenRepository implements RefreshTokenRepository.
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new refresh token repository.
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

// Create creates a new refresh token.
func (r *refreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetByID retrieves a refresh token by ID.
func (r *refreshTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token by ID: %w", err)
	}
	return &token, nil
}

// GetByTokenHash retrieves a refresh token by its hash.
func (r *refreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Preload("User").
		First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token by hash: %w", err)
	}
	return &token, nil
}

// GetByUserID retrieves all refresh tokens for a user.
func (r *refreshTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.RefreshToken, error) {
	var tokens []model.RefreshToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh tokens by user ID: %w", err)
	}
	return tokens, nil
}

// Revoke revokes a refresh token.
func (r *refreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, replacedByID *uuid.UUID) error {
	updates := map[string]interface{}{
		"revoked":     true,
		"revoked_at":  time.Now(),
	}
	if replacedByID != nil {
		updates["replaced_by_token_id"] = *replacedByID
	}
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).Where("id = ?", id).Updates(updates).Error
}

// RevokeAllForUser revokes all refresh tokens for a user.
func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_at": time.Now(),
		}).Error
}

// DeleteExpired deletes expired and revoked tokens older than the specified time.
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? OR (revoked = ? AND revoked_at < ?)", before, true, before).
		Delete(&model.RefreshToken{}).Error
}
