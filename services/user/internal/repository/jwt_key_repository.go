package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"zuri/services/user/internal/model"
)

// JWTKeyRepository defines the interface for JWT key data access.
type JWTKeyRepository interface {
	Create(ctx context.Context, key *model.JWTKey) error
	GetActivePrivateKey(ctx context.Context) (*model.JWTKey, error)
	GetActivePublicKey(ctx context.Context) (*model.JWTKey, error)
	DeactivateAllKeys(ctx context.Context) error
}

// jwtKeyRepository implements JWTKeyRepository.
type jwtKeyRepository struct {
	db *gorm.DB
}

// NewJWTKeyRepository creates a new JWT key repository.
func NewJWTKeyRepository(db *gorm.DB) JWTKeyRepository {
	return &jwtKeyRepository{db: db}
}

// Create creates a new JWT key.
func (r *jwtKeyRepository) Create(ctx context.Context, key *model.JWTKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// GetActivePrivateKey retrieves the active private key.
func (r *jwtKeyRepository) GetActivePrivateKey(ctx context.Context) (*model.JWTKey, error) {
	var key model.JWTKey
	err := r.db.WithContext(ctx).
		Where("key_type = ? AND is_active = ?", "private", true).
		First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active private key: %w", err)
	}
	return &key, nil
}

// GetActivePublicKey retrieves the active public key.
func (r *jwtKeyRepository) GetActivePublicKey(ctx context.Context) (*model.JWTKey, error) {
	var key model.JWTKey
	err := r.db.WithContext(ctx).
		Where("key_type = ? AND is_active = ?", "public", true).
		First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active public key: %w", err)
	}
	return &key, nil
}

// DeactivateAllKeys deactivates all JWT keys.
func (r *jwtKeyRepository) DeactivateAllKeys(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&model.JWTKey{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error
}
