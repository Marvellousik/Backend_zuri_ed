package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/services/content/internal/service"
	"zuri/shared/pkg/logger"
)

// CourseHandler handles course-related HTTP requests.
type CourseHandler struct {
	service   service.ContentService
	validator *validator.Validate
}

// NewCourseHandler creates a new course handler.
func NewCourseHandler(service service.ContentService) *CourseHandler {
	return &CourseHandler{
		service:   service,
		validator: validator.New(),
	}
}

// CreateCourse handles POST /api/v1/courses
func (h *CourseHandler) CreateCourse(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.CreateCourseRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	course, err := h.service.CreateCourse(c.Request().Context(), userID, &req)
	if err != nil {
		logger.Error("failed to create course", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create course")
	}

	return c.JSON(http.StatusCreated, Response{Data: course})
}

// GetCourse handles GET /api/v1/courses/:id
func (h *CourseHandler) GetCourse(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid course ID")
	}

	course, err := h.service.GetCourse(c.Request().Context(), userID, courseID)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get course", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get course")
	}

	return c.JSON(http.StatusOK, Response{Data: course})
}

// GetUserCourses handles GET /api/v1/courses
func (h *CourseHandler) GetUserCourses(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courses, err := h.service.GetUserCourses(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get courses", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get courses")
	}

	return c.JSON(http.StatusOK, Response{Data: courses})
}

// UpdateCourse handles PUT /api/v1/courses/:id
func (h *CourseHandler) UpdateCourse(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid course ID")
	}

	var req service.UpdateCourseRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	course, err := h.service.UpdateCourse(c.Request().Context(), userID, courseID, &req)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update course", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update course")
	}

	return c.JSON(http.StatusOK, Response{Data: course})
}

// DeleteCourse handles DELETE /api/v1/courses/:id
func (h *CourseHandler) DeleteCourse(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid course ID")
	}

	if err := h.service.DeleteCourse(c.Request().Context(), userID, courseID); err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete course", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete course")
	}

	return c.NoContent(http.StatusNoContent)
}
