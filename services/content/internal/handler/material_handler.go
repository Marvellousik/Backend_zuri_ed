package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/services/content/internal/service"
	"zuri/shared/pkg/logger"
)

// MaterialHandler handles material-related HTTP requests.
type MaterialHandler struct {
	service   service.ContentService
	validator *validator.Validate
}

// NewMaterialHandler creates a new material handler.
func NewMaterialHandler(service service.ContentService) *MaterialHandler {
	return &MaterialHandler{
		service:   service,
		validator: validator.New(),
	}
}

// CreateMaterial handles POST /api/v1/materials
func (h *MaterialHandler) CreateMaterial(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.CreateMaterialRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	material, err := h.service.CreateMaterial(c.Request().Context(), userID, &req)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to create material", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create material")
	}

	return c.JSON(http.StatusCreated, Response{Data: material})
}

// GetMaterial handles GET /api/v1/materials/:id
func (h *MaterialHandler) GetMaterial(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	materialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid material ID")
	}

	material, err := h.service.GetMaterial(c.Request().Context(), userID, materialID)
	if err != nil {
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get material", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get material")
	}

	return c.JSON(http.StatusOK, Response{Data: material})
}

// GetUserMaterials handles GET /api/v1/materials
func (h *MaterialHandler) GetUserMaterials(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	// Parse pagination
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	materials, err := h.service.GetUserMaterials(c.Request().Context(), userID, limit, offset)
	if err != nil {
		logger.Error("failed to get materials", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get materials")
	}

	return c.JSON(http.StatusOK, Response{Data: materials})
}

// GetCourseMaterials handles GET /api/v1/courses/:id/materials
func (h *MaterialHandler) GetCourseMaterials(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid course ID")
	}

	materials, err := h.service.GetCourseMaterials(c.Request().Context(), userID, courseID)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get course materials", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get materials")
	}

	return c.JSON(http.StatusOK, Response{Data: materials})
}

// UpdateMaterial handles PUT /api/v1/materials/:id
func (h *MaterialHandler) UpdateMaterial(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	materialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid material ID")
	}

	var req service.UpdateMaterialRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	material, err := h.service.UpdateMaterial(c.Request().Context(), userID, materialID, &req)
	if err != nil {
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update material", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update material")
	}

	return c.JSON(http.StatusOK, Response{Data: material})
}

// DeleteMaterial handles DELETE /api/v1/materials/:id
func (h *MaterialHandler) DeleteMaterial(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	materialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid material ID")
	}

	if err := h.service.DeleteMaterial(c.Request().Context(), userID, materialID); err != nil {
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete material", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete material")
	}

	return c.NoContent(http.StatusNoContent)
}

// PresignMaterial handles POST /api/v1/materials/:id/presign
func (h *MaterialHandler) PresignMaterial(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	materialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid material ID")
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	presignResp, err := h.service.GeneratePresignURL(c.Request().Context(), userID, materialID, req.Action)
	if err != nil {
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to generate presign URL", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate presign URL")
	}

	return c.JSON(http.StatusOK, Response{Data: presignResp})
}

type UpdateProcessingStatusRequest struct {
	MaterialID    string `json:"material_id" validate:"required,uuid"`
	Status        string `json:"status" validate:"required"`
	ChunksCreated int    `json:"chunks_created"`
	Error         string `json:"error,omitempty"`
}

// UpdateProcessingStatus handles internal callbacks to update material processing status.
// POST /api/v1/internal/materials/processing-status
func (h *MaterialHandler) UpdateProcessingStatus(c echo.Context) error {
	// Authenticate the request using X-Internal-API-Key or X-Internal-Key
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		internalKey = "dev-internal-key"
	}

	reqKey := c.Request().Header.Get("X-Internal-API-Key")
	if reqKey == "" {
		reqKey = c.Request().Header.Get("X-Internal-Key")
	}

	if reqKey != internalKey {
		logger.Warn("unauthorized internal callback attempt",
			zap.String("req_key", reqKey),
			zap.String("ip", c.RealIP()),
		)
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req UpdateProcessingStatusRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	materialID, err := uuid.Parse(req.MaterialID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid material ID format")
	}

	err = h.service.UpdateMaterialProcessingStatus(
		c.Request().Context(),
		materialID,
		req.Status,
		req.ChunksCreated,
		req.Error,
	)
	if err != nil {
		logger.Error("failed to update material processing status", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update processing status")
	}

	return c.JSON(http.StatusOK, Response{Message: "status updated successfully"})
}
