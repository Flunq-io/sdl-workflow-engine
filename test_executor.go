package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()

	// Test Redis connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	// This is the exact JSON format that should work (from our test)
	taskRequestedData := `{
		"config": {
			"parameters": {},
			"retries": 3,
			"timeout": 300
		},
		"context": {},
		"execution_id": "exec-12345",
		"input": {},
		"task_id": "task-test-direct-123",
		"task_name": "test_task",
		"task_type": "set",
		"workflow_id": "test-workflow"
	}`

	// Add event to Redis Stream
	streamKey := "events:workflow:test-workflow"
	
	args := &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event_id":     "test-task-requested-123",
			"event_type":   "io.flunq.task.requested",
			"source":       "test-script",
			"workflow_id":  "test-workflow",
			"execution_id": "exec-12345",
			"task_id":      "task-test-direct-123",
			"data":         taskRequestedData,
			"timestamp":    "2025-08-04T16:30:00Z",
			"spec_version": "1.0",
		},
	}

	messageID, err := client.XAdd(ctx, args).Result()
	if err != nil {
		log.Fatal("Failed to add event to Redis Stream:", err)
	}

	fmt.Printf("✅ Added task.requested event to Redis Stream\n")
	fmt.Printf("Stream: %s\n", streamKey)
	fmt.Printf("Message ID: %s\n", messageID)
	fmt.Printf("Event ID: test-task-requested-123\n")
	fmt.Printf("Task Type: set\n")
	fmt.Printf("Task Name: test_task\n")
	fmt.Println()
	fmt.Println("🔍 Check the Executor logs to see if it processes this event!")
	fmt.Println("If the Executor shows 'no executor found for task type: ', then there's still an issue.")
	fmt.Println("If it shows 'Executing set task', then the JSON parsing is working!")

	client.Close()
}
