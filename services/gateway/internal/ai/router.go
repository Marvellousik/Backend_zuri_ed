// Package ai provides the Unified AI Gateway abstractions, OpenRouter client adapter,
// task-based dynamic model routing, and streaming utilities for Zuri.
package ai

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"zuri/shared/pkg/logger"
)

// Supported Tasks
const (
	TaskChat               = "chat"
	TaskQuizGeneration     = "quiz_generation"
	TaskFlashcards         = "flashcards"
	TaskSummarizeDocument  = "summarize_document"
	TaskResearchAssistance = "research_assistance"
)

// Supported Routing Strategies
const (
	StrategySpeed              = "speed"
	StrategyQuality            = "quality"
	StrategyCost               = "cost"
	StrategyCostQualityBalance = "cost_quality_balance"
)

// TaskRoute defines model routing and default hyperparameter configuration for a task.
type TaskRoute struct {
	Task           string   `yaml:"-" json:"task"`
	Strategy       string   `yaml:"strategy" json:"strategy"`
	PrimaryModel   string   `yaml:"primary" json:"primary"`
	FallbackModels []string `yaml:"fallbacks" json:"fallbacks"`
	MaxTokens      int      `yaml:"max_tokens" json:"max_tokens"`
	Temperature    float64  `yaml:"temperature" json:"temperature"`
}

// RouterConfigFile models the YAML configuration format for ai_routing.yaml.
type RouterConfigFile struct {
	Version     string               `yaml:"version"`
	DefaultTask string               `yaml:"default_task"`
	Tasks       map[string]TaskRoute `yaml:"tasks"`
}

// DefaultRouterConfig returns the default model chains for all Zuri AI tasks.
func DefaultRouterConfig() *RouterConfigFile {
	return &RouterConfigFile{
		Version:     "2.0",
		DefaultTask: TaskChat,
		Tasks: map[string]TaskRoute{
			TaskChat: {
				Task:           TaskChat,
				Strategy:       StrategySpeed,
				PrimaryModel:   "openrouter/auto",
				FallbackModels: nil,
				MaxTokens:      2048,
				Temperature:    0.7,
			},
			TaskQuizGeneration: {
				Task:           TaskQuizGeneration,
				Strategy:       StrategyQuality,
				PrimaryModel:   "openrouter/auto",
				FallbackModels: nil,
				MaxTokens:      4096,
				Temperature:    0.2,
			},
			TaskFlashcards: {
				Task:           TaskFlashcards,
				Strategy:       StrategyCost,
				PrimaryModel:   "openrouter/auto",
				FallbackModels: nil,
				MaxTokens:      2048,
				Temperature:    0.3,
			},
			TaskSummarizeDocument: {
				Task:           TaskSummarizeDocument,
				Strategy:       StrategyCostQualityBalance,
				PrimaryModel:   "openrouter/auto",
				FallbackModels: nil,
				MaxTokens:      4096,
				Temperature:    0.3,
			},
			TaskResearchAssistance: {
				Task:           TaskResearchAssistance,
				Strategy:       StrategyQuality,
				PrimaryModel:   "openrouter/auto",
				FallbackModels: nil,
				MaxTokens:      8192,
				Temperature:    0.4,
			},
		},
	}
}

// LoadRouterConfig parses a YAML configuration file from disk.
func LoadRouterConfig(filePath string) (*RouterConfigFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read router config file %s: %w", filePath, err)
	}

	var cfg RouterConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml router config from %s: %w", filePath, err)
	}

	// Populate Task key inside each TaskRoute
	for taskName, route := range cfg.Tasks {
		route.Task = taskName
		cfg.Tasks[taskName] = route
	}

	if cfg.DefaultTask == "" {
		cfg.DefaultTask = TaskChat
	}

	return &cfg, nil
}

// ModelRouter manages task-based model selection and delegates to an underlying LLMProvider.
type ModelRouter struct {
	mu          sync.RWMutex
	routes      map[string]TaskRoute
	defaultTask string
	provider    LLMProvider
	logger      *zap.Logger
}

// NewModelRouter instantiates a ModelRouter with the specified config and LLMProvider.
func NewModelRouter(cfg *RouterConfigFile, provider LLMProvider, log *zap.Logger) *ModelRouter {
	if cfg == nil {
		cfg = DefaultRouterConfig()
	}
	if log == nil {
		log = logger.Get()
		if log == nil {
			log = zap.NewNop()
		}
	}

	routes := make(map[string]TaskRoute, len(cfg.Tasks))
	for name, route := range cfg.Tasks {
		route.Task = name
		routes[name] = route
	}

	defaultTask := cfg.DefaultTask
	if defaultTask == "" {
		defaultTask = TaskChat
	}

	return &ModelRouter{
		routes:      routes,
		defaultTask: defaultTask,
		provider:    provider,
		logger:      log,
	}
}

// NewModelRouterFromFile tries to load configuration from filePath, falling back to default candidate paths.
func NewModelRouterFromFile(filePath string, provider LLMProvider, log *zap.Logger) (*ModelRouter, error) {
	if log == nil {
		log = logger.Get()
		if log == nil {
			log = zap.NewNop()
		}
	}

	candidatePaths := []string{}
	if filePath != "" {
		candidatePaths = append(candidatePaths, filePath)
	}
	candidatePaths = append(candidatePaths,
		"config/ai_routing.yaml",
		"../config/ai_routing.yaml",
		"../../config/ai_routing.yaml",
		"../../../config/ai_routing.yaml",
	)

	var loadedConfig *RouterConfigFile
	var err error

	for _, path := range candidatePaths {
		if _, statErr := os.Stat(path); statErr == nil {
			loadedConfig, err = LoadRouterConfig(path)
			if err == nil {
				log.Info("Loaded AI routing configuration", zap.String("path", path))
				break
			}
		}
	}

	if loadedConfig == nil {
		log.Warn("AI routing YAML config not found, using built-in defaults")
		loadedConfig = DefaultRouterConfig()
	}

	return NewModelRouter(loadedConfig, provider, log), nil
}

// GetRoute returns the TaskRoute configured for a given task name.
func (r *ModelRouter) GetRoute(task string) TaskRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if task == "" {
		task = r.defaultTask
	}

	if route, ok := r.routes[task]; ok {
		return route
	}

	// Fallback to default task route
	if defaultRoute, ok := r.routes[r.defaultTask]; ok {
		return defaultRoute
	}

	// Fallback to speed chat route
	return TaskRoute{
		Task:           TaskChat,
		Strategy:       StrategySpeed,
		PrimaryModel:   "google/gemini-2.5-flash",
		FallbackModels: []string{"anthropic/claude-3-5-haiku"},
		MaxTokens:      2048,
		Temperature:    0.7,
	}
}

// RouteRequest inspects the CompletionRequest and enriches it with the task's primary model,
// fallback chain, and default hyperparameters if omitted by the caller.
func (r *ModelRouter) RouteRequest(req CompletionRequest) CompletionRequest {
	task := req.Task
	if task == "" {
		task = r.defaultTask
		req.Task = task
	}

	route := r.GetRoute(task)

	if req.Model == "" {
		req.Model = route.PrimaryModel
	}

	if len(req.FallbackModels) == 0 && len(route.FallbackModels) > 0 {
		req.FallbackModels = make([]string, len(route.FallbackModels))
		copy(req.FallbackModels, route.FallbackModels)
	}

	if req.MaxTokens <= 0 && route.MaxTokens > 0 {
		req.MaxTokens = route.MaxTokens
	}

	if req.Temperature == nil && route.Temperature > 0 {
		temp := route.Temperature
		req.Temperature = &temp
	}

	return req
}

// Complete delegates the routed CompletionRequest to the underlying LLMProvider.
func (r *ModelRouter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if r.provider == nil {
		return CompletionResponse{}, fmt.Errorf("underlying LLMProvider is not configured on ModelRouter")
	}

	routedReq := r.RouteRequest(req)
	return r.provider.Complete(ctx, routedReq)
}

// Stream delegates the routed CompletionRequest stream to the underlying LLMProvider.
func (r *ModelRouter) Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error) {
	if r.provider == nil {
		return nil, fmt.Errorf("underlying LLMProvider is not configured on ModelRouter")
	}

	routedReq := r.RouteRequest(req)
	return r.provider.Stream(ctx, routedReq)
}

// Name returns the identifier for this router.
func (r *ModelRouter) Name() string {
	if r.provider != nil {
		return fmt.Sprintf("router(%s)", r.provider.Name())
	}
	return "dynamic-model-router"
}
