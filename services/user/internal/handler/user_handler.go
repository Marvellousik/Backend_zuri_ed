package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"zuri/services/user/internal/service"
)

// UserHandler handles user profile-related HTTP requests.
type UserHandler struct {
	userService service.UserService
}

// NewUserHandler creates a new user handler.
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// UpdateProfileRequest represents a profile update request.
type UpdateProfileRequest struct {
	FirstName     string `json:"first_name,omitempty"`
	LastName      string `json:"last_name,omitempty"`
	School        string `json:"school,omitempty"`
	Department    string `json:"department,omitempty"`
	AcademicLevel string `json:"academic_level,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// GetProfile handles retrieving the current user's profile.
func (h *UserHandler) GetProfile(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	resp, err := h.userService.GetProfile(ctx, userID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// UpdateProfile handles updating the current user's profile.
func (h *UserHandler) UpdateProfile(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	svcReq := &service.UpdateProfileRequest{
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		School:        req.School,
		Department:    req.Department,
		AcademicLevel: req.AcademicLevel,
		Timezone:      req.Timezone,
	}

	resp, err := h.userService.UpdateProfile(ctx, userID, svcReq)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Profile updated successfully",
		"data":    resp,
	})
}

// ChangePassword handles changing the current user's password.
func (h *UserHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	svcReq := &service.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}

	if err := h.userService.ChangePassword(ctx, userID, svcReq); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Password changed successfully. Please log in again with your new password.",
	})
}

// AddMembershipRequest represents a request to join an institution.
type AddMembershipRequest struct {
	InstitutionID string `json:"institution_id" validate:"required"`
	Role          string `json:"role" validate:"required"`
	DepartmentID  string `json:"department_id,omitempty"`
	Identifier    string `json:"identifier,omitempty"`
	IsDefault     bool   `json:"is_default,omitempty"`
}

// SwitchInstitutionRequest represents a request to switch active institution.
type SwitchInstitutionRequest struct {
	InstitutionID string `json:"institution_id" validate:"required"`
}

// ListMemberships handles retrieving the current user's institutional memberships.
func (h *UserHandler) ListMemberships(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	memberships, err := h.userService.ListMemberships(ctx, userID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": memberships,
	})
}

// AddMembership handles adding an institutional membership for the current user.
func (h *UserHandler) AddMembership(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	var req AddMembershipRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	svcReq := &service.AddMembershipRequest{
		InstitutionID: req.InstitutionID,
		Role:          req.Role,
		DepartmentID:  req.DepartmentID,
		Identifier:    req.Identifier,
		IsDefault:     req.IsDefault,
	}

	membership, err := h.userService.AddMembership(ctx, userID, svcReq)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message": "Membership created successfully",
		"data":    membership,
	})
}

// SwitchInstitution handles switching active institutional context and returns new tokens.
func (h *UserHandler) SwitchInstitution(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	var req SwitchInstitutionRequest
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

	tokenResp, err := h.userService.SwitchActiveInstitution(ctx, userID, req.InstitutionID, clientInfo)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Switched active institution successfully",
		"data":    tokenResp,
	})
}

