package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/executor/internal/executor"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize Event Store client
	eventClient := client.NewEventClient("http://localhost:8081", "test-executor", logger)

	ctx := context.Background()

	// Create a test task result with WorkflowID and ExecutionID
	taskResult := &executor.TaskResult{
		TaskID:      "test-task-123",
		TaskName:    "test-task",
		WorkflowID:  "simple-test-workflow",
		ExecutionID: "test-execution-789",
		Success:     true,
		Output:      map[string]interface{}{"result": "success", "value": 42},
		Duration:    100 * time.Millisecond,
		ExecutedAt:  time.Now(),
	}

	// Create the task.completed event manually (same as the executor would)
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-completed-%s", taskResult.TaskID),
		Source:      "executor-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.completed",
		WorkflowID:  taskResult.WorkflowID,
		ExecutionID: taskResult.ExecutionID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":     taskResult.TaskID,
			"task_name":   taskResult.TaskName,
			"success":     taskResult.Success,
			"output":      taskResult.Output,
			"error":       taskResult.Error,
			"duration_ms": taskResult.Duration.Milliseconds(),
			"executed_at": taskResult.ExecutedAt.Format(time.RFC3339),
		},
	}

	fmt.Println("=== Publishing test task.completed event ===")
	fmt.Printf("Event ID: %s\n", event.ID)
	fmt.Printf("Event Type: %s\n", event.Type)
	fmt.Printf("WorkflowID: %s\n", event.WorkflowID)
	fmt.Printf("ExecutionID: %s\n", event.ExecutionID)
	fmt.Printf("Source: %s\n", event.Source)

	// Publish the event
	if err := eventClient.PublishEvent(ctx, event); err != nil {
		log.Fatal("Failed to publish test event:", err)
	}

	fmt.Println("✅ Successfully published task.completed event!")
	fmt.Println("\nNow check the Redis stream to see if the event was stored correctly:")
	fmt.Printf("redis-cli XRANGE events:workflow:%s - +\n", taskResult.WorkflowID)
}
