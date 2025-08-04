package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/interfaces"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// RedisStorage implements EventStorage using Redis Streams
type RedisStorage struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisStorage creates a new Redis storage implementation
func NewRedisStorage(client *redis.Client, logger *zap.Logger) interfaces.EventStorage {
	return &RedisStorage{
		client: client,
		logger: logger,
	}
}

// Store persists an event in Redis Streams
func (r *RedisStorage) Store(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Generate event ID if not provided
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	// Set timestamp if not provided
	if event.Time.IsZero() {
		event.Time = time.Now()
	}

	// Serialize event data
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create stream key based on workflow ID
	streamKey := fmt.Sprintf("events:workflow:%s", event.WorkflowID)

	// Store in Redis Stream
	args := &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event_id":     event.ID,
			"event_type":   event.Type,
			"source":       event.Source,
			"workflow_id":  event.WorkflowID,
			"data":         string(eventData),
			"timestamp":    event.Time.Unix(),
			"spec_version": event.SpecVersion,
		},
	}

	messageID, err := r.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to store event in Redis: %w", err)
	}

	// Also store in global event stream for cross-workflow queries
	globalArgs := &redis.XAddArgs{
		Stream: "events:global",
		Values: map[string]interface{}{
			"event_id":     event.ID,
			"event_type":   event.Type,
			"source":       event.Source,
			"workflow_id":  event.WorkflowID,
			"data":         string(eventData),
			"timestamp":    event.Time.Unix(),
			"spec_version": event.SpecVersion,
			"stream_id":    messageID,
		},
	}

	_, err = r.client.XAdd(ctx, globalArgs).Result()
	if err != nil {
		r.logger.Error("Failed to store event in global stream", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	r.logger.Debug("Event stored successfully",
		zap.String("event_id", event.ID),
		zap.String("workflow_id", event.WorkflowID),
		zap.String("type", event.Type),
		zap.String("message_id", messageID))

	return nil
}

// GetHistory retrieves all events for a workflow
func (r *RedisStorage) GetHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	streamKey := fmt.Sprintf("events:workflow:%s", workflowID)

	// Read all events from the stream
	result, err := r.client.XRange(ctx, streamKey, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events from Redis: %w", err)
	}

	events := make([]*cloudevents.CloudEvent, 0, len(result))
	for _, message := range result {
		event, err := r.parseEventFromMessage(message)
		if err != nil {
			r.logger.Error("Failed to parse event from Redis message",
				zap.Error(err),
				zap.String("message_id", message.ID))
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// GetSince retrieves events for a workflow since a specific version/timestamp
func (r *RedisStorage) GetSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error) {
	streamKey := fmt.Sprintf("events:workflow:%s", workflowID)

	// Read events from the stream since the specified ID
	result, err := r.client.XRange(ctx, streamKey, since, "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events since %s: %w", since, err)
	}

	events := make([]*cloudevents.CloudEvent, 0, len(result))
	for _, message := range result {
		event, err := r.parseEventFromMessage(message)
		if err != nil {
			r.logger.Error("Failed to parse event from Redis message",
				zap.Error(err),
				zap.String("message_id", message.ID))
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// GetStream retrieves events from a specific stream
func (r *RedisStorage) GetStream(ctx context.Context, streamID string, count int64) ([]*cloudevents.CloudEvent, error) {
	// Read events from the global stream
	result, err := r.client.XRevRange(ctx, "events:global", "+", "-").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events from global stream: %w", err)
	}

	// Limit results if count is specified
	if count > 0 && int64(len(result)) > count {
		result = result[:count]
	}

	events := make([]*cloudevents.CloudEvent, 0, len(result))
	for _, message := range result {
		event, err := r.parseEventFromMessage(message)
		if err != nil {
			r.logger.Error("Failed to parse event from Redis message",
				zap.Error(err),
				zap.String("message_id", message.ID))
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// Delete removes all events for a workflow
func (r *RedisStorage) Delete(ctx context.Context, workflowID string) error {
	streamKey := fmt.Sprintf("events:workflow:%s", workflowID)

	// Delete the workflow stream
	err := r.client.Del(ctx, streamKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete workflow events: %w", err)
	}

	r.logger.Info("Deleted workflow events", zap.String("workflow_id", workflowID))
	return nil
}

// GetStats returns statistics about the event store
func (r *RedisStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get global stream length
	globalLen, err := r.client.XLen(ctx, "events:global").Result()
	if err != nil {
		r.logger.Error("Failed to get global stream length", zap.Error(err))
	} else {
		stats["total_events"] = globalLen
	}

	// Get workflow stream count
	keys, err := r.client.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		r.logger.Error("Failed to get workflow stream keys", zap.Error(err))
	} else {
		stats["workflow_count"] = len(keys)
	}

	// Get Redis info
	info, err := r.client.Info(ctx, "memory").Result()
	if err != nil {
		r.logger.Error("Failed to get Redis info", zap.Error(err))
	} else {
		stats["redis_memory_info"] = info
	}

	return stats, nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// parseEventFromMessage converts a Redis stream message to a CloudEvent
func (r *RedisStorage) parseEventFromMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	values := message.Values

	// Extract event data
	eventDataStr, ok := values["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid event data")
	}

	var event cloudevents.CloudEvent
	if err := json.Unmarshal([]byte(eventDataStr), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Set Redis message ID for tracking
	event.Extensions = map[string]interface{}{
		"redis_message_id": message.ID,
	}

	return &event, nil
}
