package processor

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/flunq-io/executor/internal/executor"
	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// TaskProcessor processes task execution requests with enterprise-grade resilience
type TaskProcessor struct {
	eventStream   interfaces.EventStream
	taskExecutors map[string]executor.TaskExecutor
	logger        *zap.Logger

	subscription interfaces.StreamSubscription
	errorCh      <-chan error
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	// Per-event workers waitgroup (for graceful shutdown when concurrency > 1)
	tasksWG sync.WaitGroup

	// Concurrency control for task processing
	maxConcurrency int
	sem            chan struct{}
}

// NewTaskProcessor creates a new TaskProcessor with enterprise-grade resilience
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
		stopCh:         make(chan struct{}),
		maxConcurrency: maxConc,
		sem:            make(chan struct{}, maxConc),
	}
}

// Start starts the task processor with enhanced resilience
func (p *TaskProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting task processor with enhanced resilience")

	// Health check event stream connectivity
	if info, err := p.eventStream.GetStreamInfo(ctx); err != nil {
		p.logger.Warn("Stream info not available yet", zap.Error(err))
	} else {
		p.logger.Info("Stream info",
			zap.String("stream", info.StreamName),
			zap.Int64("message_count", info.MessageCount),
			zap.Int("consumer_groups", len(info.ConsumerGroups)))
	}

	// Subscribe to task.requested events with enhanced configuration
	filters := interfaces.StreamFilters{
		EventTypes: []string{
			"io.flunq.task.requested",
		},
		WorkflowIDs:   []string{}, // Subscribe to all workflows
		ConsumerGroup: getEnvDefault("EXECUTOR_CONSUMER_GROUP", "executor-service"),
		ConsumerName:  fmt.Sprintf("executor-%s", getEnvDefault("HOSTNAME", "unknown")),
		BatchCount:    getEnvIntDefault("EXECUTOR_STREAM_BATCH", 10),
		BlockTimeout:  getEnvDurationDefault("EXECUTOR_STREAM_BLOCK", time.Second),
	}

	// Create subscription once and keep reference for acks
	subscription, err := p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	p.subscription = subscription
	p.errorCh = subscription.Errors()
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.eventProcessingLoop(ctx)

	p.logger.Info("Task processor started successfully with enhanced resilience")

	// Optionally start background reclaim loop for orphaned pending messages
	if getEnvBoolDefault("EXECUTOR_RECLAIM_ENABLED", false) {
		go p.reclaimOrphanedMessages(ctx, filters.ConsumerGroup, filters.ConsumerName)
	}
	return nil
}

// Stop stops the task processor with graceful shutdown
func (p *TaskProcessor) Stop(ctx context.Context) error {
	p.logger.Info("Stopping task processor")

	p.running = false
	close(p.stopCh)

	// Close event stream connection
	if p.eventStream != nil {
		p.eventStream.Close()
	}

	// Wait for processing to complete
	p.wg.Wait()
	// Wait for in-flight tasks to finish or context timeout
	done := make(chan struct{})
	go func() {
		p.tasksWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-ctx.Done():
		p.logger.Warn("Timeout waiting for in-flight tasks to finish")
	}

	p.logger.Info("Task processor stopped")
	return nil
}

// eventProcessingLoop processes incoming task events with enhanced resilience
func (p *TaskProcessor) eventProcessingLoop(ctx context.Context) {
	defer p.wg.Done()
	p.logger.Info("Starting event processing loop (consumer-group mode)")

	if p.subscription == nil {
		p.logger.Error("No subscription available in processor")
		return
	}
	eventsCh := p.subscription.Events()
	errorsCh := p.subscription.Errors()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping event processing")
			return
		case <-p.stopCh:
			p.logger.Info("Stop signal received, stopping event processing")
			return
		case event := <-eventsCh:
			if event == nil {
				continue
			}

			// Overall concurrency control
			p.sem <- struct{}{}
			p.tasksWG.Add(1)
			go func(ev *cloudevents.CloudEvent) {
				defer func() {
					<-p.sem
					p.tasksWG.Done()
				}()

				// Process with retry and ack semantics
				var lastErr error
				maxRetries := 3
				for attempt := 1; attempt <= maxRetries; attempt++ {
					if err := p.processTaskEvent(ctx, ev); err != nil {
						lastErr = err
						p.logger.Error("Failed to process task event, will retry",
							zap.String("event_id", ev.ID),
							zap.String("event_type", ev.Type),
							zap.String("task_id", ev.TaskID),
							zap.Int("attempt", attempt),
							zap.Error(err))
						time.Sleep(time.Duration(attempt) * 200 * time.Millisecond) // simple backoff
						continue
					}

					// Success: acknowledge via subscription if possible
					ackID := ""
					if key, ok := ev.Extensions["ack_key"].(string); ok {
						ackID = key
					} else {
						if stream, ok1 := ev.Extensions["redis_stream"].(string); ok1 {
							if msgID, ok2 := ev.Extensions["redis_msg_id"].(string); ok2 {
								ackID = stream + "|" + msgID
							}
						}
					}
					if ackID == "" {
						p.logger.Error("CRITICAL: No ack key available; cannot acknowledge",
							zap.String("event_id", ev.ID),
							zap.Any("extensions", ev.Extensions))
					} else if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackID); err != nil {
							p.logger.Warn("Ack failed",
								zap.String("event_id", ev.ID),
								zap.String("ack_id", ackID),
								zap.Error(err))
						}
					}
					p.logger.Debug("Processed and acked task event",
						zap.String("event_id", ev.ID),
						zap.String("task_id", ev.TaskID))
					return
				}

				// After max retries: send to DLQ and log
				p.logger.Error("Task event failed after max retries; sending to DLQ",
					zap.String("event_id", ev.ID),
					zap.String("task_id", ev.TaskID),
					zap.Error(lastErr))

				// Publish to DLQ using same event with marker
				dlqEvent := ev.Clone()
				dlqEvent.Type = "io.flunq.task.dlq"
				dlqEvent.AddExtension("reason", fmt.Sprintf("max_retries_exceeded:%v", lastErr))
				_ = p.eventStream.Publish(ctx, dlqEvent)

				// After DLQ publish, acknowledge the original message to prevent it from staying pending
				if ackKey, ok := ev.Extensions["ack_key"].(string); ok {
					if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackKey); err != nil {
							p.logger.Warn("Ack after DLQ failed",
								zap.String("event_id", ev.ID),
								zap.Error(err))
						} else {
							p.logger.Info("Acked message after DLQ",
								zap.String("event_id", ev.ID))
						}
					}
				}
			}(event)
		case err := <-errorsCh:
			if err != nil {
				p.logger.Error("Event stream error", zap.Error(err))
			}
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

// Background goroutine to reclaim orphaned pending messages
func (p *TaskProcessor) reclaimOrphanedMessages(ctx context.Context, consumerGroup, consumerName string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			if p.eventStream != nil {
				if reclaimed, err := p.eventStream.ReclaimPending(ctx, consumerGroup, consumerName, 60*time.Second); err != nil {
					p.logger.Warn("Failed to reclaim pending messages", zap.Error(err))
				} else if reclaimed > 0 {
					p.logger.Info("Reclaimed pending messages", zap.Int("count", reclaimed))
				}
			}
		}
	}
}

// env helpers (scoped to processor package)
func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvIntDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDurationDefault(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getEnvBoolDefault(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if v == "1" || strings.ToLower(v) == "true" || strings.ToLower(v) == "yes" {
			return true
		}
		if v == "0" || strings.ToLower(v) == "false" || strings.ToLower(v) == "no" {
			return false
		}
	}
	return def
}
