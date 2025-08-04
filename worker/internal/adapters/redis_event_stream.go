package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/worker/internal/interfaces"
)

// RedisEventStream implements EventStream interface using Redis Streams
type RedisEventStream struct {
	client *redis.Client
	logger interfaces.Logger
}

// NewRedisEventStream creates a new Redis Streams event stream
func NewRedisEventStream(client *redis.Client, logger interfaces.Logger) interfaces.EventStream {
	return &RedisEventStream{
		client: client,
		logger: logger,
	}
}

// Subscribe creates a subscription to Redis Streams
func (r *RedisEventStream) Subscribe(ctx context.Context, filters interfaces.EventStreamFilters) (interfaces.EventStreamSubscription, error) {
	subscription := &RedisEventStreamSubscription{
		client:        r.client,
		logger:        r.logger,
		filters:       filters,
		eventsCh:      make(chan *cloudevents.CloudEvent, 100),
		errorsCh:      make(chan error, 10),
		stopCh:        make(chan struct{}),
		consumerGroup: "worker-service",
		consumerName:  fmt.Sprintf("worker-%d", time.Now().Unix()),
	}

	// Start the subscription goroutine
	go subscription.start(ctx)

	r.logger.Info("Created Redis Streams subscription",
		"event_types", filters.EventTypes,
		"workflow_ids", filters.WorkflowIDs,
		"consumer_group", subscription.consumerGroup)

	return subscription, nil
}

// Publish publishes an event to Redis Streams
func (r *RedisEventStream) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
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
		return fmt.Errorf("failed to publish event to Redis Stream: %w", err)
	}

	// Note: Removed global stream publishing - Worker only publishes to workflow streams

	r.logger.Debug("Published event to Redis Streams",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", event.WorkflowID,
		"message_id", messageID)

	return nil
}

// Close closes the Redis connection
func (r *RedisEventStream) Close() error {
	return r.client.Close()
}

// RedisEventStreamSubscription implements EventStreamSubscription for Redis Streams
type RedisEventStreamSubscription struct {
	client        *redis.Client
	logger        interfaces.Logger
	filters       interfaces.EventStreamFilters
	eventsCh      chan *cloudevents.CloudEvent
	errorsCh      chan error
	stopCh        chan struct{}
	consumerGroup string
	consumerName  string
}

// Events returns the events channel
func (s *RedisEventStreamSubscription) Events() <-chan *cloudevents.CloudEvent {
	return s.eventsCh
}

// Errors returns the errors channel
func (s *RedisEventStreamSubscription) Errors() <-chan error {
	return s.errorsCh
}

// Close closes the subscription
func (s *RedisEventStreamSubscription) Close() error {
	close(s.stopCh)
	close(s.eventsCh)
	close(s.errorsCh)
	return nil
}

// start begins reading from Redis Streams
func (s *RedisEventStreamSubscription) start(ctx context.Context) {
	s.logger.Info("Starting Redis Streams consumer",
		"consumer_group", s.consumerGroup,
		"consumer_name", s.consumerName)

	// Create consumer groups for workflow streams dynamically
	s.ensureConsumerGroups(ctx)
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping Redis Streams consumer")
			return
		case <-s.stopCh:
			s.logger.Info("Stop signal received, stopping Redis Streams consumer")
			return
		default:
			s.readFromStream(ctx)
		}
	}
}

// readFromStream reads messages from Redis Streams
func (s *RedisEventStreamSubscription) readFromStream(ctx context.Context) {
	// Read from consumer group
	// Discover workflow streams dynamically
	workflowStreams, err := s.discoverWorkflowStreams(ctx)
	if err != nil || len(workflowStreams) == 0 {
		s.logger.Info("DEBUG: No workflow streams found, sleeping")
		time.Sleep(1 * time.Second)
		return
	}

	s.logger.Info("DEBUG: Discovered workflow streams", "streams", workflowStreams, "count", len(workflowStreams))

	// Ensure consumer groups exist for all discovered streams
	s.ensureConsumerGroups(ctx)

	// Build streams array with proper stream:id pairs
	// Format: [stream1, ">", stream2, ">", ...] to read unprocessed messages
	streamsWithIDs := make([]string, 0, len(workflowStreams)*2)
	for _, stream := range workflowStreams {
		streamsWithIDs = append(streamsWithIDs, stream, ">")
	}

	streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.consumerGroup,
		Consumer: s.consumerName,
		Streams:  streamsWithIDs,
		Count:    10,
		Block:    1 * time.Second,
	}).Result()

	if err != nil {
		if err != redis.Nil && !isNoGroupError(err) && !isInvalidStreamIDError(err) {
			s.logger.Error("Redis Stream read error", "error", err)
			s.errorsCh <- fmt.Errorf("failed to read from Redis Stream: %w", err)
		} else {
			s.logger.Info("DEBUG: Redis Stream read returned nil, sleeping")
		}
		time.Sleep(100 * time.Millisecond)
		return
	}

	s.logger.Info("DEBUG: Redis Stream read successful", "stream_count", len(streams))

	// Process messages
	messageCount := 0
	for _, stream := range streams {
		s.logger.Info("DEBUG: Processing stream", "stream", stream.Stream, "message_count", len(stream.Messages))
		messageCount += len(stream.Messages)
		for _, message := range stream.Messages {
			s.logger.Info("DEBUG: Processing message", "message_id", message.ID)
			event, err := s.parseMessage(message)
			if err != nil {
				s.logger.Error("Failed to parse Redis Stream message",
					"message_id", message.ID,
					"error", err)
				continue
			}

			s.logger.Info("DEBUG: Parsed event", "event_id", event.ID, "event_type", event.Type)

			// Apply filters
			if s.shouldProcessEvent(event) {
				s.logger.Info("Sending event to channel", "event_id", event.ID, "event_type", event.Type)
				select {
				case s.eventsCh <- event:
					s.logger.Info("DEBUG: Event sent successfully", "event_id", event.ID)
					// Acknowledge the message using the correct stream name
					s.client.XAck(ctx, stream.Stream, s.consumerGroup, message.ID)
				case <-ctx.Done():
					return
				case <-s.stopCh:
					return
				}
			} else {
				s.logger.Info("DEBUG: Event filtered out", "event_id", event.ID, "event_type", event.Type)
				// Acknowledge filtered messages using the correct stream name
				s.client.XAck(ctx, stream.Stream, s.consumerGroup, message.ID)
			}
		}
	}

	// If no messages were processed, sleep to avoid busy-wait
	if messageCount == 0 {
		s.logger.Info("DEBUG: No messages processed, sleeping")
		time.Sleep(100 * time.Millisecond)
	}
}

// parseMessage converts Redis Stream message to CloudEvent
func (s *RedisEventStreamSubscription) parseMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	event := &cloudevents.CloudEvent{}

	// Extract basic fields from Redis message values
	if eventID, ok := message.Values["event_id"].(string); ok {
		event.ID = eventID
	}
	if eventType, ok := message.Values["event_type"].(string); ok {
		event.Type = eventType
	}
	if source, ok := message.Values["source"].(string); ok {
		event.Source = source
	}
	if workflowID, ok := message.Values["workflow_id"].(string); ok {
		event.WorkflowID = workflowID
	}
	if executionID, ok := message.Values["execution_id"].(string); ok {
		event.ExecutionID = executionID
	}
	if taskID, ok := message.Values["task_id"].(string); ok {
		event.TaskID = taskID
	}
	if specVersion, ok := message.Values["spec_version"].(string); ok {
		event.SpecVersion = specVersion
	}

	// Parse timestamp
	if timestamp, ok := message.Values["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			event.Time = t
		}
	}

	// Parse data payload
	if dataStr, ok := message.Values["data"].(string); ok {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
			event.Data = data
		}
	}

	return event, nil
}

// shouldProcessEvent checks if event matches filters
func (s *RedisEventStreamSubscription) shouldProcessEvent(event *cloudevents.CloudEvent) bool {
	// Filter by event types
	if len(s.filters.EventTypes) > 0 {
		found := false
		for _, eventType := range s.filters.EventTypes {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by workflow IDs
	if len(s.filters.WorkflowIDs) > 0 {
		found := false
		for _, workflowID := range s.filters.WorkflowIDs {
			if event.WorkflowID == workflowID {
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

// isNoGroupError checks if the error is a NOGROUP error
func isNoGroupError(err error) bool {
	return err != nil && (err.Error() == "NOGROUP No such key" ||
		strings.Contains(err.Error(), "NOGROUP") ||
		strings.Contains(err.Error(), "No such key"))
}

// isInvalidStreamIDError checks if the error is an invalid stream ID error
func isInvalidStreamIDError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Invalid stream ID")
}

// ensureConsumerGroups creates consumer groups for all workflow streams
func (s *RedisEventStreamSubscription) ensureConsumerGroups(ctx context.Context) {
	// Discover existing workflow streams
	keys, err := s.client.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		s.logger.Error("Failed to discover workflow streams", "error", err)
		return
	}

	// Create consumer group for each workflow stream
	for _, streamKey := range keys {
		err := s.client.XGroupCreateMkStream(ctx, streamKey, s.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			s.logger.Warn("Failed to create consumer group",
				"stream", streamKey,
				"group", s.consumerGroup,
				"error", err)
		} else if err == nil {
			// Consumer group was just created, set it to read from the beginning
			err = s.client.XGroupSetID(ctx, streamKey, s.consumerGroup, "0").Err()
			if err != nil {
				s.logger.Warn("Failed to reset consumer group position", "stream", streamKey, "group", s.consumerGroup, "error", err)
			} else {
				s.logger.Info("Created and reset consumer group", "stream", streamKey, "group", s.consumerGroup)
			}
		}
	}
}

// discoverWorkflowStreams finds all workflow streams to read from
func (s *RedisEventStreamSubscription) discoverWorkflowStreams(ctx context.Context) ([]string, error) {
	keys, err := s.client.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to discover workflow streams: %w", err)
	}

	s.logger.Info("DEBUG: Discovered workflow streams", "streams", keys, "count", len(keys))

	// Ensure consumer groups exist for discovered streams
	for _, streamKey := range keys {
		err := s.client.XGroupCreateMkStream(ctx, streamKey, s.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			s.logger.Warn("Failed to create consumer group",
				"stream", streamKey,
				"group", s.consumerGroup,
				"error", err)
		}
	}

	return keys, nil
}
