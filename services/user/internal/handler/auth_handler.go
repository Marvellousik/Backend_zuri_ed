// Package handler contains HTTP handlers for the User Service.
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"zuri/services/user/internal/service"
	"zuri/shared/pkg/auth"
	"zuri/shared/pkg/logger"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	userService service.UserService
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userService service.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// RegisterRequest represents a registration request.
type RegisterRequest struct {
	Email         string `json:"email" validate:"required,email"`
	Password      string `json:"password" validate:"required,min=8"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	School        string `json:"school"`
	Department    string `json:"department"`
	AcademicLevel string `json:"academic_level"`
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// TokenRequest represents a token refresh request.
type TokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// VerifyEmailRequest represents an email verification request.
type VerifyEmailRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// PasswordResetRequest represents a password reset request.
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordConfirmRequest represents a password reset confirmation request.
type ResetPasswordConfirmRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// Register handles user registration.
func (h *AuthHandler) Register(c echo.Context) error {
	ctx := c.Request().Context()

	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	svcReq := &service.RegisterRequest{
		Email:         req.Email,
		Password:      req.Password,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		School:        req.School,
		Department:    req.Department,
		AcademicLevel: req.AcademicLevel,
	}

	resp, err := h.userService.Register(ctx, svcReq)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message": "User registered successfully. Please check your email for verification code.",
		"data":    resp,
	})
}

// Login handles user login.
func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()

	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	clientInfo := &service.ClientInfo{
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	}

	svcReq := &service.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	resp, err := h.userService.Login(ctx, svcReq, clientInfo)
	if err != nil {
		var emailNotVerifiedErr *service.EmailNotVerifiedError
		if errors.As(err, &emailNotVerifiedErr) {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error":   "Email verification required",
				"code":    "EMAIL_NOT_VERIFIED",
				"message": "Please verify your email before logging in.",
				"user_id": emailNotVerifiedErr.UserID.String(),
			})
		}
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// RefreshToken handles token refresh.
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req TokenRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	clientInfo := &service.ClientInfo{
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	}

	resp, err := h.userService.RefreshToken(ctx, req.RefreshToken, clientInfo)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// Logout handles user logout.
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	// Extract JTI from access token if available
	accessTokenJTI := c.Request().Header.Get("X-Token-ID")
	if accessTokenJTI == "" {
		accessTokenJTI = c.Request().Header.Get("X-Token-JTI")
	}
	if accessTokenJTI == "" {
		authHeader := c.Request().Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			rawToken := strings.TrimSpace(authHeader[7:])
			parser := jwt.NewParser()
			token, _, err := parser.ParseUnverified(rawToken, &auth.Claims{})
			if err == nil {
				if claims, ok := token.Claims.(*auth.Claims); ok && claims.ID != "" {
					accessTokenJTI = claims.ID
				}
			}
		}
	}

	if err := h.userService.Logout(ctx, userID, accessTokenJTI); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Logged out successfully",
	})
}

// LogoutAll handles logout from all devices.
func (h *AuthHandler) LogoutAll(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	if err := h.userService.LogoutAll(ctx, userID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Logged out from all devices successfully",
	})
}

// VerifyEmail handles email verification.
func (h *AuthHandler) VerifyEmail(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		// Allow verification without auth if user ID is in request
		userID = c.QueryParam("user_id")
		if userID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "user ID required")
		}
	}

	var req VerifyEmailRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.userService.VerifyEmail(ctx, userID, req.Code); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Email verified successfully",
	})
}

// ResendVerification handles resending verification email.
func (h *AuthHandler) ResendVerification(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	if err := h.userService.ResendVerification(ctx, userID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Verification email resent successfully",
	})
}

// RequestPasswordReset handles password reset requests.
func (h *AuthHandler) RequestPasswordReset(c echo.Context) error {
	ctx := c.Request().Context()

	var req PasswordResetRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.userService.RequestPasswordReset(ctx, req.Email); err != nil {
		return err
	}

	// Always return success to prevent email enumeration
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "If the email exists, a password reset link has been sent",
	})
}

// ResetPassword handles password reset confirmation.
func (h *AuthHandler) ResetPassword(c echo.Context) error {
	ctx := c.Request().Context()

	var req ResetPasswordConfirmRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.userService.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Password reset successfully",
	})
}

// GetPublicKey returns the JWT public key for other services to validate tokens.
func (h *AuthHandler) GetPublicKey(c echo.Context) error {
	ctx := c.Request().Context()

	publicKey, err := h.userService.GetPublicKey(ctx)
	if err != nil {
		logger.Error("failed to get public key")
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"public_key": publicKey,
			"algorithm":  "RS256",
		},
	})
}
