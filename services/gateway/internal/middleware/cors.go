package middleware

import (
	"net/http"
	"strconv"
	"strings"
	
	"github.com/labstack/echo/v4"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

// DefaultCORSConfig returns default CORS configuration.
func DefaultCORSConfig(allowedOrigins []string) *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodPatch,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
			"X-Correlation-ID",
			"X-User-ID",
		},
		MaxAge: 86400,
	}
}

// CORSMiddleware returns CORS middleware.
func CORSMiddleware(config *CORSConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")
			
			// Check if origin is allowed
			allowOrigin := ""
			for _, allowed := range config.AllowedOrigins {
				if allowed == "*" {
					// When credentials are used, return the actual origin instead of wildcard
					allowOrigin = origin
					break
				}
				if allowed == origin {
					allowOrigin = origin
					break
				}
			}
			
			// Set CORS headers
			if allowOrigin != "" {
				c.Response().Header().Set("Access-Control-Allow-Origin", allowOrigin)
				c.Response().Header().Set("Vary", "Origin")
			}
			
			c.Response().Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			c.Response().Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			c.Response().Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
			
			// Handle preflight requests
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			
			return next(c)
		}
	}
}
