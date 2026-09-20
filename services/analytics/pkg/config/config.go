package config

import (
	"os"
	"strconv"
)

// Config holds analytics service configuration.
type Config struct {
	// Server
	Port        string
	Environment string
	LogLevel    string

	// Database
	DatabaseURL string

	// Service settings
	DefaultPageSize int
	MaxPageSize     int
}

// Load loads configuration from environment variables.
func Load() *Config {
	return &Config{
		// Server
		Port:        getEnv("PORT", "8083"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://zuri:zuri_secret@localhost:5432/zuri?sslmode=disable"),

		// Service settings
		DefaultPageSize: getEnvAsInt("DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:     getEnvAsInt("MAX_PAGE_SIZE", 100),
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
