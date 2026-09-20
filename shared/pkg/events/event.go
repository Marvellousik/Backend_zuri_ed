// Package events defines structured domain events and an event bus interface for Zuri.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Standard Academic Event Types.
const (
	EventTypeClassApproaching    = "CLASS_APPROACHING"
	EventTypeLectureProcessed    = "LECTURE_PROCESSED"
	EventTypeMaterialUploaded    = "MATERIAL_UPLOADED"
	EventTypeAssessmentCompleted = "ASSESSMENT_COMPLETED"
)

// Common error definitions for event processing.
var (
	ErrInvalidEvent          = errors.New("event cannot be nil")
	ErrMissingEventType      = errors.New("event_type is required")
	ErrDuplicateEvent        = errors.New("duplicate event: idempotency key already exists")
	ErrMissingIdempotencyKey = errors.New("idempotency_key is required for idempotent publish")
	ErrNilStreamValues       = errors.New("stream values cannot be nil")
)

// AcademicEvent represents a strongly-typed domain event for academic workflows.
type AcademicEvent struct {
	EventID        string                 `json:"event_id"`
	EventType      string                 `json:"event_type"`
	InstitutionID  string                 `json:"institution_id"`
	UserID         string                 `json:"user_id"`
	Payload        map[string]interface{} `json:"payload"`
	Timestamp      time.Time              `json:"timestamp"`
	IdempotencyKey string                 `json:"idempotency_key"`
}

// NewAcademicEvent constructs a new AcademicEvent with default UUID and timestamp if omitted.
func NewAcademicEvent(
	eventType string,
	institutionID string,
	userID string,
	idempotencyKey string,
	payload map[string]interface{},
) *AcademicEvent {
	if payload == nil {
		payload = make(map[string]interface{})
	}

	return &AcademicEvent{
		EventID:        uuid.New().String(),
		EventType:      eventType,
		InstitutionID:  institutionID,
		UserID:         userID,
		Payload:        payload,
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	}
}

// Validate ensures the event has all required fields populated.
func (e *AcademicEvent) Validate() error {
	if e == nil {
		return ErrInvalidEvent
	}
	if strings.TrimSpace(e.EventType) == "" {
		return ErrMissingEventType
	}
	if e.EventID == "" {
		e.EventID = uuid.New().String()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Payload == nil {
		e.Payload = make(map[string]interface{})
	}
	return nil
}

// ToMap serializes the AcademicEvent into key-value pairs suitable for Redis stream (XAdd).
func (e *AcademicEvent) ToMap() (map[string]interface{}, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	rawBytes, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw event: %w", err)
	}

	return map[string]interface{}{
		"event_id":        e.EventID,
		"event_type":      e.EventType,
		"institution_id":  e.InstitutionID,
		"user_id":         e.UserID,
		"payload":         string(payloadBytes),
		"timestamp":       e.Timestamp.Format(time.RFC3339Nano),
		"idempotency_key": e.IdempotencyKey,
		"raw_json":        string(rawBytes),
	}, nil
}

// EventFromMap deserializes an AcademicEvent from Redis stream message values.
func EventFromMap(values map[string]interface{}) (*AcademicEvent, error) {
	if values == nil {
		return nil, ErrNilStreamValues
	}

	// 1. Fast path: unmarshal from raw_json if present
	if raw, ok := values["raw_json"]; ok {
		var rawStr string
		switch v := raw.(type) {
		case string:
			rawStr = v
		case []byte:
			rawStr = string(v)
		}
		if rawStr != "" {
			var ev AcademicEvent
			if err := json.Unmarshal([]byte(rawStr), &ev); err == nil {
				if ev.Payload == nil {
					ev.Payload = make(map[string]interface{})
				}
				return &ev, nil
			}
		}
	}

	// 2. Fallback: unmarshal field by field
	ev := &AcademicEvent{
		Payload: make(map[string]interface{}),
	}

	if v, ok := values["event_id"].(string); ok {
		ev.EventID = v
	}
	if v, ok := values["event_type"].(string); ok {
		ev.EventType = v
	}
	if v, ok := values["institution_id"].(string); ok {
		ev.InstitutionID = v
	}
	if v, ok := values["user_id"].(string); ok {
		ev.UserID = v
	}
	if v, ok := values["idempotency_key"].(string); ok {
		ev.IdempotencyKey = v
	}

	if tsVal, ok := values["timestamp"]; ok {
		if tsStr, ok := tsVal.(string); ok && tsStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				ev.Timestamp = t
			} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				ev.Timestamp = t
			}
		}
	}

	if pVal, ok := values["payload"]; ok {
		var payloadBytes []byte
		switch p := pVal.(type) {
		case string:
			payloadBytes = []byte(p)
		case []byte:
			payloadBytes = p
		}
		if len(payloadBytes) > 0 {
			var pMap map[string]interface{}
			if err := json.Unmarshal(payloadBytes, &pMap); err == nil {
				ev.Payload = pMap
			}
		}
	}

	if err := ev.Validate(); err != nil {
		return nil, err
	}

	return ev, nil
}
