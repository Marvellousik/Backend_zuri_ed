package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RetrievalClient is a client for the Python Retrieval Service
type RetrievalClient struct {
	*Client
}

// NewRetrievalClient creates a new Retrieval Service client
func NewRetrievalClient(baseURL, internalAPIKey string) *RetrievalClient {
	config := RetrievalDefaultConfig()
	return &RetrievalClient{
		Client: NewClient("retrieval", baseURL, internalAPIKey, config, false), // No circuit breaker for retrieval
	}
}

// RetrieveContext retrieves context chunks for a query
func (c *RetrievalClient) RetrieveContext(ctx context.Context, query, userID string, topK int) (*RetrieveResponse, error) {
	req := RetrieveRequest{
		Query:  query,
		UserID: userID,
		TopK:   topK,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, "/retrieve", body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieve context failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result RetrieveResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// RetrieveContextForMaterial retrieves context filtered by specific material
func (c *RetrievalClient) RetrieveContextForMaterial(ctx context.Context, query, userID, materialID string, topK int) (*RetrieveResponse, error) {
	req := RetrieveRequest{
		Query:      query,
		UserID:     userID,
		MaterialID: materialID,
		TopK:       topK,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, "/retrieve", body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieve context failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result RetrieveResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
