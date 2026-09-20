package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/redis"
)

// AIQuotaConfig holds AI quota configuration.
type AIQuotaConfig struct {
	RedisClient  *redis.Client
	DailyQuota   int
	PathPrefixes []string
}

// AIDailyQuotaMiddleware returns middleware that enforces a daily quota per user for AI endpoints.
func AIDailyQuotaMiddleware(config *AIQuotaConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			if !isAIEndpoint(path, config.PathPrefixes) {
				return next(c)
			}

			userID := c.Get("user_id")
			if userID == nil {
				return next(c)
			}

			uid, ok := userID.(string)
			if !ok || uid == "" {
				return next(c)
			}

			ctx := c.Request().Context()
			key := "ai_quota:" + uid

			count, err := config.RedisClient.Increment(ctx, key)
			if err != nil {
				logger.Error("ai quota increment failed", zap.Error(err))
				return next(c)
			}

			// Set TTL on first request of the day
			if count == 1 {
				if err := config.RedisClient.Expire(ctx, key, 24*time.Hour); err != nil {
					logger.Error("ai quota expire failed", zap.Error(err))
				}
			}

			remaining := config.DailyQuota - int(count)
			if remaining < 0 {
				remaining = 0
			}

			c.Response().Header().Set("X-AI-Quota-Limit", strconv.Itoa(config.DailyQuota))
			c.Response().Header().Set("X-AI-Quota-Remaining", strconv.Itoa(remaining))

			if int(count) > config.DailyQuota {
				c.Response().Header().Set("X-AI-Quota-Exceeded", "true")
				return echo.NewHTTPError(http.StatusTooManyRequests, "daily ai quota exceeded")
			}

			return next(c)
		}
	}
}
