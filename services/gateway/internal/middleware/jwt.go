// Package middleware provides HTTP middleware for the Gateway.
package middleware

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	
	"zuri/shared/pkg/logger"
)

// RedisClient defines the interface for Redis operations needed by JWT validation.
type RedisClient interface {
	Exists(ctx context.Context, key string) (bool, error)
}

// JWTConfig holds JWT middleware configuration.
type JWTConfig struct {
	PublicKey   *rsa.PublicKey
	RedisClient RedisClient
	Skipper     func(c echo.Context) bool
}

// JWTValidator validates JWT tokens.
type JWTValidator struct {
	publicKey   *rsa.PublicKey
	redisClient RedisClient
}

// Claims represents JWT claims.
type Claims struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	TokenType     string `json:"token_type"`
	InstitutionID string `json:"institution_id,omitempty"`
	jwt.RegisteredClaims
}

// NewJWTValidator creates a new JWT validator.
func NewJWTValidator(publicKey *rsa.PublicKey, redisClient ...RedisClient) *JWTValidator {
	v := &JWTValidator{publicKey: publicKey}
	if len(redisClient) > 0 {
		v.redisClient = redisClient[0]
	}
	return v
}

// SetRedisClient sets the Redis client for blacklist checking.
func (v *JWTValidator) SetRedisClient(client RedisClient) {
	v.redisClient = client
}

// IsBlacklisted checks if the given token ID (jti) is blacklisted in Redis.
// It checks both "token_blacklist:<jti>" and "blacklist:<jti>".
func (v *JWTValidator) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if v.redisClient == nil || jti == "" {
		return false, nil
	}

	exists, err := v.redisClient.Exists(ctx, fmt.Sprintf("token_blacklist:%s", jti))
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	exists, err = v.redisClient.Exists(ctx, fmt.Sprintf("blacklist:%s", jti))
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Validate validates a JWT token and returns claims.
func (v *JWTValidator) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})
	
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		return nil, errors.New("invalid token")
	}
	
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	
	// Ensure it's an access token
	if claims.TokenType != "access" {
		return nil, errors.New("invalid token type")
	}
	
	return claims, nil
}

// JWTMiddleware returns Echo middleware for JWT validation.
func JWTMiddleware(validator *JWTValidator, skipper func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if skipper returns true
			if skipper != nil && skipper(c) {
				return next(c)
			}
			
			// Extract token from Authorization header or query param (needed for WebSocket)
			var tokenString string
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				// Parse Bearer token
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
				}
				tokenString = parts[1]
			} else {
				// Fallback to query param for WebSocket auth
				tokenString = c.QueryParam("token")
				if tokenString == "" {
					return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
				}
			}
			
			// Validate token
			claims, err := validator.Validate(tokenString)
			if err != nil {
				logger.Warn("token validation failed",
					zap.String("error", err.Error()),
					zap.String("path", c.Request().URL.Path),
				)
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			// Add Redis token blacklist check: check if token ID (claims.ID / jti) exists in Redis key token_blacklist:<jti> or blacklist:<jti>
			if claims.ID != "" && validator.redisClient != nil {
				blacklisted, err := validator.IsBlacklisted(c.Request().Context(), claims.ID)
				if err != nil {
					logger.Error("failed to check token blacklist",
						zap.Error(err),
						zap.String("jti", claims.ID),
						zap.String("user_id", claims.UserID),
					)
				} else if blacklisted {
					logger.Warn("blacklisted token rejected",
						zap.String("jti", claims.ID),
						zap.String("user_id", claims.UserID),
						zap.String("path", c.Request().URL.Path),
					)
					return echo.NewHTTPError(http.StatusUnauthorized, "token has been revoked")
				}
			}
			
			// Store claims in context
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
			c.Set("token_id", claims.ID)
			c.Set("institution_id", claims.InstitutionID)
			c.Set("claims", claims)
			
			return next(c)
		}
	}
}

// PublicKeySkipper returns a skipper that skips JWT validation for public paths.
func PublicKeySkipper(publicPaths []string) func(echo.Context) bool {
	return func(c echo.Context) bool {
		path := c.Request().URL.Path
		for _, publicPath := range publicPaths {
			if strings.HasPrefix(path, publicPath) {
				return true
			}
		}
		return false
	}
}
