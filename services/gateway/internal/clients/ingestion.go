package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// IngestionClient is a client for the Python Ingestion Service
type IngestionClient struct {
	*Client
}

// NewIngestionClient creates a new Ingestion Service client
func NewIngestionClient(baseURL, internalAPIKey string) *IngestionClient {
	config := IngestionDefaultConfig()
	return &IngestionClient{
		Client: NewClient("ingestion", baseURL, internalAPIKey, config, false), // No circuit breaker for ingestion
	}
}

// ProcessDocument sends a document for processing
func (c *IngestionClient) ProcessDocument(ctx context.Context, req ProcessDocumentRequest) (*ProcessDocumentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, "/process", body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("process document failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result ProcessDocumentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetTaskStatus gets the status of a processing task
func (c *IngestionClient) GetTaskStatus(ctx context.Context, taskID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/task/%s", taskID)
	
	resp, err := c.doWithRetry(ctx, http.MethodGet, path, nil, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get task status failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}
