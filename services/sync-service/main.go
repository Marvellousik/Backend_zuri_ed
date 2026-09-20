package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"zuri/services/sync-service/handlers"
	"zuri/services/sync-service/websocket"

	"zuri/shared/pkg/config"
	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/middleware"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		logger.Warn(fmt.Sprintf("No .env file found: %v", err))
	}

	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	if err := logger.Initialize(logLevel); err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize logger: %v", err))
		os.Exit(1)
	}
	logger.Info("Starting Sync Service...")

	// Load configuration
	cfgLoader := config.NewLoader().
		Default("PORT", "8085").
		Default("DATABASE_URL", "postgres://zuri:zuri_secret@localhost:5432/zuri?sslmode=disable")
	cfg, err := cfgLoader.Load()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to connect to database: %v", err))
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("Connected to database")

	// Create WebSocket hub
	hub := websocket.NewHub(db)
	go hub.Run()

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	// CORS is handled by the Gateway - don't set here to avoid duplicate headers
	// r.Use(middleware.CORS())

	// Register routes
	h := handlers.NewHandler(db, hub)
	h.RegisterRoutes(r)

	// Get port
	port := cfg.Port

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(fmt.Sprintf("Failed to start server: %v", err))
			os.Exit(1)
		}
	}()

	logger.Info(fmt.Sprintf("Sync Service running on port %s", port))
	logger.Info(fmt.Sprintf("WebSocket endpoint: ws://localhost:%s/api/v1/sync", port))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(fmt.Sprintf("Server forced to shutdown: %v", err))
	}

	logger.Info("Server exited")
}
