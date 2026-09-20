// Package config provides Gateway-specific configuration.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds Gateway configuration.
type Config struct {
	Port                    string
	Environment             string
	LogLevel                string
	
	// Redis
	RedisURL                string
	
	// Rate Limiting
	RateLimitRPM            int
	AIRateLimitRPM          int
	AIDailyQuota            int
	
	// Circuit Breaker
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration
	
	// CORS
	AllowedOrigins          []string
	
	// Go Upstream Services
	UserServiceURL          string
	ContentServiceURL       string
	AnalyticsServiceURL     string
	NotificationServiceURL  string
	SyncServiceURL          string
	AcademicServiceURL      string
	
	// AI Service & Providers
	AIServiceURL            string
	AIServiceTimeout        time.Duration
	OpenRouterAPIKey        string
	
	// Security
	InternalAPIKey          string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		Environment:             getEnv("ENVIRONMENT", "development"),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		RedisURL:                getEnv("REDIS_URL", "localhost:6379"),
		RateLimitRPM:            getEnvInt("RATE_LIMIT_RPM", 100),
		AIRateLimitRPM:          getEnvInt("AI_RATE_LIMIT_RPM", 100),	// Increased from 20 for development
		AIDailyQuota:            getEnvInt("AI_DAILY_QUOTA", 50),
		CircuitBreakerThreshold: getEnvInt("CIRCUIT_BREAKER_THRESHOLD", 3),
		CircuitBreakerTimeout:   getEnvDuration("CIRCUIT_BREAKER_TIMEOUT", 60*time.Second),
		AllowedOrigins:          getEnvSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		UserServiceURL:          getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		ContentServiceURL:       getEnv("CONTENT_SERVICE_URL", "http://localhost:8082"),
		AnalyticsServiceURL:     getEnv("ANALYTICS_SERVICE_URL", "http://localhost:8083"),
		NotificationServiceURL:  getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8084"),
		SyncServiceURL:          getEnv("SYNC_SERVICE_URL", "http://localhost:8085"),
		AcademicServiceURL:      getEnv("ACADEMIC_SERVICE_URL", "http://localhost:8086"),
		AIServiceURL:            getEnv("AI_SERVICE_URL", "http://localhost:5005"),
		AIServiceTimeout:        getEnvDuration("AI_SERVICE_TIMEOUT", 120*time.Second),
		OpenRouterAPIKey:        getEnv("OPENROUTER_API_KEY", ""),
		InternalAPIKey:          getEnv("INTERNAL_API_KEY", "dev-internal-key"),
	}
	
	return cfg, nil
}

// GetServiceURL returns the URL for a given service name.
func (c *Config) GetServiceURL(serviceName string) string {
	switch serviceName {
	case "user":
		return c.UserServiceURL
	case "content":
		return c.ContentServiceURL
	case "analytics":
		return c.AnalyticsServiceURL
	case "notification":
		return c.NotificationServiceURL
	case "sync":
		return c.SyncServiceURL
	case "academic", "courses", "timetable":
		return c.AcademicServiceURL
	case "ai":
		return c.AIServiceURL
	default:
		return ""
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
