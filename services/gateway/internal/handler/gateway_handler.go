// Package handler provides HTTP handlers for the Gateway.
package handler

import (
	"crypto/rsa"
	"net/http"
	"strings"
	
	"github.com/labstack/echo/v4"
	
	"zuri/services/gateway/internal/config"
	"zuri/services/gateway/internal/middleware"
	"zuri/services/gateway/internal/proxy"
)

// GatewayHandler handles gateway requests.
type GatewayHandler struct {
	config        *config.Config
	proxy         *proxy.ReverseProxy
	jwtValidator  *middleware.JWTValidator
	rateLimiter   echo.MiddlewareFunc
}

// NewGatewayHandler creates a new gateway handler.
func NewGatewayHandler(cfg *config.Config, p *proxy.ReverseProxy, publicKey *rsa.PublicKey, rateLimiter echo.MiddlewareFunc, redisClient ...middleware.RedisClient) *GatewayHandler {
	var rc middleware.RedisClient
	if len(redisClient) > 0 {
		rc = redisClient[0]
	}
	return &GatewayHandler{
		config:       cfg,
		proxy:        p,
		jwtValidator: middleware.NewJWTValidator(publicKey, rc),
		rateLimiter:  rateLimiter,
	}
}

// RegisterRoutes registers all routes.
func (h *GatewayHandler) RegisterRoutes(e *echo.Echo) {
	// Public routes (no auth required)
	public := e.Group("")
	{
		// Health check
		public.GET("/health", h.HealthCheck)
		
		// User Service - Public auth endpoints
		public.POST("/api/v1/auth/register", h.ProxyToUserService)
		public.POST("/api/v1/auth/login", h.ProxyToUserService)
		public.POST("/api/v1/auth/refresh", h.ProxyToUserService)
		public.POST("/api/v1/auth/verify-email", h.ProxyToUserService)
		public.POST("/api/v1/auth/resend-verification", h.ProxyToUserService)
		public.POST("/api/v1/auth/forgot-password", h.ProxyToUserService)
		public.POST("/api/v1/auth/reset-password", h.ProxyToUserService)
		public.GET("/api/v1/auth/public-key", h.ProxyToUserService)
	}
	
	// Protected routes (JWT required)
	protected := e.Group("/api/v1")
	protected.Use(middleware.JWTMiddleware(
		h.jwtValidator,
		middleware.PublicKeySkipper([]string{
			"/api/v1/auth/register",
			"/api/v1/auth/login",
			"/api/v1/auth/refresh",
			"/api/v1/auth/verify-email",
			"/api/v1/auth/resend-verification",
			"/api/v1/auth/forgot-password",
			"/api/v1/auth/reset-password",
			"/api/v1/auth/public-key",
		}),
	))
	protected.Use(h.rateLimiter)
	{
		// User Service
		protected.POST("/auth/logout", h.ProxyToUserService)
		protected.POST("/auth/logout-all", h.ProxyToUserService)
		protected.GET("/users/me", h.ProxyToUserService)
		protected.PUT("/users/me", h.ProxyToUserService)
		protected.POST("/users/me/change-password", h.ProxyToUserService)
		protected.GET("/users/me/sessions", h.ProxyToUserService)
		protected.DELETE("/users/me/sessions/:id", h.ProxyToUserService)
		protected.GET("/users/me/memberships", h.ProxyToUserService)
		protected.POST("/users/me/memberships", h.ProxyToUserService)
		protected.POST("/users/me/switch-institution", h.ProxyToUserService)
		
		// Content Service
		protected.GET("/courses", h.ProxyToContentService)
		protected.POST("/courses", h.ProxyToContentService)
		protected.GET("/courses/:id", h.ProxyToContentService)
		protected.PUT("/courses/:id", h.ProxyToContentService)
		protected.DELETE("/courses/:id", h.ProxyToContentService)
		protected.GET("/courses/:id/materials", h.ProxyToContentService)
		protected.GET("/materials", h.ProxyToContentService)
		protected.POST("/materials", h.ProxyToContentService)
		protected.GET("/materials/:id", h.ProxyToContentService)
		protected.PUT("/materials/:id", h.ProxyToContentService)
		protected.DELETE("/materials/:id", h.ProxyToContentService)
		protected.POST("/materials/:id/presign", h.ProxyToContentService)
		protected.POST("/webhooks/material-uploaded", h.ProxyToContentService)
		
		// Quizzes
		protected.GET("/quizzes", h.ProxyToContentService)
		protected.POST("/quizzes", h.ProxyToContentService)
		protected.GET("/quizzes/:id", h.ProxyToContentService)
		protected.POST("/quizzes/:id/submit", h.ProxyToAnalyticsService)
		
		// Flashcards
		protected.GET("/flashcard-decks", h.ProxyToContentService)
		protected.POST("/flashcard-decks", h.ProxyToContentService)
		protected.GET("/flashcard-decks/:id", h.ProxyToContentService)
		
		// Analytics Service
		protected.GET("/analytics/study-streak", h.ProxyToAnalyticsService)
		protected.GET("/analytics/study-stats", h.ProxyToAnalyticsService)
		protected.POST("/analytics/study-sessions", h.ProxyToAnalyticsService)
		protected.GET("/analytics/topic-mastery", h.ProxyToAnalyticsService)
		protected.GET("/analytics/topics-for-review", h.ProxyToAnalyticsService)
		protected.GET("/analytics/quiz-history", h.ProxyToAnalyticsService)
		protected.GET("/analytics/goals", h.ProxyToAnalyticsService)
		protected.POST("/analytics/goals", h.ProxyToAnalyticsService)
		protected.PUT("/analytics/goals/:id", h.ProxyToAnalyticsService)
		protected.DELETE("/analytics/goals/:id", h.ProxyToAnalyticsService)
		protected.POST("/analytics/goals/:id/complete", h.ProxyToAnalyticsService)
		protected.GET("/analytics/ai-usage", h.ProxyToAnalyticsService)
		protected.POST("/analytics/ai-interactions", h.ProxyToAnalyticsService)
		
		// Quiz attempts (Analytics Service)
		protected.POST("/quizzes/:id/start", h.ProxyToAnalyticsService)
		protected.GET("/quiz-attempts", h.ProxyToAnalyticsService)
		protected.GET("/quiz-attempts/:id", h.ProxyToAnalyticsService)
		protected.POST("/quiz-attempts/:id/answers", h.ProxyToAnalyticsService)
		protected.POST("/quiz-attempts/:id/complete", h.ProxyToAnalyticsService)
		
		// Academic Service (Phase 1-4 Endpoints)
		protected.GET("/academic/courses", h.ProxyToAcademicService)
		protected.GET("/academic/courses/:id", h.ProxyToAcademicService)
		protected.POST("/academic/courses/enroll", h.ProxyToAcademicService)
		protected.GET("/academic/timetable", h.ProxyToAcademicService)
		protected.GET("/academic/timetable/next", h.ProxyToAcademicService)
		protected.POST("/academic/timetable/slots", h.ProxyToAcademicService)
		protected.GET("/academic/context-gaps", h.ProxyToAcademicService)
		protected.POST("/academic/context-gaps/:id/resolve", h.ProxyToAcademicService)
		protected.POST("/academic/learning-signals", h.ProxyToAcademicService)
		protected.GET("/academic/courses/:id/knowledge-state", h.ProxyToAcademicService)
		protected.GET("/academic/courses/:id/weak-topics", h.ProxyToAcademicService)
		protected.GET("/academic/courses/:id/lecturer-signals", h.ProxyToAcademicService)
		protected.POST("/academic/courses/:id/partner/chat", h.ProxyToAcademicService)
		protected.GET("/academic/courses/:id/partner/history", h.ProxyToAcademicService)
		protected.GET("/academic/today", h.ProxyToAcademicService)
		protected.POST("/academic/today/interventions/:id/act", h.ProxyToAcademicService)
		protected.POST("/academic/today/interventions/:id/dismiss", h.ProxyToAcademicService)
		protected.POST("/academic/today/events/dispatch", h.ProxyToAcademicService)
		protected.POST("/academic/today/events/notify", h.ProxyToAcademicService)
		protected.POST("/academic/courses/:id/lectures/:lecture_id/ingest", h.ProxyToAcademicService)
		
		// AI Service (Control Plane, 13-stage Pipeline & Async Jobs)
		protected.POST("/ai/execute", h.ProxyToAIService)
		protected.POST("/ai/execute/stream", h.ProxyToAIService)
		protected.POST("/ai/jobs", h.ProxyToAIService)
		protected.GET("/ai/jobs/:id", h.ProxyToAIService)
		protected.POST("/ai/jobs/:id/cancel", h.ProxyToAIService)
		protected.GET("/ai/analytics/costs", h.ProxyToAIService)
		
		// Notification Service
		protected.GET("/notifications/preferences", h.ProxyToNotificationService)
		protected.PUT("/notifications/preferences", h.ProxyToNotificationService)
		protected.POST("/notifications/devices/register", h.ProxyToNotificationService)
		protected.DELETE("/notifications/devices/:token", h.ProxyToNotificationService)
		protected.GET("/notifications/reminders", h.ProxyToNotificationService)
		protected.POST("/notifications/reminders", h.ProxyToNotificationService)
		protected.DELETE("/notifications/reminders/:id", h.ProxyToNotificationService)
		protected.GET("/notifications/history", h.ProxyToNotificationService)
		
		// Sync Service
		protected.GET("/sync/state", h.ProxyToSyncService)
		protected.POST("/sync/ack", h.ProxyToSyncService)
		protected.GET("/sync/events", h.ProxyToSyncService)
		protected.POST("/sync/events", h.ProxyToSyncService)
		protected.GET("/events", h.ProxyToSyncService)
		protected.POST("/events", h.ProxyToSyncService)
		protected.GET("/presence", h.ProxyToSyncService)
		protected.PUT("/presence", h.ProxyToSyncService)
		protected.GET("/presence/online", h.ProxyToSyncService)
	}
	
	// WebSocket endpoint (handled separately for upgrade support)
	ws := e.Group("/api/v1")
	ws.Use(middleware.JWTMiddleware(
		h.jwtValidator,
		middleware.PublicKeySkipper([]string{}),
	))
	ws.Use(h.rateLimiter)
	{
		// WebSocket upgrade endpoint for sync service
		ws.GET("/ws", h.ProxyWebSocketToSyncService)
	}
}

// HealthCheck handles health check requests.
func (h *GatewayHandler) HealthCheck(c echo.Context) error {
	// Check upstream services
	services := map[string]string{
		"user":         h.config.UserServiceURL,
		"content":      h.config.ContentServiceURL,
		"analytics":    h.config.AnalyticsServiceURL,
		"notification": h.config.NotificationServiceURL,
		"sync":         h.config.SyncServiceURL,
		"academic":     h.config.AcademicServiceURL,
		"ai":           h.config.AIServiceURL,
	}
	
	status := make(map[string]string)
	overallStatus := "healthy"
	
	for name, url := range services {
		if err := h.proxy.HealthCheck(url); err != nil {
			status[name] = "unhealthy"
			overallStatus = "degraded"
		} else {
			status[name] = "healthy"
		}
	}
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"service":  "gateway",
		"status":   overallStatus,
		"upstream": status,
	})
}

// ProxyToUserService proxies to User Service.
func (h *GatewayHandler) ProxyToUserService(c echo.Context) error {
	targetURL := h.config.UserServiceURL
	injectUserID := c.Request().Method != "GET" || strings.Contains(c.Request().URL.Path, "/me")
	return h.proxy.ProxyRequest(c, targetURL, "user", injectUserID)
}

// ProxyToContentService proxies to Content Service.
func (h *GatewayHandler) ProxyToContentService(c echo.Context) error {
	targetURL := h.config.ContentServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "content", true)
}

// ProxyToAnalyticsService proxies to Analytics Service.
func (h *GatewayHandler) ProxyToAnalyticsService(c echo.Context) error {
	targetURL := h.config.AnalyticsServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "analytics", true)
}

// ProxyToAcademicService proxies to Academic Service.
func (h *GatewayHandler) ProxyToAcademicService(c echo.Context) error {
	targetURL := h.config.AcademicServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "academic", true)
}

// ProxyToAIService proxies to AI Service with circuit breaker.
func (h *GatewayHandler) ProxyToAIService(c echo.Context) error {
	targetURL := h.config.AIServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "ai", true)
}

// ProxyToNotificationService proxies to Notification Service.
func (h *GatewayHandler) ProxyToNotificationService(c echo.Context) error {
	targetURL := h.config.NotificationServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "notification", true)
}

// ProxyToSyncService proxies to Sync Service.
func (h *GatewayHandler) ProxyToSyncService(c echo.Context) error {
	targetURL := h.config.SyncServiceURL
	return h.proxy.ProxyRequest(c, targetURL, "sync", true)
}

// ProxyWebSocketToSyncService proxies WebSocket connections to Sync Service.
func (h *GatewayHandler) ProxyWebSocketToSyncService(c echo.Context) error {
	targetURL := h.config.SyncServiceURL
	return h.proxy.ProxyWebSocket(c, targetURL+"/api/v1/sync", true)
}
