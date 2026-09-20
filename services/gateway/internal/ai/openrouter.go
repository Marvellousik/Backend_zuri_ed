// Package ai provides the Unified AI Gateway abstractions, OpenRouter client adapter,
// task-based dynamic model routing, and streaming utilities for Zuri.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"zuri/shared/pkg/logger"
)

const (
	DefaultOpenRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"
	DefaultHTTPReferer       = "https://zuri.academy"
	DefaultXTitle            = "Zuri Academic Engine"
	DefaultModel             = "openrouter/auto"
)

// ModelPricing defines per-million token pricing in USD.
type ModelPricing struct {
	PromptPerMillion     float64 `json:"prompt_per_million"`
	CompletionPerMillion float64 `json:"completion_per_million"`
}

// defaultPricingTable contains reference pricing for models commonly routed in Zuri.
var defaultPricingTable = map[string]ModelPricing{
	"openrouter/auto":                       {PromptPerMillion: 0.15, CompletionPerMillion: 0.60},
	"google/gemini-2.5-flash":               {PromptPerMillion: 0.075, CompletionPerMillion: 0.30},
	"google/gemini-2.5-flash-lite":          {PromptPerMillion: 0.0375, CompletionPerMillion: 0.15},
	"google/gemini-2.5-pro":                 {PromptPerMillion: 1.25, CompletionPerMillion: 5.00},
	"anthropic/claude-3-5-haiku":            {PromptPerMillion: 0.80, CompletionPerMillion: 4.00},
	"anthropic/claude-3-7-sonnet":           {PromptPerMillion: 3.00, CompletionPerMillion: 15.00},
	"openai/gpt-4o":                         {PromptPerMillion: 2.50, CompletionPerMillion: 10.00},
	"openai/gpt-4o-mini":                    {PromptPerMillion: 0.15, CompletionPerMillion: 0.60},
	"meta-llama/llama-3.3-70b-instruct":     {PromptPerMillion: 0.12, CompletionPerMillion: 0.30},
	"meta-llama/llama-3.1-8b-instruct:free": {PromptPerMillion: 0.00, CompletionPerMillion: 0.00},
	"deepseek/deepseek-chat":                {PromptPerMillion: 0.14, CompletionPerMillion: 0.28},
}

// DefaultModelPricing exposes reference model pricing for cost tracking.
var DefaultModelPricing = defaultPricingTable

// CalculateTokenCost calculates the estimated USD cost for a model and token usage.
func CalculateTokenCost(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := defaultPricingTable[model]
	if !ok {
		// Try without provider prefix (e.g. "gemini-2.5-flash")
		parts := strings.Split(model, "/")
		if len(parts) == 2 {
			pricing, ok = defaultPricingTable[parts[1]]
		}
	}
	if !ok {
		// Default generic pricing: $0.20 per 1M prompt, $0.80 per 1M completion
		pricing = ModelPricing{PromptPerMillion: 0.20, CompletionPerMillion: 0.80}
	}
	promptCost := (float64(promptTokens) / 1_000_000.0) * pricing.PromptPerMillion
	compCost := (float64(completionTokens) / 1_000_000.0) * pricing.CompletionPerMillion
	return promptCost + compCost
}

// OpenRouterAdapter implements LLMProvider for the OpenRouter API.
type OpenRouterAdapter struct {
	apiKey      string
	baseURL     string
	httpReferer string
	xTitle      string
	httpClient  *http.Client
	logger      *zap.Logger
}

// OpenRouterOption provides functional options to configure OpenRouterAdapter.
type OpenRouterOption func(*OpenRouterAdapter)

// WithBaseURL overrides the default OpenRouter completions endpoint.
func WithBaseURL(url string) OpenRouterOption {
	return func(a *OpenRouterAdapter) {
		if url != "" {
			a.baseURL = url
		}
	}
}

// WithHTTPClient sets a custom http.Client (e.g. for testing with mock servers).
func WithHTTPClient(client *http.Client) OpenRouterOption {
	return func(a *OpenRouterAdapter) {
		if client != nil {
			a.httpClient = client
		}
	}
}

// WithHTTPReferer overrides the default HTTP-Referer header.
func WithHTTPReferer(referer string) OpenRouterOption {
	return func(a *OpenRouterAdapter) {
		if referer != "" {
			a.httpReferer = referer
		}
	}
}

// WithXTitle overrides the default X-Title header.
func WithXTitle(title string) OpenRouterOption {
	return func(a *OpenRouterAdapter) {
		if title != "" {
			a.xTitle = title
		}
	}
}

// WithLogger overrides the logger instance.
func WithLogger(l *zap.Logger) OpenRouterOption {
	return func(a *OpenRouterAdapter) {
		if l != nil {
			a.logger = l
		}
	}
}

// NewOpenRouterAdapter creates an OpenRouter adapter instance.
func NewOpenRouterAdapter(apiKey string, opts ...OpenRouterOption) *OpenRouterAdapter {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	adapter := &OpenRouterAdapter{
		apiKey:      apiKey,
		baseURL:     DefaultOpenRouterBaseURL,
		httpReferer: DefaultHTTPReferer,
		xTitle:      DefaultXTitle,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger.Get(),
	}

	for _, opt := range opts {
		opt(adapter)
	}

	if adapter.logger == nil {
		adapter.logger = zap.NewNop()
	}

	return adapter
}

// Name returns the provider identifier.
func (a *OpenRouterAdapter) Name() string {
	return "openrouter"
}

// Request and response DTOs for OpenRouter API
type openRouterRequestBody struct {
	Model       string    `json:"model"`
	Models      []string  `json:"models,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

type openRouterUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost,omitempty"`
	Cost             float64 `json:"cost,omitempty"`
}

type openRouterChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type openRouterResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []openRouterChoice `json:"choices"`
	Usage   openRouterUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type openRouterStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type openRouterStreamChoice struct {
	Index        int                   `json:"index"`
	Delta        openRouterStreamDelta `json:"delta"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type openRouterStreamResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object,omitempty"`
	Created int64                    `json:"created,omitempty"`
	Model   string                   `json:"model,omitempty"`
	Choices []openRouterStreamChoice `json:"choices"`
	Usage   *openRouterUsage         `json:"usage,omitempty"`
	Error   *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// buildCandidateModels builds an ordered deduplicated list of models to try.
func buildCandidateModels(primary string, fallbacks []string) []string {
	seen := make(map[string]struct{})
	var candidates []string

	if primary != "" {
		seen[primary] = struct{}{}
		candidates = append(candidates, primary)
	}

	for _, fb := range fallbacks {
		fb = strings.TrimSpace(fb)
		if fb == "" {
			continue
		}
		if _, exists := seen[fb]; !exists {
			seen[fb] = struct{}{}
			candidates = append(candidates, fb)
		}
	}

	if len(candidates) == 0 {
		candidates = []string{DefaultModel}
	}

	return candidates
}

// parseCost extracts CostUSD from usage.total_cost, usage.cost, or calculates token cost.
func parseCost(model string, u openRouterUsage) float64 {
	if u.TotalCost > 0 {
		return u.TotalCost
	}
	if u.Cost > 0 {
		return u.Cost
	}
	return CalculateTokenCost(model, u.PromptTokens, u.CompletionTokens)
}

// Complete executes a non-streaming chat completion with automatic fallback model support.
func (a *OpenRouterAdapter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	candidates := buildCandidateModels(req.Model, req.FallbackModels)
	var lastErr error
	start := time.Now()

	for i, currentModel := range candidates {
		if ctx.Err() != nil {
			return CompletionResponse{}, ctx.Err()
		}

		remaining := candidates[i:]
		payload := openRouterRequestBody{
			Model:       currentModel,
			Models:      remaining,
			Messages:    req.Messages,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			MaxTokens:   req.MaxTokens,
			Stream:      false,
			Stop:        req.Stop,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("failed to marshal openrouter request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("failed to create http request: %w", err)
		}

		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
		httpReq.Header.Set("HTTP-Referer", a.httpReferer)
		httpReq.Header.Set("X-Title", a.xTitle)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := a.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("model %s failed with network error: %w", currentModel, err)
			a.logger.Warn("OpenRouter model attempt failed",
				zap.String("model", currentModel),
				zap.Int("candidate_index", i),
				zap.Error(err),
			)
			continue
		}

		respBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("model %s failed reading response body: %w", currentModel, readErr)
			a.logger.Warn("OpenRouter failed reading response body",
				zap.String("model", currentModel),
				zap.Error(readErr),
			)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("model %s returned status %d: %s", currentModel, resp.StatusCode, string(respBytes))
			a.logger.Warn("OpenRouter model returned non-200 status",
				zap.String("model", currentModel),
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(respBytes)),
			)
			continue
		}

		var orResp openRouterResponse
		if err := json.Unmarshal(respBytes, &orResp); err != nil {
			lastErr = fmt.Errorf("model %s failed unmarshaling json response: %w", currentModel, err)
			a.logger.Warn("OpenRouter failed unmarshaling response",
				zap.String("model", currentModel),
				zap.Error(err),
			)
			continue
		}

		if orResp.Error != nil {
			lastErr = fmt.Errorf("model %s returned API error: %s (code: %d)", currentModel, orResp.Error.Message, orResp.Error.Code)
			a.logger.Warn("OpenRouter API returned error block",
				zap.String("model", currentModel),
				zap.String("api_error", orResp.Error.Message),
			)
			continue
		}

		actualModel := orResp.Model
		if actualModel == "" {
			actualModel = currentModel
		}

		cost := parseCost(actualModel, orResp.Usage)

		var msg Message
		var finishReason string
		if len(orResp.Choices) > 0 {
			msg = orResp.Choices[0].Message
			finishReason = orResp.Choices[0].FinishReason
		}

		return CompletionResponse{
			ID:           orResp.ID,
			Object:       orResp.Object,
			Created:      orResp.Created,
			Model:        actualModel,
			Message:      msg,
			Content:      msg.Content,
			FinishReason: finishReason,
			Usage: Usage{
				PromptTokens:     orResp.Usage.PromptTokens,
				CompletionTokens: orResp.Usage.CompletionTokens,
				TotalTokens:      orResp.Usage.TotalTokens,
				CostUSD:          cost,
			},
			CostUSD:   cost,
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return CompletionResponse{}, fmt.Errorf("all candidate models failed for completion: %w", lastErr)
}

// Stream initiates an SSE streaming chat completion with automatic fallback model support.
func (a *OpenRouterAdapter) Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error) {
	candidates := buildCandidateModels(req.Model, req.FallbackModels)
	var lastErr error

	for i, currentModel := range candidates {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		remaining := candidates[i:]
		payload := openRouterRequestBody{
			Model:       currentModel,
			Models:      remaining,
			Messages:    req.Messages,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			MaxTokens:   req.MaxTokens,
			Stream:      true,
			Stop:        req.Stop,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal streaming request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}

		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
		httpReq.Header.Set("HTTP-Referer", a.httpReferer)
		httpReq.Header.Set("X-Title", a.xTitle)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := a.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("stream model %s failed with network error: %w", currentModel, err)
			a.logger.Warn("OpenRouter streaming attempt failed",
				zap.String("model", currentModel),
				zap.Error(err),
			)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("stream model %s returned status %d: %s", currentModel, resp.StatusCode, string(errBytes))
			a.logger.Warn("OpenRouter streaming non-200 status",
				zap.String("model", currentModel),
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(errBytes)),
			)
			continue
		}

		out := make(chan Chunk, 100)
		go a.processSSEStream(ctx, resp.Body, currentModel, out)
		return out, nil
	}

	return nil, fmt.Errorf("all candidate models failed for streaming: %w", lastErr)
}

// processSSEStream reads lines from the SSE response body and streams Chunks into the out channel.
func (a *OpenRouterAdapter) processSSEStream(ctx context.Context, body io.ReadCloser, defaultModel string, out chan<- Chunk) {
	defer body.Close()
	defer close(out)

	// Guarantee immediate closure when context is canceled
	closeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-closeDone:
		}
	}()
	defer close(closeDone)

	scanner := bufio.NewScanner(body)
	// Buffer up to 1MB per line for large delta chunks
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var (
		currentModel = defaultModel
		chunkID      string
	)

	for scanner.Scan() {
		if ctx.Err() != nil {
			select {
			case out <- Chunk{Err: ctx.Err()}:
			default:
			}
			return
		}

		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip comments or keep-alive headers (e.g. ": OPENROUTER PROCESSING")
		if strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "[DONE]" {
			break
		}

		var streamChunk openRouterStreamResponse
		if err := json.Unmarshal([]byte(data), &streamChunk); err != nil {
			a.logger.Debug("failed to parse SSE chunk JSON", zap.String("data", data), zap.Error(err))
			continue
		}

		if streamChunk.Error != nil {
			select {
			case <-ctx.Done():
				return
			case out <- Chunk{
				Err: fmt.Errorf("stream error from OpenRouter: %s (code: %d)", streamChunk.Error.Message, streamChunk.Error.Code),
			}:
			}
			return
		}

		if streamChunk.ID != "" {
			chunkID = streamChunk.ID
		}
		if streamChunk.Model != "" {
			currentModel = streamChunk.Model
		}

		chunk := Chunk{
			ID:    chunkID,
			Model: currentModel,
		}

		if len(streamChunk.Choices) > 0 {
			choice := streamChunk.Choices[0]
			chunk.Content = choice.Delta.Content
			chunk.Role = choice.Delta.Role
			chunk.FinishReason = choice.FinishReason
		}

		if streamChunk.Usage != nil {
			cost := parseCost(currentModel, *streamChunk.Usage)
			chunk.Usage = &Usage{
				PromptTokens:     streamChunk.Usage.PromptTokens,
				CompletionTokens: streamChunk.Usage.CompletionTokens,
				TotalTokens:      streamChunk.Usage.TotalTokens,
				CostUSD:          cost,
			}
			chunk.CostUSD = cost
		}

		select {
		case <-ctx.Done():
			select {
			case out <- Chunk{Err: ctx.Err()}:
			default:
			}
			return
		case out <- chunk:
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		select {
		case out <- Chunk{Err: fmt.Errorf("SSE scanner error: %w", err)}:
		default:
		}
	}
}
