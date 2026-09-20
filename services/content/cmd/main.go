package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"zuri/services/content/internal/handler"
	"zuri/services/content/internal/repository"
	"zuri/services/content/internal/service"
	contentConfig "zuri/services/content/pkg/config"
	"zuri/shared/pkg/database"
	"zuri/shared/pkg/logger"
)

func main() {
	// Load configuration
	cfg := contentConfig.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("starting content service",
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
	courseRepo := repository.NewCourseRepository(db)
	materialRepo := repository.NewMaterialRepository(db)
	quizRepo := repository.NewQuizRepository(db)
	flashcardRepo := repository.NewFlashcardRepository(db)

	// Initialize MinIO client
	endpoint := cfg.MinIOEndpoint
	useSSL := cfg.MinIOUseSSL

	if cfg.MinIOPublicURL != "" {
		if u, err := url.Parse(cfg.MinIOPublicURL); err == nil {
			endpoint = u.Host
			useSSL = (u.Scheme == "https")
		}
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: useSSL,
		Region: "us-east-1",
	})
	if err != nil {
		logger.Fatal("failed to initialize MinIO client", zap.Error(err))
	}
	logger.Info("MinIO client initialized", zap.String("endpoint", endpoint), zap.String("bucket", cfg.MinIOBucket))

	// Ensure the bucket exists (Non-blocking check to prevent startup crash if public DNS is unreachable internally)
	go func() {
		ctxBucket, cancelBucket := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelBucket()

		exists, err := minioClient.BucketExists(ctxBucket, cfg.MinIOBucket)
		if err != nil {
			logger.Warn("failed to check if MinIO bucket exists (skipping auto-creation)", zap.Error(err))
			return
		}
		if !exists {
			logger.Info("MinIO bucket does not exist, creating it", zap.String("bucket", cfg.MinIOBucket))
			err = minioClient.MakeBucket(ctxBucket, cfg.MinIOBucket, minio.MakeBucketOptions{
				Region: "us-east-1",
			})
			if err != nil {
				logger.Error("failed to create MinIO bucket", zap.Error(err))
				return
			}
			logger.Info("MinIO bucket created successfully", zap.String("bucket", cfg.MinIOBucket))
		} else {
			logger.Info("MinIO bucket already exists", zap.String("bucket", cfg.MinIOBucket))
		}

		// Set bucket policy to public read for materials/* so the ingestion service can download files
		policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + cfg.MinIOBucket + `/materials/*"]}]}`
		err = minioClient.SetBucketPolicy(ctxBucket, cfg.MinIOBucket, policy)
		if err != nil {
			logger.Warn("failed to set MinIO bucket policy", zap.Error(err))
		} else {
			logger.Info("MinIO bucket policy set to public read for materials/*", zap.String("bucket", cfg.MinIOBucket))
		}
	}()

	// Initialize services
	contentService := service.NewContentService(
		courseRepo,
		materialRepo,
		quizRepo,
		flashcardRepo,
		minioClient,
		cfg.MinIOBucket,
		cfg.MinIOPublicURL,
	)

	// Initialize handlers
	courseHandler := handler.NewCourseHandler(contentService)
	materialHandler := handler.NewMaterialHandler(contentService)
	quizHandler := handler.NewQuizHandler(contentService)
	flashcardHandler := handler.NewFlashcardHandler(contentService)

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

	// Course routes
	api := e.Group("/api/v1")
	{
		// Courses
		api.POST("/courses", courseHandler.CreateCourse)
		api.GET("/courses", courseHandler.GetUserCourses)
		api.GET("/courses/:id", courseHandler.GetCourse)
		api.PUT("/courses/:id", courseHandler.UpdateCourse)
		api.DELETE("/courses/:id", courseHandler.DeleteCourse)

		// Course materials
		api.GET("/courses/:id/materials", materialHandler.GetCourseMaterials)

		// Materials
		api.POST("/materials", materialHandler.CreateMaterial)
		api.GET("/materials", materialHandler.GetUserMaterials)
		api.GET("/materials/:id", materialHandler.GetMaterial)
		api.PUT("/materials/:id", materialHandler.UpdateMaterial)
		api.DELETE("/materials/:id", materialHandler.DeleteMaterial)
		api.POST("/materials/:id/presign", materialHandler.PresignMaterial)
		api.POST("/internal/materials/processing-status", materialHandler.UpdateProcessingStatus)

		// Quizzes
		api.POST("/quizzes", quizHandler.CreateQuiz)
		api.GET("/quizzes", quizHandler.GetUserQuizzes)
		api.GET("/quizzes/:id", quizHandler.GetQuiz)
		api.PUT("/quizzes/:id", quizHandler.UpdateQuiz)
		api.DELETE("/quizzes/:id", quizHandler.DeleteQuiz)

		// Course quizzes
		api.GET("/courses/:id/quizzes", quizHandler.GetCourseQuizzes)

		// Quiz questions
		api.POST("/quizzes/:id/questions", quizHandler.AddQuestion)
		api.PUT("/quizzes/questions/:id", quizHandler.UpdateQuestion)
		api.DELETE("/quizzes/questions/:id", quizHandler.DeleteQuestion)

		// Flashcard decks
		api.POST("/flashcard-decks", flashcardHandler.CreateDeck)
		api.GET("/flashcard-decks", flashcardHandler.GetUserDecks)
		api.GET("/flashcard-decks/:id", flashcardHandler.GetDeck)
		api.PUT("/flashcard-decks/:id", flashcardHandler.UpdateDeck)
		api.DELETE("/flashcard-decks/:id", flashcardHandler.DeleteDeck)

		// Flashcards
		api.POST("/flashcard-decks/:id/cards", flashcardHandler.AddCard)
		api.PUT("/flashcards/:id", flashcardHandler.UpdateCard)
		api.DELETE("/flashcards/:id", flashcardHandler.DeleteCard)
	}

	// Start server
	go func() {
		if err := e.Start(cfg.GetServerAddress()); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	logger.Info("content service started", zap.String("port", cfg.Port))

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down content service")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server", zap.Error(err))
	}

	logger.Info("content service stopped")
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
			zap.String("user_agent", c.Request().UserAgent()),
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
