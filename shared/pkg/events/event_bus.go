// Package events provides an event bus implementation for publishing and subscribing to domain events.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Default configuration constants.
const (
	DefaultStream            = "zuri:events:academic"
	DefaultDLQStream         = "zuri:events:dlq"
	DefaultIdempotencyPrefix = "events:idempotency:"
	DefaultIdempotencyTTL    = 24 * time.Hour
	DefaultMaxRetries        = 3
	DefaultRetryBackoff      = 50 * time.Millisecond
	DefaultReadCount         = int64(10)
	DefaultBlockTime         = 2 * time.Second
)

// EventBus represents the core publish-subscribe contract for asynchronous domain events.
type EventBus interface {
	// Publish publishes an event to the default event stream.
	Publish(ctx context.Context, event *AcademicEvent) error

	// PublishWithIdempotency publishes an event ensuring idempotency using Redis SETNX.
	PublishWithIdempotency(ctx context.Context, event *AcademicEvent) error

	// Subscribe listens for events on a stream using consumer groups with automatic retries and DLQ routing.
	Subscribe(ctx context.Context, stream string, group string, consumer string, handler func(event *AcademicEvent) error) error
}

// RedisClient defines the subset of Redis operations used by RedisEventBus.
// Both *redis.Client and test mock implementations satisfy this interface.
type RedisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	XGroupCreate(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

// DLQEvent records comprehensive metadata when an event exceeds maximum retry attempts.
type DLQEvent struct {
	EventID        string                 `json:"event_id"`
	EventType      string                 `json:"event_type"`
	OriginalStream string                 `json:"original_stream"`
	ConsumerGroup  string                 `json:"consumer_group"`
	ConsumerName   string                 `json:"consumer_name"`
	OriginalMsgID  string                 `json:"original_message_id"`
	RetryCount     int                    `json:"retry_count"`
	ErrorMessage   string                 `json:"error_message"`
	FailedAt       time.Time              `json:"failed_at"`
	OriginalEvent  *AcademicEvent         `json:"original_event,omitempty"`
	RawValues      map[string]interface{} `json:"raw_values,omitempty"`
}

// RedisEventBus implements EventBus backed by Redis Streams, consumer groups, and DLQ.
type RedisEventBus struct {
	client            RedisClient
	defaultStream     string
	dlqStream         string
	idempotencyPrefix string
	idempotencyTTL    time.Duration
	maxRetries        int
	retryBackoff      time.Duration
	readCount         int64
	blockTime         time.Duration
	logger            *zap.Logger
}

// Option configures RedisEventBus parameters.
type Option func(*RedisEventBus)

// WithDefaultStream sets the default stream name for publishing.
func WithDefaultStream(stream string) Option {
	return func(b *RedisEventBus) {
		if stream != "" {
			b.defaultStream = stream
		}
	}
}

// WithDLQStream sets the Dead-Letter Queue stream name.
func WithDLQStream(dlqStream string) Option {
	return func(b *RedisEventBus) {
		if dlqStream != "" {
			b.dlqStream = dlqStream
		}
	}
}

// WithIdempotencyPrefix sets the Redis key prefix for idempotency tracking.
func WithIdempotencyPrefix(prefix string) Option {
	return func(b *RedisEventBus) {
		if prefix != "" {
			b.idempotencyPrefix = prefix
		}
	}
}

// WithIdempotencyTTL sets the TTL for stored idempotency keys.
func WithIdempotencyTTL(ttl time.Duration) Option {
	return func(b *RedisEventBus) {
		if ttl > 0 {
			b.idempotencyTTL = ttl
		}
	}
}

// WithMaxRetries sets the maximum number of consumer retry attempts before routing to DLQ.
func WithMaxRetries(retries int) Option {
	return func(b *RedisEventBus) {
		if retries > 0 {
			b.maxRetries = retries
		}
	}
}

// WithRetryBackoff sets the pause duration between retry attempts.
func WithRetryBackoff(backoff time.Duration) Option {
	return func(b *RedisEventBus) {
		b.retryBackoff = backoff
	}
}

// WithReadCount sets the batch size for XReadGroup.
func WithReadCount(count int64) Option {
	return func(b *RedisEventBus) {
		if count > 0 {
			b.readCount = count
		}
	}
}

// WithBlockTime sets the blocking timeout for XReadGroup.
func WithBlockTime(block time.Duration) Option {
	return func(b *RedisEventBus) {
		if block >= 0 {
			b.blockTime = block
		}
	}
}

// WithLogger sets the structured logger.
func WithLogger(logger *zap.Logger) Option {
	return func(b *RedisEventBus) {
		if logger != nil {
			b.logger = logger
		}
	}
}

// NewRedisEventBus initializes a new Redis-backed EventBus.
func NewRedisEventBus(client RedisClient, opts ...Option) *RedisEventBus {
	bus := &RedisEventBus{
		client:            client,
		defaultStream:     DefaultStream,
		dlqStream:         DefaultDLQStream,
		idempotencyPrefix: DefaultIdempotencyPrefix,
		idempotencyTTL:    DefaultIdempotencyTTL,
		maxRetries:        DefaultMaxRetries,
		retryBackoff:      DefaultRetryBackoff,
		readCount:         DefaultReadCount,
		blockTime:         DefaultBlockTime,
		logger:            zap.NewNop(),
	}

	for _, opt := range opts {
		opt(bus)
	}

	return bus
}

// DefaultStream returns the configured default stream name.
func (b *RedisEventBus) DefaultStream() string {
	return b.defaultStream
}

// DLQStream returns the configured dead-letter queue stream name.
func (b *RedisEventBus) DLQStream() string {
	return b.dlqStream
}

// IdempotencyPrefix returns the configured idempotency key prefix.
func (b *RedisEventBus) IdempotencyPrefix() string {
	return b.idempotencyPrefix
}

// IdempotencyTTL returns the configured idempotency key TTL.
func (b *RedisEventBus) IdempotencyTTL() time.Duration {
	return b.idempotencyTTL
}

// MaxRetries returns the maximum retry limit.
func (b *RedisEventBus) MaxRetries() int {
	return b.maxRetries
}

// Publish publishes an event to the default stream.
func (b *RedisEventBus) Publish(ctx context.Context, event *AcademicEvent) error {
	return b.PublishToStream(ctx, b.defaultStream, event)
}

// PublishToStream publishes an event to a specific target stream.
func (b *RedisEventBus) PublishToStream(ctx context.Context, stream string, event *AcademicEvent) error {
	if event == nil {
		return ErrInvalidEvent
	}
	if err := event.Validate(); err != nil {
		return err
	}

	if stream == "" {
		stream = b.defaultStream
	}

	values, err := event.ToMap()
	if err != nil {
		return fmt.Errorf("failed to marshal event for stream: %w", err)
	}

	_, err = b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to publish event to stream %s: %w", stream, err)
	}

	return nil
}

// PublishWithIdempotency publishes an event ensuring idempotency using Redis SETNX with TTL.
// If the idempotency key already exists, ErrDuplicateEvent is returned.
func (b *RedisEventBus) PublishWithIdempotency(ctx context.Context, event *AcademicEvent) error {
	return b.PublishWithIdempotencyToStream(ctx, b.defaultStream, event)
}

// PublishWithIdempotencyToStream publishes an event to a specific stream with idempotency deduplication.
func (b *RedisEventBus) PublishWithIdempotencyToStream(ctx context.Context, stream string, event *AcademicEvent) error {
	if event == nil {
		return ErrInvalidEvent
	}
	if strings.TrimSpace(event.IdempotencyKey) == "" {
		return ErrMissingIdempotencyKey
	}

	idempotencyKey := b.idempotencyPrefix + event.IdempotencyKey

	// SETNX key value EX ttl
	ok, err := b.client.SetNX(ctx, idempotencyKey, event.EventID, b.idempotencyTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to evaluate idempotency key: %w", err)
	}
	if !ok {
		return ErrDuplicateEvent
	}

	// Attempt publish
	if err := b.PublishToStream(ctx, stream, event); err != nil {
		// Roll back idempotency key on publish failure so client may retry safely
		_ = b.client.Del(ctx, idempotencyKey).Err()
		return fmt.Errorf("failed to publish idempotent event: %w", err)
	}

	return nil
}

// Subscribe listens for events on a Redis Stream using consumer groups.
// It handles automatic retries and dead-letter queue routing if maxRetries is exceeded.
func (b *RedisEventBus) Subscribe(
	ctx context.Context,
	stream string,
	group string,
	consumer string,
	handler func(event *AcademicEvent) error,
) error {
	if stream == "" {
		stream = b.defaultStream
	}
	if strings.TrimSpace(group) == "" {
		return errors.New("group name cannot be empty")
	}
	if strings.TrimSpace(consumer) == "" {
		consumer = uuid.New().String()
	}
	if handler == nil {
		return errors.New("event handler cannot be nil")
	}

	// 1. Create consumer group if not already created
	err := b.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !isConsumerGroupAlreadyExists(err) {
		return fmt.Errorf("failed to create consumer group %s on stream %s: %w", group, stream, err)
	}

	currentID := "0" // Drain pending messages first

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		blockTime := b.blockTime
		if currentID == "0" {
			blockTime = 0
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, currentID},
			Count:    b.readCount,
			Block:    blockTime,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				if currentID == "0" {
					currentID = ">"
				}
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Transient error backoff
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
			continue
		}

		hasMessages := false
		for _, s := range streams {
			if len(s.Messages) > 0 {
				hasMessages = true
			}
			for _, msg := range s.Messages {
				b.processMessage(ctx, stream, group, consumer, msg, handler)
			}
		}

		if currentID == "0" && !hasMessages {
			currentID = ">"
		}
	}
}

// processMessage handles execution, retries, acknowledgment, and DLQ routing for a single stream message.
func (b *RedisEventBus) processMessage(
	ctx context.Context,
	stream string,
	group string,
	consumer string,
	msg redis.XMessage,
	handler func(event *AcademicEvent) error,
) {
	retryKey := fmt.Sprintf("events:retries:%s:%s", group, msg.ID)

	event, parseErr := EventFromMap(msg.Values)
	if parseErr != nil {
		b.logger.Warn("routing malformed stream message directly to DLQ",
			zap.String("stream", stream),
			zap.String("msg_id", msg.ID),
			zap.Error(parseErr),
		)
		_ = b.routeToDLQ(ctx, stream, group, consumer, msg.ID, nil, msg.Values, parseErr, 0)
		_ = b.client.XAck(ctx, stream, group, msg.ID).Err()
		return
	}

	for {
		handlerErr := handler(event)
		if handlerErr == nil {
			// Successful execution -> acknowledge and clear retry tracker
			_ = b.client.XAck(ctx, stream, group, msg.ID).Err()
			_ = b.client.Del(ctx, retryKey).Err()
			return
		}

		// Handler failed -> record attempt
		attempts, _ := b.client.Incr(ctx, retryKey).Result()
		_ = b.client.Expire(ctx, retryKey, 24*time.Hour).Err()

		if int(attempts) >= b.maxRetries {
			b.logger.Error("event exceeded max retry attempts, routing to DLQ",
				zap.String("stream", stream),
				zap.String("group", group),
				zap.String("msg_id", msg.ID),
				zap.Int("attempts", int(attempts)),
				zap.Error(handlerErr),
			)

			_ = b.routeToDLQ(ctx, stream, group, consumer, msg.ID, event, msg.Values, handlerErr, int(attempts))
			_ = b.client.XAck(ctx, stream, group, msg.ID).Err()
			_ = b.client.Del(ctx, retryKey).Err()
			return
		}

		if ctx.Err() != nil {
			return
		}

		if b.retryBackoff > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(b.retryBackoff):
			}
		}
	}
}

// routeToDLQ publishes a failed message to the Dead-Letter Queue stream.
func (b *RedisEventBus) routeToDLQ(
	ctx context.Context,
	stream string,
	group string,
	consumer string,
	msgID string,
	event *AcademicEvent,
	rawValues map[string]interface{},
	handlerErr error,
	retries int,
) error {
	dlqEvent := DLQEvent{
		OriginalStream: stream,
		ConsumerGroup:  group,
		ConsumerName:   consumer,
		OriginalMsgID:  msgID,
		RetryCount:     retries,
		ErrorMessage:   handlerErr.Error(),
		FailedAt:       time.Now().UTC(),
		OriginalEvent:  event,
		RawValues:      rawValues,
	}

	if event != nil {
		dlqEvent.EventID = event.EventID
		dlqEvent.EventType = event.EventType
	} else if id, ok := rawValues["event_id"].(string); ok {
		dlqEvent.EventID = id
	}

	rawJSON, err := json.Marshal(dlqEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ event: %w", err)
	}

	dlqValues := map[string]interface{}{
		"event_id":            dlqEvent.EventID,
		"event_type":          dlqEvent.EventType,
		"original_stream":     stream,
		"consumer_group":      group,
		"consumer_name":       consumer,
		"original_message_id": msgID,
		"retry_count":         retries,
		"error":               handlerErr.Error(),
		"failed_at":           dlqEvent.FailedAt.Format(time.RFC3339Nano),
		"raw_json":            string(rawJSON),
	}

	if event != nil {
		if event.InstitutionID != "" {
			dlqValues["institution_id"] = event.InstitutionID
		}
		if event.UserID != "" {
			dlqValues["user_id"] = event.UserID
		}
		if event.IdempotencyKey != "" {
			dlqValues["idempotency_key"] = event.IdempotencyKey
		}
	}

	_, err = b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.dlqStream,
		Values: dlqValues,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to write message to DLQ stream %s: %w", b.dlqStream, err)
	}

	return nil
}

// isConsumerGroupAlreadyExists checks for the standard Redis BUSYGROUP error string.
func isConsumerGroupAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "BUSYGROUP")
}
