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

// QuizHandler handles quiz-related HTTP requests.
type QuizHandler struct {
	service   service.ContentService
	validator *validator.Validate
}

// NewQuizHandler creates a new quiz handler.
func NewQuizHandler(service service.ContentService) *QuizHandler {
	return &QuizHandler{
		service:   service,
		validator: validator.New(),
	}
}

// CreateQuiz handles POST /api/v1/quizzes
func (h *QuizHandler) CreateQuiz(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.CreateQuizRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	quiz, err := h.service.CreateQuiz(c.Request().Context(), userID, &req)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to create quiz", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create quiz")
	}

	return c.JSON(http.StatusCreated, Response{Data: quiz})
}

// GetQuiz handles GET /api/v1/quizzes/:id
func (h *QuizHandler) GetQuiz(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid quiz ID")
	}

	quiz, err := h.service.GetQuiz(c.Request().Context(), userID, quizID)
	if err != nil {
		if err == service.ErrQuizNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "quiz not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get quiz", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get quiz")
	}

	return c.JSON(http.StatusOK, Response{Data: quiz})
}

// GetUserQuizzes handles GET /api/v1/quizzes
func (h *QuizHandler) GetUserQuizzes(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizzes, err := h.service.GetUserQuizzes(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get quizzes", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get quizzes")
	}

	return c.JSON(http.StatusOK, Response{Data: quizzes})
}

// GetCourseQuizzes handles GET /api/v1/courses/:id/quizzes
func (h *QuizHandler) GetCourseQuizzes(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid course ID")
	}

	quizzes, err := h.service.GetCourseQuizzes(c.Request().Context(), userID, courseID)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get course quizzes", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get quizzes")
	}

	return c.JSON(http.StatusOK, Response{Data: quizzes})
}

// UpdateQuiz handles PUT /api/v1/quizzes/:id
func (h *QuizHandler) UpdateQuiz(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid quiz ID")
	}

	var req service.UpdateQuizRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	quiz, err := h.service.UpdateQuiz(c.Request().Context(), userID, quizID, &req)
	if err != nil {
		if err == service.ErrQuizNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "quiz not found")
		}
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update quiz", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update quiz")
	}

	return c.JSON(http.StatusOK, Response{Data: quiz})
}

// DeleteQuiz handles DELETE /api/v1/quizzes/:id
func (h *QuizHandler) DeleteQuiz(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid quiz ID")
	}

	if err := h.service.DeleteQuiz(c.Request().Context(), userID, quizID); err != nil {
		if err == service.ErrQuizNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "quiz not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete quiz", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete quiz")
	}

	return c.NoContent(http.StatusNoContent)
}

// AddQuestion handles POST /api/v1/quizzes/:id/questions
func (h *QuizHandler) AddQuestion(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid quiz ID")
	}

	var req service.AddQuestionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	question, err := h.service.AddQuizQuestion(c.Request().Context(), userID, quizID, &req)
	if err != nil {
		if err == service.ErrQuizNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "quiz not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to add question", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add question")
	}

	return c.JSON(http.StatusCreated, Response{Data: question})
}

// UpdateQuestion handles PUT /api/v1/quizzes/questions/:id
func (h *QuizHandler) UpdateQuestion(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	questionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}

	var req service.UpdateQuestionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	question, err := h.service.UpdateQuizQuestion(c.Request().Context(), userID, questionID, &req)
	if err != nil {
		if err == service.ErrQuestionNotFound || err.Error() == "question not found" {
			return echo.NewHTTPError(http.StatusNotFound, "question not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update question", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update question")
	}

	return c.JSON(http.StatusOK, Response{Data: question})
}

// DeleteQuestion handles DELETE /api/v1/quizzes/questions/:id
func (h *QuizHandler) DeleteQuestion(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	questionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}

	if err := h.service.DeleteQuizQuestion(c.Request().Context(), userID, questionID); err != nil {
		if err == service.ErrQuestionNotFound || err.Error() == "question not found" {
			return echo.NewHTTPError(http.StatusNotFound, "question not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete question", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete question")
	}

	return c.NoContent(http.StatusNoContent)
}
