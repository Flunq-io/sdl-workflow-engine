package processor

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/flunq-io/executor/internal/executor"
	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// TaskProcessor processes task execution requests
type TaskProcessor struct {
	eventStream   interfaces.EventStream
	taskExecutors map[string]executor.TaskExecutor
	logger        *zap.Logger

	subscription interfaces.StreamSubscription
	running      bool
	wg           sync.WaitGroup

	// Concurrency control for task processing
	maxConcurrency int
	sem            chan struct{}
}

// NewTaskProcessor creates a new TaskProcessor
func NewTaskProcessor(
	eventStream interfaces.EventStream,
	taskExecutors map[string]executor.TaskExecutor,
	logger *zap.Logger,
) *TaskProcessor {
	// Determine concurrency from env (default 4)
	maxConc := 4
	if v := os.Getenv("EXECUTOR_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConc = n
		}
	}
	return &TaskProcessor{
		eventStream:    eventStream,
		taskExecutors:  taskExecutors,
		logger:         logger,
		maxConcurrency: maxConc,
		sem:            make(chan struct{}, maxConc),
	}
}

// Start starts the task processor
func (p *TaskProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting task processor")

	// Subscribe to task.requested events
	filters := interfaces.StreamFilters{
		EventTypes: []string{
			"io.flunq.task.requested",
		},
		WorkflowIDs:   []string{}, // Subscribe to all workflows
		ConsumerGroup: "executor-service",
		ConsumerName:  "executor-service-instance",
	}

	subscription, err := p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	p.subscription = subscription
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.processEvents(ctx)

	p.logger.Info("Task processor started successfully")
	return nil
}

// Stop stops the task processor
func (p *TaskProcessor) Stop(ctx context.Context) error {
	p.logger.Info("Stopping task processor")

	p.running = false

	if p.subscription != nil {
		p.subscription.Close()
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Task processor stopped")
		return nil
	case <-ctx.Done():
		p.logger.Warn("Task processor stop timed out")
		return ctx.Err()
	}
}

// processEvents processes incoming task events
func (p *TaskProcessor) processEvents(ctx context.Context) {
	defer p.wg.Done()

	for p.running {
		select {
		case event := <-p.subscription.Events():
			// Concurrency gate
			p.sem <- struct{}{}
			go func(ev *cloudevents.CloudEvent) {
				defer func() { <-p.sem }()
				if err := p.processTaskEvent(ctx, ev); err != nil {
					p.logger.Error("Failed to process task event",
						zap.String("event_id", ev.ID),
						zap.String("event_type", ev.Type),
						zap.Error(err))
				}
			}(event)
		case err := <-p.subscription.Errors():
			p.logger.Error("Event stream error", zap.Error(err))
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping event processing")
			return
		}
	}
}

// processTaskEvent processes a single task event
func (p *TaskProcessor) processTaskEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	p.logger.Info("Processing task event",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
		zap.String("workflow_id", event.WorkflowID))

	// Parse task request from event
	taskRequest, err := executor.ParseTaskRequestFromEvent(event)
	if err != nil {
		return fmt.Errorf("failed to parse task request: %w", err)
	}

	// Find appropriate task executor
	taskExecutor, exists := p.taskExecutors[taskRequest.TaskType]
	if !exists {
		return fmt.Errorf("no executor found for task type: %s", taskRequest.TaskType)
	}

	// Execute the task
	result, err := taskExecutor.Execute(ctx, taskRequest)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Publish task completed event (pass tenant ID from original event)
	if err := p.publishTaskCompletedEvent(ctx, result, event.TenantID); err != nil {
		return fmt.Errorf("failed to publish task completed event: %w", err)
	}

	p.logger.Info("Successfully processed task event",
		zap.String("event_id", event.ID),
		zap.String("task_id", taskRequest.TaskID),
		zap.String("task_type", taskRequest.TaskType),
		zap.Bool("success", result.Success))

	return nil
}

// publishTaskCompletedEvent publishes a task completed event
func (p *TaskProcessor) publishTaskCompletedEvent(ctx context.Context, result *executor.TaskResult, tenantID string) error {
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-completed-%s", result.TaskID),
		Source:      "executor-service", // Changed from "task-service"
		SpecVersion: "1.0",
		Type:        "io.flunq.task.completed",
		TenantID:    tenantID,           // ← Add tenant ID
		WorkflowID:  result.WorkflowID,  // ← Add this
		ExecutionID: result.ExecutionID, // ← Add this
		TaskID:      result.TaskID,      // ← Add this
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":     result.TaskID,
			"task_name":   result.TaskName,
			"tenant_id":   tenantID,
			"success":     result.Success,
			"output":      result.Output,
			"error":       result.Error,
			"duration_ms": result.Duration.Milliseconds(),
			"executed_at": result.ExecutedAt.Format(time.RFC3339),
			// Include complete TaskData for enhanced I/O storage
			"data": map[string]interface{}{
				"input":  result.Input,  // Add input data
				"output": result.Output, // Include output data
				"metadata": map[string]interface{}{
					"started_at":    result.StartedAt.Format(time.RFC3339),
					"completed_at":  result.ExecutedAt.Format(time.RFC3339),
					"duration_ms":   result.Duration.Milliseconds(),
					"task_type":     result.TaskType,
					"activity_name": "executor_activity",
				},
			},
		},
	}

	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish task completed event: %w", err)
	}

	p.logger.Info("Published task completed event",
		zap.String("task_id", result.TaskID),
		zap.String("task_name", result.TaskName),
		zap.Bool("success", result.Success),
		zap.String("event_id", event.ID))

	return nil
}
