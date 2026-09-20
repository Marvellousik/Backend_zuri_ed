// Package ai provides the Unified AI Gateway abstractions, OpenRouter client adapter,
// task-based dynamic model routing, and streaming utilities for Zuri.
package ai

import (
	"context"
	"time"
)

// LLMProvider defines the universal interface for large language model completions and streaming.
type LLMProvider interface {
	// Complete sends a prompt to the LLM and returns the complete non-streamed response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Stream initiates an SSE streaming session and returns a read-only channel emitting Chunks.
	Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error)

	// Name returns the provider's unique identifier (e.g., "openrouter").
	Name() string
}

// Message represents a single message turn in an LLM conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// Chunk represents a single streaming token or delta received from an LLM stream.
type Chunk struct {
	ID           string  `json:"id,omitempty"`
	Content      string  `json:"content"`
	Role         string  `json:"role,omitempty"`
	Model        string  `json:"model,omitempty"`
	Usage        *Usage  `json:"usage,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
	Err          error   `json:"-"`
}

// Usage captures token consumption counts and computed monetary cost in USD.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// CompletionRequest represents an incoming generation request.
type CompletionRequest struct {
	Task           string                 `json:"task,omitempty"`
	Model          string                 `json:"model,omitempty"`
	FallbackModels []string               `json:"fallback_models,omitempty"`
	Messages       []Message              `json:"messages"`
	Temperature    *float64               `json:"temperature,omitempty"`
	TopP           *float64               `json:"top_p,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Stream         bool                   `json:"stream,omitempty"`
	Stop           []string               `json:"stop,omitempty"`
	UserID         string                 `json:"user_id,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionResponse represents the full response from a non-streaming LLM invocation.
type CompletionResponse struct {
	ID           string                 `json:"id,omitempty"`
	Object       string                 `json:"object,omitempty"`
	Created      int64                  `json:"created,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Message      Message                `json:"message,omitempty"`
	Content      string                 `json:"content,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	Usage        Usage                  `json:"usage,omitempty"`
	CostUSD      float64                `json:"cost_usd,omitempty"`
	LatencyMs    int64                  `json:"latency_ms,omitempty"`
	Cached       bool                   `json:"cached,omitempty"`
	CreatedAt    time.Time              `json:"created_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
