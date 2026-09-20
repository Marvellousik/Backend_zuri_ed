package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// AudioClient is a client for the Python Audio Service
type AudioClient struct {
	*Client
}

// NewAudioClient creates a new Audio Service client
func NewAudioClient(baseURL, internalAPIKey string) *AudioClient {
	config := AudioDefaultConfig()
	return &AudioClient{
		Client: NewClient("audio", baseURL, internalAPIKey, config, false), // No circuit breaker for audio
	}
}

// SpeechToText converts audio to text
func (c *AudioClient) SpeechToText(ctx context.Context, audioData []byte, contentType, language string) (*TranscriptionResponse, error) {
	// Determine file extension based on contentType
	ext := "bin"
	if strings.Contains(contentType, "wav") {
		ext = "wav"
	} else if strings.Contains(contentType, "mp3") {
		ext = "mp3"
	} else if strings.Contains(contentType, "ogg") {
		ext = "ogg"
	}

	// Build multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add audio file
	part, err := writer.CreateFormFile("audio", fmt.Sprintf("audio.%s", ext))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add language field
	if err := writer.WriteField("language", language); err != nil {
		return nil, fmt.Errorf("failed to write language field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	url := c.config.BaseURL + "/api/v1/ai/speech-to-text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Internal-Key", c.config.InternalAPIKey)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("speech to text failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result TranscriptionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetSupportedLanguages returns the list of supported languages
func (c *AudioClient) GetSupportedLanguages(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doWithRetry(ctx, http.MethodGet, "/api/v1/ai/languages", nil, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get languages failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}