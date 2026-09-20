package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds content service configuration.
type Config struct {
	// Server
	Port        string
	Environment string
	LogLevel    string

	// Database
	DatabaseURL string

	// JWT (for token validation if needed directly)
	JWTPublicKeyPath string

	// Service settings
	MaxUploadSizeMB int
	DefaultPageSize int
	MaxPageSize     int

	// MinIO / S3-compatible storage
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	MinIOPublicURL string
}

// Load loads configuration from environment variables.
func Load() *Config {
	return &Config{
		// Server
		Port:        getEnv("PORT", "8082"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://zuri:zuri_secret@localhost:5432/zuri?sslmode=disable"),

		// JWT
		JWTPublicKeyPath: getEnv("JWT_PUBLIC_KEY_PATH", "/app/config/jwt_public.pem"),

		// Service settings
		MaxUploadSizeMB: getEnvAsInt("MAX_UPLOAD_SIZE_MB", 50),
		DefaultPageSize: getEnvAsInt("DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:     getEnvAsInt("MAX_PAGE_SIZE", 100),

		// MinIO / S3-compatible storage
		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin_secret"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "zuri-materials"),
		MinIOUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		MinIOPublicURL: getEnv("MINIO_PUBLIC_URL", ""),
	}
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetMaxUploadSize returns the maximum upload size in bytes.
func (c *Config) GetMaxUploadSize() int64 {
	return int64(c.MaxUploadSizeMB) * 1024 * 1024
}

// GetServerAddress returns the server address.
func (c *Config) GetServerAddress() string {
	return ":" + c.Port
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
