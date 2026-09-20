package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"zuri/services/gateway/internal/circuitbreaker"
)

// OrchestratorClient is a client for the Python AI Orchestrator Service
type OrchestratorClient struct {
	*Client
}

// NewOrchestratorClient creates a new AI Orchestrator client with circuit breaker
func NewOrchestratorClient(baseURL, internalAPIKey string) *OrchestratorClient {
	config := OrchestratorDefaultConfig()
	return &OrchestratorClient{
		Client: NewClient("orchestrator", baseURL, internalAPIKey, config, true), // Enable circuit breaker
	}
}

// Chat sends a chat request to the AI Orchestrator
func (c *OrchestratorClient) Chat(ctx context.Context, query, userID string, contextChunks []string, conversationID *string) (*ChatResponse, error) {
	req := ChatRequest{
		Query:          query,
		UserID:         userID,
		ContextChunks:  contextChunks,
		ConversationID: conversationID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, "/chat", body, "application/json")
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result ChatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ChatWithMaterial sends a chat request with specific material context
func (c *OrchestratorClient) ChatWithMaterial(ctx context.Context, query, userID, materialID string, contextChunks []string, conversationID *string) (*ChatResponse, error) {
	req := ChatRequest{
		Query:          query,
		UserID:         userID,
		MaterialID:     &materialID,
		ContextChunks:  contextChunks,
		ConversationID: conversationID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, "/chat", body, "application/json")
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result ChatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetConversationHistory retrieves conversation history
func (c *OrchestratorClient) GetConversationHistory(ctx context.Context, conversationID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/conversation/%s", conversationID)
	
	resp, err := c.doWithRetry(ctx, http.MethodGet, path, nil, "application/json")
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get conversation failed: HTTP %d - %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}

// ClearConversation clears a conversation history
func (c *OrchestratorClient) ClearConversation(ctx context.Context, conversationID string) error {
	path := fmt.Sprintf("/conversation/%s", conversationID)
	
	resp, err := c.doWithRetry(ctx, http.MethodDelete, path, nil, "application/json")
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return err
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clear conversation failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// IsCircuitBreakerOpen returns true if the circuit breaker is open
func (c *OrchestratorClient) IsCircuitBreakerOpen() bool {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.State() == circuitbreaker.StateOpen
	}
	return false
}
