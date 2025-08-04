package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// Example showing how services integrate with the Event Store

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create Event Store client
	eventClient := client.NewEventClient("http://localhost:8081", "example-service", logger)

	ctx := context.Background()

	// Example 1: Publishing Events
	publishEventsExample(ctx, eventClient, logger)

	// Example 2: Subscribing to Events
	subscribeToEventsExample(ctx, eventClient, logger)

	// Example 3: Getting Event History
	getEventHistoryExample(ctx, eventClient, logger)
}

// Example 1: Publishing Events
func publishEventsExample(ctx context.Context, client *client.EventClient, logger *zap.Logger) {
	logger.Info("=== Publishing Events Example ===")

	workflowID := uuid.New().String()
	executionID := uuid.New().String()

	// Publish workflow started event
	workflowStarted := cloudevents.NewWorkflowEvent(
		uuid.New().String(),
		"example-service",
		cloudevents.WorkflowStarted,
		workflowID,
	)
	workflowStarted.SetData(map[string]interface{}{
		"workflow_name": "user-onboarding",
		"started_by":    "user-123",
		"input_data": map[string]interface{}{
			"email": "user@example.com",
			"name":  "John Doe",
		},
	})

	if err := client.PublishEvent(ctx, workflowStarted); err != nil {
		logger.Error("Failed to publish workflow started event", zap.Error(err))
		return
	}

	// Publish task started event
	taskStarted := cloudevents.NewTaskEvent(
		uuid.New().String(),
		"example-service",
		cloudevents.TaskStarted,
		workflowID,
		executionID,
		"validate-email",
	)
	taskStarted.SetData(map[string]interface{}{
		"task_name": "validate-email",
		"input": map[string]interface{}{
			"email": "user@example.com",
		},
	})

	if err := client.PublishEvent(ctx, taskStarted); err != nil {
		logger.Error("Failed to publish task started event", zap.Error(err))
		return
	}

	// Publish task completed event
	taskCompleted := cloudevents.NewTaskEvent(
		uuid.New().String(),
		"example-service",
		cloudevents.TaskCompleted,
		workflowID,
		executionID,
		"validate-email",
	)
	taskCompleted.SetData(map[string]interface{}{
		"task_name": "validate-email",
		"result": map[string]interface{}{
			"valid":  true,
			"reason": "Email format is valid",
		},
		"duration_ms": 150,
	})

	if err := client.PublishEvent(ctx, taskCompleted); err != nil {
		logger.Error("Failed to publish task completed event", zap.Error(err))
		return
	}

	logger.Info("Successfully published events", zap.String("workflow_id", workflowID))
}

// Example 2: Subscribing to Events
func subscribeToEventsExample(ctx context.Context, client *client.EventClient, logger *zap.Logger) {
	logger.Info("=== Subscribing to Events Example ===")

	// Subscribe to specific event types
	eventTypes := []string{
		cloudevents.WorkflowStarted,
		cloudevents.TaskCompleted,
		cloudevents.WorkflowCompleted,
	}

	// Subscribe to specific workflows (empty means all workflows)
	workflowIDs := []string{}

	// Custom filters
	filters := map[string]string{
		"environment": "development",
	}

	subscription, err := client.SubscribeWebSocket(ctx, eventTypes, workflowIDs, filters)
	if err != nil {
		logger.Error("Failed to create WebSocket subscription", zap.Error(err))
		return
	}
	defer subscription.Close()

	logger.Info("WebSocket subscription established, listening for events...")

	// Listen for events for 30 seconds
	timeout := time.After(30 * time.Second)

	for {
		select {
		case event := <-subscription.Events():
			logger.Info("Received event",
				zap.String("event_id", event.ID),
				zap.String("event_type", event.Type),
				zap.String("source", event.Source),
				zap.String("workflow_id", event.WorkflowID),
				zap.Any("data", event.Data))

			// Process the event based on type
			switch event.Type {
			case cloudevents.WorkflowStarted:
				logger.Info("Processing workflow started event")
				// Handle workflow started logic
				
			case cloudevents.TaskCompleted:
				logger.Info("Processing task completed event")
				// Handle task completed logic
				
			case cloudevents.WorkflowCompleted:
				logger.Info("Processing workflow completed event")
				// Handle workflow completed logic
			}

		case err := <-subscription.Errors():
			logger.Error("WebSocket subscription error", zap.Error(err))
			return

		case <-subscription.Done():
			logger.Info("WebSocket subscription closed")
			return

		case <-timeout:
			logger.Info("Subscription timeout reached")
			return
		}
	}
}

// Example 3: Getting Event History
func getEventHistoryExample(ctx context.Context, client *client.EventClient, logger *zap.Logger) {
	logger.Info("=== Getting Event History Example ===")

	// This would be a real workflow ID in practice
	workflowID := "example-workflow-123"

	events, err := client.GetEventHistory(ctx, workflowID)
	if err != nil {
		logger.Error("Failed to get event history", zap.Error(err))
		return
	}

	logger.Info("Retrieved event history",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	// Process each event in chronological order
	for i, event := range events {
		logger.Info("Event in history",
			zap.Int("sequence", i+1),
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.Time("timestamp", event.Time),
			zap.Any("data", event.Data))
	}
}

// Example service integration patterns

// WorkerService example - how the Worker service would integrate
type WorkerService struct {
	eventClient *client.EventClient
	logger      *zap.Logger
}

func NewWorkerService(eventStoreURL string, logger *zap.Logger) *WorkerService {
	return &WorkerService{
		eventClient: client.NewEventClient(eventStoreURL, "worker-service", logger),
		logger:      logger,
	}
}

func (w *WorkerService) StartWorkflow(ctx context.Context, workflowID string, definition interface{}) error {
	// Publish workflow started event
	event := cloudevents.NewWorkflowEvent(
		uuid.New().String(),
		"worker-service",
		cloudevents.WorkflowStarted,
		workflowID,
	)
	event.SetData(map[string]interface{}{
		"definition": definition,
		"started_at": time.Now(),
	})

	return w.eventClient.PublishEvent(ctx, event)
}

func (w *WorkerService) SubscribeToTaskEvents(ctx context.Context) error {
	// Subscribe to task-related events
	eventTypes := []string{
		cloudevents.TaskCompleted,
		cloudevents.TaskFailed,
	}

	subscription, err := w.eventClient.SubscribeWebSocket(ctx, eventTypes, nil, nil)
	if err != nil {
		return err
	}

	// Process events in background
	go func() {
		defer subscription.Close()
		
		for {
			select {
			case event := <-subscription.Events():
				w.handleTaskEvent(ctx, event)
			case err := <-subscription.Errors():
				w.logger.Error("Task event subscription error", zap.Error(err))
				return
			case <-subscription.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (w *WorkerService) handleTaskEvent(ctx context.Context, event *cloudevents.CloudEvent) {
	w.logger.Info("Handling task event",
		zap.String("event_type", event.Type),
		zap.String("workflow_id", event.WorkflowID),
		zap.String("task_id", event.TaskID))

	switch event.Type {
	case cloudevents.TaskCompleted:
		// Continue workflow execution
		w.continueWorkflow(ctx, event.WorkflowID, event.TaskID)
		
	case cloudevents.TaskFailed:
		// Handle task failure
		w.handleTaskFailure(ctx, event.WorkflowID, event.TaskID, event.Data)
	}
}

func (w *WorkerService) continueWorkflow(ctx context.Context, workflowID, completedTaskID string) {
	// Implementation would continue workflow execution
	w.logger.Info("Continuing workflow after task completion",
		zap.String("workflow_id", workflowID),
		zap.String("completed_task", completedTaskID))
}

func (w *WorkerService) handleTaskFailure(ctx context.Context, workflowID, failedTaskID string, errorData interface{}) {
	// Implementation would handle task failure
	w.logger.Error("Handling task failure",
		zap.String("workflow_id", workflowID),
		zap.String("failed_task", failedTaskID),
		zap.Any("error_data", errorData))
}
