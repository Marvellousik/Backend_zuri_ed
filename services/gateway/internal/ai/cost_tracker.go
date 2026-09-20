package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"zuri/shared/pkg/redis"
)

var (
	// ErrMissingUserID is returned when a usage record does not contain a user ID.
	ErrMissingUserID = errors.New("ai cost tracker: user_id is required")
	// ErrMissingTaskType is returned when a usage record does not contain a task type.
	ErrMissingTaskType = errors.New("ai cost tracker: task_type is required")
	// ErrMissingModel is returned when a usage record does not contain a model.
	ErrMissingModel = errors.New("ai cost tracker: model_used is required")
	// ErrInvalidCost is returned when a negative cost is provided.
	ErrInvalidCost = errors.New("ai cost tracker: cost_usd cannot be negative")
)

// UsageRecord holds telemetry for a single AI invocation.
type UsageRecord struct {
	InstitutionID    string    `json:"institution_id"`
	UserID           string    `json:"user_id"`
	TaskType         string    `json:"task_type"`
	ModelUsed        string    `json:"model_used"`
	CostUSD          float64   `json:"cost_usd"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	LatencyMs        int64     `json:"latency_ms"`
	Timestamp        time.Time `json:"timestamp"`
}

// TotalTokens returns the sum of prompt and completion tokens.
func (r *UsageRecord) TotalTokens() int {
	return r.PromptTokens + r.CompletionTokens
}

// Validate ensures required fields are set on the usage record.
func (r *UsageRecord) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrMissingUserID
	}
	if strings.TrimSpace(r.TaskType) == "" {
		return ErrMissingTaskType
	}
	if strings.TrimSpace(r.ModelUsed) == "" {
		return ErrMissingModel
	}
	if r.CostUSD < 0 {
		return ErrInvalidCost
	}
	return nil
}

// BudgetCeiling defines the maximum spending allowed for an institution over a sliding window.
type BudgetCeiling struct {
	InstitutionID string        `json:"institution_id"`
	MaxSpendUSD   float64       `json:"max_spend_usd"`
	Window        time.Duration `json:"window"`
}

// BudgetCheckResult reports the budget evaluation outcome against an institutional ceiling.
type BudgetCheckResult struct {
	Allowed        bool          `json:"allowed"`
	CurrentSpend   float64       `json:"current_spend_usd"`
	BudgetCeiling  float64       `json:"budget_ceiling_usd"`
	RemainingSpend float64       `json:"remaining_spend_usd"`
	Window         time.Duration `json:"window"`
	ResetAt        time.Time     `json:"reset_at"`
	Reason         string        `json:"reason,omitempty"`
}

// UsageSummary aggregates AI usage statistics over a dimension (institution, user, task, global).
type UsageSummary struct {
	Identifier            string           `json:"identifier"`
	TotalRequests         int64            `json:"total_requests"`
	TotalCostUSD          float64          `json:"total_cost_usd"`
	TotalPromptTokens     int64            `json:"total_prompt_tokens"`
	TotalCompletionTokens int64            `json:"total_completion_tokens"`
	TotalTokens           int64            `json:"total_tokens"`
	AverageLatencyMs      float64          `json:"average_latency_ms"`
	ByModel               map[string]int64 `json:"by_model,omitempty"`
	ByTaskType            map[string]int64 `json:"by_task_type,omitempty"`
}

// CostTrackerStore abstracts sliding window storage and metric aggregation.
type CostTrackerStore interface {
	RecordSpend(ctx context.Context, key string, score float64, member string, ttl time.Duration) error
	GetSlidingWindowSpend(ctx context.Context, key string, minScore float64) (float64, error)
	RemoveOldSpend(ctx context.Context, key string, minScore float64) error
	IncrementMetrics(ctx context.Context, key string, costUSD float64, promptTokens, completionTokens int, latencyMs int64, model, taskType string) error
	GetMetrics(ctx context.Context, key string) (*UsageSummary, error)
	ResetMetrics(ctx context.Context, key string) error
}

// CostTracker manages AI usage telemetry, cost computation, aggregations, and budget enforcement.
type CostTracker struct {
	store          CostTrackerStore
	keyPrefix      string
	budgetCeilings map[string]*BudgetCeiling
	defaultCeiling *BudgetCeiling
	customPricing  map[string]ModelPricing
	maxWindow      time.Duration
	mu             sync.RWMutex
}

// CostTrackerOption configures the CostTracker.
type CostTrackerOption func(*CostTracker)

// WithCostTrackerKeyPrefix sets a custom Redis key prefix (default: "ai_usage:").
func WithCostTrackerKeyPrefix(prefix string) CostTrackerOption {
	return func(ct *CostTracker) {
		if prefix != "" {
			ct.keyPrefix = prefix
		}
	}
}

// WithDefaultBudget sets a fallback budget ceiling for institutions with no custom ceiling.
func WithDefaultBudget(maxSpendUSD float64, window time.Duration) CostTrackerOption {
	return func(ct *CostTracker) {
		if maxSpendUSD > 0 && window > 0 {
			ct.defaultCeiling = &BudgetCeiling{
				InstitutionID: "DEFAULT",
				MaxSpendUSD:   maxSpendUSD,
				Window:        window,
			}
		}
	}
}

// WithCustomPricing registers or overrides model token pricing.
func WithCustomPricing(model string, promptCostPer1M, completionCostPer1M float64) CostTrackerOption {
	return func(ct *CostTracker) {
		ct.customPricing[strings.ToLower(strings.TrimSpace(model))] = ModelPricing{
			PromptPerMillion:     promptCostPer1M,
			CompletionPerMillion: completionCostPer1M,
		}
	}
}

// NewCostTracker creates a CostTracker backed by a CostTrackerStore.
func NewCostTracker(store CostTrackerStore, opts ...CostTrackerOption) *CostTracker {
	if store == nil {
		store = NewMemoryCostTrackerStore()
	}

	ct := &CostTracker{
		store:          store,
		keyPrefix:      "ai_usage:",
		budgetCeilings: make(map[string]*BudgetCeiling),
		customPricing:  make(map[string]ModelPricing),
		maxWindow:      30 * 24 * time.Hour, // 30-day max TTL for sliding window entries
	}

	for _, opt := range opts {
		opt(ct)
	}

	return ct
}

// NewRedisCostTracker creates a CostTracker backed by Zuri's shared Redis client.
func NewRedisCostTracker(client *redis.Client, opts ...CostTrackerOption) *CostTracker {
	var store CostTrackerStore
	if client != nil && client.Client() != nil {
		store = NewRedisCostTrackerStore(client.Client())
	} else {
		store = NewMemoryCostTrackerStore()
	}
	return NewCostTracker(store, opts...)
}

// SetBudgetCeiling configures the maximum spend ceiling for an institution.
func (ct *CostTracker) SetBudgetCeiling(institutionID string, maxSpendUSD float64, window time.Duration) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	id := strings.TrimSpace(institutionID)
	if id == "" {
		id = "DEFAULT"
	}

	ct.budgetCeilings[id] = &BudgetCeiling{
		InstitutionID: id,
		MaxSpendUSD:   maxSpendUSD,
		Window:        window,
	}
}

// GetBudgetCeiling returns the configured budget ceiling for an institution.
func (ct *CostTracker) GetBudgetCeiling(institutionID string) (*BudgetCeiling, bool) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	id := strings.TrimSpace(institutionID)
	ceiling, ok := ct.budgetCeilings[id]
	if ok {
		return ceiling, true
	}

	if ct.defaultCeiling != nil {
		return ct.defaultCeiling, true
	}

	return nil, false
}

// RemoveBudgetCeiling removes the configured ceiling for an institution.
func (ct *CostTracker) RemoveBudgetCeiling(institutionID string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.budgetCeilings, strings.TrimSpace(institutionID))
}

// RegisterModelPricing registers or updates token pricing for a specific model.
func (ct *CostTracker) RegisterModelPricing(model string, promptCostPer1M, completionCostPer1M float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.customPricing[strings.ToLower(strings.TrimSpace(model))] = ModelPricing{
		PromptPerMillion:     promptCostPer1M,
		CompletionPerMillion: completionCostPer1M,
	}
}

// CalculateCost computes the estimated monetary cost in USD for a model call.
func (ct *CostTracker) CalculateCost(model string, promptTokens, completionTokens int) float64 {
	ct.mu.RLock()
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	custom, ok := ct.customPricing[normalizedModel]
	ct.mu.RUnlock()

	if ok {
		promptCost := (float64(promptTokens) / 1_000_000.0) * custom.PromptPerMillion
		compCost := (float64(completionTokens) / 1_000_000.0) * custom.CompletionPerMillion
		return promptCost + compCost
	}

	return CalculateTokenCost(model, promptTokens, completionTokens)
}

// RecordUsage logs an AI usage event, updates sliding window spend, and increments aggregated counters.
func (ct *CostTracker) RecordUsage(ctx context.Context, record *UsageRecord) error {
	if record == nil {
		return errors.New("ai cost tracker: usage record cannot be nil")
	}

	if err := record.Validate(); err != nil {
		return err
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	// Auto-compute cost if not explicitly supplied
	if record.CostUSD <= 0 {
		record.CostUSD = ct.CalculateCost(record.ModelUsed, record.PromptTokens, record.CompletionTokens)
	}

	instID := strings.TrimSpace(record.InstitutionID)
	if instID == "" {
		instID = "GLOBAL"
	}

	// 1. Record spend in sliding window for institution
	score := float64(record.Timestamp.UnixNano())
	member := fmt.Sprintf("%.6f:%d:%s", record.CostUSD, record.Timestamp.UnixNano(), uuid.New().String())
	spendKey := ct.keyPrefix + "spend:window:" + instID
	if err := ct.store.RecordSpend(ctx, spendKey, score, member, ct.maxWindow); err != nil {
		return fmt.Errorf("failed to record spend in sliding window: %w", err)
	}

	// 2. Increment aggregated metrics across dimensions
	keys := []string{
		ct.keyPrefix + "agg:inst:" + instID,
		ct.keyPrefix + "agg:user:" + record.UserID,
		ct.keyPrefix + "agg:task:" + record.TaskType,
		ct.keyPrefix + "agg:global",
	}

	for _, key := range keys {
		if err := ct.store.IncrementMetrics(
			ctx,
			key,
			record.CostUSD,
			record.PromptTokens,
			record.CompletionTokens,
			record.LatencyMs,
			record.ModelUsed,
			record.TaskType,
		); err != nil {
			return fmt.Errorf("failed to increment metrics for %s: %w", key, err)
		}
	}

	return nil
}

// CheckSpend evaluates whether the institution has enough budget remaining in its sliding window.
func (ct *CostTracker) CheckSpend(ctx context.Context, institutionID string, estimatedCostUSD float64) (*BudgetCheckResult, error) {
	instID := strings.TrimSpace(institutionID)
	if instID == "" {
		instID = "GLOBAL"
	}

	ceiling, exists := ct.GetBudgetCeiling(instID)
	if !exists {
		// No ceiling defined -> unconstrained budget
		return &BudgetCheckResult{
			Allowed:        true,
			CurrentSpend:   0.0,
			BudgetCeiling:  math.MaxFloat64,
			RemainingSpend: math.MaxFloat64,
			Window:         24 * time.Hour,
			ResetAt:        time.Now().Add(24 * time.Hour),
		}, nil
	}

	return ct.CheckSpendAgainstCeiling(ctx, instID, ceiling.MaxSpendUSD, ceiling.Window, estimatedCostUSD)
}

// CheckSpendAgainstCeiling checks current sliding-window spend against an explicit ceiling and window duration.
func (ct *CostTracker) CheckSpendAgainstCeiling(
	ctx context.Context,
	institutionID string,
	ceilingUSD float64,
	window time.Duration,
	estimatedCostUSD float64,
) (*BudgetCheckResult, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}

	now := time.Now().UTC()
	minScore := float64(now.Add(-window).UnixNano())
	spendKey := ct.keyPrefix + "spend:window:" + institutionID

	currentSpend, err := ct.store.GetSlidingWindowSpend(ctx, spendKey, minScore)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate sliding window spend: %w", err)
	}

	projectedSpend := currentSpend + estimatedCostUSD
	allowed := projectedSpend <= ceilingUSD
	remaining := ceilingUSD - currentSpend
	if remaining < 0 {
		remaining = 0
	}

	var reason string
	if !allowed {
		reason = fmt.Sprintf(
			"budget ceiling exceeded for institution '%s': current spend $%.4f + requested $%.4f exceeds ceiling $%.4f",
			institutionID, currentSpend, estimatedCostUSD, ceilingUSD,
		)
	}

	return &BudgetCheckResult{
		Allowed:        allowed,
		CurrentSpend:   currentSpend,
		BudgetCeiling:  ceilingUSD,
		RemainingSpend: remaining,
		Window:         window,
		ResetAt:        now.Add(window),
		Reason:         reason,
	}, nil
}

// CheckBudget is a convenience method evaluating whether an institution ceiling allows 0-delta spend.
func (ct *CostTracker) CheckBudget(ctx context.Context, ceiling *BudgetCeiling) (*BudgetCheckResult, error) {
	if ceiling == nil {
		return ct.CheckSpend(ctx, "", 0.0)
	}
	return ct.CheckSpendAgainstCeiling(ctx, ceiling.InstitutionID, ceiling.MaxSpendUSD, ceiling.Window, 0.0)
}

// GetSlidingWindowSpend returns current total spend for an institution over the given window.
func (ct *CostTracker) GetSlidingWindowSpend(ctx context.Context, institutionID string, window time.Duration) (float64, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	minScore := float64(time.Now().UTC().Add(-window).UnixNano())
	return ct.store.GetSlidingWindowSpend(ctx, ct.keyPrefix+"spend:window:"+institutionID, minScore)
}

// GetInstitutionSummary returns aggregated AI usage metrics for a specific institution.
func (ct *CostTracker) GetInstitutionSummary(ctx context.Context, institutionID string) (*UsageSummary, error) {
	instID := strings.TrimSpace(institutionID)
	if instID == "" {
		instID = "GLOBAL"
	}
	summary, err := ct.store.GetMetrics(ctx, ct.keyPrefix+"agg:inst:"+instID)
	if err != nil {
		return nil, err
	}
	summary.Identifier = instID
	return summary, nil
}

// GetUserSummary returns aggregated AI usage metrics for a specific user.
func (ct *CostTracker) GetUserSummary(ctx context.Context, userID string) (*UsageSummary, error) {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return nil, ErrMissingUserID
	}
	summary, err := ct.store.GetMetrics(ctx, ct.keyPrefix+"agg:user:"+uid)
	if err != nil {
		return nil, err
	}
	summary.Identifier = uid
	return summary, nil
}

// GetTaskTypeSummary returns aggregated AI usage metrics for a specific task type (e.g. "chat", "quiz").
func (ct *CostTracker) GetTaskTypeSummary(ctx context.Context, taskType string) (*UsageSummary, error) {
	tt := strings.TrimSpace(taskType)
	if tt == "" {
		return nil, ErrMissingTaskType
	}
	summary, err := ct.store.GetMetrics(ctx, ct.keyPrefix+"agg:task:"+tt)
	if err != nil {
		return nil, err
	}
	summary.Identifier = tt
	return summary, nil
}

// GetGlobalSummary returns aggregated AI usage metrics across the entire platform.
func (ct *CostTracker) GetGlobalSummary(ctx context.Context) (*UsageSummary, error) {
	summary, err := ct.store.GetMetrics(ctx, ct.keyPrefix+"agg:global")
	if err != nil {
		return nil, err
	}
	summary.Identifier = "GLOBAL"
	return summary, nil
}

// ResetInstitutionMetrics resets aggregated metrics and sliding-window spend for an institution.
func (ct *CostTracker) ResetInstitutionMetrics(ctx context.Context, institutionID string) error {
	instID := strings.TrimSpace(institutionID)
	if instID == "" {
		instID = "GLOBAL"
	}
	_ = ct.store.ResetMetrics(ctx, ct.keyPrefix+"agg:inst:"+instID)
	_ = ct.store.ResetMetrics(ctx, ct.keyPrefix+"spend:window:"+instID)
	return nil
}

// ==================== Redis Store Implementation ====================

// RedisCostTrackerStore implements CostTrackerStore using go-redis/v9.
type RedisCostTrackerStore struct {
	client goredis.Cmdable
}

// NewRedisCostTrackerStore creates a new Redis-backed cost tracker store.
func NewRedisCostTrackerStore(client goredis.Cmdable) *RedisCostTrackerStore {
	return &RedisCostTrackerStore{client: client}
}

// RecordSpend inserts a spend entry into the Redis sorted set and sets the TTL.
func (s *RedisCostTrackerStore) RecordSpend(ctx context.Context, key string, score float64, member string, ttl time.Duration) error {
	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: score, Member: member})
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// GetSlidingWindowSpend purges expired entries and sums spend in the window.
func (s *RedisCostTrackerStore) GetSlidingWindowSpend(ctx context.Context, key string, minScore float64) (float64, error) {
	// Purge expired entries older than minScore
	_ = s.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("(%f", minScore)).Err()

	// Query entries currently in window
	entries, err := s.client.ZRangeByScore(ctx, key, &goredis.ZRangeBy{
		Min: fmt.Sprintf("%f", minScore),
		Max: "+inf",
	}).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, nil
		}
		return 0, err
	}

	var totalSpend float64
	for _, entry := range entries {
		// Member format: "<cost_usd>:<timestamp_nano>:<uuid>"
		parts := strings.Split(entry, ":")
		if len(parts) > 0 {
			cost, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				totalSpend += cost
			}
		}
	}

	return totalSpend, nil
}

// RemoveOldSpend removes spend entries with scores strictly less than minScore.
func (s *RedisCostTrackerStore) RemoveOldSpend(ctx context.Context, key string, minScore float64) error {
	return s.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("(%f", minScore)).Err()
}

// IncrementMetrics atomically increments usage telemetry in a Redis hash.
func (s *RedisCostTrackerStore) IncrementMetrics(
	ctx context.Context,
	key string,
	costUSD float64,
	promptTokens, completionTokens int,
	latencyMs int64,
	model, taskType string,
) error {
	pipe := s.client.Pipeline()
	pipe.HIncrBy(ctx, key, "total_requests", 1)
	pipe.HIncrByFloat(ctx, key, "total_cost_usd", costUSD)
	pipe.HIncrBy(ctx, key, "prompt_tokens", int64(promptTokens))
	pipe.HIncrBy(ctx, key, "completion_tokens", int64(completionTokens))
	pipe.HIncrBy(ctx, key, "total_latency_ms", latencyMs)
	if model != "" {
		pipe.HIncrBy(ctx, key, "model:"+model, 1)
	}
	if taskType != "" {
		pipe.HIncrBy(ctx, key, "task:"+taskType, 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// GetMetrics retrieves and parses aggregated usage metrics from a Redis hash.
func (s *RedisCostTrackerStore) GetMetrics(ctx context.Context, key string) (*UsageSummary, error) {
	data, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return &UsageSummary{}, nil
		}
		return nil, err
	}

	summary := &UsageSummary{
		ByModel:    make(map[string]int64),
		ByTaskType: make(map[string]int64),
	}

	var totalLatency int64

	for k, v := range data {
		switch {
		case k == "total_requests":
			summary.TotalRequests, _ = strconv.ParseInt(v, 10, 64)
		case k == "total_cost_usd":
			summary.TotalCostUSD, _ = strconv.ParseFloat(v, 64)
		case k == "prompt_tokens":
			summary.TotalPromptTokens, _ = strconv.ParseInt(v, 10, 64)
		case k == "completion_tokens":
			summary.TotalCompletionTokens, _ = strconv.ParseInt(v, 10, 64)
		case k == "total_latency_ms":
			totalLatency, _ = strconv.ParseInt(v, 10, 64)
		case strings.HasPrefix(k, "model:"):
			modelName := strings.TrimPrefix(k, "model:")
			count, _ := strconv.ParseInt(v, 10, 64)
			summary.ByModel[modelName] = count
		case strings.HasPrefix(k, "task:"):
			taskName := strings.TrimPrefix(k, "task:")
			count, _ := strconv.ParseInt(v, 10, 64)
			summary.ByTaskType[taskName] = count
		}
	}

	summary.TotalTokens = summary.TotalPromptTokens + summary.TotalCompletionTokens
	if summary.TotalRequests > 0 {
		summary.AverageLatencyMs = float64(totalLatency) / float64(summary.TotalRequests)
	}

	return summary, nil
}

// ResetMetrics deletes the metric key in Redis.
func (s *RedisCostTrackerStore) ResetMetrics(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// ==================== Memory Store Implementation ====================

type memorySpendItem struct {
	score float64
	cost  float64
}

// MemoryCostTrackerStore provides a thread-safe in-memory store for testing and development.
type MemoryCostTrackerStore struct {
	mu      sync.RWMutex
	spend   map[string][]memorySpendItem
	metrics map[string]*UsageSummary
	lat     map[string]int64
}

// NewMemoryCostTrackerStore creates a new in-memory CostTrackerStore.
func NewMemoryCostTrackerStore() *MemoryCostTrackerStore {
	return &MemoryCostTrackerStore{
		spend:   make(map[string][]memorySpendItem),
		metrics: make(map[string]*UsageSummary),
		lat:     make(map[string]int64),
	}
}

// RecordSpend records a spend item in memory.
func (m *MemoryCostTrackerStore) RecordSpend(ctx context.Context, key string, score float64, member string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cost float64
	parts := strings.Split(member, ":")
	if len(parts) > 0 {
		cost, _ = strconv.ParseFloat(parts[0], 64)
	}

	m.spend[key] = append(m.spend[key], memorySpendItem{
		score: score,
		cost:  cost,
	})
	return nil
}

// GetSlidingWindowSpend calculates the total spend within the sliding window.
func (m *MemoryCostTrackerStore) GetSlidingWindowSpend(ctx context.Context, key string, minScore float64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := m.spend[key]
	var valid []memorySpendItem
	var total float64

	for _, it := range items {
		if it.score >= minScore {
			valid = append(valid, it)
			total += it.cost
		}
	}

	m.spend[key] = valid
	return total, nil
}

// RemoveOldSpend purges entries with score < minScore.
func (m *MemoryCostTrackerStore) RemoveOldSpend(ctx context.Context, key string, minScore float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := m.spend[key]
	var valid []memorySpendItem
	for _, it := range items {
		if it.score >= minScore {
			valid = append(valid, it)
		}
	}
	m.spend[key] = valid
	return nil
}

// IncrementMetrics increments in-memory aggregate metrics.
func (m *MemoryCostTrackerStore) IncrementMetrics(
	ctx context.Context,
	key string,
	costUSD float64,
	promptTokens, completionTokens int,
	latencyMs int64,
	model, taskType string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sum, ok := m.metrics[key]
	if !ok {
		sum = &UsageSummary{
			ByModel:    make(map[string]int64),
			ByTaskType: make(map[string]int64),
		}
		m.metrics[key] = sum
	}

	sum.TotalRequests++
	sum.TotalCostUSD += costUSD
	sum.TotalPromptTokens += int64(promptTokens)
	sum.TotalCompletionTokens += int64(completionTokens)
	sum.TotalTokens = sum.TotalPromptTokens + sum.TotalCompletionTokens
	m.lat[key] += latencyMs

	if model != "" {
		sum.ByModel[model]++
	}
	if taskType != "" {
		sum.ByTaskType[taskType]++
	}

	if sum.TotalRequests > 0 {
		summaryLatency := float64(m.lat[key]) / float64(sum.TotalRequests)
		sum.AverageLatencyMs = summaryLatency
	}

	return nil
}

// GetMetrics retrieves a copy of aggregate metrics from memory.
func (m *MemoryCostTrackerStore) GetMetrics(ctx context.Context, key string) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sum, ok := m.metrics[key]
	if !ok {
		return &UsageSummary{
			ByModel:    make(map[string]int64),
			ByTaskType: make(map[string]int64),
		}, nil
	}

	// Deep copy maps to avoid concurrent map read/write
	byModel := make(map[string]int64)
	for k, v := range sum.ByModel {
		byModel[k] = v
	}

	byTask := make(map[string]int64)
	for k, v := range sum.ByTaskType {
		byTask[k] = v
	}

	return &UsageSummary{
		Identifier:            sum.Identifier,
		TotalRequests:         sum.TotalRequests,
		TotalCostUSD:          sum.TotalCostUSD,
		TotalPromptTokens:     sum.TotalPromptTokens,
		TotalCompletionTokens: sum.TotalCompletionTokens,
		TotalTokens:           sum.TotalTokens,
		AverageLatencyMs:      sum.AverageLatencyMs,
		ByModel:               byModel,
		ByTaskType:            byTask,
	}, nil
}

// ResetMetrics clears metrics and spend for the given key.
func (m *MemoryCostTrackerStore) ResetMetrics(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.metrics, key)
	delete(m.lat, key)
	delete(m.spend, key)
	return nil
}
