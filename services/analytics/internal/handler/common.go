package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Response is the standard API response format.
type Response struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// getUserID extracts the user ID from the context (set by JWT middleware).
func getUserID(c echo.Context) (uuid.UUID, error) {
	// Get user_id from context (set by gateway X-User-ID header)
	userIDStr := c.Request().Header.Get("X-User-ID")
	if userIDStr == "" {
		// Try to get from context (for direct testing)
		if uid := c.Get("user_id"); uid != nil {
			if id, ok := uid.(string); ok {
				userIDStr = id
			}
		}
	}

	if userIDStr == "" {
		return uuid.Nil, errors.New("user ID not found")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

// HealthCheck handles health check requests.
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "analytics",
	})
}
