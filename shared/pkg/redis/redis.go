// Package redis provides Redis client wrapper with helper functions.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis client with additional functionality.
type Client struct {
	client *redis.Client
}

// Config holds Redis configuration.
type Config struct {
	Addr     string
	Password string
	DB       int
}

// NewClient creates a new Redis client.
func NewClient(cfg *Config) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 10,
	})

	return &Client{client: client}, nil
}

// Ping checks if Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// Client returns the underlying go-redis client.
func (c *Client) Client() *redis.Client {
	return c.client
}

// Get retrieves a value from Redis.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// GetJSON retrieves and unmarshals a JSON value from Redis.
func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	return json.Unmarshal([]byte(val), dest)
}

// Set stores a value in Redis with optional expiration.
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		val = string(bytes)
	}

	return c.client.Set(ctx, key, val, expiration).Err()
}

// Delete removes a key from Redis.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists in Redis.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Increment atomically increments a counter.
func (c *Client) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// IncrementBy increments a counter by a specific amount.
func (c *Client) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

// Decrement atomically decrements a counter.
func (c *Client) Decrement(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, key).Result()
}

// Expire sets an expiration on a key.
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// TTL returns the remaining time to live of a key.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	Window      time.Duration
	MaxRequests int
	KeyPrefix   string
}

// CheckRateLimit checks if a request is within rate limits using sliding window.
// Returns true if allowed, along with remaining requests and reset time.
func (c *Client) CheckRateLimit(ctx context.Context, identifier string, config *RateLimitConfig) (allowed bool, remaining int, resetAt time.Time, err error) {
	key := fmt.Sprintf("%s:%s", config.KeyPrefix, identifier)
	now := time.Now().Unix()
	windowStart := now - int64(config.Window.Seconds())

	pipe := c.client.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)

	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})

	// Set expiration on the key
	pipe.Expire(ctx, key, config.Window)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("rate limit check failed: %w", err)
	}

	count := int(countCmd.Val())
	remaining = config.MaxRequests - count

	if remaining < 0 {
		// Get the oldest request to calculate reset time
		oldest, err := c.client.ZRange(ctx, key, 0, 0).Result()
		if err != nil || len(oldest) == 0 {
			return false, 0, time.Now().Add(config.Window), nil
		}

		oldestScore, _ := strconv.ParseFloat(oldest[0], 64)
		resetAt = time.Unix(int64(oldestScore), 0).Add(config.Window)
		return false, 0, resetAt, nil
	}

	return true, remaining, time.Now().Add(config.Window), nil
}

// Publish publishes a message to a Redis channel.
func (c *Client) Publish(ctx context.Context, channel string, message interface{}) error {
	var msg string
	switch v := message.(type) {
	case string:
		msg = v
	case []byte:
		msg = string(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		msg = string(bytes)
	}

	return c.client.Publish(ctx, channel, msg).Err()
}

// Subscribe subscribes to Redis channels.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// AddToSortedSet adds a member with score to a sorted set.
func (c *Client) AddToSortedSet(ctx context.Context, key string, score float64, member interface{}) error {
	return c.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// GetSortedSetRange retrieves members from a sorted set within a score range.
func (c *Client) GetSortedSetRange(ctx context.Context, key string, min, max float64) ([]redis.Z, error) {
	return c.client.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{Min: fmt.Sprintf("%f", min), Max: fmt.Sprintf("%f", max)}).Result()
}

// RemoveFromSortedSet removes members from a sorted set.
func (c *Client) RemoveFromSortedSet(ctx context.Context, key string, members ...interface{}) error {
	return c.client.ZRem(ctx, key, members...).Err()
}

// XAdd adds a message to a Redis Stream.
func (c *Client) XAdd(ctx context.Context, stream string, values map[string]interface{}) (string, error) {
	return c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Result()
}

// XRead reads messages from Redis Streams.
func (c *Client) XRead(ctx context.Context, streams []string, ids []string, count int64, block time.Duration) ([]redis.XStream, error) {
	return c.client.XRead(ctx, &redis.XReadArgs{
		Streams: append(streams, ids...),
		Count:   count,
		Block:   block,
	}).Result()
}

// XGroupCreate creates a consumer group for a stream.
func (c *Client) XGroupCreate(ctx context.Context, stream, group, startID string) error {
	return c.client.XGroupCreate(ctx, stream, group, startID).Err()
}

// XReadGroup reads messages as part of a consumer group.
func (c *Client) XReadGroup(ctx context.Context, group, consumer string, streams []string, ids []string, count int64, block time.Duration) ([]redis.XStream, error) {
	return c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  append(streams, ids...),
		Count:    count,
		Block:    block,
	}).Result()
}

// XAck acknowledges messages in a stream.
func (c *Client) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return c.client.XAck(ctx, stream, group, ids...).Err()
}

// HealthCheck performs a health check on Redis.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}
