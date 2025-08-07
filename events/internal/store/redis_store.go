package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// RedisEventStore implements event storage using Redis Streams
type RedisEventStore struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisEventStore creates a new Redis-based event store
func NewRedisEventStore(client *redis.Client, logger *zap.Logger) *RedisEventStore {
	return &RedisEventStore{
		client: client,
		logger: logger,
	}
}

// StoreEvent stores a CloudEvent in Redis Streams
func (r *RedisEventStore) StoreEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
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

	// Determine storage strategy based on event type (execution-only approach with multi-tenancy)
	var streamKey string
	if event.ExecutionID != "" && strings.TrimSpace(event.ExecutionID) != "" {
		// Execution-related events go to tenant-specific execution streams
		if event.TenantID != "" && strings.TrimSpace(event.TenantID) != "" {
			streamKey = fmt.Sprintf("events:execution:%s:%s", event.TenantID, event.ExecutionID)
		} else {
			// Fallback for events without tenant ID (backward compatibility)
			streamKey = fmt.Sprintf("events:execution:%s", event.ExecutionID)
		}
		r.logger.Info("DEBUG: Using execution stream", zap.String("stream_key", streamKey), zap.String("tenant_id", event.TenantID), zap.String("execution_id", event.ExecutionID))
	} else {
		// Workflow-level events (creation, deletion, etc.) should not create streams
		// These are just metadata operations and don't need event streams
		return fmt.Errorf("workflow-level events should not be stored in event streams - execution_id is required")
	}

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
			"data":         string(eventData),
			"timestamp":    event.Time.Unix(),
			"spec_version": event.SpecVersion,
		},
	}

	messageID, err := r.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to store event in Redis: %w", err)
	}

	// No global stream needed - execution-only approach

	r.logger.Debug("Event stored successfully",
		zap.String("event_id", event.ID),
		zap.String("execution_id", event.ExecutionID),
		zap.String("workflow_id", event.WorkflowID),
		zap.String("type", event.Type),
		zap.String("message_id", messageID))

	return nil
}

// GetEventHistory retrieves all events for a workflow (aggregated from all executions)
func (r *RedisEventStore) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	// Get all execution IDs for this workflow
	executionIDs, err := r.GetWorkflowExecutions(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow executions: %w", err)
	}

	var allEvents []*cloudevents.CloudEvent

	// Collect events from all executions
	for _, executionID := range executionIDs {
		executionEvents, err := r.GetExecutionHistory(ctx, executionID)
		if err != nil {
			r.logger.Error("Failed to get execution events",
				zap.String("execution_id", executionID),
				zap.Error(err))
			continue
		}
		allEvents = append(allEvents, executionEvents...)
	}

	// Sort events by timestamp (oldest first)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Time.Before(allEvents[j].Time)
	})

	return allEvents, nil
}

// GetExecutionHistory retrieves all events for a specific execution
func (r *RedisEventStore) GetExecutionHistory(ctx context.Context, executionID string) ([]*cloudevents.CloudEvent, error) {
	streamKey := fmt.Sprintf("events:execution:%s", executionID)

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

// GetWorkflowExecutions retrieves all execution IDs for a workflow
func (r *RedisEventStore) GetWorkflowExecutions(ctx context.Context, workflowID string) ([]string, error) {
	// Search for all execution streams that belong to this workflow
	pattern := "events:execution:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to search execution streams: %w", err)
	}

	var executionIDs []string
	for _, key := range keys {
		// Extract execution ID from key (events:execution:{executionID})
		if len(key) > 17 { // len("events:execution:") = 17
			executionID := key[17:]

			// Check if this execution belongs to the workflow by reading the first event
			firstEvent, err := r.getFirstEventFromStream(ctx, key)
			if err != nil {
				continue // Skip if we can't read the stream
			}

			if firstEvent != nil && firstEvent.WorkflowID == workflowID {
				executionIDs = append(executionIDs, executionID)
			}
		}
	}

	return executionIDs, nil
}

// getFirstEventFromStream gets the first event from a stream (for workflow ID lookup)
func (r *RedisEventStore) getFirstEventFromStream(ctx context.Context, streamKey string) (*cloudevents.CloudEvent, error) {
	result, err := r.client.XRange(ctx, streamKey, "-", "+").Result()
	if err != nil || len(result) == 0 {
		return nil, err
	}

	return r.parseEventFromMessage(result[0])
}

// GetEventsSince retrieves events for a workflow since a specific version/timestamp (from execution streams)
func (r *RedisEventStore) GetEventsSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error) {
	// Get all execution IDs for this workflow
	executionIDs, err := r.GetWorkflowExecutions(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow executions: %w", err)
	}

	var allEvents []*cloudevents.CloudEvent

	// Collect events from all executions since the specified time
	for _, executionID := range executionIDs {
		streamKey := fmt.Sprintf("events:execution:%s", executionID)

		// Read events from the execution stream since the specified ID
		result, err := r.client.XRange(ctx, streamKey, since, "+").Result()
		if err != nil {
			r.logger.Error("Failed to read events from execution stream",
				zap.String("execution_id", executionID),
				zap.Error(err))
			continue
		}

		for _, message := range result {
			event, err := r.parseEventFromMessage(message)
			if err != nil {
				r.logger.Error("Failed to parse event from Redis message",
					zap.Error(err),
					zap.String("message_id", message.ID))
				continue
			}
			allEvents = append(allEvents, event)
		}
	}

	// Sort events by timestamp (oldest first)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Time.Before(allEvents[j].Time)
	})

	return allEvents, nil
}

// GetStreamEvents retrieves events from a specific stream
func (r *RedisEventStore) GetStreamEvents(ctx context.Context, streamID string, count int64) ([]*cloudevents.CloudEvent, error) {
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

// parseEventFromMessage converts a Redis stream message to a CloudEvent
func (r *RedisEventStore) parseEventFromMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	values := message.Values
	event := &cloudevents.CloudEvent{}

	// Extract CloudEvent fields from Redis message values (not from data field)
	if eventID, ok := values["event_id"].(string); ok {
		event.ID = eventID
	}
	if eventType, ok := values["event_type"].(string); ok {
		event.Type = eventType
	}
	if source, ok := values["source"].(string); ok {
		event.Source = source
	}
	if workflowID, ok := values["workflow_id"].(string); ok {
		event.WorkflowID = workflowID
	}
	if executionID, ok := values["execution_id"].(string); ok {
		event.ExecutionID = executionID
	}
	if taskID, ok := values["task_id"].(string); ok {
		event.TaskID = taskID
	}
	if specVersion, ok := values["spec_version"].(string); ok {
		event.SpecVersion = specVersion
	}

	// Parse timestamp
	if timestampStr, ok := values["timestamp"].(string); ok {
		if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			event.Time = timestamp
		}
	}

	// Parse data payload from the data field
	if eventDataStr, ok := values["data"].(string); ok {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(eventDataStr), &data); err == nil {
			event.Data = data
		}
	}

	// Set Redis message ID for tracking
	event.Extensions = map[string]interface{}{
		"redis_message_id": message.ID,
	}

	return event, nil
}

// GetStats returns statistics about the event store
func (r *RedisEventStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
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

// DeleteWorkflowEvents deletes all events for a specific workflow
func (r *RedisEventStore) DeleteWorkflowEvents(ctx context.Context, workflowID string) error {
	streamKey := fmt.Sprintf("events:workflow:%s", workflowID)

	// Delete the workflow stream
	err := r.client.Del(ctx, streamKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete workflow events: %w", err)
	}

	r.logger.Info("Deleted workflow events", zap.String("workflow_id", workflowID))
	return nil
}
