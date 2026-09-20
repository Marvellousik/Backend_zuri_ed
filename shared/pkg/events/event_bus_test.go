package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuri/shared/pkg/events"
)

// --- FakeRedisClient: In-memory simulation of Redis operations for tests ---

type fakeEntry struct {
	val       string
	expiresAt time.Time
}

type FakeRedisClient struct {
	mu          sync.Mutex
	data        map[string]fakeEntry
	streams     map[string][]redis.XMessage
	groups      map[string]map[string]bool
	readOffsets map[string]map[string]int // stream -> group -> offset
	acks        map[string]map[string][]string
	msgSeq      int64
	failXAdd    bool
}

func NewFakeRedisClient() *FakeRedisClient {
	return &FakeRedisClient{
		data:        make(map[string]fakeEntry),
		streams:     make(map[string][]redis.XMessage),
		groups:      make(map[string]map[string]bool),
		readOffsets: make(map[string]map[string]int),
		acks:        make(map[string]map[string][]string),
	}
}

func (f *FakeRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	if entry, exists := f.data[key]; exists {
		if entry.expiresAt.IsZero() || entry.expiresAt.After(now) {
			return redis.NewBoolResult(false, nil)
		}
	}

	var exp time.Time
	if expiration > 0 {
		exp = now.Add(expiration)
	}
	f.data[key] = fakeEntry{val: fmt.Sprint(value), expiresAt: exp}
	return redis.NewBoolResult(true, nil)
}

func (f *FakeRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	}
	f.data[key] = fakeEntry{val: fmt.Sprint(value), expiresAt: exp}
	return redis.NewStatusResult("OK", nil)
}

func (f *FakeRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if entry, exists := f.data[key]; exists {
		if entry.expiresAt.IsZero() || entry.expiresAt.After(time.Now()) {
			return redis.NewStringResult(entry.val, nil)
		}
		delete(f.data, key)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (f *FakeRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	var deleted int64
	for _, k := range keys {
		if _, exists := f.data[k]; exists {
			delete(f.data, k)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

func (f *FakeRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	var cur int64
	if entry, exists := f.data[key]; exists {
		_, _ = fmt.Sscanf(entry.val, "%d", &cur)
	}
	cur++
	f.data[key] = fakeEntry{val: fmt.Sprintf("%d", cur)}
	return redis.NewIntResult(cur, nil)
}

func (f *FakeRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if entry, exists := f.data[key]; exists {
		entry.expiresAt = time.Now().Add(expiration)
		f.data[key] = entry
		return redis.NewBoolResult(true, nil)
	}
	return redis.NewBoolResult(false, nil)
}

func (f *FakeRedisClient) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.failXAdd {
		return redis.NewStringResult("", errors.New("redis connection refused"))
	}

	seq := atomic.AddInt64(&f.msgSeq, 1)
	msgID := fmt.Sprintf("%d-0", seq)
	var values map[string]interface{}
	switch v := a.Values.(type) {
	case map[string]interface{}:
		values = v
	default:
		values = make(map[string]interface{})
	}

	msg := redis.XMessage{
		ID:     msgID,
		Values: values,
	}

	f.streams[a.Stream] = append(f.streams[a.Stream], msg)
	return redis.NewStringResult(msgID, nil)
}

func (f *FakeRedisClient) XGroupCreate(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	return f.XGroupCreateMkStream(ctx, stream, group, start)
}

func (f *FakeRedisClient) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.groups[stream]; !ok {
		f.groups[stream] = make(map[string]bool)
	}
	if f.groups[stream][group] {
		return redis.NewStatusResult("", errors.New("BUSYGROUP Consumer Group name already exists"))
	}
	f.groups[stream][group] = true
	return redis.NewStatusResult("OK", nil)
}

func (f *FakeRedisClient) XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(a.Streams) < 2 {
		return redis.NewXStreamSliceCmdResult(nil, errors.New("invalid streams arguments"))
	}
	stream := a.Streams[0]
	id := a.Streams[1]

	allMsgs := f.streams[stream]

	if _, ok := f.readOffsets[stream]; !ok {
		f.readOffsets[stream] = make(map[string]int)
	}

	if id == "0" {
		// No pending messages simulation in this test
		return redis.NewXStreamSliceCmdResult(nil, redis.Nil)
	}

	// Read new messages (">")
	offset := f.readOffsets[stream][a.Group]
	if offset >= len(allMsgs) {
		return redis.NewXStreamSliceCmdResult(nil, redis.Nil)
	}

	var batch []redis.XMessage
	count := int(a.Count)
	if count <= 0 {
		count = 10
	}

	for i := offset; i < len(allMsgs) && len(batch) < count; i++ {
		batch = append(batch, allMsgs[i])
	}
	f.readOffsets[stream][a.Group] = offset + len(batch)

	res := []redis.XStream{
		{
			Stream:   stream,
			Messages: batch,
		},
	}
	return redis.NewXStreamSliceCmdResult(res, nil)
}

func (f *FakeRedisClient) XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.acks[stream]; !ok {
		f.acks[stream] = make(map[string][]string)
	}
	f.acks[stream][group] = append(f.acks[stream][group], ids...)
	return redis.NewIntResult(int64(len(ids)), nil)
}

func (f *FakeRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (f *FakeRedisClient) GetStreamMessages(stream string) []redis.XMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]redis.XMessage, len(f.streams[stream]))
	copy(copied, f.streams[stream])
	return copied
}

func (f *FakeRedisClient) GetAcks(stream, group string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acks[stream][group]
}

// --- UNIT TESTS ---

func TestAcademicEvent_MarshalingAndValidation(t *testing.T) {
	t.Run("creates valid academic event with all required fields", func(t *testing.T) {
		event := events.NewAcademicEvent(
			events.EventTypeClassApproaching,
			"inst_veritas_01",
			"usr_student_42",
			"idem_key_001",
			map[string]interface{}{
				"course_code": "CSC301",
				"course_name": "Data Structures & Algorithms",
				"room":        "Lab 3",
				"starts_in_m": 15,
			},
		)

		require.NotNil(t, event)
		assert.NotEmpty(t, event.EventID)
		assert.Equal(t, events.EventTypeClassApproaching, event.EventType)
		assert.Equal(t, "inst_veritas_01", event.InstitutionID)
		assert.Equal(t, "usr_student_42", event.UserID)
		assert.Equal(t, "idem_key_001", event.IdempotencyKey)
		assert.False(t, event.Timestamp.IsZero())
		assert.Equal(t, "CSC301", event.Payload["course_code"])

		err := event.Validate()
		assert.NoError(t, err)
	})

	t.Run("all standard event types validate successfully", func(t *testing.T) {
		types := []string{
			events.EventTypeClassApproaching,
			events.EventTypeLectureProcessed,
			events.EventTypeMaterialUploaded,
			events.EventTypeAssessmentCompleted,
		}

		for _, et := range types {
			ev := events.NewAcademicEvent(et, "inst_1", "usr_1", "idem_1", nil)
			assert.NoError(t, ev.Validate())
			assert.Equal(t, et, ev.EventType)
		}
	})

	t.Run("validation fails on empty event type", func(t *testing.T) {
		ev := &events.AcademicEvent{
			InstitutionID: "inst_1",
			UserID:        "usr_1",
		}
		err := ev.Validate()
		assert.ErrorIs(t, err, events.ErrMissingEventType)
	})

	t.Run("validation fails on nil event", func(t *testing.T) {
		var ev *events.AcademicEvent
		err := ev.Validate()
		assert.ErrorIs(t, err, events.ErrInvalidEvent)
	})

	t.Run("ToMap and EventFromMap full roundtrip", func(t *testing.T) {
		original := events.NewAcademicEvent(
			events.EventTypeLectureProcessed,
			"inst_99",
			"usr_lecturer_12",
			"idem_lecture_888",
			map[string]interface{}{
				"lecture_id":  "lec_555",
				"chunks":      float64(42),
				"duration_s":  float64(3600),
				"transcribed": true,
			},
		)

		values, err := original.ToMap()
		require.NoError(t, err)
		require.NotNil(t, values)

		assert.Equal(t, original.EventID, values["event_id"])
		assert.Equal(t, original.EventType, values["event_type"])
		assert.Equal(t, original.InstitutionID, values["institution_id"])
		assert.Equal(t, original.UserID, values["user_id"])
		assert.Equal(t, original.IdempotencyKey, values["idempotency_key"])
		assert.NotEmpty(t, values["payload"])
		assert.NotEmpty(t, values["raw_json"])

		// Reconstruct from map
		reconstructed, err := events.EventFromMap(values)
		require.NoError(t, err)
		require.NotNil(t, reconstructed)

		assert.Equal(t, original.EventID, reconstructed.EventID)
		assert.Equal(t, original.EventType, reconstructed.EventType)
		assert.Equal(t, original.InstitutionID, reconstructed.InstitutionID)
		assert.Equal(t, original.UserID, reconstructed.UserID)
		assert.Equal(t, original.IdempotencyKey, reconstructed.IdempotencyKey)
		assert.Equal(t, original.Timestamp.Unix(), reconstructed.Timestamp.Unix())
		assert.Equal(t, "lec_555", reconstructed.Payload["lecture_id"])
		assert.Equal(t, float64(42), reconstructed.Payload["chunks"])
		assert.Equal(t, true, reconstructed.Payload["transcribed"])
	})

	t.Run("EventFromMap fallback when raw_json is omitted", func(t *testing.T) {
		ts := time.Now().UTC().Truncate(time.Millisecond)
		values := map[string]interface{}{
			"event_id":        "evt-manual-1",
			"event_type":      events.EventTypeMaterialUploaded,
			"institution_id":  "inst_delta",
			"user_id":         "usr_teacher",
			"payload":         `{"filename":"syllabus.pdf","size_bytes":1048576}`,
			"timestamp":       ts.Format(time.RFC3339Nano),
			"idempotency_key": "idem_manual_1",
		}

		ev, err := events.EventFromMap(values)
		require.NoError(t, err)
		assert.Equal(t, "evt-manual-1", ev.EventID)
		assert.Equal(t, events.EventTypeMaterialUploaded, ev.EventType)
		assert.Equal(t, "inst_delta", ev.InstitutionID)
		assert.Equal(t, "syllabus.pdf", ev.Payload["filename"])
		assert.Equal(t, float64(1048576), ev.Payload["size_bytes"])
	})

	t.Run("JSON standard marshaling roundtrip", func(t *testing.T) {
		original := events.NewAcademicEvent(
			events.EventTypeAssessmentCompleted,
			"inst_1",
			"usr_student",
			"idem_quiz_1",
			map[string]interface{}{"score": float64(95), "passed": true},
		)

		b, err := json.Marshal(original)
		require.NoError(t, err)

		var parsed events.AcademicEvent
		err = json.Unmarshal(b, &parsed)
		require.NoError(t, err)

		assert.Equal(t, original.EventID, parsed.EventID)
		assert.Equal(t, original.EventType, parsed.EventType)
		assert.Equal(t, float64(95), parsed.Payload["score"])
		assert.Equal(t, true, parsed.Payload["passed"])
	})
}

func TestRedisEventBus_Publish(t *testing.T) {
	client := NewFakeRedisClient()
	bus := events.NewRedisEventBus(client)

	ctx := context.Background()

	t.Run("publishes event to default stream", func(t *testing.T) {
		event := events.NewAcademicEvent(
			events.EventTypeMaterialUploaded,
			"inst_veritas",
			"usr_prof",
			"idem_mat_1",
			map[string]interface{}{"title": "Calculus Notes"},
		)

		err := bus.Publish(ctx, event)
		require.NoError(t, err)

		msgs := client.GetStreamMessages(events.DefaultStream)
		require.Len(t, msgs, 1)

		assert.Equal(t, event.EventID, msgs[0].Values["event_id"])
		assert.Equal(t, events.EventTypeMaterialUploaded, msgs[0].Values["event_type"])
		assert.Equal(t, "inst_veritas", msgs[0].Values["institution_id"])
	})

	t.Run("publishes event to custom stream", func(t *testing.T) {
		event := events.NewAcademicEvent(
			events.EventTypeAssessmentCompleted,
			"inst_veritas",
			"usr_student",
			"idem_ass_1",
			map[string]interface{}{"score": float64(100)},
		)

		customStream := "zuri:events:assessments"
		err := bus.PublishToStream(ctx, customStream, event)
		require.NoError(t, err)

		msgs := client.GetStreamMessages(customStream)
		require.Len(t, msgs, 1)
		assert.Equal(t, event.EventID, msgs[0].Values["event_id"])
	})

	t.Run("rejects nil event", func(t *testing.T) {
		err := bus.Publish(ctx, nil)
		assert.ErrorIs(t, err, events.ErrInvalidEvent)
	})

	t.Run("rejects event missing event_type", func(t *testing.T) {
		err := bus.Publish(ctx, &events.AcademicEvent{})
		assert.ErrorIs(t, err, events.ErrMissingEventType)
	})
}

func TestRedisEventBus_IdempotencyDeduplication(t *testing.T) {
	client := NewFakeRedisClient()
	bus := events.NewRedisEventBus(
		client,
		events.WithIdempotencyPrefix("events:idempotency:"),
		events.WithIdempotencyTTL(12*time.Hour),
	)

	ctx := context.Background()

	t.Run("first publish succeeds and records idempotency key in Redis", func(t *testing.T) {
		event := events.NewAcademicEvent(
			events.EventTypeClassApproaching,
			"inst_01",
			"usr_01",
			"idem_class_101",
			map[string]interface{}{"starts_in": 10},
		)

		err := bus.PublishWithIdempotency(ctx, event)
		require.NoError(t, err)

		// Verify stream has exactly 1 event
		msgs := client.GetStreamMessages(events.DefaultStream)
		require.Len(t, msgs, 1)

		// Verify idempotency key exists in Redis with expected value
		keyCmd := client.Get(ctx, "events:idempotency:idem_class_101")
		require.NoError(t, keyCmd.Err())
		assert.Equal(t, event.EventID, keyCmd.Val())
	})

	t.Run("second publish with duplicate idempotency key is rejected", func(t *testing.T) {
		duplicateEvent := events.NewAcademicEvent(
			events.EventTypeClassApproaching,
			"inst_01",
			"usr_01",
			"idem_class_101", // duplicate key!
			map[string]interface{}{"starts_in": 10},
		)

		err := bus.PublishWithIdempotency(ctx, duplicateEvent)
		require.Error(t, err)
		assert.ErrorIs(t, err, events.ErrDuplicateEvent, "must return ErrDuplicateEvent on duplicate idempotency key")

		// Verify stream STILL only has 1 message (duplicate was not published!)
		msgs := client.GetStreamMessages(events.DefaultStream)
		assert.Len(t, msgs, 1, "duplicate message must not be published to stream")
	})

	t.Run("fails when idempotency key is missing", func(t *testing.T) {
		eventWithoutIdem := events.NewAcademicEvent(
			events.EventTypeLectureProcessed,
			"inst_01",
			"usr_01",
			"", // empty idempotency key
			map[string]interface{}{},
		)

		err := bus.PublishWithIdempotency(ctx, eventWithoutIdem)
		assert.ErrorIs(t, err, events.ErrMissingIdempotencyKey)
	})

	t.Run("rolls back idempotency key if stream publish fails", func(t *testing.T) {
		client.failXAdd = true
		defer func() { client.failXAdd = false }()

		failedEvent := events.NewAcademicEvent(
			events.EventTypeLectureProcessed,
			"inst_01",
			"usr_01",
			"idem_rollback_test",
			nil,
		)

		err := bus.PublishWithIdempotency(ctx, failedEvent)
		require.Error(t, err)

		// Verify idempotency key was rolled back (deleted)
		keyCmd := client.Get(ctx, "events:idempotency:idem_rollback_test")
		assert.ErrorIs(t, keyCmd.Err(), redis.Nil, "idempotency key must be cleaned up on failed publish")
	})
}

func TestRedisEventBus_DLQRouting_OnMaxRetries(t *testing.T) {
	client := NewFakeRedisClient()
	maxRetries := 3
	dlqStream := "zuri:events:dlq"

	bus := events.NewRedisEventBus(
		client,
		events.WithMaxRetries(maxRetries),
		events.WithRetryBackoff(1*time.Millisecond), // fast backoff for tests
		events.WithDLQStream(dlqStream),
	)

	// Publish an event that will fail processing
	failingEvent := events.NewAcademicEvent(
		events.EventTypeAssessmentCompleted,
		"inst_dlq_test",
		"usr_dlq_student",
		"idem_dlq_001",
		map[string]interface{}{"assessment_id": "quiz_404"},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := bus.Publish(ctx, failingEvent)
	require.NoError(t, err)

	var executionAttempts int32
	failingHandler := func(event *events.AcademicEvent) error {
		atomic.AddInt32(&executionAttempts, 1)
		return errors.New("simulated internal service crash")
	}

	stream := events.DefaultStream
	group := "analytics-processor"
	consumer := "worker-1"

	// Run Subscribe in background
	subErrCh := make(chan error, 1)
	go func() {
		subErrCh <- bus.Subscribe(ctx, stream, group, consumer, failingHandler)
	}()

	// Wait for handler to execute maxRetries times and route to DLQ
	assert.Eventually(t, func() bool {
		dlqMsgs := client.GetStreamMessages(dlqStream)
		return len(dlqMsgs) > 0
	}, 2*time.Second, 10*time.Millisecond, "expected failed message to be routed to DLQ")

	// Cancel context to stop subscriber
	cancel()

	// 1. Verify handler was retried exactly maxRetries times
	assert.Equal(t, int32(maxRetries), atomic.LoadInt32(&executionAttempts), "must retry exactly maxRetries times")

	// 2. Verify DLQ message payload and structure
	dlqMsgs := client.GetStreamMessages(dlqStream)
	require.Len(t, dlqMsgs, 1)

	dlqMsg := dlqMsgs[0]
	assert.Equal(t, failingEvent.EventID, dlqMsg.Values["event_id"])
	assert.Equal(t, events.EventTypeAssessmentCompleted, dlqMsg.Values["event_type"])
	assert.Equal(t, stream, dlqMsg.Values["original_stream"])
	assert.Equal(t, group, dlqMsg.Values["consumer_group"])
	assert.Equal(t, consumer, dlqMsg.Values["consumer_name"])
	assert.Equal(t, maxRetries, dlqMsg.Values["retry_count"])
	assert.Equal(t, "simulated internal service crash", dlqMsg.Values["error"])
	assert.NotEmpty(t, dlqMsg.Values["raw_json"])

	// Parse the raw_json of DLQEvent
	var dlqEvent events.DLQEvent
	err = json.Unmarshal([]byte(dlqMsg.Values["raw_json"].(string)), &dlqEvent)
	require.NoError(t, err)
	assert.Equal(t, failingEvent.EventID, dlqEvent.EventID)
	assert.Equal(t, maxRetries, dlqEvent.RetryCount)
	assert.Equal(t, "simulated internal service crash", dlqEvent.ErrorMessage)

	// 3. Verify original message was acknowledged so it does not poison the stream
	acks := client.GetAcks(stream, group)
	assert.Contains(t, acks, "1-0", "original message must be ACKed after routing to DLQ")
}

func TestRedisEventBus_Subscribe_Success(t *testing.T) {
	client := NewFakeRedisClient()
	bus := events.NewRedisEventBus(client)

	event := events.NewAcademicEvent(
		events.EventTypeClassApproaching,
		"inst_success",
		"usr_student",
		"idem_success_1",
		map[string]interface{}{"course": "MAT101"},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	receivedEventCh := make(chan *events.AcademicEvent, 1)
	successHandler := func(ev *events.AcademicEvent) error {
		receivedEventCh <- ev
		return nil
	}

	stream := events.DefaultStream
	group := "notification-group"
	consumer := "notif-worker-1"

	go func() {
		_ = bus.Subscribe(ctx, stream, group, consumer, successHandler)
	}()

	select {
	case received := <-receivedEventCh:
		assert.Equal(t, event.EventID, received.EventID)
		assert.Equal(t, event.EventType, received.EventType)
		assert.Equal(t, "MAT101", received.Payload["course"])
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event in handler")
	}

	// Verify ACK was called
	assert.Eventually(t, func() bool {
		acks := client.GetAcks(stream, group)
		return len(acks) == 1
	}, 1*time.Second, 10*time.Millisecond)

	// Verify nothing was routed to DLQ
	dlqMsgs := client.GetStreamMessages(bus.DLQStream())
	assert.Empty(t, dlqMsgs, "successful message must not be routed to DLQ")
}

func TestRedisEventBus_Subscribe_PoisonMessageRouting(t *testing.T) {
	client := NewFakeRedisClient()
	dlqStream := "zuri:events:dlq"
	bus := events.NewRedisEventBus(
		client,
		events.WithDLQStream(dlqStream),
	)

	// Inject malformed/poison message directly into stream
	client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: events.DefaultStream,
		Values: map[string]interface{}{
			"corrupted_data": "{not-json",
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlerCalled := false
	handler := func(ev *events.AcademicEvent) error {
		handlerCalled = true
		return nil
	}

	group := "poison-group"
	consumer := "poison-worker"

	go func() {
		_ = bus.Subscribe(ctx, events.DefaultStream, group, consumer, handler)
	}()

	// Verify poison message is immediately routed to DLQ and ACKed
	assert.Eventually(t, func() bool {
		dlqMsgs := client.GetStreamMessages(dlqStream)
		return len(dlqMsgs) == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.False(t, handlerCalled, "handler should not be called for unparseable poison message")

	acks := client.GetAcks(events.DefaultStream, group)
	assert.Len(t, acks, 1, "poison message must be ACKed to unblock stream")
}

func TestRedisEventBus_Options(t *testing.T) {
	client := NewFakeRedisClient()
	bus := events.NewRedisEventBus(
		client,
		events.WithDefaultStream("custom:stream"),
		events.WithDLQStream("custom:dlq"),
		events.WithIdempotencyPrefix("custom:idempotency:"),
		events.WithIdempotencyTTL(48*time.Hour),
		events.WithMaxRetries(5),
		events.WithRetryBackoff(200*time.Millisecond),
		events.WithReadCount(25),
		events.WithBlockTime(5*time.Second),
	)

	assert.Equal(t, "custom:stream", bus.DefaultStream())
	assert.Equal(t, "custom:dlq", bus.DLQStream())
	assert.Equal(t, "custom:idempotency:", bus.IdempotencyPrefix())
	assert.Equal(t, 48*time.Hour, bus.IdempotencyTTL())
	assert.Equal(t, 5, bus.MaxRetries())
}
