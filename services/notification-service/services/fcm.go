package services

import (
	"context"
	"os"

	"go.uber.org/zap"
	"zuri/shared/pkg/logger"
)

// FCMService handles Firebase Cloud Messaging
// Note: This is a simplified implementation without actual Firebase SDK
// In production, you would integrate with Firebase Admin SDK
type FCMService struct {
	enabled bool
}

// NewFCMService creates a new FCM service
func NewFCMService() *FCMService {
	// Check for service account file
	serviceAccountPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH")
	if serviceAccountPath == "" {
		logger.Warn("FIREBASE_SERVICE_ACCOUNT_PATH not set, FCM service disabled")
		return &FCMService{enabled: false}
	}

	logger.Info("FCM service initialized (Firebase integration would go here)")
	return &FCMService{enabled: true}
}

// SendNotification sends a push notification to a device
// In a real implementation, this would use Firebase Admin SDK
func (s *FCMService) SendNotification(ctx context.Context, token, title, body string, data map[string]interface{}) error {
	if !s.enabled {
		logger.Debug("FCM service is disabled, notification not sent",
			zap.String("token_prefix", token[:min(len(token), 20)]))
		return nil
	}

	// Log the notification that would be sent
	logger.Info("[FCM] Would send notification",
		zap.String("token_prefix", token[:20]),
		zap.String("title", title),
		zap.String("body", body),
	)
	return nil
}

// SendMulticast sends a notification to multiple devices
func (s *FCMService) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]interface{}) error {
	if !s.enabled {
		logger.Debug("FCM service is disabled, multicast not sent")
		return nil
	}

	logger.Info("[FCM] Would send multicast",
		zap.Int("device_count", len(tokens)),
		zap.String("title", title),
	)
	return nil
}

// IsEnabled returns whether FCM is enabled
func (s *FCMService) IsEnabled() bool {
	return s.enabled
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
