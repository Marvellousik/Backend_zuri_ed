package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"zuri/services/analytics/internal/handler"
	"zuri/services/analytics/internal/repository"
	"zuri/services/analytics/internal/service"
	analyticsConfig "zuri/services/analytics/pkg/config"
	"zuri/shared/pkg/database"
	"zuri/shared/pkg/logger"
)

func main() {
	// Load configuration
	cfg := analyticsConfig.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("starting analytics service",
		zap.String("port", cfg.Port),
		zap.String("environment", cfg.Environment),
	)

	// Initialize database
	dbConfig := database.DefaultConfig(cfg.DatabaseURL)
	db, err := database.New(dbConfig)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	// Ensure database connection is closed on shutdown
	defer func() {
		if sqlDB, err := db.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	logger.Info("connected to database")

	// Initialize repositories
	attemptRepo := repository.NewQuizAttemptRepository(db)
	sessionRepo := repository.NewStudySessionRepository(db)
	masteryRepo := repository.NewTopicMasteryRepository(db)
	aiRepo := repository.NewAIInteractionRepository(db)
	goalRepo := repository.NewLearningGoalRepository(db)

	// Initialize services
	analyticsService := service.NewAnalyticsService(
		attemptRepo,
		sessionRepo,
		masteryRepo,
		aiRepo,
		goalRepo,
	)

	// Initialize handlers
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true
	e.Validator = &customValidator{validator: validator.New()}

	// Middleware
	e.Use(middleware.Recover())
	// CORS is handled by the Gateway - don't set here to avoid duplicate headers
	// e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
	// 	AllowOrigins: []string{"*"},
	// 	AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	// 	AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-User-ID", "X-Correlation-ID"},
	// }))
	e.Use(loggerMiddleware)
	e.Use(correlationIDMiddleware)

	// Routes
	e.GET("/health", handler.HealthCheck)

	// Analytics routes
	api := e.Group("/api/v1")
	{
		// Quiz attempts
		api.GET("/quiz-attempts", analyticsHandler.GetUserQuizAttempts)
		api.GET("/quiz-attempts/:id", analyticsHandler.GetQuizAttempt)
		api.POST("/quizzes/:id/start", analyticsHandler.StartQuizAttempt)
		api.POST("/quiz-attempts/:id/answers", analyticsHandler.SubmitAnswer)
		api.POST("/quiz-attempts/:id/complete", analyticsHandler.CompleteQuizAttempt)

		// Study sessions & analytics
		api.GET("/analytics/study-streak", analyticsHandler.GetStudyStreak)
		api.GET("/analytics/study-stats", analyticsHandler.GetStudyStats)
		api.POST("/analytics/study-sessions", analyticsHandler.RecordStudySession)

		// Topic mastery
		api.GET("/analytics/topic-mastery", analyticsHandler.GetTopicMastery)
		api.GET("/analytics/topics-for-review", analyticsHandler.GetTopicsForReview)

		// Learning goals
		api.GET("/analytics/goals", analyticsHandler.GetLearningGoals)
		api.POST("/analytics/goals", analyticsHandler.CreateLearningGoal)
		api.PUT("/analytics/goals/:id", analyticsHandler.UpdateLearningGoal)
		api.DELETE("/analytics/goals/:id", analyticsHandler.DeleteLearningGoal)
		api.POST("/analytics/goals/:id/complete", analyticsHandler.CompleteLearningGoal)

		// AI usage tracking
		api.GET("/analytics/ai-usage", analyticsHandler.GetAIUsageStats)
		api.POST("/analytics/ai-interactions", analyticsHandler.TrackAIInteraction)
	}

	// Start server
	go func() {
		if err := e.Start(cfg.GetServerAddress()); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	logger.Info("analytics service started", zap.String("port", cfg.Port))

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down analytics service")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server", zap.Error(err))
	}

	logger.Info("analytics service stopped")
}

// customValidator wraps go-playground validator for Echo.
type customValidator struct {
	validator *validator.Validate
}

func (cv *customValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// Middleware

func loggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)

		latency := time.Since(start)
		status := c.Response().Status

		logger.Info("request completed",
			zap.String("method", c.Request().Method),
			zap.String("path", c.Request().URL.Path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("ip", c.RealIP()),
		)

		return err
	}
}

func correlationIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		correlationID := c.Request().Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = generateCorrelationID()
		}

		c.Set("correlation_id", correlationID)
		c.Response().Header().Set("X-Correlation-ID", correlationID)

		return next(c)
	}
}

func generateCorrelationID() string {
	return fmt.Sprintf("%d-%s", time.Now().Unix(), generateRandomString(8))
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
