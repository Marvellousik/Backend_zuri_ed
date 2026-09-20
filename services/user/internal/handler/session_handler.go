package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"zuri/services/user/internal/service"
)

// SessionHandler handles session management HTTP requests.
type SessionHandler struct {
	userService service.UserService
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(userService service.UserService) *SessionHandler {
	return &SessionHandler{userService: userService}
}

// ListSessions handles retrieving all active sessions for the user.
func (h *SessionHandler) ListSessions(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessions, err := h.userService.ListSessions(ctx, userID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": sessions,
	})
}

// RevokeSession handles revoking a specific session.
func (h *SessionHandler) RevokeSession(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session ID required")
	}

	if err := h.userService.RevokeSession(ctx, userID, sessionID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Session revoked successfully",
	})
}
