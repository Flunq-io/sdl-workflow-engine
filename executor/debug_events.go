package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	ctx := context.Background()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	// Initialize Event Store client
	eventClient := client.NewEventClient("http://localhost:8081", "debug-service", logger)

	// 1. Check if there are any workflow streams in Redis
	fmt.Println("=== Checking Redis Streams ===")
	keys, err := redisClient.Keys(ctx, "events:workflow:*").Result()
	if err != nil {
		log.Fatal("Failed to get Redis keys:", err)
	}
	fmt.Printf("Found %d workflow streams: %v\n", len(keys), keys)

	// 2. Check if there are any task.requested events in the streams
	for _, streamKey := range keys {
		fmt.Printf("\n--- Checking stream: %s ---\n", streamKey)
		
		// Read recent messages from the stream
		messages, err := redisClient.XRevRange(ctx, streamKey, "+", "-").Result()
		if err != nil {
			fmt.Printf("Error reading from stream %s: %v\n", streamKey, err)
			continue
		}

		fmt.Printf("Found %d messages in stream %s\n", len(messages), streamKey)
		
		// Look for task.requested events
		for i, msg := range messages {
			if i >= 5 { // Only check first 5 messages
				break
			}
			
			if eventType, ok := msg.Values["type"].(string); ok {
				fmt.Printf("  Message %s: type=%s\n", msg.ID, eventType)
				
				if eventType == "io.flunq.task.requested" {
					fmt.Printf("    Found task.requested event!\n")
					if data, ok := msg.Values["data"].(string); ok {
						fmt.Printf("    Data: %s\n", data)
					}
				}
			}
		}
	}

	// 3. Publish a test task.requested event
	fmt.Println("\n=== Publishing Test Task Event ===")
	testEvent := &cloudevents.CloudEvent{
		ID:          "test-task-debug-123",
		Source:      "debug-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.requested",
		WorkflowID:  "debug-workflow-456",
		ExecutionID: "debug-execution-789",
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":   "debug-task-123",
			"task_name": "debug-task",
			"task_type": "set",
			"config": map[string]interface{}{
				"parameters": map[string]interface{}{
					"test_key": "test_value",
				},
			},
		},
	}

	if err := eventClient.PublishEvent(ctx, testEvent); err != nil {
		log.Fatal("Failed to publish test event:", err)
	}
	fmt.Println("Published test task.requested event")

	// 4. Wait a bit and check for task.completed events
	fmt.Println("\n=== Waiting for task.completed event ===")
	time.Sleep(5 * time.Second)

	// Check for task.completed events in the workflow stream
	workflowStream := fmt.Sprintf("events:workflow:%s", testEvent.WorkflowID)
	fmt.Printf("Checking for completed events in stream: %s\n", workflowStream)
	
	messages, err := redisClient.XRevRange(ctx, workflowStream, "+", "-").Result()
	if err != nil {
		log.Fatal("Failed to read from workflow stream:", err)
	}

	fmt.Printf("Found %d total messages in workflow stream\n", len(messages))
	
	foundCompleted := false
	for _, msg := range messages {
		if eventType, ok := msg.Values["type"].(string); ok {
			fmt.Printf("  Message %s: type=%s\n", msg.ID, eventType)
			
			if eventType == "io.flunq.task.completed" {
				foundCompleted = true
				fmt.Printf("    ✅ Found task.completed event!\n")
				
				// Print the event details
				if data, ok := msg.Values["data"].(string); ok {
					var eventData map[string]interface{}
					if err := json.Unmarshal([]byte(data), &eventData); err == nil {
						prettyData, _ := json.MarshalIndent(eventData, "    ", "  ")
						fmt.Printf("    Data: %s\n", string(prettyData))
					}
				}
				
				if workflowID, ok := msg.Values["workflow_id"].(string); ok {
					fmt.Printf("    WorkflowID: %s\n", workflowID)
				}
				if executionID, ok := msg.Values["execution_id"].(string); ok {
					fmt.Printf("    ExecutionID: %s\n", executionID)
				}
			}
		}
	}

	if !foundCompleted {
		fmt.Println("    ❌ No task.completed event found!")
		fmt.Println("    This suggests the executor service is not processing events correctly.")
	}

	// 5. Check executor service consumer group
	fmt.Println("\n=== Checking Executor Consumer Group ===")
	for _, streamKey := range keys {
		groups, err := redisClient.XInfoGroups(ctx, streamKey).Result()
		if err != nil {
			fmt.Printf("Error getting groups for stream %s: %v\n", streamKey, err)
			continue
		}
		
		fmt.Printf("Stream %s has %d consumer groups:\n", streamKey, len(groups))
		for _, group := range groups {
			fmt.Printf("  Group: %s, Consumers: %d, Pending: %d\n", 
				group.Name, group.Consumers, group.Pending)
		}
	}
}
