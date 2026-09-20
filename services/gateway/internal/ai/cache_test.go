package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputePromptHash(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful academic tutor."},
		{Role: "user", Content: "Explain dynamic programming."},
	}

	hash1 := ComputePromptHash("chat", messages, 0.7)
	hash2 := ComputePromptHash("chat", messages, 0.7)
	assert.NotEmpty(t, hash1)
	assert.Equal(t, hash1, hash2, "Prompt hash should be strictly deterministic")

	// Varying temperature changes hash
	hashDiffTemp := ComputePromptHash("chat", messages, 0.2)
	assert.NotEqual(t, hash1, hashDiffTemp)

	// Varying task type changes hash
	hashDiffTask := ComputePromptHash("quiz_generation", messages, 0.7)
	assert.NotEqual(t, hash1, hashDiffTask)
}

func TestCache_GetAndSet(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryCacheStore()
	cache := NewCache(store, WithDefaultTTL(1*time.Hour))

	messages := []Message{
		{Role: "user", Content: "Generate flashcard on binary search trees."},
	}
	hash := ComputePromptHash("flashcards", messages, 0.0)

	// 1. Initial get should miss
	missResp, err := cache.Get(ctx, hash)
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Nil(t, missResp)

	// 2. Set response
	origResp := &CompletionResponse{
		Content:   "Front: Binary Search Tree\nBack: A sorted binary tree where left < root < right.",
		Model:     "meta-llama/llama-3.3-70b-instruct",
		Usage:     Usage{PromptTokens: 25, CompletionTokens: 20, TotalTokens: 45},
		CostUSD:   0.00005,
		LatencyMs: 120,
	}

	err = cache.Set(ctx, hash, origResp, 1*time.Hour)
	require.NoError(t, err)

	// 3. Cache hit
	hitResp, err := cache.Get(ctx, hash)
	require.NoError(t, err)
	require.NotNil(t, hitResp)
	assert.Equal(t, origResp.Content, hitResp.Content)
	assert.Equal(t, origResp.Model, hitResp.Model)
	assert.Equal(t, origResp.CostUSD, hitResp.CostUSD)

	// 4. Invalidation
	err = cache.Delete(ctx, hash)
	require.NoError(t, err)

	missAgain, err := cache.Get(ctx, hash)
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Nil(t, missAgain)
}
