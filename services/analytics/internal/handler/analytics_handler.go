package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/services/analytics/internal/service"
	"zuri/shared/pkg/logger"
)

// AnalyticsHandler handles analytics-related HTTP requests.
type AnalyticsHandler struct {
	service   service.AnalyticsService
	validator *validator.Validate
}

// NewAnalyticsHandler creates a new analytics handler.
func NewAnalyticsHandler(service service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		service:   service,
		validator: validator.New(),
	}
}

// ==================== Quiz Attempt Handlers ====================

// StartQuizAttempt handles POST /api/v1/quizzes/:id/start
func (h *AnalyticsHandler) StartQuizAttempt(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	quizID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid quiz ID")
	}

	attempt, err := h.service.StartQuizAttempt(c.Request().Context(), userID, quizID)
	if err != nil {
		logger.Error("failed to start quiz attempt", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to start attempt")
	}

	return c.JSON(http.StatusCreated, Response{Data: attempt})
}

// SubmitAnswer handles POST /api/v1/quiz-attempts/:id/answers
func (h *AnalyticsHandler) SubmitAnswer(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	attemptID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid attempt ID")
	}

	var req struct {
		QuestionID       string `json:"question_id" validate:"required"`
		Answer           string `json:"answer" validate:"required"`
		TimeTakenSeconds int    `json:"time_taken_seconds"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}

	answer, err := h.service.SubmitAnswer(c.Request().Context(), userID, attemptID, questionID, req.Answer, req.TimeTakenSeconds)
	if err != nil {
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		if err.Error() == "attempt is not in progress" {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		logger.Error("failed to submit answer", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to submit answer")
	}

	return c.JSON(http.StatusCreated, Response{Data: answer})
}

// CompleteQuizAttempt handles POST /api/v1/quiz-attempts/:id/complete
func (h *AnalyticsHandler) CompleteQuizAttempt(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	attemptID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid attempt ID")
	}

	attempt, err := h.service.CompleteQuizAttempt(c.Request().Context(), userID, attemptID)
	if err != nil {
		if err == service.ErrAttemptNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "attempt not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to complete quiz attempt", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to complete attempt")
	}

	return c.JSON(http.StatusOK, Response{Data: attempt})
}

// GetQuizAttempt handles GET /api/v1/quiz-attempts/:id
func (h *AnalyticsHandler) GetQuizAttempt(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	attemptID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid attempt ID")
	}

	attempt, err := h.service.GetAttempt(c.Request().Context(), userID, attemptID)
	if err != nil {
		if err == service.ErrAttemptNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "attempt not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get quiz attempt", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get attempt")
	}

	return c.JSON(http.StatusOK, Response{Data: attempt})
}

// GetUserQuizAttempts handles GET /api/v1/quiz-attempts
func (h *AnalyticsHandler) GetUserQuizAttempts(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if limit <= 0 {
		limit = 20
	}

	attempts, err := h.service.GetUserAttempts(c.Request().Context(), userID, limit, offset)
	if err != nil {
		logger.Error("failed to get quiz attempts", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get attempts")
	}

	return c.JSON(http.StatusOK, Response{Data: attempts})
}

// ==================== Study Session Handlers ====================

// GetStudyStreak handles GET /api/v1/analytics/study-streak
func (h *AnalyticsHandler) GetStudyStreak(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	streak, err := h.service.GetStudyStreak(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get study streak", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get streak")
	}

	return c.JSON(http.StatusOK, Response{Data: map[string]interface{}{
		"current_streak": streak,
	}})
}

// GetStudyStats handles GET /api/v1/analytics/study-stats
func (h *AnalyticsHandler) GetStudyStats(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	stats, err := h.service.GetUserStudyStats(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get study stats", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get stats")
	}

	// Get streak separately
	streak, _ := h.service.GetStudyStreak(c.Request().Context(), userID)

	return c.JSON(http.StatusOK, Response{Data: map[string]interface{}{
		"total_study_days":         stats.TotalStudyDays,
		"total_study_minutes":      stats.TotalStudyMinutes,
		"total_quizzes_completed":  stats.TotalQuizzesCompleted,
		"total_materials_reviewed": stats.TotalMaterialsReviewed,
		"last_study_date":          stats.LastStudyDate,
		"current_streak":           streak,
	}})
}

// RecordStudySession handles POST /api/v1/analytics/study-sessions
func (h *AnalyticsHandler) RecordStudySession(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.RecordStudySessionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.SessionDate.IsZero() {
		req.SessionDate = time.Now()
	}

	if err := h.service.RecordStudySession(c.Request().Context(), userID, &req); err != nil {
		logger.Error("failed to record study session", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to record session")
	}

	return c.JSON(http.StatusCreated, Response{Message: "study session recorded"})
}

// ==================== Topic Mastery Handlers ====================

// GetTopicMastery handles GET /api/v1/analytics/topic-mastery
func (h *AnalyticsHandler) GetTopicMastery(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	mastery, err := h.service.GetTopicMastery(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get topic mastery", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get mastery")
	}

	return c.JSON(http.StatusOK, Response{Data: mastery})
}

// GetTopicsForReview handles GET /api/v1/analytics/topics-for-review
func (h *AnalyticsHandler) GetTopicsForReview(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	topics, err := h.service.GetTopicsForReview(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get topics for review", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get topics")
	}

	return c.JSON(http.StatusOK, Response{Data: topics})
}

// ==================== Learning Goal Handlers ====================

// CreateLearningGoal handles POST /api/v1/analytics/goals
func (h *AnalyticsHandler) CreateLearningGoal(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.CreateLearningGoalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	goal, err := h.service.CreateLearningGoal(c.Request().Context(), userID, &req)
	if err != nil {
		logger.Error("failed to create learning goal", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create goal")
	}

	return c.JSON(http.StatusCreated, Response{Data: goal})
}

// GetLearningGoals handles GET /api/v1/analytics/goals
func (h *AnalyticsHandler) GetLearningGoals(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	includeCompleted := c.QueryParam("include_completed") == "true"

	goals, err := h.service.GetLearningGoals(c.Request().Context(), userID, includeCompleted)
	if err != nil {
		logger.Error("failed to get learning goals", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get goals")
	}

	return c.JSON(http.StatusOK, Response{Data: goals})
}

// UpdateLearningGoal handles PUT /api/v1/analytics/goals/:id
func (h *AnalyticsHandler) UpdateLearningGoal(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	goalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid goal ID")
	}

	var req service.UpdateLearningGoalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	goal, err := h.service.UpdateLearningGoal(c.Request().Context(), userID, goalID, &req)
	if err != nil {
		if err.Error() == "goal not found" {
			return echo.NewHTTPError(http.StatusNotFound, "goal not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update learning goal", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update goal")
	}

	return c.JSON(http.StatusOK, Response{Data: goal})
}

// DeleteLearningGoal handles DELETE /api/v1/analytics/goals/:id
func (h *AnalyticsHandler) DeleteLearningGoal(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	goalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid goal ID")
	}

	if err := h.service.DeleteLearningGoal(c.Request().Context(), userID, goalID); err != nil {
		if err.Error() == "goal not found" {
			return echo.NewHTTPError(http.StatusNotFound, "goal not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete learning goal", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete goal")
	}

	return c.JSON(http.StatusOK, Response{Message: "goal deleted"})
}

// CompleteLearningGoal handles POST /api/v1/analytics/goals/:id/complete
func (h *AnalyticsHandler) CompleteLearningGoal(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	goalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid goal ID")
	}

	if err := h.service.CompleteLearningGoal(c.Request().Context(), userID, goalID); err != nil {
		if err.Error() == "goal not found" {
			return echo.NewHTTPError(http.StatusNotFound, "goal not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to complete learning goal", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to complete goal")
	}

	return c.JSON(http.StatusOK, Response{Message: "goal marked as complete"})
}

// ==================== AI Usage Handlers ====================

// TrackAIInteraction handles POST /api/v1/analytics/ai-interactions
func (h *AnalyticsHandler) TrackAIInteraction(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.TrackAIInteractionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.TrackAIInteraction(c.Request().Context(), userID, &req); err != nil {
		logger.Error("failed to track AI interaction", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to track interaction")
	}

	return c.JSON(http.StatusCreated, Response{Message: "interaction tracked"})
}

// GetAIUsageStats handles GET /api/v1/analytics/ai-usage
func (h *AnalyticsHandler) GetAIUsageStats(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days <= 0 {
		days = 30
	}

	stats, err := h.service.GetAIUsageStats(c.Request().Context(), userID, days)
	if err != nil {
		logger.Error("failed to get AI usage stats", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get stats")
	}

	return c.JSON(http.StatusOK, Response{Data: stats})
}
