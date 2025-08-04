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

	// Serialize only the event data (not the entire CloudEvent)
	var eventDataStr string
	if event.Data != nil {
		eventDataBytes, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		eventDataStr = string(eventDataBytes)
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
			"execution_id": event.ExecutionID,
			"task_id":      event.TaskID,
			"data":         eventDataStr,
			"timestamp":    event.Time.Format(time.RFC3339),
			"spec_version": event.SpecVersion,
		},
	}

	messageID, err := r.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to store event in Redis: %w", err)
	}

	// Note: Removed global stream storage for cleaner architecture
	// Each workflow has its own isolated event stream

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

// GetStream retrieves events from a specific workflow stream
func (r *RedisStorage) GetStream(ctx context.Context, streamID string, count int64) ([]*cloudevents.CloudEvent, error) {
	// Read events from the specified workflow stream
	streamKey := fmt.Sprintf("events:workflow:%s", streamID)
	result, err := r.client.XRevRange(ctx, streamKey, "+", "-").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events from workflow stream %s: %w", streamKey, err)
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

	// Count total events across all workflow streams
	totalEvents := int64(0)

	// Get workflow stream count and total events
	keys, err := r.client.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		r.logger.Error("Failed to get workflow stream keys", zap.Error(err))
	} else {
		stats["workflow_count"] = len(keys)

		// Count events in each workflow stream
		for _, key := range keys {
			length, err := r.client.XLen(ctx, key).Result()
			if err == nil {
				totalEvents += length
			}
		}
		stats["total_events"] = totalEvents
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
