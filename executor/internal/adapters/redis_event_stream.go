package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/executor/internal/interfaces"
)

// RedisEventStream implements EventStream using Redis Streams
type RedisEventStream struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisEventStream creates a new Redis Streams event stream
func NewRedisEventStream(client *redis.Client, logger *zap.Logger) interfaces.EventStream {
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
		consumerGroup: "executor-service",
		consumerName:  fmt.Sprintf("executor-service-%s", uuid.New().String()[:8]),
	}

	// Start the subscription goroutine
	go subscription.start(ctx)

	r.logger.Info("Created Redis Streams subscription",
		zap.Strings("event_types", filters.EventTypes),
		zap.Strings("workflow_ids", filters.WorkflowIDs),
		zap.String("consumer_group", subscription.consumerGroup))

	return subscription, nil
}

// RedisEventStreamSubscription represents a Redis Streams subscription
type RedisEventStreamSubscription struct {
	client        *redis.Client
	logger        *zap.Logger
	filters       interfaces.EventStreamFilters
	eventsCh      chan *cloudevents.CloudEvent
	errorsCh      chan error
	stopCh        chan struct{}
	consumerGroup string
	consumerName  string
}

// Events returns a channel of incoming events
func (s *RedisEventStreamSubscription) Events() <-chan *cloudevents.CloudEvent {
	return s.eventsCh
}

// Errors returns a channel of subscription errors
func (s *RedisEventStreamSubscription) Errors() <-chan error {
	return s.errorsCh
}

// Close closes the subscription
func (s *RedisEventStreamSubscription) Close() error {
	close(s.stopCh)
	return nil
}

// start starts the subscription and begins reading from Redis Streams
func (s *RedisEventStreamSubscription) start(ctx context.Context) {
	s.logger.Info("Starting Redis Streams consumer",
		zap.String("consumer_group", s.consumerGroup),
		zap.String("consumer_name", s.consumerName))

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
			s.readFromStreams(ctx)
		}
	}
}

// ensureConsumerGroups creates consumer groups for all workflow streams
func (s *RedisEventStreamSubscription) ensureConsumerGroups(ctx context.Context) {
	// Discover existing workflow streams
	keys, err := s.client.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		s.logger.Error("Failed to discover workflow streams", zap.Error(err))
		return
	}

	// Create consumer group for each workflow stream
	for _, streamKey := range keys {
		err := s.client.XGroupCreateMkStream(ctx, streamKey, s.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			s.logger.Warn("Failed to create consumer group",
				zap.String("stream", streamKey),
				zap.String("group", s.consumerGroup),
				zap.Error(err))
		}
	}
}

// readFromStreams reads messages from all workflow streams
func (s *RedisEventStreamSubscription) readFromStreams(ctx context.Context) {
	// Discover workflow streams dynamically
	workflowStreams, err := s.discoverWorkflowStreams(ctx)
	if err != nil {
		s.logger.Debug("Failed to discover workflow streams", zap.Error(err))
		time.Sleep(1 * time.Second)
		return
	}

	if len(workflowStreams) == 0 {
		// No workflow streams exist yet, wait and retry
		time.Sleep(1 * time.Second)
		return
	}

	// Build streams array with proper stream:id pairs
	// Format: [stream1, "0", stream2, "0", ...] to read all unprocessed messages
	streamsWithIDs := make([]string, 0, len(workflowStreams)*2)
	for _, stream := range workflowStreams {
		streamsWithIDs = append(streamsWithIDs, stream, "0")
	}

	// Read from all workflow streams
	streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.consumerGroup,
		Consumer: s.consumerName,
		Streams:  streamsWithIDs,
		Count:    10,
		Block:    1 * time.Second,
	}).Result()

	if err != nil {
		if err != redis.Nil && !isNoGroupError(err) && !isInvalidStreamIDError(err) {
			s.errorsCh <- fmt.Errorf("failed to read from Redis Stream: %w", err)
		}
		return
	}

	// Process messages
	for _, stream := range streams {
		for _, message := range stream.Messages {
			event, err := s.parseMessage(message)
			if err != nil {
				s.logger.Error("Failed to parse Redis Stream message",
					zap.String("message_id", message.ID),
					zap.Error(err))
				continue
			}

			// Apply filters
			if s.shouldProcessEvent(event) {
				select {
				case s.eventsCh <- event:
					// Acknowledge the message using the correct stream name
					s.client.XAck(ctx, stream.Stream, s.consumerGroup, message.ID)
				case <-ctx.Done():
					return
				case <-s.stopCh:
					return
				}
			} else {
				// Acknowledge filtered messages using the correct stream name
				s.client.XAck(ctx, stream.Stream, s.consumerGroup, message.ID)
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

	// Ensure consumer groups exist for discovered streams
	for _, streamKey := range keys {
		err := s.client.XGroupCreateMkStream(ctx, streamKey, s.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			s.logger.Warn("Failed to create consumer group",
				zap.String("stream", streamKey),
				zap.String("group", s.consumerGroup),
				zap.Error(err))
		}
	}

	return keys, nil
}

// parseMessage parses a Redis Stream message into a CloudEvent
func (s *RedisEventStreamSubscription) parseMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	event := &cloudevents.CloudEvent{}

	// Extract basic fields
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

	// Parse data
	if dataStr, ok := message.Values["data"].(string); ok {
		s.logger.Debug("Raw data string", zap.String("data", dataStr))
		var outerData map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &outerData); err == nil {
			s.logger.Debug("Parsed outer data", zap.Any("outer_data", outerData))
			// The Event Store double-wraps the data, so we need to extract the inner data
			if innerData, ok := outerData["data"].(map[string]interface{}); ok {
				s.logger.Debug("Found inner data", zap.Any("inner_data", innerData))
				event.Data = innerData
			} else {
				s.logger.Debug("No inner data found, using outer data", zap.Any("outer_data", outerData))
				// Fallback to outer data if no inner data found
				event.Data = outerData
			}
		} else {
			s.logger.Error("Failed to parse JSON data", zap.String("data", dataStr), zap.Error(err))
		}
	}

	return event, nil
}

// shouldProcessEvent checks if an event should be processed based on filters
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

	// Filter by sources
	if len(s.filters.Sources) > 0 {
		found := false
		for _, source := range s.filters.Sources {
			if event.Source == source {
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
