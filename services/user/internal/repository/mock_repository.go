// Package repository provides mock implementations for testing.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"zuri/services/user/internal/model"
)

// MockUserRepository is a mock implementation of UserRepository.
type MockUserRepository struct {
	CreateFunc               func(ctx context.Context, user *model.User) error
	GetByIDFunc              func(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmailFunc           func(ctx context.Context, email string) (*model.User, error)
	UpdateFunc               func(ctx context.Context, user *model.User) error
	DeleteFunc               func(ctx context.Context, id uuid.UUID) error
	SoftDeleteFunc           func(ctx context.Context, id uuid.UUID) error
	ExistsByEmailFunc        func(ctx context.Context, email string) (bool, error)
	UpdatePasswordFunc       func(ctx context.Context, userID uuid.UUID, passwordHash string) error
	SetVerificationCodeFunc  func(ctx context.Context, userID uuid.UUID, code string, expiresAt time.Time) error
	VerifyEmailFunc          func(ctx context.Context, userID uuid.UUID) error
	UpdateProfileFunc        func(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, user)
	}
	return nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *MockUserRepository) Update(ctx context.Context, user *model.User) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, user)
	}
	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if m.ExistsByEmailFunc != nil {
		return m.ExistsByEmailFunc(ctx, email)
	}
	return false, nil
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, userID, passwordHash)
	}
	return nil
}

func (m *MockUserRepository) SetVerificationCode(ctx context.Context, userID uuid.UUID, code string, expiresAt time.Time) error {
	if m.SetVerificationCodeFunc != nil {
		return m.SetVerificationCodeFunc(ctx, userID, code, expiresAt)
	}
	return nil
}

func (m *MockUserRepository) VerifyEmail(ctx context.Context, userID uuid.UUID) error {
	if m.VerifyEmailFunc != nil {
		return m.VerifyEmailFunc(ctx, userID)
	}
	return nil
}

func (m *MockUserRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	if m.UpdateProfileFunc != nil {
		return m.UpdateProfileFunc(ctx, userID, updates)
	}
	return nil
}

// MockRefreshTokenRepository is a mock implementation of RefreshTokenRepository.
type MockRefreshTokenRepository struct {
	CreateFunc          func(ctx context.Context, token *model.RefreshToken) error
	GetByIDFunc         func(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error)
	GetByTokenHashFunc  func(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	GetByUserIDFunc     func(ctx context.Context, userID uuid.UUID) ([]model.RefreshToken, error)
	RevokeFunc          func(ctx context.Context, id uuid.UUID, replacedByID *uuid.UUID) error
	RevokeAllForUserFunc func(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredFunc   func(ctx context.Context, before time.Time) error
}

func (m *MockRefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, token)
	}
	return nil
}

func (m *MockRefreshTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockRefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	if m.GetByTokenHashFunc != nil {
		return m.GetByTokenHashFunc(ctx, tokenHash)
	}
	return nil, nil
}

func (m *MockRefreshTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.RefreshToken, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockRefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, replacedByID *uuid.UUID) error {
	if m.RevokeFunc != nil {
		return m.RevokeFunc(ctx, id, replacedByID)
	}
	return nil
}

func (m *MockRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	if m.RevokeAllForUserFunc != nil {
		return m.RevokeAllForUserFunc(ctx, userID)
	}
	return nil
}

func (m *MockRefreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	if m.DeleteExpiredFunc != nil {
		return m.DeleteExpiredFunc(ctx, before)
	}
	return nil
}

// MockSessionRepository is a mock implementation of SessionRepository.
type MockSessionRepository struct {
	CreateFunc              func(ctx context.Context, session *model.UserSession) error
	GetByIDFunc             func(ctx context.Context, id uuid.UUID) (*model.UserSession, error)
	GetByUserIDFunc         func(ctx context.Context, userID uuid.UUID) ([]model.UserSession, error)
	GetByRefreshTokenIDFunc func(ctx context.Context, refreshTokenID uuid.UUID) (*model.UserSession, error)
	UpdateLastActiveFunc    func(ctx context.Context, id uuid.UUID) error
	RevokeFunc              func(ctx context.Context, id uuid.UUID) error
	RevokeAllForUserFunc    func(ctx context.Context, userID uuid.UUID, exceptSessionID *uuid.UUID) error
}

func (m *MockSessionRepository) Create(ctx context.Context, session *model.UserSession) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, session)
	}
	return nil
}

func (m *MockSessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.UserSession, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockSessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserSession, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockSessionRepository) GetByRefreshTokenID(ctx context.Context, refreshTokenID uuid.UUID) (*model.UserSession, error) {
	if m.GetByRefreshTokenIDFunc != nil {
		return m.GetByRefreshTokenIDFunc(ctx, refreshTokenID)
	}
	return nil, nil
}

func (m *MockSessionRepository) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	if m.UpdateLastActiveFunc != nil {
		return m.UpdateLastActiveFunc(ctx, id)
	}
	return nil
}

func (m *MockSessionRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	if m.RevokeFunc != nil {
		return m.RevokeFunc(ctx, id)
	}
	return nil
}

func (m *MockSessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, exceptSessionID *uuid.UUID) error {
	if m.RevokeAllForUserFunc != nil {
		return m.RevokeAllForUserFunc(ctx, userID, exceptSessionID)
	}
	return nil
}

// MockPasswordResetRepository is a mock implementation of PasswordResetRepository.
type MockPasswordResetRepository struct {
	CreateFunc       func(ctx context.Context, reset *model.PasswordReset) error
	GetByTokenHashFunc func(ctx context.Context, tokenHash string) (*model.PasswordReset, error)
	MarkAsUsedFunc   func(ctx context.Context, id uuid.UUID) error
	DeleteExpiredFunc func(ctx context.Context, before time.Time) error
}

func (m *MockPasswordResetRepository) Create(ctx context.Context, reset *model.PasswordReset) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, reset)
	}
	return nil
}

func (m *MockPasswordResetRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.PasswordReset, error) {
	if m.GetByTokenHashFunc != nil {
		return m.GetByTokenHashFunc(ctx, tokenHash)
	}
	return nil, nil
}

func (m *MockPasswordResetRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	if m.MarkAsUsedFunc != nil {
		return m.MarkAsUsedFunc(ctx, id)
	}
	return nil
}

func (m *MockPasswordResetRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	if m.DeleteExpiredFunc != nil {
		return m.DeleteExpiredFunc(ctx, before)
	}
	return nil
}

// MockJWTKeyRepository is a mock implementation of JWTKeyRepository.
type MockJWTKeyRepository struct {
	CreateFunc           func(ctx context.Context, key *model.JWTKey) error
	GetActivePrivateKeyFunc func(ctx context.Context) (*model.JWTKey, error)
	GetActivePublicKeyFunc  func(ctx context.Context) (*model.JWTKey, error)
	DeactivateAllKeysFunc   func(ctx context.Context) error
}

func (m *MockJWTKeyRepository) Create(ctx context.Context, key *model.JWTKey) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, key)
	}
	return nil
}

func (m *MockJWTKeyRepository) GetActivePrivateKey(ctx context.Context) (*model.JWTKey, error) {
	if m.GetActivePrivateKeyFunc != nil {
		return m.GetActivePrivateKeyFunc(ctx)
	}
	return nil, nil
}

func (m *MockJWTKeyRepository) GetActivePublicKey(ctx context.Context) (*model.JWTKey, error) {
	if m.GetActivePublicKeyFunc != nil {
		return m.GetActivePublicKeyFunc(ctx)
	}
	return nil, nil
}

func (m *MockJWTKeyRepository) DeactivateAllKeys(ctx context.Context) error {
	if m.DeactivateAllKeysFunc != nil {
		return m.DeactivateAllKeysFunc(ctx)
	}
	return nil
}

// MockMembershipRepository is a mock implementation of MembershipRepository.
type MockMembershipRepository struct {
	CreateFunc                  func(ctx context.Context, membership *model.InstitutionMembership) error
	GetByIDFunc                 func(ctx context.Context, id uuid.UUID) (*model.InstitutionMembership, error)
	GetByUserAndInstitutionFunc func(ctx context.Context, userID uuid.UUID, institutionID string) (*model.InstitutionMembership, error)
	ListByUserIDFunc            func(ctx context.Context, userID uuid.UUID) ([]model.InstitutionMembership, error)
	ListByInstitutionIDFunc     func(ctx context.Context, institutionID string) ([]model.InstitutionMembership, error)
	UpdateFunc                  func(ctx context.Context, membership *model.InstitutionMembership) error
	SetDefaultFunc              func(ctx context.Context, userID uuid.UUID, membershipID uuid.UUID) error
	DeleteFunc                  func(ctx context.Context, id uuid.UUID) error
}

func (m *MockMembershipRepository) Create(ctx context.Context, membership *model.InstitutionMembership) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, membership)
	}
	return nil
}

func (m *MockMembershipRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.InstitutionMembership, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockMembershipRepository) GetByUserAndInstitution(ctx context.Context, userID uuid.UUID, institutionID string) (*model.InstitutionMembership, error) {
	if m.GetByUserAndInstitutionFunc != nil {
		return m.GetByUserAndInstitutionFunc(ctx, userID, institutionID)
	}
	return nil, nil
}

func (m *MockMembershipRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.InstitutionMembership, error) {
	if m.ListByUserIDFunc != nil {
		return m.ListByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockMembershipRepository) ListByInstitutionID(ctx context.Context, institutionID string) ([]model.InstitutionMembership, error) {
	if m.ListByInstitutionIDFunc != nil {
		return m.ListByInstitutionIDFunc(ctx, institutionID)
	}
	return nil, nil
}

func (m *MockMembershipRepository) Update(ctx context.Context, membership *model.InstitutionMembership) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, membership)
	}
	return nil
}

func (m *MockMembershipRepository) SetDefault(ctx context.Context, userID uuid.UUID, membershipID uuid.UUID) error {
	if m.SetDefaultFunc != nil {
		return m.SetDefaultFunc(ctx, userID, membershipID)
	}
	return nil
}

func (m *MockMembershipRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

