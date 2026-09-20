// Package client provides HTTP clients for external services.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// NotificationClient provides methods to interact with the Notification Service.
type NotificationClient struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
}

// NewNotificationClient creates a new notification service client.
func NewNotificationClient(baseURL, internalKey string) *NotificationClient {
	return &NotificationClient{
		baseURL:     baseURL,
		internalKey: internalKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendVerificationEmail queues a verification email for the user.
func (c *NotificationClient) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email, name, code string) error {
	reqBody := map[string]interface{}{
		"user_id": userID,
		"type":    "email",
		"title":   "Your Zuri Verification Code",
		"body":    fmt.Sprintf("Your verification code is: %s", code),
		"data": map[string]interface{}{
			"template": "email_verification",
			"Name":     name,
			"Code":     code,
		},
	}
	return c.sendInternal(ctx, reqBody)
}

// SendPasswordResetEmail queues a password reset email for the user.
func (c *NotificationClient) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email, name, code string) error {
	reqBody := map[string]interface{}{
		"user_id": userID,
		"type":    "email",
		"title":   "Password Reset Request",
		"body":    fmt.Sprintf("Your password reset code is: %s", code),
		"data": map[string]interface{}{
			"template": "password_reset",
			"Name":     name,
			"Code":     code,
		},
	}
	return c.sendInternal(ctx, reqBody)
}

func (c *NotificationClient) sendInternal(ctx context.Context, reqBody map[string]interface{}) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/internal/send", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.internalKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	return nil
}
