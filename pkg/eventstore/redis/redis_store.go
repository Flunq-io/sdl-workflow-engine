package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flunq.io/pkg/eventstore"
	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/go-redis/redis/v8"
)

// RedisEventStore implements EventStore using Redis Streams
type RedisEventStore struct {
	client *redis.Client
	logger eventstore.Logger
}

// NewRedisEventStore creates a new Redis-based event store
func NewRedisEventStore(redisURL string, logger eventstore.Logger) (eventstore.EventStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisEventStore{
		client: client,
		logger: logger,
	}, nil
}

// Publish publishes an event to a Redis stream
func (r *RedisEventStore) Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error {
	// Convert CloudEvent to Redis stream fields
	fields := map[string]interface{}{
		"id":          event.ID,
		"source":      event.Source,
		"specversion": event.SpecVersion,
		"type":        event.Type,
		"time":        event.Time.Format(time.RFC3339),
	}

	// Add workflow and execution IDs if present
	if workflowID := event.Extensions["workflowid"]; workflowID != nil {
		fields["workflowid"] = workflowID
	}
	if executionID := event.Extensions["executionid"]; executionID != nil {
		fields["executionid"] = executionID
	}
	if taskID := event.Extensions["taskid"]; taskID != nil {
		fields["taskid"] = taskID
	}

	// Serialize event data
	if event.Data != nil {
		dataBytes, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		fields["data"] = string(dataBytes)
	}

	// Publish to Redis stream
	result := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: fields,
	})

	if err := result.Err(); err != nil {
		return fmt.Errorf("failed to publish event to Redis stream %s: %w", stream, err)
	}

	r.logger.Debug("Published event to Redis stream",
		"stream", stream,
		"event_id", event.ID,
		"message_id", result.Val())

	return nil
}

// Subscribe subscribes to events from Redis streams
func (r *RedisEventStore) Subscribe(ctx context.Context, config eventstore.SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error) {
	eventCh := make(chan *cloudevents.CloudEvent, 100)
	errorCh := make(chan error, 10)

	// Create consumer group if it doesn't exist
	for _, stream := range config.Streams {
		err := r.client.XGroupCreateMkStream(ctx, stream, config.ConsumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return nil, nil, fmt.Errorf("failed to create consumer group %s for stream %s: %w", config.ConsumerGroup, stream, err)
		}
	}

	go r.subscriptionLoop(ctx, config, eventCh, errorCh)

	return eventCh, errorCh, nil
}

// subscriptionLoop runs the subscription loop
func (r *RedisEventStore) subscriptionLoop(ctx context.Context, config eventstore.SubscriptionConfig, eventCh chan<- *cloudevents.CloudEvent, errorCh chan<- error) {
	defer close(eventCh)
	defer close(errorCh)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read from streams
			streams := make([]string, 0, len(config.Streams)*2)
			for _, stream := range config.Streams {
				streams = append(streams, stream, config.FromMessageID)
			}

			result, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    config.ConsumerGroup,
				Consumer: config.ConsumerName,
				Streams:  streams,
				Count:    config.Count,
				Block:    config.BlockTime,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No new messages
				}
				select {
				case errorCh <- fmt.Errorf("failed to read from Redis streams: %w", err):
				case <-ctx.Done():
					return
				}
				continue
			}

			// Process messages
			for _, stream := range result {
				for _, message := range stream.Messages {
					event, err := r.parseMessage(message)
					if err != nil {
						select {
						case errorCh <- fmt.Errorf("failed to parse message %s: %w", message.ID, err):
						case <-ctx.Done():
							return
						}
						continue
					}

					// Apply filters
					if r.shouldProcessEvent(event, config) {
						select {
						case eventCh <- event:
							// Acknowledge message
							r.client.XAck(ctx, stream.Stream, config.ConsumerGroup, message.ID)
						case <-ctx.Done():
							return
						}
					} else {
						// Acknowledge filtered messages
						r.client.XAck(ctx, stream.Stream, config.ConsumerGroup, message.ID)
					}
				}
			}
		}
	}
}

// ReadHistory reads event history from a Redis stream
func (r *RedisEventStore) ReadHistory(ctx context.Context, stream string, fromID string) ([]*cloudevents.CloudEvent, error) {
	result, err := r.client.XRange(ctx, stream, fromID, "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read history from Redis stream %s: %w", stream, err)
	}

	events := make([]*cloudevents.CloudEvent, 0, len(result))
	for _, message := range result {
		event, err := r.parseMessage(message)
		if err != nil {
			r.logger.Error("Failed to parse message in history", "message_id", message.ID, "error", err)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// CreateConsumerGroup creates a consumer group
func (r *RedisEventStore) CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error {
	for _, stream := range streams {
		err := r.client.XGroupCreateMkStream(ctx, stream, groupName, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("failed to create consumer group %s for stream %s: %w", groupName, stream, err)
		}
	}
	return nil
}

// CreateCheckpoint saves a checkpoint
func (r *RedisEventStore) CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error {
	key := fmt.Sprintf("checkpoint:%s:%s", groupName, stream)
	return r.client.Set(ctx, key, messageID, 0).Err()
}

// GetLastCheckpoint gets the last checkpoint
func (r *RedisEventStore) GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error) {
	key := fmt.Sprintf("checkpoint:%s:%s", groupName, stream)
	result := r.client.Get(ctx, key)
	if result.Err() == redis.Nil {
		return "0", nil // Start from beginning if no checkpoint
	}
	return result.Result()
}

// Close closes the Redis connection
func (r *RedisEventStore) Close() error {
	return r.client.Close()
}

// parseMessage converts a Redis stream message to a CloudEvent
func (r *RedisEventStore) parseMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	event := &cloudevents.CloudEvent{
		Extensions: make(map[string]interface{}),
	}

	// Parse required fields
	if id, ok := message.Values["id"].(string); ok {
		event.ID = id
	}
	if source, ok := message.Values["source"].(string); ok {
		event.Source = source
	}
	if specVersion, ok := message.Values["specversion"].(string); ok {
		event.SpecVersion = specVersion
	}
	if eventType, ok := message.Values["type"].(string); ok {
		event.Type = eventType
	}
	if timeStr, ok := message.Values["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = t
		}
	}

	// Parse extensions
	if workflowID, ok := message.Values["workflowid"].(string); ok {
		event.Extensions["workflowid"] = workflowID
	}
	if executionID, ok := message.Values["executionid"].(string); ok {
		event.Extensions["executionid"] = executionID
	}
	if taskID, ok := message.Values["taskid"].(string); ok {
		event.Extensions["taskid"] = taskID
	}

	// Parse data
	if dataStr, ok := message.Values["data"].(string); ok {
		var data interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
			event.Data = data
		}
	}

	return event, nil
}

// shouldProcessEvent checks if an event should be processed based on filters
func (r *RedisEventStore) shouldProcessEvent(event *cloudevents.CloudEvent, config eventstore.SubscriptionConfig) bool {
	// Filter by event type
	if len(config.EventTypes) > 0 {
		found := false
		for _, eventType := range config.EventTypes {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by workflow ID
	if len(config.WorkflowIDs) > 0 {
		workflowID, ok := event.Extensions["workflowid"].(string)
		if !ok {
			return false
		}
		found := false
		for _, wfID := range config.WorkflowIDs {
			if workflowID == wfID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
