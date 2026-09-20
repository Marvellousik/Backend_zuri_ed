package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageRecord_Validation(t *testing.T) {
	record := UsageRecord{
		InstitutionID:    "veritas_uni",
		UserID:           "user_123",
		TaskType:         "quiz_generation",
		ModelUsed:        "anthropic/claude-3.5-sonnet",
		CostUSD:          0.002,
		PromptTokens:     150,
		CompletionTokens: 50,
		LatencyMs:        450,
		Timestamp:        time.Now(),
	}

	assert.NoError(t, record.Validate())
	assert.Equal(t, 200, record.TotalTokens())

	// Missing user_id fails
	invalidUser := record
	invalidUser.UserID = ""
	assert.ErrorIs(t, invalidUser.Validate(), ErrMissingUserID)

	// Missing task_type fails
	invalidTask := record
	invalidTask.TaskType = ""
	assert.ErrorIs(t, invalidTask.Validate(), ErrMissingTaskType)

	// Missing model_used fails
	invalidModel := record
	invalidModel.ModelUsed = ""
	assert.ErrorIs(t, invalidModel.Validate(), ErrMissingModel)

	// Negative cost fails
	invalidCost := record
	invalidCost.CostUSD = -0.5
	assert.ErrorIs(t, invalidCost.Validate(), ErrInvalidCost)
}

func TestCostTracker_RecordAndBudgetCeilings(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryCostTrackerStore()
	tracker := NewCostTracker(store)

	institutionID := "veritas_uni"
	ceiling := &BudgetCeiling{
		InstitutionID: institutionID,
		MaxSpendUSD:   0.05,
		Window:        1 * time.Hour,
	}

	// 1. Initial check under ceiling
	check1, err := tracker.CheckBudget(ctx, ceiling)
	require.NoError(t, err)
	assert.True(t, check1.Allowed)
	assert.Equal(t, 0.05, check1.RemainingSpend)

	// 2. Record spend
	rec1 := &UsageRecord{
		InstitutionID:    institutionID,
		UserID:           "user_1",
		TaskType:         "chat",
		ModelUsed:        "google/gemini-2.5-flash",
		CostUSD:          0.02,
		PromptTokens:     100,
		CompletionTokens: 50,
		LatencyMs:        300,
		Timestamp:        time.Now(),
	}
	err = tracker.RecordUsage(ctx, rec1)
	require.NoError(t, err)

	// 3. Check remaining budget
	check2, err := tracker.CheckBudget(ctx, ceiling)
	require.NoError(t, err)
	assert.True(t, check2.Allowed)
	assert.InDelta(t, 0.03, check2.RemainingSpend, 0.001)

	// 4. Record spend that breaches ceiling
	rec2 := &UsageRecord{
		InstitutionID:    institutionID,
		UserID:           "user_2",
		TaskType:         "quiz_generation",
		ModelUsed:        "anthropic/claude-3.5-sonnet",
		CostUSD:          0.04,
		PromptTokens:     200,
		CompletionTokens: 100,
		LatencyMs:        600,
		Timestamp:        time.Now(),
	}
	err = tracker.RecordUsage(ctx, rec2)
	require.NoError(t, err)

	// 5. Check budget should now be rejected
	check3, err := tracker.CheckBudget(ctx, ceiling)
	require.NoError(t, err)
	assert.False(t, check3.Allowed, "Budget should be exceeded")
	assert.LessOrEqual(t, check3.RemainingSpend, 0.0)
	assert.Contains(t, check3.Reason, "exceeded")
}
