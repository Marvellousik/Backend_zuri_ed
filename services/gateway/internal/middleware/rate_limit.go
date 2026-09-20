package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	
	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/redis"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	RedisClient    *redis.Client
	DefaultRPM     int
	AIRPM          int
	AIDailyQuota   int
	AIPathPrefixes []string
}

// isAIEndpoint checks if a path matches any of the AI endpoint prefixes.
func isAIEndpoint(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// RateLimitMiddleware returns middleware for rate limiting.
func RateLimitMiddleware(config *RateLimitConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prefer authenticated user ID; fall back to IP for unauthenticated routes
			identifier := c.Get("user_id")
			keyPrefix := "ratelimit:user"
			if identifier == nil || identifier == "" {
				identifier = c.RealIP()
				keyPrefix = "ratelimit:ip"
			}
			identifierStr := identifier.(string)
			
			path := c.Request().URL.Path
			
			// Determine rate limit based on path
			isAIEndpoint := false
			for _, prefix := range config.AIPathPrefixes {
				if strings.HasPrefix(path, prefix) {
					isAIEndpoint = true
					break
				}
			}
			
			maxRequests := config.DefaultRPM
			if isAIEndpoint {
				maxRequests = config.AIRPM
			}
			
			// Check rate limit
			allowed, remaining, resetAt, err := config.RedisClient.CheckRateLimit(
				c.Request().Context(),
				identifierStr,
				&redis.RateLimitConfig{
					Window:      time.Minute,
					MaxRequests: maxRequests,
					KeyPrefix:   keyPrefix,
				},
			)
			
			if err != nil {
				logger.Error("rate limit check failed", zap.Error(err))
				// Allow request on error (fail open)
				return next(c)
			}
			
			// Set rate limit headers
			c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Response().Header().Set("X-RateLimit-Reset", resetAt.Format(time.RFC3339))
			
			if !allowed {
				logger.Warn("rate limit exceeded",
					zap.String("identifier", identifierStr),
					zap.String("path", path),
					zap.Int("limit", maxRequests),
				)
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}
			
			// Daily AI quota enforcement
			logger.Info("quota check",
				zap.Bool("is_ai", isAIEndpoint),
				zap.String("key_prefix", keyPrefix),
				zap.Int("daily_quota", config.AIDailyQuota),
				zap.String("user_id", identifierStr),
			)
			if isAIEndpoint && keyPrefix == "ratelimit:user" && config.AIDailyQuota > 0 {
				today := time.Now().Format("2006-01-02")
				quotaKey := fmt.Sprintf("quota:user:%s:%s", identifierStr, today)
				
				count, err := config.RedisClient.Increment(c.Request().Context(), quotaKey)
				if err != nil {
					logger.Error("daily quota increment failed", zap.Error(err))
				} else {
					if count == 1 {
						_ = config.RedisClient.Expire(c.Request().Context(), quotaKey, 48*time.Hour)
					}
					
					c.Response().Header().Set("X-Quota-Limit", strconv.Itoa(config.AIDailyQuota))
					remaining := config.AIDailyQuota - int(count)
					if remaining < 0 {
						remaining = 0
					}
					c.Response().Header().Set("X-Quota-Remaining", strconv.Itoa(remaining))
					
					if count > int64(config.AIDailyQuota) {
						logger.Warn("daily AI quota exceeded",
							zap.String("user_id", identifierStr),
							zap.String("path", path),
							zap.Int64("count", count),
						)
						return echo.NewHTTPError(http.StatusTooManyRequests, "daily AI quota exceeded")
					}
				}
			}
			
			return next(c)
		}
	}
}
