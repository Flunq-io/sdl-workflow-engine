package eventstreaming

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// RedisEventStream implements the generic EventStream interface using Redis Streams
type RedisEventStream struct {
	client *redis.Client
	logger Logger
}

// Logger interface for dependency injection
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// NewRedisEventStream creates a new Redis Streams event stream
func NewRedisEventStream(client *redis.Client, logger Logger) interfaces.EventStream {
	return &RedisEventStream{
		client: client,
		logger: logger,
	}
}

// getStreamKey generates a stream key using tenant and workflow isolation
// Format: tenant:{tenantId}:workflow:{workflowId}:events
func (r *RedisEventStream) getStreamKey(event *cloudevents.CloudEvent) string {
	// Extract tenant ID from the event
	tenantID := event.TenantID
	if tenantID == "" {
		tenantID = "default"
		r.logger.Warn("Event has empty TenantID, using default",
			"event_id", event.ID,
			"event_type", event.Type,
			"workflow_id", event.WorkflowID)
	}

	// Use the workflow instance ID as the stream key (each workflow gets its own stream)
	workflowID := "default"
	if event.WorkflowID != "" {
		workflowID = event.WorkflowID
	} else {
		r.logger.Warn("Event has empty WorkflowID, using default",
			"event_id", event.ID,
			"event_type", event.Type,
			"tenant_id", tenantID)
	}

	streamKey := fmt.Sprintf("tenant:%s:workflow:%s:events", tenantID, workflowID)
	r.logger.Debug("Generated stream key",
		"stream_key", streamKey,
		"tenant_id", tenantID,
		"workflow_id", workflowID,
		"event_id", event.ID,
		"event_type", event.Type)

	return streamKey
}

// discoverStreams finds all streams matching the subscription filters
func (r *RedisEventStream) discoverStreams(ctx context.Context, filters interfaces.StreamFilters) []string {
	// Use SCAN to find all keys matching the tenant:*:workflow:*:events pattern
	var streams []string
	pattern := "tenant:*:workflow:*:events"

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Check if this stream matches our filters
		if r.streamMatchesFilters(key, filters) {
			streams = append(streams, key)
		}
	}

	if err := iter.Err(); err != nil {
		r.logger.Error("Error scanning for streams", "error", err)
	}

	return streams
}

// streamMatchesFilters checks if a stream key matches the subscription filters
func (r *RedisEventStream) streamMatchesFilters(streamKey string, filters interfaces.StreamFilters) bool {
	// For now, accept all streams - we'll filter events at the message level
	// In the future, we could parse the stream key to match tenant/workflow filters
	return true
}

// Subscribe creates a subscription to Redis Streams
func (r *RedisEventStream) Subscribe(ctx context.Context, filters interfaces.StreamFilters) (interfaces.StreamSubscription, error) {
	subscription := &RedisStreamSubscription{
		client:        r.client,
		logger:        r.logger,
		filters:       filters,
		eventsCh:      make(chan *cloudevents.CloudEvent, 100),
		errorsCh:      make(chan error, 10),
		stopCh:        make(chan struct{}),
		consumerGroup: filters.ConsumerGroup,
		consumerName:  filters.ConsumerName,
	}

	// Generate consumer name if not provided
	if subscription.consumerName == "" {
		subscription.consumerName = fmt.Sprintf("%s-%s", filters.ConsumerGroup, uuid.New().String()[:8])
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
	// Determine stream key using tenant and workflow isolation
	streamKey := r.getStreamKey(event)

	// Convert CloudEvent to Redis stream fields using CloudEvents standard field names
	fields := map[string]interface{}{
		"id":          event.ID,
		"source":      event.Source,
		"specversion": event.SpecVersion,
		"type":        event.Type,
		"time":        event.Time.Format(time.RFC3339),
	}

	// Add flunq.io extension fields
	if event.WorkflowID != "" {
		fields["workflowid"] = event.WorkflowID
	}
	if event.ExecutionID != "" {
		fields["executionid"] = event.ExecutionID
	}
	if event.TaskID != "" {
		fields["taskid"] = event.TaskID
	}
	if event.TenantID != "" {
		fields["tenantid"] = event.TenantID
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
	args := &redis.XAddArgs{
		Stream: streamKey,
		Values: fields,
	}

	messageID, err := r.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish event to Redis Stream: %w", err)
	}

	r.logger.Debug("Published event to Redis Streams",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", event.WorkflowID,
		"stream", streamKey,
		"message_id", messageID)

	return nil
}

// GetEventHistory retrieves all events for a specific workflow from tenant-isolated streams
func (r *RedisEventStream) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	r.logger.Debug("Fetching event history for workflow", "workflow_id", workflowID)

	// Discover all tenant streams that contain events for this workflow
	// Pattern: tenant:*:workflow:{workflowID}:events
	pattern := fmt.Sprintf("tenant:*:workflow:%s:events", workflowID)
	streamKeys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to discover streams for workflow %s: %w", workflowID, err)
	}

	if len(streamKeys) == 0 {
		r.logger.Debug("No streams found for workflow", "workflow_id", workflowID, "pattern", pattern)
		return []*cloudevents.CloudEvent{}, nil
	}

	r.logger.Debug("Found streams for workflow",
		"workflow_id", workflowID,
		"stream_count", len(streamKeys),
		"streams", streamKeys)

	// Collect all events from all streams for this workflow
	var allEvents []*cloudevents.CloudEvent

	for _, streamKey := range streamKeys {
		// Read all events from this stream
		result, err := r.client.XRange(ctx, streamKey, "-", "+").Result()
		if err != nil {
			r.logger.Error("Failed to read from stream",
				"stream", streamKey,
				"workflow_id", workflowID,
				"error", err)
			continue // Skip this stream and continue with others
		}

		// Parse events from this stream
		for _, message := range result {
			event, err := r.parseRedisMessage(message)
			if err != nil {
				r.logger.Error("Failed to parse message",
					"message_id", message.ID,
					"stream", streamKey,
					"error", err)
				continue
			}

			// Only include events for this specific workflow
			if event.WorkflowID == workflowID {
				allEvents = append(allEvents, event)
			}
		}
	}

	r.logger.Info("Retrieved event history",
		"workflow_id", workflowID,
		"total_events", len(allEvents),
		"streams_checked", len(streamKeys))

	return allEvents, nil
}

// parseRedisMessage parses a Redis Stream message into a CloudEvent
func (r *RedisEventStream) parseRedisMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	event := &cloudevents.CloudEvent{}

	// Extract basic fields using CloudEvents standard field names
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
		if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = parsedTime
		}
	}

	// Extract flunq.io extension fields
	if workflowID, ok := message.Values["workflowid"].(string); ok {
		event.WorkflowID = workflowID
	}
	if executionID, ok := message.Values["executionid"].(string); ok {
		event.ExecutionID = executionID
	}
	if taskID, ok := message.Values["taskid"].(string); ok {
		event.TaskID = taskID
	}
	if tenantID, ok := message.Values["tenantid"].(string); ok {
		event.TenantID = tenantID
	}

	// Parse event data
	if dataStr, ok := message.Values["data"].(string); ok && dataStr != "" {
		var data interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}
		event.Data = data
	}

	return event, nil
}

// CreateConsumerGroup creates a consumer group
func (r *RedisEventStream) CreateConsumerGroup(ctx context.Context, groupName string) error {
	streamKey := "events:global"
	err := r.client.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("failed to create consumer group %s: %w", groupName, err)
	}
	return nil
}

// DeleteConsumerGroup deletes a consumer group
func (r *RedisEventStream) DeleteConsumerGroup(ctx context.Context, groupName string) error {
	streamKey := "events:global"
	return r.client.XGroupDestroy(ctx, streamKey, groupName).Err()
}

// GetStreamInfo returns stream information
func (r *RedisEventStream) GetStreamInfo(ctx context.Context) (*interfaces.StreamInfo, error) {
	streamKey := "events:global"
	info, err := r.client.XInfoStream(ctx, streamKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	return &interfaces.StreamInfo{
		StreamName:      streamKey,
		MessageCount:    info.Length,
		ConsumerGroups:  []string{}, // Would need additional call to get groups
		LastMessageTime: time.Now(), // Would need to parse from last message
		Metadata:        map[string]string{},
	}, nil
}

// Close closes the stream connection
func (r *RedisEventStream) Close() error {
	return r.client.Close()
}

// RedisStreamSubscription represents a Redis Streams subscription
type RedisStreamSubscription struct {
	client        *redis.Client
	logger        Logger
	filters       interfaces.StreamFilters
	eventsCh      chan *cloudevents.CloudEvent
	errorsCh      chan error
	stopCh        chan struct{}
	consumerGroup string
	consumerName  string
}

// Events returns the events channel
func (s *RedisStreamSubscription) Events() <-chan *cloudevents.CloudEvent {
	return s.eventsCh
}

// Errors returns the errors channel
func (s *RedisStreamSubscription) Errors() <-chan error {
	return s.errorsCh
}

// Acknowledge acknowledges processing of an event
func (s *RedisStreamSubscription) Acknowledge(ctx context.Context, eventID string) error {
	streamKey := "events:global"
	return s.client.XAck(ctx, streamKey, s.consumerGroup, eventID).Err()
}

// Close closes the subscription
func (s *RedisStreamSubscription) Close() error {
	close(s.stopCh)
	return nil
}

// start begins the subscription loop
func (s *RedisStreamSubscription) start(ctx context.Context) {
	// Initial stream discovery and consumer group setup
	streams := s.setupStreamsAndGroups(ctx)

	// Set up periodic stream rediscovery
	rediscoveryTicker := time.NewTicker(5 * time.Second) // Rediscover more frequently to pick up new workflow streams
	defer rediscoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-rediscoveryTicker.C:
			// Periodically rediscover streams to pick up new workflow streams
			s.logger.Debug("Rediscovering streams...")
			newStreams := s.setupStreamsAndGroups(ctx)
			if len(newStreams) != len(streams) {
				s.logger.Info("Stream topology changed",
					"old_count", len(streams),
					"new_count", len(newStreams),
					"streams", newStreams)
				streams = newStreams
			}
		default:
			// Skip if no streams to read from (wait for streams to be discovered)
			if len(streams) == 0 {
				s.logger.Debug("No streams discovered yet, waiting...")
				time.Sleep(5 * time.Second) // Wait longer when no streams exist
				continue
			}

			// Prepare streams for XReadGroup
			// Redis expects Streams: [s1, s2, ..., id1, id2, ...]
			names := make([]string, 0, len(streams))
			ids := make([]string, 0, len(streams))
			for _, stream := range streams {
				if stream == "" {
					s.logger.Error("Empty stream name detected, skipping", "streams", streams)
					continue
				}
				names = append(names, stream)
				ids = append(ids, ">")
			}
			streamArgs := append(names, ids...)

			// Skip if no valid streams after filtering
			if len(streamArgs) == 0 {
				s.logger.Debug("No valid streams after filtering, waiting...")
				time.Sleep(5 * time.Second)
				continue
			}

			s.logger.Debug("Reading from Redis streams",
				"consumer_group", s.consumerGroup,
				"consumer_name", s.consumerName,
				"stream_args", streamArgs)

			// Read from all Redis streams
			result, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    s.consumerGroup,
				Consumer: s.consumerName,
				Streams:  streamArgs,
				Count:    10,
				Block:    time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No new messages
				}

				// Handle specific Redis errors gracefully
				errStr := err.Error()
				if strings.Contains(errStr, "NOGROUP") || strings.Contains(errStr, "NOSTREAM") {
					s.logger.Warn("Stream or consumer group missing, recreating...", "error", err)
					// Rediscover streams and recreate consumer groups
					streams = s.setupStreamsAndGroups(ctx)
					time.Sleep(5 * time.Second) // Wait before retrying
					continue
				}

				s.logger.Error("Failed to read from Redis streams", "error", err)
				s.errorsCh <- fmt.Errorf("failed to read from Redis streams: %w", err)
				time.Sleep(1 * time.Second) // Prevent rapid error loops
				continue
			}

			// Process messages from all streams
			for _, stream := range result {
				streamName := stream.Stream
				for _, message := range stream.Messages {
					event, err := s.parseMessage(message)
					if err != nil {
						s.logger.Error("Failed to parse message", "error", err, "message_id", message.ID, "stream", streamName)
						continue
					}

					// Apply filters
					if s.shouldProcessEvent(event) {
						// Attach ack metadata so the consumer can ack after successful processing
						if event.Extensions == nil {
							event.Extensions = make(map[string]interface{})
						}
						ackKey := fmt.Sprintf("%s|%s", streamName, message.ID)
						event.Extensions["ack_key"] = ackKey
						event.Extensions["redis_stream"] = streamName
						event.Extensions["redis_msg_id"] = message.ID
						s.eventsCh <- event
					}
				}
			}
		}
	}
}

// parseMessage parses a Redis Stream message into a CloudEvent
func (s *RedisStreamSubscription) parseMessage(message redis.XMessage) (*cloudevents.CloudEvent, error) {
	event := &cloudevents.CloudEvent{}

	// Extract basic fields using CloudEvents standard field names
	if id, ok := message.Values["id"].(string); ok {
		event.ID = id
	}
	if eventType, ok := message.Values["type"].(string); ok {
		event.Type = eventType
	}
	if source, ok := message.Values["source"].(string); ok {
		event.Source = source
	}
	if specVersion, ok := message.Values["specversion"].(string); ok {
		event.SpecVersion = specVersion
	}

	// Parse timestamp using CloudEvents standard field name
	if timeStr, ok := message.Values["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = t
		}
	}

	// Parse flunq.io extension fields
	if workflowID, ok := message.Values["workflowid"].(string); ok {
		event.WorkflowID = workflowID
	}
	if executionID, ok := message.Values["executionid"].(string); ok {
		event.ExecutionID = executionID
	}
	if taskID, ok := message.Values["taskid"].(string); ok {
		event.TaskID = taskID
	}
	if tenantID, ok := message.Values["tenantid"].(string); ok {
		event.TenantID = tenantID
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

// setupStreamsAndGroups discovers streams and creates consumer groups
func (s *RedisStreamSubscription) setupStreamsAndGroups(ctx context.Context) []string {
	// Create a temporary RedisEventStream to access discovery methods
	eventStream := &RedisEventStream{client: s.client, logger: s.logger}

	// Discover streams based on filters
	streams := eventStream.discoverStreams(ctx, s.filters)
	if len(streams) == 0 {
		s.logger.Info("No streams found matching filters, will wait for streams to be created")
		// DO NOT fall back to default stream - wait for real streams to be created
		return []string{} // Return empty list, periodic rediscovery will find streams later
	}

	// Create consumer groups for all discovered streams
	for _, streamKey := range streams {
		err := s.client.XGroupCreateMkStream(ctx, streamKey, s.consumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			s.logger.Warn("Failed to create consumer group for stream",
				"stream", streamKey,
				"group", s.consumerGroup,
				"error", err)
			continue
		}
		s.logger.Debug("Consumer group ready for stream",
			"stream", streamKey,
			"group", s.consumerGroup)
	}

	return streams
}

// shouldProcessEvent checks if an event should be processed based on filters
func (s *RedisStreamSubscription) shouldProcessEvent(event *cloudevents.CloudEvent) bool {
	// Check event type filter
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

	// Check workflow ID filter
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

	// Check source filter
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
