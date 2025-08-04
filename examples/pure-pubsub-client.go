package main

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/interfaces"
	"github.com/flunq-io/events/internal/publisher"
	"github.com/flunq-io/events/internal/storage"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// Example showing how to use the Event Store in pure pub/sub mode
// without any HTTP/gRPC servers - just direct Redis connections

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Connect directly to Redis (same as Event Store)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	// Create storage and publisher using the same interfaces as Event Store
	eventStorage := storage.NewRedisStorage(redisClient, logger)
	eventPublisher := publisher.NewRedisPublisher(redisClient, logger)

	// Example 1: Direct event publishing (bypassing Event Store service)
	logger.Info("=== Example 1: Direct Event Publishing ===")
	
	event := &cloudevents.CloudEvent{
		ID:          uuid.New().String(),
		Source:      "example-client",
		SpecVersion: "1.0",
		Type:        "io.flunq.workflow.started",
		WorkflowID:  "direct-workflow-123",
		Data: map[string]interface{}{
			"message": "Direct publish without HTTP",
			"client":  "pure-pubsub-client",
		},
	}

	// Store the event
	if err := eventStorage.Store(ctx, event); err != nil {
		logger.Error("Failed to store event", zap.Error(err))
		return
	}

	// Publish to subscribers
	if err := eventPublisher.Publish(ctx, event); err != nil {
		logger.Error("Failed to publish event", zap.Error(err))
		return
	}

	logger.Info("Event published directly", zap.String("event_id", event.ID))

	// Example 2: Subscribe to events
	logger.Info("=== Example 2: Event Subscription ===")

	subscription := &interfaces.Subscription{
		ID:          uuid.New().String(),
		ServiceName: "example-client",
		EventTypes:  []string{"io.flunq.workflow.started", "io.flunq.task.completed"},
		WorkflowIDs: []string{}, // Empty means all workflows
		Filters:     map[string]string{},
	}

	sub, err := eventPublisher.Subscribe(ctx, subscription)
	if err != nil {
		logger.Error("Failed to create subscription", zap.Error(err))
		return
	}
	defer sub.Close()

	logger.Info("Subscription created, listening for events...")

	// Listen for events for 10 seconds
	timeout := time.After(10 * time.Second)

	go func() {
		// Publish a few more events to demonstrate subscription
		time.Sleep(2 * time.Second)
		
		for i := 0; i < 3; i++ {
			testEvent := &cloudevents.CloudEvent{
				ID:          uuid.New().String(),
				Source:      "test-publisher",
				SpecVersion: "1.0",
				Type:        "io.flunq.task.completed",
				WorkflowID:  "test-workflow-456",
				Data: map[string]interface{}{
					"task_name": "test-task",
					"iteration": i,
				},
			}

			eventStorage.Store(ctx, testEvent)
			eventPublisher.Publish(ctx, testEvent)
			
			time.Sleep(1 * time.Second)
		}
	}()

	eventCount := 0
	for {
		select {
		case event := <-sub.Events():
			eventCount++
			logger.Info("Received event",
				zap.Int("count", eventCount),
				zap.String("event_id", event.ID),
				zap.String("type", event.Type),
				zap.String("source", event.Source),
				zap.String("workflow_id", event.WorkflowID),
				zap.Any("data", event.Data))

		case err := <-sub.Errors():
			logger.Error("Subscription error", zap.Error(err))

		case <-timeout:
			logger.Info("Subscription timeout reached")
			goto cleanup
		}
	}

cleanup:
	// Example 3: Retrieve stored events
	logger.Info("=== Example 3: Event History Retrieval ===")

	// Get events for the direct workflow
	events, err := eventStorage.GetHistory(ctx, "direct-workflow-123")
	if err != nil {
		logger.Error("Failed to get event history", zap.Error(err))
		return
	}

	logger.Info("Retrieved event history",
		zap.String("workflow_id", "direct-workflow-123"),
		zap.Int("event_count", len(events)))

	for i, event := range events {
		logger.Info("Historical event",
			zap.Int("index", i),
			zap.String("event_id", event.ID),
			zap.String("type", event.Type),
			zap.Time("time", event.Time))
	}

	// Get stats
	stats, err := eventStorage.GetStats(ctx)
	if err != nil {
		logger.Error("Failed to get stats", zap.Error(err))
		return
	}

	logger.Info("Event store statistics", zap.Any("stats", stats))

	logger.Info("Pure pub/sub client example completed")
}

// This example demonstrates:
// 1. Direct Redis connection without Event Store HTTP service
// 2. Using the same storage and publisher interfaces
// 3. Publishing events directly to Redis
// 4. Subscribing to events via Redis pub/sub
// 5. Retrieving stored events from Redis streams
//
// Benefits of this approach:
// - No HTTP overhead
// - Direct Redis performance
// - Same interfaces as Event Store service
// - Can switch between modes easily
// - Suitable for high-throughput scenarios
