// Package config provides environment variable loading with validation.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	// Common settings
	ServiceName string
	Environment string
	LogLevel    string
	Port        string

	// Database
	DatabaseURL string

	// Redis
	RedisURL      string
	RedisPassword string
	RedisDB       int

	// Model Provider Keys
	OpenRouterAPIKey string

	// Service-specific configs
	UserService     *UserServiceConfig
	GatewayService  *GatewayConfig
	ContentService  *ContentConfig
	AnalyticsConfig *AnalyticsConfig
	Notification    *NotificationConfig
}

// UserServiceConfig holds User Service specific configuration.
type UserServiceConfig struct {
	PrivateKeyEncryptionKey string
	BcryptCost              int
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	VerificationCodeTTL     time.Duration
	SMTPHost                string
	SMTPPort                int
	SMTPUser                string
	SMTPPassword            string
	SMTPFrom                string
	FrontendURL             string
	NotificationServiceURL  string
	InternalAPIKey          string
}

// GatewayConfig holds Gateway specific configuration.
type GatewayConfig struct {
	PublicKeyPath     string
	RateLimitRPM      int
	AIRateLimitRPM    int
	AllowedOrigins    []string
	UserServiceURL    string
	ContentServiceURL string
	OpenRouterAPIKey  string
}

// ContentConfig holds Content Service specific configuration.
type ContentConfig struct {
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string
}

// AnalyticsConfig holds Analytics Service specific configuration.
type AnalyticsConfig struct {
	QuizAnswerSimilarityThreshold float64
}

// NotificationConfig holds Notification Service specific configuration.
type NotificationConfig struct {
	FCMCredentialsJSON string
	QuietHoursStart    int
	QuietHoursEnd      int
}

// Loader handles configuration loading and validation.
type Loader struct {
	required []string
	defaults map[string]string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{
		defaults: make(map[string]string),
	}
}

// Require marks environment variables as required.
func (l *Loader) Require(vars ...string) *Loader {
	l.required = append(l.required, vars...)
	return l
}

// Default sets default values for environment variables.
func (l *Loader) Default(key, value string) *Loader {
	l.defaults[key] = value
	return l
}

// Load loads and validates the configuration.
func (l *Loader) Load() (*Config, error) {
	// Check required variables
	for _, key := range l.required {
		if os.Getenv(key) == "" {
			if defaultVal, ok := l.defaults[key]; ok {
				os.Setenv(key, defaultVal)
			} else {
				return nil, fmt.Errorf("required environment variable %s is not set", key)
			}
		}
	}

	cfg := &Config{
		ServiceName:      getEnv("SERVICE_NAME", "zuri"),
		Environment:      getEnv("ENVIRONMENT", "development"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisURL:         getEnv("REDIS_URL", "localhost:6379"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
	}

	// Parse Redis DB
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
		}
		cfg.RedisDB = db
	}

	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")

	return cfg, nil
}

// LoadUserServiceConfig loads User Service specific configuration.
func (l *Loader) LoadUserServiceConfig() (*UserServiceConfig, error) {
	bcryptCost, err := strconv.Atoi(getEnv("BCRYPT_COST", "12"))
	if err != nil {
		return nil, fmt.Errorf("invalid BCRYPT_COST: %w", err)
	}

	accessTokenTTL, err := time.ParseDuration(getEnv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}

	refreshTokenTTL, err := time.ParseDuration(getEnv("REFRESH_TOKEN_TTL", "720h")) // 30 days
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	return &UserServiceConfig{
		PrivateKeyEncryptionKey: os.Getenv("PRIVATE_KEY_ENCRYPTION_KEY"),
		BcryptCost:              bcryptCost,
		AccessTokenTTL:          accessTokenTTL,
		RefreshTokenTTL:         refreshTokenTTL,
		VerificationCodeTTL:     15 * time.Minute,
		SMTPHost:                os.Getenv("SMTP_HOST"),
		SMTPPort:                smtpPort,
		SMTPUser:                os.Getenv("SMTP_USER"),
		SMTPPassword:            os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                getEnv("SMTP_FROM", "noreply@zuri.app"),
		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:3000"),
		NotificationServiceURL:  getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8084"),
		InternalAPIKey:          getEnv("INTERNAL_API_KEY", "dev-internal-key"),
	}, nil
}

// LoadGatewayConfig loads Gateway specific configuration.
func (l *Loader) LoadGatewayConfig() (*GatewayConfig, error) {
	rateLimit, err := strconv.Atoi(getEnv("RATE_LIMIT_RPM", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_RPM: %w", err)
	}

	aiRateLimit, err := strconv.Atoi(getEnv("AI_RATE_LIMIT_RPM", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid AI_RATE_LIMIT_RPM: %w", err)
	}

	origins := strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return &GatewayConfig{
		PublicKeyPath:     os.Getenv("PUBLIC_KEY_PATH"),
		RateLimitRPM:      rateLimit,
		AIRateLimitRPM:    aiRateLimit,
		AllowedOrigins:    origins,
		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		ContentServiceURL: getEnv("CONTENT_SERVICE_URL", "http://localhost:8082"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
	}, nil
}

// LoadContentConfig loads Content Service specific configuration.
func (l *Loader) LoadContentConfig() (*ContentConfig, error) {
	return &ContentConfig{
		S3Endpoint:  getEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    getEnv("S3_BUCKET", "zuri-content"),
		S3Region:    getEnv("S3_REGION", "us-east-1"),
	}, nil
}

// getEnv retrieves an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MustLoad loads configuration and panics on error.
func MustLoad(loader func() (*Config, error)) *Config {
	cfg, err := loader()
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}
