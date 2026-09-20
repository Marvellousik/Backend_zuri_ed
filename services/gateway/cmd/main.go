// Package main is the entry point for the API Gateway.
package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	
	"zuri/services/gateway/internal/config"
	"zuri/services/gateway/internal/handler"
	"zuri/services/gateway/internal/middleware"
	"zuri/services/gateway/internal/proxy"
	"zuri/shared/pkg/auth"
	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/redis"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	
	// Initialize logger
	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(err)
	}
	defer logger.Sync()
	
	logger.Info("starting api gateway",
		zap.String("port", cfg.Port),
		zap.String("environment", cfg.Environment),
	)
	
	// Initialize Redis client
	redisClient, err := redis.NewClient(&redis.Config{
		Addr: cfg.RedisURL,
	})
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()
	
	// Verify Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	cancel()
	
	logger.Info("connected to redis")
	
	// Load RSA public key from User Service
	publicKey, err := loadPublicKey(cfg.UserServiceURL)
	if err != nil {
		logger.Fatal("failed to load public key", zap.Error(err))
	}
	
	logger.Info("loaded rsa public key")
	
	// Create reverse proxy
	reverseProxy := proxy.NewReverseProxy(
		cfg.CircuitBreakerThreshold,
		cfg.CircuitBreakerTimeout,
		cfg.InternalAPIKey,
	)
	
	// Setup Echo
	e := echo.New()
	e.HideBanner = true
	
	// Recovery middleware
	e.Use(echomiddleware.Recover())
	
	// CORS middleware
	e.Use(middleware.CORSMiddleware(middleware.DefaultCORSConfig(cfg.AllowedOrigins)))
	
	// Correlation ID middleware
	e.Use(correlationIDMiddleware())
	
	// Rate limiting middleware (applied to authenticated groups so user_id is available)
	rateLimiter := middleware.RateLimitMiddleware(&middleware.RateLimitConfig{
		RedisClient:    redisClient,
		DefaultRPM:     cfg.RateLimitRPM,
		AIRPM:          cfg.AIRateLimitRPM,
		AIDailyQuota:   cfg.AIDailyQuota,
		AIPathPrefixes: []string{"/api/v1/ai/", "/api/v1/reading/", "/api/v1/study/", "/api/v1/writing/"},
	})
	
	// Logger middleware
	e.Use(loggerMiddleware())
	
	// Register routes
	gatewayHandler := handler.NewGatewayHandler(cfg, reverseProxy, publicKey, rateLimiter, redisClient)
	gatewayHandler.RegisterRoutes(e)
	
	// Start server
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()
	
	logger.Info("api gateway started", zap.String("port", cfg.Port))
	
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("shutting down api gateway")
	
	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to gracefully shutdown server", zap.Error(err))
	}
	
	logger.Info("api gateway stopped")
}

// loadPublicKey fetches the RSA public key from User Service.
func loadPublicKey(userServiceURL string) (*rsa.PublicKey, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get(userServiceURL + "/api/v1/auth/public-key")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	
	var result struct {
		Data struct {
			PublicKey string `json:"public_key"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return auth.LoadPublicKeyFromPEM([]byte(result.Data.PublicKey))
}

// correlationIDMiddleware extracts or generates correlation ID.
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

// loggerMiddleware logs requests with structured logging.
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
