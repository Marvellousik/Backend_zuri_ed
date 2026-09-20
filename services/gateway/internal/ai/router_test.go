package ai_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"zuri/services/gateway/internal/ai"
)

// mockLLMProvider records requests and returns canned responses
type mockLLMProvider struct {
	lastRequest ai.CompletionRequest
	resp        ai.CompletionResponse
	streamChan  chan ai.Chunk
}

func (m *mockLLMProvider) Complete(ctx context.Context, req ai.CompletionRequest) (ai.CompletionResponse, error) {
	m.lastRequest = req
	return m.resp, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req ai.CompletionRequest) (<-chan ai.Chunk, error) {
	m.lastRequest = req
	return m.streamChan, nil
}

func (m *mockLLMProvider) Name() string {
	return "mock-provider"
}

func TestDefaultRouterConfig(t *testing.T) {
	cfg := ai.DefaultRouterConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "2.0", cfg.Version)
	assert.Equal(t, ai.TaskChat, cfg.DefaultTask)

	// chat -> speed
	chatRoute, ok := cfg.Tasks[ai.TaskChat]
	require.True(t, ok)
	assert.Equal(t, ai.StrategySpeed, chatRoute.Strategy)
	assert.Equal(t, "openrouter/auto", chatRoute.PrimaryModel)

	// quiz_generation -> quality
	quizRoute, ok := cfg.Tasks[ai.TaskQuizGeneration]
	require.True(t, ok)
	assert.Equal(t, ai.StrategyQuality, quizRoute.Strategy)
	assert.Equal(t, "openrouter/auto", quizRoute.PrimaryModel)

	// flashcards -> cost
	fcRoute, ok := cfg.Tasks[ai.TaskFlashcards]
	require.True(t, ok)
	assert.Equal(t, ai.StrategyCost, fcRoute.Strategy)
	assert.Equal(t, "openrouter/auto", fcRoute.PrimaryModel)

	// summarize_document -> cost_quality_balance
	sumRoute, ok := cfg.Tasks[ai.TaskSummarizeDocument]
	require.True(t, ok)
	assert.Equal(t, ai.StrategyCostQualityBalance, sumRoute.Strategy)
	assert.Equal(t, "openrouter/auto", sumRoute.PrimaryModel)

	// research_assistance -> quality
	resRoute, ok := cfg.Tasks[ai.TaskResearchAssistance]
	require.True(t, ok)
	assert.Equal(t, ai.StrategyQuality, resRoute.Strategy)
	assert.Equal(t, "openrouter/auto", resRoute.PrimaryModel)
}

func TestLoadRouterConfigFile(t *testing.T) {
	// Root ai_routing.yaml file
	configPath := filepath.Join("..", "..", "..", "..", "config", "ai_routing.yaml")
	cfg, err := ai.LoadRouterConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "2.0", cfg.Version)
	assert.Equal(t, "chat", cfg.DefaultTask)
	assert.Len(t, cfg.Tasks, 5)

	chat := cfg.Tasks["chat"]
	assert.Equal(t, "speed", chat.Strategy)
	assert.Equal(t, "openrouter/auto", chat.PrimaryModel)
	assert.Equal(t, 2048, chat.MaxTokens)

	quiz := cfg.Tasks["quiz_generation"]
	assert.Equal(t, "quality", quiz.Strategy)
	assert.Equal(t, "openrouter/auto", quiz.PrimaryModel)
	assert.Equal(t, 4096, quiz.MaxTokens)

	flashcards := cfg.Tasks["flashcards"]
	assert.Equal(t, "cost", flashcards.Strategy)
	assert.Equal(t, "openrouter/auto", flashcards.PrimaryModel)

	sum := cfg.Tasks["summarize_document"]
	assert.Equal(t, "cost_quality_balance", sum.Strategy)
	assert.Equal(t, "openrouter/auto", sum.PrimaryModel)

	research := cfg.Tasks["research_assistance"]
	assert.Equal(t, "quality", research.Strategy)
	assert.Equal(t, "openrouter/auto", research.PrimaryModel)
}

func TestModelRouter_GetRoute(t *testing.T) {
	router := ai.NewModelRouter(ai.DefaultRouterConfig(), nil, zap.NewNop())

	// Exact task match
	route := router.GetRoute(ai.TaskQuizGeneration)
	assert.Equal(t, ai.TaskQuizGeneration, route.Task)
	assert.Equal(t, ai.StrategyQuality, route.Strategy)
	assert.Equal(t, "openrouter/auto", route.PrimaryModel)

	// Empty task -> default (chat)
	defaultRoute := router.GetRoute("")
	assert.Equal(t, ai.TaskChat, defaultRoute.Task)
	assert.Equal(t, ai.StrategySpeed, defaultRoute.Strategy)

	// Unknown task -> fallback to default (chat)
	unknownRoute := router.GetRoute("unknown_future_task")
	assert.Equal(t, ai.TaskChat, unknownRoute.Task)
}

func TestModelRouter_RouteRequest(t *testing.T) {
	router := ai.NewModelRouter(ai.DefaultRouterConfig(), nil, zap.NewNop())

	// 1. Unspecified task and model -> gets chat defaults
	req1 := ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}
	routed1 := router.RouteRequest(req1)
	assert.Equal(t, ai.TaskChat, routed1.Task)
	assert.Equal(t, "openrouter/auto", routed1.Model)
	assert.Equal(t, 2048, routed1.MaxTokens)
	require.NotNil(t, routed1.Temperature)
	assert.Equal(t, 0.7, *routed1.Temperature)

	// 2. Specified task quiz_generation
	req2 := ai.CompletionRequest{
		Task:     ai.TaskQuizGeneration,
		Messages: []ai.Message{{Role: "user", Content: "quiz"}},
	}
	routed2 := router.RouteRequest(req2)
	assert.Equal(t, "openrouter/auto", routed2.Model)
	assert.Equal(t, 4096, routed2.MaxTokens)
	require.NotNil(t, routed2.Temperature)
	assert.Equal(t, 0.2, *routed2.Temperature)

	// 3. User overrides model and temperature
	customTemp := 0.95
	req3 := ai.CompletionRequest{
		Task:        ai.TaskFlashcards,
		Model:       "custom/my-model",
		Temperature: &customTemp,
		Messages:    []ai.Message{{Role: "user", Content: "fc"}},
	}
	routed3 := router.RouteRequest(req3)
	assert.Equal(t, "custom/my-model", routed3.Model)
	assert.Equal(t, 0.95, *routed3.Temperature)
	assert.Empty(t, routed3.FallbackModels)
}

func TestModelRouter_Delegation_CompleteAndStream(t *testing.T) {
	mockProv := &mockLLMProvider{
		resp: ai.CompletionResponse{
			ID:      "resp-1",
			Model:   "openrouter/auto",
			CostUSD: 0.002,
		},
		streamChan: make(chan ai.Chunk, 1),
	}
	mockProv.streamChan <- ai.Chunk{Content: "delegated chunk"}
	close(mockProv.streamChan)

	router := ai.NewModelRouter(ai.DefaultRouterConfig(), mockProv, zap.NewNop())
	assert.Equal(t, "router(mock-provider)", router.Name())

	// Test Complete delegation
	ctx := context.Background()
	req := ai.CompletionRequest{
		Task:     ai.TaskQuizGeneration,
		Messages: []ai.Message{{Role: "user", Content: "test"}},
	}
	resp, err := router.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "resp-1", resp.ID)
	assert.Equal(t, "openrouter/auto", mockProv.lastRequest.Model)

	// Test Stream delegation
	streamChan, err := router.Stream(ctx, req)
	require.NoError(t, err)
	chunk := <-streamChan
	assert.Equal(t, "delegated chunk", chunk.Content)
}

func TestModelRouter_NoProviderError(t *testing.T) {
	router := ai.NewModelRouter(ai.DefaultRouterConfig(), nil, zap.NewNop())
	ctx := context.Background()
	req := ai.CompletionRequest{Task: "chat"}

	_, err := router.Complete(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "underlying LLMProvider is not configured")

	_, err = router.Stream(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "underlying LLMProvider is not configured")
}
