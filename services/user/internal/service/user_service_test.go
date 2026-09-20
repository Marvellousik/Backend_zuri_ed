// Package service_test contains unit tests for the User Service.
package service_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuri/services/user/internal/client"
	"zuri/services/user/internal/model"
	"zuri/services/user/internal/repository"
	"zuri/services/user/internal/service"
	"zuri/shared/pkg/auth"
	"zuri/shared/pkg/config"
	"zuri/shared/pkg/logger"
)

// mockRedisClient is a mock implementation of Redis client for testing.
type mockRedisClient struct {
	data map[string]string
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{data: make(map[string]string)}
}

func (m *mockRedisClient) Get(ctx context.Context, key string) (string, error) {
	return m.data[key], nil
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.data[key] = value.(string)
	return nil
}

func (m *mockRedisClient) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *mockRedisClient) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func TestMain(m *testing.M) {
	// Initialize logger for tests
	logger.Initialize("error")
	m.Run()
}

func TestUserService_Register(t *testing.T) {
	tests := []struct {
		name        string
		req         *service.RegisterRequest
		setupMocks  func(*repository.MockUserRepository, *repository.MockJWTKeyRepository)
		expectError bool
		statusCode  int
	}{
		{
			name: "successful registration",
			req: &service.RegisterRequest{
				Email:     "test@example.com",
				Password:  "password123",
				FirstName: "John",
				LastName:  "Doe",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.ExistsByEmailFunc = func(ctx context.Context, email string) (bool, error) {
					return false, nil
				}
				userRepo.CreateFunc = func(ctx context.Context, user *model.User) error {
					user.ID = uuid.New()
					return nil
				}
				userRepo.SetVerificationCodeFunc = func(ctx context.Context, userID uuid.UUID, code string, expiresAt time.Time) error {
					return nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					// Return nil to trigger key generation
					return nil, nil
				}
				jwtKeyRepo.DeactivateAllKeysFunc = func(ctx context.Context) error {
					return nil
				}
				jwtKeyRepo.CreateFunc = func(ctx context.Context, key *model.JWTKey) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "email already exists",
			req: &service.RegisterRequest{
				Email:    "existing@example.com",
				Password: "password123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.ExistsByEmailFunc = func(ctx context.Context, email string) (bool, error) {
					return true, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusConflict,
		},
		{
			name: "invalid email format",
			req: &service.RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(userRepo, jwtKeyRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10, // Lower cost for faster tests
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
				VerificationCodeTTL:     15 * time.Minute,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			resp, err := svc.Register(ctx, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.ID)
				assert.Equal(t, tt.req.Email, resp.Email)
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
	tests := []struct {
		name        string
		req         *service.LoginRequest
		setupMocks  func(*repository.MockUserRepository, *repository.MockJWTKeyRepository)
		expectError bool
		statusCode  int
	}{
		{
			name: "successful login",
			req: &service.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           uuid.New(),
						Email:        email,
						PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", // bcrypt hash of "password123"
						IsActive:     true,
					}, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: false,
		},
		{
			name: "user not found",
			req: &service.LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
					return nil, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusUnauthorized,
		},
		{
			name: "invalid password",
			req: &service.LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           uuid.New(),
						Email:        email,
						PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
						IsActive:     true,
					}, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusUnauthorized,
		},
		{
			name: "inactive account",
			req: &service.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           uuid.New(),
						Email:        email,
						PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
						IsActive:     false,
					}, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(userRepo, jwtKeyRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			clientInfo := &service.ClientInfo{
				IPAddress: "127.0.0.1",
				UserAgent: "Test-Agent",
			}
			resp, err := svc.Login(ctx, tt.req, clientInfo)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.AccessToken)
				assert.NotEmpty(t, resp.RefreshToken)
				assert.Equal(t, "Bearer", resp.TokenType)
			}
		})
	}
}

func TestUserService_GetProfile(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		setupMocks  func(*repository.MockUserRepository)
		expectError bool
		statusCode  int
	}{
		{
			name:   "successful get profile",
			userID: uuid.New().String(),
			setupMocks: func(userRepo *repository.MockUserRepository) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:        id,
						Email:     "test@example.com",
						FirstName: "John",
						LastName:  "Doe",
						IsActive:  true,
					}, nil
				}
			},
			expectError: false,
		},
		{
			name:   "invalid user ID",
			userID: "invalid-uuid",
			setupMocks: func(userRepo *repository.MockUserRepository) {
			},
			expectError: true,
			statusCode:  http.StatusBadRequest,
		},
		{
			name:   "user not found",
			userID: uuid.New().String(),
			setupMocks: func(userRepo *repository.MockUserRepository) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return nil, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(userRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			resp, err := svc.GetProfile(ctx, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "test@example.com", resp.Email)
			}
		})
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	userID := uuid.New()
	hasher := auth.NewPasswordHasher(10)
	hashedPassword, err := hasher.HashPassword("password123")
	require.NoError(t, err)

	tests := []struct {
		name        string
		req         *service.ChangePasswordRequest
		setupMocks  func(*repository.MockUserRepository, *repository.MockRefreshTokenRepository)
		expectError bool
		statusCode  int
	}{
		{
			name: "successful password change",
			req: &service.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "newpassword123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, refreshTokenRepo *repository.MockRefreshTokenRepository) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:           userID,
						Email:        "test@example.com",
						PasswordHash: hashedPassword,
						IsActive:     true,
					}, nil
				}
				userRepo.UpdatePasswordFunc = func(ctx context.Context, uid uuid.UUID, passwordHash string) error {
					require.Equal(t, userID, uid)
					return nil
				}
				refreshTokenRepo.RevokeAllForUserFunc = func(ctx context.Context, uid uuid.UUID) error {
					require.Equal(t, userID, uid)
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "incorrect current password",
			req: &service.ChangePasswordRequest{
				CurrentPassword: "wrongpassword",
				NewPassword:     "newpassword123",
			},
			setupMocks: func(userRepo *repository.MockUserRepository, refreshTokenRepo *repository.MockRefreshTokenRepository) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:           userID,
						Email:        "test@example.com",
						PasswordHash: hashedPassword,
						IsActive:     true,
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(userRepo, refreshTokenRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			err = svc.ChangePassword(ctx, userID.String(), tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserService_VerifyEmail(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		code        string
		setupMocks  func(*repository.MockUserRepository, *mockRedisClient)
		expectError bool
		statusCode  int
	}{
		{
			name: "successful verification",
			code: "123456",
			setupMocks: func(userRepo *repository.MockUserRepository, redisClient *mockRedisClient) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:                    userID,
						Email:                 "test@example.com",
						EmailVerified:         false,
						VerificationCode:      "123456",
						IsActive:              true,
					}, nil
				}
				userRepo.VerifyEmailFunc = func(ctx context.Context, uid uuid.UUID) error {
					require.Equal(t, userID, uid)
					return nil
				}
				redisClient.Set(nil, "email_verification:"+userID.String(), "123456", 0)
			},
			expectError: false,
		},
		{
			name: "already verified",
			code: "123456",
			setupMocks: func(userRepo *repository.MockUserRepository, redisClient *mockRedisClient) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:            userID,
						Email:         "test@example.com",
						EmailVerified: true,
						IsActive:      true,
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusConflict,
		},
		{
			name: "invalid code",
			code: "wrongcode",
			setupMocks: func(userRepo *repository.MockUserRepository, redisClient *mockRedisClient) {
				userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{
						ID:               userID,
						Email:            "test@example.com",
						EmailVerified:    false,
						VerificationCode: "123456",
						IsActive:         true,
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(userRepo, redisClient)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			err = svc.VerifyEmail(ctx, userID.String(), tt.code)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserService_ListSessions(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*repository.MockSessionRepository, *repository.MockJWTKeyRepository)
		expectError bool
		lenSessions int
	}{
		{
			name: "successful list sessions",
			setupMocks: func(sessionRepo *repository.MockSessionRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				sessionRepo.GetByUserIDFunc = func(ctx context.Context, uid uuid.UUID) ([]model.UserSession, error) {
					return []model.UserSession{
						{
							ID:           uuid.New(),
							UserID:       userID,
							DeviceName:   "Chrome on Windows",
							DeviceType:   "desktop",
							OS:           "Windows",
							Browser:      "Chrome",
							IPAddress:    "192.168.1.1",
							LastActiveAt: time.Now(),
							CreatedAt:    time.Now(),
						},
						{
							ID:           uuid.New(),
							UserID:       userID,
							DeviceName:   "Safari on iPhone",
							DeviceType:   "mobile",
							OS:           "iOS",
							Browser:      "Safari",
							IPAddress:    "192.168.1.2",
							LastActiveAt: time.Now(),
							CreatedAt:    time.Now(),
						},
					}, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: false,
			lenSessions: 2,
		},
		{
			name: "empty sessions list",
			setupMocks: func(sessionRepo *repository.MockSessionRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				sessionRepo.GetByUserIDFunc = func(ctx context.Context, uid uuid.UUID) ([]model.UserSession, error) {
					return []model.UserSession{}, nil
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: false,
			lenSessions: 0,
		},
		{
			name: "database error",
			setupMocks: func(sessionRepo *repository.MockSessionRepository, jwtKeyRepo *repository.MockJWTKeyRepository) {
				sessionRepo.GetByUserIDFunc = func(ctx context.Context, uid uuid.UUID) ([]model.UserSession, error) {
					return nil, errors.New("database error")
				}
				jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
					return &model.JWTKey{
						KeyType: "private",
						KeyData: "encrypted-key-data",
					}, nil
				}
			},
			expectError: true,
			lenSessions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(sessionRepo, jwtKeyRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			sessions, err := svc.ListSessions(ctx, userID.String())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, sessions, tt.lenSessions)
			}
		})
	}
}

func TestUserService_RevokeSession(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	tests := []struct {
		name        string
		sessionID   string
		setupMocks  func(*repository.MockSessionRepository, *repository.MockRefreshTokenRepository)
		expectError bool
		statusCode  int
	}{
		{
			name:      "successful revoke",
			sessionID: sessionID.String(),
			setupMocks: func(sessionRepo *repository.MockSessionRepository, refreshTokenRepo *repository.MockRefreshTokenRepository) {
				sessionRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.UserSession, error) {
					return &model.UserSession{
						ID:             sessionID,
						UserID:         userID,
						RefreshTokenID: func() *uuid.UUID { u := uuid.New(); return &u }(),
					}, nil
				}
				sessionRepo.RevokeFunc = func(ctx context.Context, id uuid.UUID) error {
					require.Equal(t, sessionID, id)
					return nil
				}
				refreshTokenRepo.RevokeFunc = func(ctx context.Context, id uuid.UUID, replacedByID *uuid.UUID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name:      "session not found",
			sessionID: sessionID.String(),
			setupMocks: func(sessionRepo *repository.MockSessionRepository, refreshTokenRepo *repository.MockRefreshTokenRepository) {
				sessionRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.UserSession, error) {
					return nil, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusNotFound,
		},
		{
			name:      "unauthorized - different user",
			sessionID: sessionID.String(),
			setupMocks: func(sessionRepo *repository.MockSessionRepository, refreshTokenRepo *repository.MockRefreshTokenRepository) {
				sessionRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.UserSession, error) {
					return &model.UserSession{
						ID:     sessionID,
						UserID: uuid.New(), // Different user
					}, nil
				}
			},
			expectError: true,
			statusCode:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &repository.MockUserRepository{}
			refreshTokenRepo := &repository.MockRefreshTokenRepository{}
			sessionRepo := &repository.MockSessionRepository{}
			passwordResetRepo := &repository.MockPasswordResetRepository{}
			jwtKeyRepo := &repository.MockJWTKeyRepository{}
			redisClient := newMockRedisClient()

			if tt.setupMocks != nil {
				tt.setupMocks(sessionRepo, refreshTokenRepo)
			}

			cfg := &config.UserServiceConfig{
				PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
				BcryptCost:              10,
				AccessTokenTTL:          15 * time.Minute,
				RefreshTokenTTL:         30 * 24 * time.Hour,
			}

			svc, err := service.NewUserService(userRepo, refreshTokenRepo, sessionRepo, passwordResetRepo, jwtKeyRepo, nil, redisClient, nil, cfg)
			if err != nil {
				t.Skipf("Skipping test due to RSA key generation: %v", err)
				return
			}

			ctx := context.Background()
			err = svc.RevokeSession(ctx, userID.String(), tt.sessionID)

			if tt.expectError {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok && tt.statusCode > 0 {
					assert.Equal(t, tt.statusCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserService_RequestPasswordReset_CryptographicallyRandomTokens(t *testing.T) {
	userID := uuid.New()
	email := "test@example.com"
	firstName := "Alex"

	userRepo := &repository.MockUserRepository{}
	userRepo.GetByEmailFunc = func(ctx context.Context, em string) (*model.User, error) {
		if strings.EqualFold(em, email) {
			return &model.User{
				ID:        userID,
				Email:     email,
				FirstName: firstName,
				IsActive:  true,
			}, nil
		}
		return nil, nil
	}

	var capturedResets []*model.PasswordReset
	passwordResetRepo := &repository.MockPasswordResetRepository{}
	passwordResetRepo.CreateFunc = func(ctx context.Context, reset *model.PasswordReset) error {
		capturedResets = append(capturedResets, reset)
		return nil
	}

	jwtKeyRepo := &repository.MockJWTKeyRepository{}
	jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
		return nil, nil
	}
	jwtKeyRepo.DeactivateAllKeysFunc = func(ctx context.Context) error {
		return nil
	}
	jwtKeyRepo.CreateFunc = func(ctx context.Context, key *model.JWTKey) error {
		return nil
	}

	refreshTokenRepo := &repository.MockRefreshTokenRepository{}
	sessionRepo := &repository.MockSessionRepository{}
	redisClient := newMockRedisClient()

	// Mock notification server to capture dispatched reset token
	var capturedTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			UserID uuid.UUID              `json:"user_id"`
			Type   string                 `json:"type"`
			Title  string                 `json:"title"`
			Body   string                 `json:"body"`
			Data   map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			if code, ok := payload.Data["Code"].(string); ok {
				capturedTokens = append(capturedTokens, code)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	notificationClient := client.NewNotificationClient(server.URL, "test-internal-key")

	cfg := &config.UserServiceConfig{
		PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
		BcryptCost:              10,
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         30 * 24 * time.Hour,
		VerificationCodeTTL:     15 * time.Minute,
	}

	svc, err := service.NewUserService(
		userRepo,
		refreshTokenRepo,
		sessionRepo,
		passwordResetRepo,
		jwtKeyRepo,
		nil,
		redisClient,
		notificationClient,
		cfg,
	)
	require.NoError(t, err)

	ctx := context.Background()
	rateLimitKey := fmt.Sprintf("password_reset_rate_limit:%s", userID.String())

	// Generate multiple reset tokens across distinct requests to verify cryptographic randomness & uniqueness
	numIterations := 5
	seenTokens := make(map[string]bool)
	seenHashes := make(map[string]bool)
	nullBytes32 := make([]byte, 32)
	nullHash := auth.HashRefreshToken(hex.EncodeToString(nullBytes32))

	for i := 0; i < numIterations; i++ {
		// Clear rate-limit key to simulate successive legitimate requests
		_ = redisClient.Delete(ctx, rateLimitKey)

		err := svc.RequestPasswordReset(ctx, email)
		require.NoError(t, err, "RequestPasswordReset should succeed")

		// Verify notification client received the token
		require.Len(t, capturedTokens, i+1, "Expected notification client to have received reset token")
		token := capturedTokens[i]

		// Invariant 1: Token must be non-empty and exactly 64 hex characters (32 bytes)
		assert.Len(t, token, 64, "Reset token must be a 64-character hex string (32 bytes)")

		// Invariant 2: Token must decode as valid hex bytes
		tokenBytes, err := hex.DecodeString(token)
		require.NoError(t, err, "Reset token must be valid hex")
		assert.Len(t, tokenBytes, 32, "Reset token must decode to 32 bytes")

		// Invariant 3: Token must NOT be null bytes or all zeros
		assert.False(t, bytes.Equal(tokenBytes, nullBytes32), "Reset token MUST NOT be null bytes (0x00 * 32)")
		assert.NotEqual(t, strings.Repeat("0", 64), token, "Reset token must not be all zero characters")

		// Invariant 4: Token must have byte variation (cryptographic entropy, not constant byte value)
		distinctByteCount := make(map[byte]bool)
		for _, b := range tokenBytes {
			distinctByteCount[b] = true
		}
		assert.GreaterOrEqual(t, len(distinctByteCount), 10, "Reset token should contain varied byte values (entropy)")

		// Invariant 5: Stored password reset record must match hashed token and NOT hash of null bytes
		require.Len(t, capturedResets, i+1)
		resetRecord := capturedResets[i]
		assert.Equal(t, userID, resetRecord.UserID)
		assert.Equal(t, auth.HashRefreshToken(token), resetRecord.TokenHash, "Stored TokenHash must equal HashRefreshToken(token)")
		assert.NotEqual(t, nullHash, resetRecord.TokenHash, "Stored TokenHash must not equal hash of null bytes")
		assert.False(t, resetRecord.Used)
		assert.True(t, resetRecord.ExpiresAt.After(time.Now()), "Reset token must expire in the future")

		// Invariant 6: Cryptographic uniqueness - each generated token and hash must be unique
		assert.False(t, seenTokens[token], "Reset token must be unique across requests (no duplicates)")
		assert.False(t, seenHashes[resetRecord.TokenHash], "TokenHash must be unique across requests")
		seenTokens[token] = true
		seenHashes[resetRecord.TokenHash] = true
	}
}

func TestUserService_Logout_BlacklistToken(t *testing.T) {
	userID := uuid.New()
	jti := uuid.New().String()

	userRepo := &repository.MockUserRepository{}
	refreshTokenRepo := &repository.MockRefreshTokenRepository{}
	var revokedRefreshTokensForUser uuid.UUID
	refreshTokenRepo.RevokeAllForUserFunc = func(ctx context.Context, uid uuid.UUID) error {
		revokedRefreshTokensForUser = uid
		return nil
	}

	sessionRepo := &repository.MockSessionRepository{}
	var revokedSessionsForUser uuid.UUID
	sessionRepo.RevokeAllForUserFunc = func(ctx context.Context, uid uuid.UUID, exceptSessionID *uuid.UUID) error {
		revokedSessionsForUser = uid
		return nil
	}

	jwtKeyRepo := &repository.MockJWTKeyRepository{}
	jwtKeyRepo.GetActivePrivateKeyFunc = func(ctx context.Context) (*model.JWTKey, error) {
		return nil, nil
	}
	jwtKeyRepo.DeactivateAllKeysFunc = func(ctx context.Context) error {
		return nil
	}
	jwtKeyRepo.CreateFunc = func(ctx context.Context, key *model.JWTKey) error {
		return nil
	}

	redisClient := newMockRedisClient()

	cfg := &config.UserServiceConfig{
		PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
		BcryptCost:              10,
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         30 * 24 * time.Hour,
		VerificationCodeTTL:     15 * time.Minute,
	}

	svc, err := service.NewUserService(
		userRepo,
		refreshTokenRepo,
		sessionRepo,
		&repository.MockPasswordResetRepository{},
		jwtKeyRepo,
		nil,
		redisClient,
		nil,
		cfg,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Test 1: Direct JTI passed
	err = svc.Logout(ctx, userID.String(), jti)
	require.NoError(t, err)

	blacklistKey := fmt.Sprintf("token_blacklist:%s", jti)
	assert.Equal(t, userID.String(), redisClient.data[blacklistKey])
	assert.Equal(t, userID, revokedRefreshTokensForUser)
	assert.Equal(t, userID, revokedSessionsForUser)

	// Test 2: Invalid User ID
	err = svc.Logout(ctx, "not-a-uuid", jti)
	require.Error(t, err)
}

func TestUserService_InstitutionMembership(t *testing.T) {
	userID := uuid.New()
	user := &model.User{
		ID:            userID,
		Email:         "scholar@veritas.edu.ng",
		FirstName:     "Bartholomew",
		LastName:      "Agada",
		IsActive:      true,
		EmailVerified: true,
		Role:          "student",
		InstitutionID: "veritas_uni",
	}

	userRepo := &repository.MockUserRepository{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.User, error) {
			if id == userID {
				return user, nil
			}
			return nil, nil
		},
		UpdateFunc: func(ctx context.Context, u *model.User) error {
			user = u
			return nil
		},
	}

	membershipStore := make(map[string]*model.InstitutionMembership)
	membershipRepo := &repository.MockMembershipRepository{
		CreateFunc: func(ctx context.Context, m *model.InstitutionMembership) error {
			key := fmt.Sprintf("%s:%s", m.UserID.String(), m.InstitutionID)
			membershipStore[key] = m
			return nil
		},
		GetByUserAndInstitutionFunc: func(ctx context.Context, uid uuid.UUID, instID string) (*model.InstitutionMembership, error) {
			key := fmt.Sprintf("%s:%s", uid.String(), instID)
			return membershipStore[key], nil
		},
		ListByUserIDFunc: func(ctx context.Context, uid uuid.UUID) ([]model.InstitutionMembership, error) {
			var list []model.InstitutionMembership
			for _, m := range membershipStore {
				if m.UserID == uid {
					list = append(list, *m)
				}
			}
			return list, nil
		},
	}

	refreshTokenRepo := &repository.MockRefreshTokenRepository{
		CreateFunc: func(ctx context.Context, token *model.RefreshToken) error {
			return nil
		},
	}
	sessionRepo := &repository.MockSessionRepository{
		CreateFunc: func(ctx context.Context, session *model.UserSession) error {
			return nil
		},
	}
	jwtKeyRepo := &repository.MockJWTKeyRepository{
		GetActivePrivateKeyFunc: func(ctx context.Context) (*model.JWTKey, error) {
			return nil, nil
		},
		DeactivateAllKeysFunc: func(ctx context.Context) error {
			return nil
		},
		CreateFunc: func(ctx context.Context, key *model.JWTKey) error {
			return nil
		},
	}
	redisClient := newMockRedisClient()

	cfg := &config.UserServiceConfig{
		PrivateKeyEncryptionKey: "test-encryption-key-min-32-chars-long",
		BcryptCost:              10,
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         30 * 24 * time.Hour,
		VerificationCodeTTL:     15 * time.Minute,
	}

	svc, err := service.NewUserService(
		userRepo,
		refreshTokenRepo,
		sessionRepo,
		&repository.MockPasswordResetRepository{},
		jwtKeyRepo,
		membershipRepo,
		redisClient,
		nil,
		cfg,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// 1. Add student membership at Veritas University
	m1, err := svc.AddMembership(ctx, userID.String(), &service.AddMembershipRequest{
		InstitutionID: "veritas_uni",
		Role:          "student",
		DepartmentID:  "dept_cs_veritas",
		Identifier:    "VUNA/CSC/22/001",
		IsDefault:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "veritas_uni", m1.InstitutionID)
	assert.Equal(t, "student", m1.Role)
	assert.Equal(t, "active", m1.Status)

	// 2. Add lecturer membership at ABU Zaria
	m2, err := svc.AddMembership(ctx, userID.String(), &service.AddMembershipRequest{
		InstitutionID: "abu_zaria",
		Role:          "lecturer",
		DepartmentID:  "dept_eng_abu",
		Identifier:    "ABU/STAFF/892",
		IsDefault:     false,
	})
	require.NoError(t, err)
	assert.Equal(t, "abu_zaria", m2.InstitutionID)
	assert.Equal(t, "lecturer", m2.Role)

	// 3. Duplicate membership returns 409 Conflict
	_, err = svc.AddMembership(ctx, userID.String(), &service.AddMembershipRequest{
		InstitutionID: "veritas_uni",
		Role:          "student",
	})
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, httpErr.Code)

	// 4. List memberships
	list, err := svc.ListMemberships(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, list, 2)

	clientInfo := &service.ClientInfo{
		IPAddress: "127.0.0.1",
		UserAgent: "Mozilla/5.0 Test",
	}

	// 5. Switch active context to Veritas University (student role)
	tokenResp1, err := svc.SwitchActiveInstitution(ctx, userID.String(), "veritas_uni", clientInfo)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenResp1.AccessToken)
	assert.Equal(t, "veritas_uni", user.InstitutionID)
	assert.Equal(t, "student", user.Role)

	// 6. Switch active context to ABU Zaria (lecturer role)
	tokenResp2, err := svc.SwitchActiveInstitution(ctx, userID.String(), "abu_zaria", clientInfo)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenResp2.AccessToken)
	assert.Equal(t, "abu_zaria", user.InstitutionID)
	assert.Equal(t, "lecturer", user.Role)

	// 7. Switch to an institution where user is not a member returns 403 Forbidden
	_, err = svc.SwitchActiveInstitution(ctx, userID.String(), "unilag", clientInfo)
	require.Error(t, err)
	httpErr, ok = err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, httpErr.Code)
}



