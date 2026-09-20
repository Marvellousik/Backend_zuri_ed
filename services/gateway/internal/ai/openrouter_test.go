package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"zuri/services/gateway/internal/ai"
)

func TestOpenRouterAdapter_Complete_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "https://zuri.academy", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "Zuri Academic Engine", r.Header.Get("X-Title"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var reqBody map[string]interface{}
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)
		assert.Equal(t, "google/gemini-2.5-flash", reqBody["model"])

		respJSON := `{
			"id": "gen-123456",
			"object": "chat.completion",
			"created": 1710000000,
			"model": "google/gemini-2.5-flash",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Photosynthesis is the process by which green plants produce energy."
					},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 50,
				"completion_tokens": 12,
				"total_tokens": 62,
				"total_cost": 0.000015
			}
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respJSON))
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Task:  "chat",
		Model: "google/gemini-2.5-flash",
		Messages: []ai.Message{
			{Role: "user", Content: "Explain photosynthesis"},
		},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "gen-123456", resp.ID)
	assert.Equal(t, "google/gemini-2.5-flash", resp.Model)
	assert.Equal(t, "assistant", resp.Message.Role)
	assert.Equal(t, "Photosynthesis is the process by which green plants produce energy.", resp.Message.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 50, resp.Usage.PromptTokens)
	assert.Equal(t, 12, resp.Usage.CompletionTokens)
	assert.Equal(t, 62, resp.Usage.TotalTokens)
	assert.InDelta(t, 0.000015, resp.CostUSD, 0.000001)
	assert.InDelta(t, 0.000015, resp.Usage.CostUSD, 0.000001)
	assert.GreaterOrEqual(t, resp.LatencyMs, int64(0))
}

func TestOpenRouterAdapter_Complete_CostFallback_Calculation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response without total_cost (0 or missing)
		respJSON := `{
			"id": "gen-calc",
			"model": "google/gemini-2.5-pro",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Advanced quantum mechanics calculation."
					},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 1000,
				"completion_tokens": 500,
				"total_tokens": 1500,
				"total_cost": 0.0
			}
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respJSON))
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Model: "google/gemini-2.5-pro",
		Messages: []ai.Message{
			{Role: "user", Content: "Calculate energy eigenvalues"},
		},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)

	// Pricing for google/gemini-2.5-pro: $1.25 / 1M prompt, $5.00 / 1M completion
	// 1000 prompt = $0.00125
	// 500 completion = $0.0025
	// Total expected = $0.00375
	expectedCost := ai.CalculateTokenCost("google/gemini-2.5-pro", 1000, 500)
	assert.InDelta(t, 0.00375, expectedCost, 0.00001)
	assert.InDelta(t, expectedCost, resp.CostUSD, 0.00001)
	assert.InDelta(t, expectedCost, resp.Usage.CostUSD, 0.00001)
}

func TestOpenRouterAdapter_Complete_ModelFallback(t *testing.T) {
	var callCount int32
	var requestedModels []string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &reqBody)
		requestedModels = append(requestedModels, reqBody.Model)

		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			// First call (primary model) fails with 500
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"Primary model overloaded","code":500}}`))
			return
		}

		// Second call (fallback model) succeeds
		respJSON := `{
			"id": "gen-fallback",
			"model": "anthropic/claude-3-5-haiku",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Response from fallback model"
					},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 20,
				"completion_tokens": 10,
				"total_tokens": 30,
				"total_cost": 0.000056
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respJSON))
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Model:          "google/gemini-2.5-pro",
		FallbackModels: []string{"anthropic/claude-3-5-haiku"},
		Messages: []ai.Message{
			{Role: "user", Content: "Test fallback"},
		},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "gen-fallback", resp.ID)
	assert.Equal(t, "anthropic/claude-3-5-haiku", resp.Model)
	assert.Equal(t, "Response from fallback model", resp.Message.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	assert.Equal(t, []string{"google/gemini-2.5-pro", "anthropic/claude-3-5-haiku"}, requestedModels)
}

func TestOpenRouterAdapter_Complete_AllModelsFail(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"Bad Gateway","code":502}}`))
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Model:          "model-a",
		FallbackModels: []string{"model-b"},
		Messages: []ai.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := adapter.Complete(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all candidate models failed")
}

func TestOpenRouterAdapter_Stream_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		_ = json.Unmarshal(body, &reqBody)
		assert.Equal(t, true, reqBody["stream"])

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send comments and SSE chunks
		lines := []string{
			": OPENROUTER PROCESSING",
			`data: {"id":"chunk-1","model":"google/gemini-2.5-flash","choices":[{"delta":{"role":"assistant","content":"Learning "},"finish_reason":null}]}`,
			`data: {"id":"chunk-1","model":"google/gemini-2.5-flash","choices":[{"delta":{"content":"Go "},"finish_reason":null}]}`,
			`data: {"id":"chunk-1","model":"google/gemini-2.5-flash","choices":[{"delta":{"content":"is fun!"},"finish_reason":null}]}`,
			`data: {"id":"chunk-1","model":"google/gemini-2.5-flash","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":15,"completion_tokens":6,"total_tokens":21,"total_cost":0.000004}}`,
			"data: [DONE]",
		}

		for _, line := range lines {
			_, _ = fmt.Fprintf(w, "%s\n\n", line)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Model: "google/gemini-2.5-flash",
		Messages: []ai.Message{
			{Role: "user", Content: "Stream test"},
		},
	}

	streamChan, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	var sb strings.Builder
	var lastChunk ai.Chunk
	var receivedChunks []ai.Chunk

	for chunk := range streamChan {
		require.NoError(t, chunk.Err)
		receivedChunks = append(receivedChunks, chunk)
		sb.WriteString(chunk.Content)
		lastChunk = chunk
	}

	assert.Equal(t, "Learning Go is fun!", sb.String())
	assert.GreaterOrEqual(t, len(receivedChunks), 3)
	assert.Equal(t, "stop", lastChunk.FinishReason)
	assert.NotNil(t, lastChunk.Usage)
	assert.Equal(t, 21, lastChunk.Usage.TotalTokens)
	assert.InDelta(t, 0.000004, lastChunk.CostUSD, 0.0000001)
}

func TestOpenRouterAdapter_Stream_ModelFallback(t *testing.T) {
	var callCount int32

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			// First model fails
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"Temporarily unavailable"}}`))
			return
		}

		// Fallback succeeds with stream
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"id":"stream-fb","model":"anthropic/claude-3-5-haiku","choices":[{"delta":{"content":"fallback stream chunk"}}]}`)
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx := context.Background()
	req := ai.CompletionRequest{
		Model:          "model-down",
		FallbackModels: []string{"model-up"},
		Messages: []ai.Message{
			{Role: "user", Content: "test"},
		},
	}

	streamChan, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	var received []ai.Chunk
	for chunk := range streamChan {
		require.NoError(t, chunk.Err)
		received = append(received, chunk)
	}

	require.Len(t, received, 1)
	assert.Equal(t, "fallback stream chunk", received[0].Content)
	assert.Equal(t, "anthropic/claude-3-5-haiku", received[0].Model)
}

func TestOpenRouterAdapter_Stream_ContextCancellation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Keep emitting slowly
		for i := 0; i < 50; i++ {
			_, err := fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"tick"}}]}`)
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer mockServer.Close()

	adapter := ai.NewOpenRouterAdapter(
		"test-api-key",
		ai.WithBaseURL(mockServer.URL),
		ai.WithHTTPClient(mockServer.Client()),
		ai.WithLogger(zap.NewNop()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := ai.CompletionRequest{
		Model: "google/gemini-2.5-flash",
		Messages: []ai.Message{
			{Role: "user", Content: "Stream with timeout"},
		},
	}

	streamChan, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	// Stream channel should close gracefully when context expires
	done := make(chan struct{})
	go func() {
		for range streamChan {
			// Read until closed
		}
		close(done)
	}()

	select {
	case <-done:
		// Succeeded in terminating cleanly
	case <-time.After(1 * time.Second):
		t.Fatal("stream did not terminate after context cancellation")
	}
}

func TestOpenRouterAdapter_Name(t *testing.T) {
	adapter := ai.NewOpenRouterAdapter("test")
	assert.Equal(t, "openrouter", adapter.Name())
}
