// Package repository provides data access layer for the User Service.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"zuri/services/user/internal/model"
)

// MembershipRepository defines the interface for institution membership data access.
type MembershipRepository interface {
	Create(ctx context.Context, membership *model.InstitutionMembership) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.InstitutionMembership, error)
	GetByUserAndInstitution(ctx context.Context, userID uuid.UUID, institutionID string) (*model.InstitutionMembership, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.InstitutionMembership, error)
	ListByInstitutionID(ctx context.Context, institutionID string) ([]model.InstitutionMembership, error)
	Update(ctx context.Context, membership *model.InstitutionMembership) error
	SetDefault(ctx context.Context, userID uuid.UUID, membershipID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type membershipRepository struct {
	db *gorm.DB
}

// NewMembershipRepository creates a new membership repository.
func NewMembershipRepository(db *gorm.DB) MembershipRepository {
	return &membershipRepository{db: db}
}

// Create persists a new institution membership.
func (r *membershipRepository) Create(ctx context.Context, membership *model.InstitutionMembership) error {
	return r.db.WithContext(ctx).Create(membership).Error
}

// GetByID retrieves a membership by primary ID.
func (r *membershipRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.InstitutionMembership, error) {
	var m model.InstitutionMembership
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get membership by ID: %w", err)
	}
	return &m, nil
}

// GetByUserAndInstitution retrieves a membership for a specific user and institution.
func (r *membershipRepository) GetByUserAndInstitution(ctx context.Context, userID uuid.UUID, institutionID string) (*model.InstitutionMembership, error) {
	var m model.InstitutionMembership
	err := r.db.WithContext(ctx).Where("user_id = ? AND institution_id = ?", userID, institutionID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get membership by user and institution: %w", err)
	}
	return &m, nil
}

// ListByUserID retrieves all memberships for a given user.
func (r *membershipRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.InstitutionMembership, error) {
	var memberships []model.InstitutionMembership
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships by user ID: %w", err)
	}
	return memberships, nil
}

// ListByInstitutionID retrieves all memberships within an institution.
func (r *membershipRepository) ListByInstitutionID(ctx context.Context, institutionID string) ([]model.InstitutionMembership, error) {
	var memberships []model.InstitutionMembership
	err := r.db.WithContext(ctx).Where("institution_id = ?", institutionID).Order("created_at ASC").Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships by institution ID: %w", err)
	}
	return memberships, nil
}

// Update updates an existing membership.
func (r *membershipRepository) Update(ctx context.Context, membership *model.InstitutionMembership) error {
	return r.db.WithContext(ctx).Save(membership).Error
}

// SetDefault marks one membership as default for a user, unmarking others.
func (r *membershipRepository) SetDefault(ctx context.Context, userID uuid.UUID, membershipID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.InstitutionMembership{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.InstitutionMembership{}).
			Where("user_id = ? AND id = ?", userID, membershipID).
			Update("is_default", true).Error
	})
}

// Delete removes a membership.
func (r *membershipRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.InstitutionMembership{}, id).Error
}
