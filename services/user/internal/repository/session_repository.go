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

// SessionRepository defines the interface for session data access.
type SessionRepository interface {
	Create(ctx context.Context, session *model.UserSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.UserSession, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserSession, error)
	GetByRefreshTokenID(ctx context.Context, refreshTokenID uuid.UUID) (*model.UserSession, error)
	UpdateLastActive(ctx context.Context, id uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, exceptSessionID *uuid.UUID) error
}

// sessionRepository implements SessionRepository.
type sessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

// Create creates a new session.
func (r *sessionRepository) Create(ctx context.Context, session *model.UserSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID retrieves a session by ID.
func (r *sessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.UserSession, error) {
	var session model.UserSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session by ID: %w", err)
	}
	return &session, nil
}

// GetByUserID retrieves all sessions for a user.
func (r *sessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserSession, error) {
	var sessions []model.UserSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("last_active_at DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions by user ID: %w", err)
	}
	return sessions, nil
}

// GetByRefreshTokenID retrieves a session by refresh token ID.
func (r *sessionRepository) GetByRefreshTokenID(ctx context.Context, refreshTokenID uuid.UUID) (*model.UserSession, error) {
	var session model.UserSession
	err := r.db.WithContext(ctx).
		Where("refresh_token_id = ?", refreshTokenID).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session by refresh token ID: %w", err)
	}
	return &session, nil
}

// UpdateLastActive updates the last active timestamp.
func (r *sessionRepository) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("id = ?", id).
		Update("last_active_at", time.Now()).Error
}

// Revoke revokes a session.
func (r *sessionRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("id = ?", id).
		Update("revoked_at", time.Now()).Error
}

// RevokeAllForUser revokes all sessions for a user, optionally except one.
func (r *sessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, exceptSessionID *uuid.UUID) error {
	query := r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID)
	
	if exceptSessionID != nil {
		query = query.Where("id != ?", *exceptSessionID)
	}
	
	return query.Update("revoked_at", time.Now()).Error
}
