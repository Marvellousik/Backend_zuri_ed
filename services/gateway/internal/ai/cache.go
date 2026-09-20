// Package ai provides semantic caching and cost tracking for AI gateway operations.
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"zuri/shared/pkg/redis"
)

var (
	// ErrEmptyHash is returned when a cache operation is attempted with an empty hash.
	ErrEmptyHash = errors.New("ai cache: hash cannot be empty")
	// ErrNilResponse is returned when attempting to cache a nil completion response.
	ErrNilResponse = errors.New("ai cache: completion response cannot be nil")
	// ErrCacheMiss is a sentinel error indicating a requested key was not found.
	ErrCacheMiss = errors.New("ai cache: key not found")
)

// ComputePromptHash generates a deterministic SHA-256 hash from task type, messages, and temperature.
func ComputePromptHash(taskType string, messages []Message, temperature float64) string {
	h := sha256.New()

	// Normalize task type
	h.Write([]byte(strings.ToLower(strings.TrimSpace(taskType))))
	h.Write([]byte{0})

	// Format temperature with fixed precision (4 decimal places)
	tempStr := strconv.FormatFloat(temperature, 'f', 4, 64)
	h.Write([]byte(tempStr))
	h.Write([]byte{0})

	// Serialize messages in canonical order
	for _, msg := range messages {
		h.Write([]byte(strings.ToLower(strings.TrimSpace(msg.Role))))
		h.Write([]byte{':'})
		h.Write([]byte(msg.Content))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ComputeSimplePromptHash generates a deterministic SHA-256 hash for a single user prompt string.
func ComputeSimplePromptHash(taskType, prompt string, temperature float64) string {
	return ComputePromptHash(taskType, []Message{{Role: "user", Content: prompt}}, temperature)
}

// CacheStore abstracts the key-value storage engine (Redis or in-memory) for the semantic cache.
type CacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Cache provides deterministic AI prompt response caching.
type Cache struct {
	store      CacheStore
	keyPrefix  string
	defaultTTL time.Duration
}

// CacheOption defines a functional option for configuring the Cache.
type CacheOption func(*Cache)

// WithKeyPrefix sets a custom Redis key prefix (default: "ai_cache:").
func WithKeyPrefix(prefix string) CacheOption {
	return func(c *Cache) {
		if prefix != "" {
			c.keyPrefix = prefix
		}
	}
}

// WithDefaultTTL sets the default cache entry TTL (default: 24h).
func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *Cache) {
		if ttl > 0 {
			c.defaultTTL = ttl
		}
	}
}

// NewCache creates a new semantic prompt Cache using any CacheStore.
func NewCache(store CacheStore, opts ...CacheOption) *Cache {
	if store == nil {
		store = NewMemoryCacheStore()
	}

	c := &Cache{
		store:      store,
		keyPrefix:  "ai_cache:",
		defaultTTL: 24 * time.Hour,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// NewRedisCache creates a new semantic prompt Cache using the shared Zuri Redis client.
func NewRedisCache(client *redis.Client, opts ...CacheOption) *Cache {
	var store CacheStore
	if client != nil {
		store = client
	} else {
		store = NewMemoryCacheStore()
	}
	return NewCache(store, opts...)
}

// Get retrieves a cached AI completion response by its prompt hash.
// If the hash is not found, it returns (nil, nil).
func (c *Cache) Get(ctx context.Context, hash string) (*CompletionResponse, error) {
	if strings.TrimSpace(hash) == "" {
		return nil, ErrEmptyHash
	}

	key := c.keyPrefix + hash
	val, err := c.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to retrieve from ai cache: %w", err)
	}

	if val == "" {
		return nil, ErrCacheMiss
	}

	var resp CompletionResponse
	if err := json.Unmarshal([]byte(val), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached completion: %w", err)
	}

	// Normalize Content field if Message.Content is set
	if resp.Content == "" && resp.Message.Content != "" {
		resp.Content = resp.Message.Content
	}

	// Mark as cached hit
	resp.Cached = true
	return &resp, nil
}

// Set stores a completion response in the cache with the specified TTL.
// If ttl <= 0, the cache's default TTL is used.
func (c *Cache) Set(ctx context.Context, hash string, resp *CompletionResponse, ttl time.Duration) error {
	if strings.TrimSpace(hash) == "" {
		return ErrEmptyHash
	}
	if resp == nil {
		return ErrNilResponse
	}

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	// Ensure Content and Message.Content are in sync
	if resp.Content != "" && resp.Message.Content == "" {
		resp.Message = Message{
			Role:    "assistant",
			Content: resp.Content,
		}
	} else if resp.Content == "" && resp.Message.Content != "" {
		resp.Content = resp.Message.Content
	}

	key := c.keyPrefix + hash
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal completion response: %w", err)
	}

	if err := c.store.Set(ctx, key, string(data), ttl); err != nil {
		return fmt.Errorf("failed to store in ai cache: %w", err)
	}

	return nil
}

// Delete removes a cached entry by prompt hash.
func (c *Cache) Delete(ctx context.Context, hash string) error {
	if strings.TrimSpace(hash) == "" {
		return ErrEmptyHash
	}
	return c.store.Delete(ctx, c.keyPrefix+hash)
}

// Exists checks if a cached entry exists for the prompt hash.
func (c *Cache) Exists(ctx context.Context, hash string) (bool, error) {
	resp, err := c.Get(ctx, hash)
	if err != nil {
		return false, err
	}
	return resp != nil, nil
}

// ComputeHash is a convenience method that calls ComputePromptHash.
func (c *Cache) ComputeHash(taskType string, messages []Message, temperature float64) string {
	return ComputePromptHash(taskType, messages, temperature)
}

// ==================== In-Memory Store for Testing & Fallback ====================

type memoryCacheEntry struct {
	value     string
	expiresAt time.Time
}

// MemoryCacheStore provides a thread-safe in-memory CacheStore implementation for testing and fallback.
type MemoryCacheStore struct {
	mu      sync.RWMutex
	entries map[string]memoryCacheEntry
}

// NewMemoryCacheStore creates a new in-memory cache store.
func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{
		entries: make(map[string]memoryCacheEntry),
	}
}

// Get retrieves a value from memory.
func (m *MemoryCacheStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entries[key]
	if !ok {
		return "", nil
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return "", nil
	}

	return entry.value, nil
}

// Set stores a value in memory with an expiration.
func (m *MemoryCacheStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		val = string(bytes)
	}

	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	m.entries[key] = memoryCacheEntry{
		value:     val,
		expiresAt: expiresAt,
	}
	return nil
}

// Delete removes keys from memory.
func (m *MemoryCacheStore) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, k := range keys {
		delete(m.entries, k)
	}
	return nil
}
