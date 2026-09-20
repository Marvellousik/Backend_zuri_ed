// Package main is the entry point for the User Service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"zuri/services/user/internal/client"
	"zuri/services/user/internal/handler"
	custommiddleware "zuri/services/user/internal/middleware"
	"zuri/services/user/internal/repository"
	"zuri/services/user/internal/service"
	"zuri/shared/pkg/config"
	"zuri/shared/pkg/database"
	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/redis"
)

func main() {
	// Load configuration
	cfgLoader := config.NewLoader().
		Require("DATABASE_URL", "REDIS_URL", "PRIVATE_KEY_ENCRYPTION_KEY").
		Default("PORT", "8081").
		Default("LOG_LEVEL", "info")

	cfg, err := cfgLoader.Load()
	if err != nil {
		panic(err)
	}

	userCfg, err := cfgLoader.LoadUserServiceConfig()
	if err != nil {
		panic(err)
	}

	// Initialize logger
	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("starting user service",
		zap.String("port", cfg.Port),
		zap.String("environment", cfg.Environment),
	)

	// Initialize database
	dbConfig := database.DefaultConfig(cfg.DatabaseURL)
	db, err := database.New(dbConfig)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	// Verify database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.HealthCheck(ctx); err != nil {
		logger.Fatal("database health check failed", zap.Error(err))
	}
	cancel()

	logger.Info("connected to database")

	// Initialize Redis
	redisConfig := &redis.Config{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}

	// Verify Redis connection
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	cancel()

	logger.Info("connected to redis")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db.DB)
	sessionRepo := repository.NewSessionRepository(db.DB)
	passwordResetRepo := repository.NewPasswordResetRepository(db.DB)
	jwtKeyRepo := repository.NewJWTKeyRepository(db.DB)
	membershipRepo := repository.NewMembershipRepository(db.DB)

	// Initialize notification client
	notificationClient := client.NewNotificationClient(
		userCfg.NotificationServiceURL,
		userCfg.InternalAPIKey,
	)

	// Initialize services
	userSvc, err := service.NewUserService(
		userRepo,
		refreshTokenRepo,
		sessionRepo,
		passwordResetRepo,
		jwtKeyRepo,
		membershipRepo,
		redisClient,
		notificationClient,
		userCfg,
	)
	if err != nil {
		logger.Fatal("failed to initialize user service", zap.Error(err))
	}

	// Initialize handlers
	authHandler := handler.NewAuthHandler(userSvc)
	userHandler := handler.NewUserHandler(userSvc)
	sessionHandler := handler.NewSessionHandler(userSvc)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	// Register custom validator
	custommiddleware.Register(e)

	// Setup middleware
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Logger())
	e.Use(correlationIDMiddleware())
	e.Use(loggerMiddleware())
	// CORS is handled by the Gateway - don't set here to avoid duplicate headers
	// e.Use(corsMiddleware())

	// Setup routes
	setupRoutes(e, authHandler, userHandler, sessionHandler)

	// Start server
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	logger.Info("user service started", zap.String("port", cfg.Port))

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down user service")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to gracefully shutdown server", zap.Error(err))
	}

	// Close database connection
	if err := db.Close(); err != nil {
		logger.Error("failed to close database connection", zap.Error(err))
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		logger.Error("failed to close redis connection", zap.Error(err))
	}

	logger.Info("user service stopped")
}

// setupRoutes configures all HTTP routes.
func setupRoutes(
	e *echo.Echo,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	sessionHandler *handler.SessionHandler,
) {
	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"service": "user-service",
		})
	})

	// Public routes (no authentication required)
	public := e.Group("/api/v1")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/refresh", authHandler.RefreshToken)
		public.POST("/auth/verify-email", authHandler.VerifyEmail)
		public.POST("/auth/resend-verification", authHandler.ResendVerification)
		public.POST("/auth/forgot-password", authHandler.RequestPasswordReset)
		public.POST("/auth/reset-password", authHandler.ResetPassword)
		public.GET("/auth/public-key", authHandler.GetPublicKey)
	}

	// Protected routes (authentication required)
	protected := e.Group("/api/v1")
	protected.Use(requireUserID())
	{
		// Auth
		protected.POST("/auth/logout", authHandler.Logout)
		protected.POST("/auth/logout-all", authHandler.LogoutAll)

		// User profile
		protected.GET("/users/me", userHandler.GetProfile)
		protected.PUT("/users/me", userHandler.UpdateProfile)
		protected.POST("/users/me/change-password", userHandler.ChangePassword)

		// Institutional Memberships
		protected.GET("/users/me/memberships", userHandler.ListMemberships)
		protected.POST("/users/me/memberships", userHandler.AddMembership)
		protected.POST("/users/me/switch-institution", userHandler.SwitchInstitution)

		// Sessions
		protected.GET("/users/me/sessions", sessionHandler.ListSessions)
		protected.DELETE("/users/me/sessions/:id", sessionHandler.RevokeSession)
	}
}

// correlationIDMiddleware extracts or generates a correlation ID for each request.
func correlationIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			correlationID := c.Request().Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = logger.GenerateCorrelationID()
			}

			// Store in context
			ctx := logger.SetCorrelationID(c.Request().Context(), correlationID)
			c.SetRequest(c.Request().WithContext(ctx))

			// Add to response headers
			c.Response().Header().Set("X-Correlation-ID", correlationID)

			return next(c)
		}
	}
}

// loggerMiddleware logs each request with structured logging.
func loggerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			latency := time.Since(start)
			correlationID := logger.GetCorrelationID(c.Request().Context())

			fields := []zap.Field{
				zap.String("method", c.Request().Method),
				zap.String("path", c.Request().URL.Path),
				zap.Int("status", c.Response().Status),
				zap.Duration("latency", latency),
				zap.String("ip", c.RealIP()),
				zap.String("user_agent", c.Request().UserAgent()),
				zap.String("correlation_id", correlationID),
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
				logger.Error("request failed", fields...)
			} else {
				logger.Info("request completed", fields...)
			}

			return err
		}
	}
}

// corsMiddleware configures CORS for the API.
func corsMiddleware() echo.MiddlewareFunc {
	return echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"}, // Configure based on environment
		AllowMethods: []string{
			echo.GET,
			echo.POST,
			echo.PUT,
			echo.DELETE,
			echo.PATCH,
			echo.OPTIONS,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
			"X-Correlation-ID",
			"X-User-ID",
		},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}

// requireUserID middleware ensures X-User-ID header is present.
func requireUserID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := c.Request().Header.Get("X-User-ID")
			if userID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
			}
			return next(c)
		}
	}
}
