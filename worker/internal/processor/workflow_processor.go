package processor

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// WorkflowProcessor implements the core event processing pattern for the Worker service
type WorkflowProcessor struct {
	eventStream    sharedinterfaces.EventStream // Shared event streaming for both subscribing and publishing
	database       interfaces.Database
	sharedDatabase sharedinterfaces.Database // Direct access to shared database for execution updates
	workflowEngine interfaces.WorkflowEngine
	serializer     interfaces.ProtobufSerializer
	logger         interfaces.Logger
	metrics        interfaces.Metrics

	// Wait scheduling abstraction
	waitScheduler WaitScheduler

	// Processing state
	subscription sharedinterfaces.StreamSubscription
	errorCh      <-chan error
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	// Per-event workers waitgroup (for graceful shutdown when concurrency > 1)
	tasksWG sync.WaitGroup
	// Concurrency control
	maxConcurrency int
	sem            chan struct{}

	// Workflow processing locks (one per workflow to prevent concurrent processing)
	workflowLocks map[string]*sync.Mutex
	locksMutex    sync.RWMutex
}

// NewWorkflowProcessor creates a new workflow processor
func NewWorkflowProcessor(
	eventStream sharedinterfaces.EventStream, // Shared event streaming for both subscribing and publishing
	database interfaces.Database,
	sharedDatabase sharedinterfaces.Database, // Direct access to shared database for execution updates
	workflowEngine interfaces.WorkflowEngine,
	serializer interfaces.ProtobufSerializer,
	logger interfaces.Logger,
	metrics interfaces.Metrics,
) interfaces.WorkflowProcessor {
	// Determine concurrency from env (default 4)
	maxConc := 4
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConc = n
		}
	}
	return &WorkflowProcessor{
		eventStream:    eventStream,
		database:       database,
		sharedDatabase: sharedDatabase,
		workflowEngine: workflowEngine,
		serializer:     serializer,
		logger:         logger,
		metrics:        metrics,
		waitScheduler:  NewTimerWaitScheduler(eventStream),
		stopCh:         make(chan struct{}),
		workflowLocks:  make(map[string]*sync.Mutex),
		maxConcurrency: maxConc,
		sem:            make(chan struct{}, maxConc),
	}
}

// Start starts the workflow processor
func (p *WorkflowProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting workflow processor")

	// Health check Redis / stream connectivity
	if info, err := p.eventStream.GetStreamInfo(ctx); err != nil {
		p.logger.Warn("Stream info not available yet", "error", err)
	} else {
		p.logger.Info("Stream info",
			"stream", info.StreamName,
			"message_count", info.MessageCount,
			"consumer_groups", info.ConsumerGroups)
	}

	// Use shared event streaming for tenant-isolated streams
	filters := sharedinterfaces.StreamFilters{
		EventTypes: []string{
			"io.flunq.workflow.created",
			"io.flunq.execution.started",
			"io.flunq.task.completed",
			"io.flunq.timer.fired", // Resume waits
		},
		WorkflowIDs:   []string{}, // Subscribe to all workflows
		ConsumerGroup: getEnvDefault("WORKER_CONSUMER_GROUP", "worker-service"),
		ConsumerName:  fmt.Sprintf("worker-%s", uuid.New().String()[:8]),
		BatchCount:    getEnvIntDefault("WORKER_STREAM_BATCH", 10),
		BlockTimeout:  getEnvDurationDefault("WORKER_STREAM_BLOCK", time.Second),
	}

	// Create subscription once and keep reference for acks
	var err error
	p.subscription, err = p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	// Get channels from subscription (for errors)
	p.errorCh = p.subscription.Errors()
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.eventProcessingLoop(ctx)

	p.logger.Info("Workflow processor started successfully")

	// Optionally start background reclaim loop for orphaned pending messages
	if getEnvBoolDefault("WORKER_RECLAIM_ENABLED", false) {
		go p.reclaimOrphanedMessages(ctx, filters.ConsumerGroup, filters.ConsumerName)
	}
	return nil
}

// Background goroutine to reclaim orphaned pending messages
func (p *WorkflowProcessor) reclaimOrphanedMessages(ctx context.Context, consumerGroup, consumerName string) {
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
					p.logger.Warn("Failed to reclaim pending messages", "error", err)
				} else if reclaimed > 0 {
					p.logger.Info("Reclaimed pending messages", "count", reclaimed)
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

// Stop stops the workflow processor
func (p *WorkflowProcessor) Stop(ctx context.Context) error {
	p.logger.Info("Stopping workflow processor")

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

	p.logger.Info("Workflow processor stopped")
	return nil
}

// eventProcessingLoop processes incoming workflow events using consumer-group semantics
func (p *WorkflowProcessor) eventProcessingLoop(ctx context.Context) {
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

				// Per-workflow locking is handled inside ProcessWorkflowEvent; avoid double-locking here
				workflowID := ev.WorkflowID

				// Process with retry and ack semantics
				var lastErr error
				maxRetries := 3
				for attempt := 1; attempt <= maxRetries; attempt++ {
					if err := p.ProcessWorkflowEvent(ctx, ev); err != nil {
						lastErr = err
						p.logger.Error("Failed to process event, will retry",
							"event_id", ev.ID,
							"event_type", ev.Type,
							"workflow_id", workflowID,
							"attempt", attempt,
							"error", err)
						p.metrics.IncrementCounter("workflow_event_processing_errors", map[string]string{
							"event_type":  ev.Type,
							"workflow_id": workflowID,
							"attempt":     fmt.Sprintf("%d", attempt),
						})
						time.Sleep(time.Duration(attempt) * 200 * time.Millisecond) // simple backoff
						continue
					}

					// Success: acknowledge via subscription if possible (prefer ack_key; fallback to stream|msgID)
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
						p.logger.Error("CRITICAL: No ack key available; cannot acknowledge", "event_id", ev.ID, "extensions", ev.Extensions)
					} else if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackID); err != nil {
							p.logger.Warn("Ack failed", "event_id", ev.ID, "ack_id", ackID, "error", err)
						}
					}
					p.logger.Debug("Processed and acked event", "event_id", ev.ID, "workflow_id", workflowID)
					return
				}

				// After max retries: send to DLQ and log
				p.logger.Error("Event failed after max retries; sending to DLQ",

					"event_id", ev.ID,
					"workflow_id", workflowID,
					"error", lastErr)
				p.metrics.IncrementCounter("workflow_event_dlq", map[string]string{"event_type": ev.Type})

				// Publish to DLQ using same event with marker
				dlqEvent := ev.Clone()
				dlqEvent.Type = "io.flunq.event.dlq"
				dlqEvent.AddExtension("reason", fmt.Sprintf("max_retries_exceeded:%v", lastErr))
				_ = p.eventStream.Publish(ctx, dlqEvent)

				// After DLQ publish, acknowledge the original message to prevent it from staying pending
				if ackKey, ok := ev.Extensions["ack_key"].(string); ok {
					if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackKey); err != nil {
							p.logger.Warn("Ack after DLQ failed", "event_id", ev.ID, "error", err)
						} else {
							p.logger.Info("Acked message after DLQ", "event_id", ev.ID)
						}
					}
				}
			}(event)
		case err := <-errorsCh:
			if err != nil {
				p.logger.Error("Event stream error", "error", err)
				p.metrics.IncrementCounter("workflow_subscription_errors", nil)
			}
		}
	}
}

// ProcessWorkflowEvent processes any workflow-related event following the exact sequence
func (p *WorkflowProcessor) ProcessWorkflowEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	timer := p.metrics.StartTimer("workflow_event_processing_duration", map[string]string{
		"event_type":  event.Type,
		"workflow_id": event.WorkflowID,
	})
	defer timer()

	p.logger.Info("Processing workflow event",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", event.WorkflowID,
		"execution_id", event.ExecutionID,
		"tenant_id", event.TenantID)

	// Event data (debug)
	p.logger.Info("Event data debug",
		"data_type", fmt.Sprintf("%T", event.Data),
		"data_value", event.Data,
		"extensions", event.Extensions)

	// Get workflow-specific lock to prevent concurrent processing of same workflow
	workflowLock := p.getWorkflowLock(event.WorkflowID)
	workflowLock.Lock()
	defer workflowLock.Unlock()

	// Skip workflow.created events - the API service already handles workflow creation
	if event.Type == "io.flunq.workflow.created" {
		p.logger.Debug("Skipping workflow.created event - handled by API service",
			"event_id", event.ID,
			"workflow_id", event.WorkflowID)
		return nil
	}

	// Skip events published by this worker to prevent infinite loops
	if event.Source == "worker-service" && event.Type == "io.flunq.task.completed" {
		p.logger.Debug("Skipping self-published task.completed event",
			"event_id", event.ID,
			"workflow_id", event.WorkflowID)
		return nil
	}

	// Only process task.completed events from executor-service or workflow-engine (for wait tasks)
	if event.Type == "io.flunq.task.completed" && event.Source != "executor-service" && event.Source != "workflow-engine" {
		p.logger.Debug("Skipping task.completed event from non-executor/non-engine source",
			"event_id", event.ID,
			"source", event.Source,
			"workflow_id", event.WorkflowID)
		return nil
	}

	// Step 1: Fetch Event History for Current Execution
	// Use execution-specific filtering to avoid interference between different executions
	var events []*cloudevents.CloudEvent
	var err error

	if event.ExecutionID != "" {
		// Filter events by execution ID to isolate this execution's state
		events, err = p.fetchEventHistoryForExecution(ctx, event.WorkflowID, event.ExecutionID)
		if err != nil {
			return fmt.Errorf("failed to fetch execution-specific event history: %w", err)
		}

		p.logger.Info("Using execution-specific event filtering",
			"workflow_id", event.WorkflowID,
			"execution_id", event.ExecutionID,
			"filtered_events", len(events))
	} else {
		// Fallback to all events if no execution ID (shouldn't happen in normal flow)
		p.logger.Warn("No execution ID in event, using all workflow events",
			"event_id", event.ID,
			"event_type", event.Type,
			"workflow_id", event.WorkflowID)

		events, err = p.fetchCompleteEventHistory(ctx, event.WorkflowID)
		if err != nil {
			return fmt.Errorf("failed to fetch complete event history: %w", err)
		}
	}

	// Step 2: Get Workflow Definition (with tenant context from event)
	p.logger.Info("Getting workflow definition with tenant context",
		"workflow_id", event.WorkflowID,
		"tenant_id", event.TenantID,
		"event_type", event.Type)

	definition, err := p.getWorkflowDefinitionWithTenant(ctx, event.TenantID, event.WorkflowID)
	if err != nil {
		p.logger.Error("CRITICAL: Failed to get workflow definition",
			"workflow_id", event.WorkflowID,
			"tenant_id", event.TenantID,
			"error", err)
		return fmt.Errorf("failed to get workflow definition: %w", err)
	}

	p.logger.Info("Successfully retrieved workflow definition",
		"workflow_id", event.WorkflowID,
		"tenant_id", event.TenantID,
		"definition_name", definition.Name,
		"definition_id", definition.Id)

	// Step 3: Rebuild Complete Workflow State
	p.logger.Info("Rebuilding complete workflow state", "workflow_id", event.WorkflowID, "event_count", len(events))
	state, err := p.rebuildCompleteWorkflowState(ctx, definition, events)
	if err != nil {
		p.logger.Error("CRITICAL: Failed to rebuild workflow state", "workflow_id", event.WorkflowID, "error", err)
		return fmt.Errorf("failed to rebuild workflow state: %w", err)
	}
	p.logger.Info("Successfully rebuilt workflow state", "workflow_id", event.WorkflowID, "status", state.Status)

	// Step 4: Process New Event
	p.logger.Info("Processing new event", "workflow_id", event.WorkflowID, "event_type", event.Type)
	if err := p.processNewEvent(ctx, state, event); err != nil {
		p.logger.Error("CRITICAL: Failed to process new event", "workflow_id", event.WorkflowID, "event_type", event.Type, "error", err)
		return fmt.Errorf("failed to process new event: %w", err)
	}

	// Step 5: Execute Next SDL Step
	p.logger.Info("Executing next SDL step", "workflow_id", event.WorkflowID)
	if err := p.executeNextSDLStep(ctx, state, definition); err != nil {
		p.logger.Error("CRITICAL: Failed to execute next SDL step", "workflow_id", event.WorkflowID, "error", err)
		return fmt.Errorf("failed to execute next SDL step: %w", err)
	}

	// Step 6: Update Workflow Record in Database
	if err := p.updateWorkflowRecord(ctx, state); err != nil {
		return fmt.Errorf("failed to update workflow record: %w", err)
	}

	p.logger.Info("Successfully processed workflow event",
		"event_id", event.ID,
		"workflow_id", event.WorkflowID,
		"current_step", state.CurrentStep,
		"status", state.Status)

	p.metrics.IncrementCounter("workflow_events_processed", map[string]string{
		"event_type":  event.Type,
		"workflow_id": event.WorkflowID,
	})

	return nil
}

// getWorkflowLock gets or creates a workflow-specific lock
func (p *WorkflowProcessor) getWorkflowLock(workflowID string) *sync.Mutex {
	p.locksMutex.RLock()
	if lock, exists := p.workflowLocks[workflowID]; exists {
		p.locksMutex.RUnlock()
		return lock
	}
	p.locksMutex.RUnlock()

	p.locksMutex.Lock()
	defer p.locksMutex.Unlock()

	// Double-check after acquiring write lock
	if lock, exists := p.workflowLocks[workflowID]; exists {
		return lock
	}

	// Create new lock
	lock := &sync.Mutex{}
	p.workflowLocks[workflowID] = lock
	return lock
}

// fetchCompleteEventHistory fetches all events for a workflow from the beginning
func (p *WorkflowProcessor) fetchCompleteEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	p.logger.Debug("Fetching complete event history", "workflow_id", workflowID)

	// Use the shared event streaming to get event history for this workflow
	// This will automatically discover and read from the correct tenant-specific streams
	events, err := p.eventStream.GetEventHistory(ctx, workflowID)
	if err != nil {
		p.logger.Error("Failed to fetch event history from shared event streaming",
			"workflow_id", workflowID,
			"error", err)
		return nil, fmt.Errorf("failed to fetch event history: %w", err)
	}

	p.logger.Info("Fetched complete event history",
		"workflow_id", workflowID,
		"filtered_events", len(events))

	return events, nil
}

// fetchEventHistoryForExecution fetches event history filtered by execution ID
func (p *WorkflowProcessor) fetchEventHistoryForExecution(ctx context.Context, workflowID, executionID string) ([]*cloudevents.CloudEvent, error) {
	p.logger.Debug("Fetching event history for execution",
		"workflow_id", workflowID,
		"execution_id", executionID)

	// Get all events for the workflow
	allEvents, err := p.eventStream.GetEventHistory(ctx, workflowID)
	if err != nil {
		p.logger.Error("Failed to fetch event history from shared event streaming",
			"workflow_id", workflowID,
			"execution_id", executionID,
			"error", err)
		return nil, fmt.Errorf("failed to fetch event history: %w", err)
	}

	// Filter events by execution ID (strict filtering - only include events with matching execution ID)
	var filteredEvents []*cloudevents.CloudEvent
	for _, event := range allEvents {
		// Only include events that have the exact execution ID match
		// Exclude events with empty execution IDs to prevent cross-execution contamination
		if event.ExecutionID != "" && event.ExecutionID == executionID {
			filteredEvents = append(filteredEvents, event)
			p.logger.Info("Including event in execution filter",
				"event_id", event.ID,
				"event_type", event.Type,
				"execution_id", event.ExecutionID)
		} else if event.ExecutionID == "" {
			p.logger.Info("Excluding event with empty execution ID",
				"event_id", event.ID,
				"event_type", event.Type)
		} else {
			p.logger.Info("Excluding event with different execution ID",
				"event_id", event.ID,
				"event_type", event.Type,
				"event_execution_id", event.ExecutionID,
				"target_execution_id", executionID)
		}
	}

	p.logger.Info("Fetched and filtered event history for execution",
		"workflow_id", workflowID,
		"execution_id", executionID,
		"total_events", len(allEvents),
		"filtered_events", len(filteredEvents))

	return filteredEvents, nil
}

// NOTE: handleWorkflowCreated method removed - workflow creation is handled by API service only

// getWorkflowDefinitionWithTenant gets the workflow definition with tenant context
func (p *WorkflowProcessor) getWorkflowDefinitionWithTenant(ctx context.Context, tenantID, workflowID string) (*gen.WorkflowDefinition, error) {
	// Use the tenant-aware method for efficient lookup
	definition, err := p.database.GetWorkflowDefinitionWithTenant(ctx, tenantID, workflowID)
	if err != nil {
		p.logger.Error("Failed to get workflow definition with tenant",
			"workflow_id", workflowID,
			"tenant_id", tenantID,
			"error", err)
		return nil, err
	}

	p.logger.Info("Retrieved workflow definition with tenant context",
		"workflow_id", workflowID,
		"tenant_id", tenantID,
		"workflow_name", definition.Name)
	return definition, nil
}

// rebuildCompleteWorkflowState rebuilds the complete workflow state from event history
func (p *WorkflowProcessor) rebuildCompleteWorkflowState(ctx context.Context, definition *gen.WorkflowDefinition, events []*cloudevents.CloudEvent) (*gen.WorkflowState, error) {
	p.logger.Debug("Rebuilding workflow state from events",
		"workflow_id", definition.Id,
		"event_count", len(events))

	// Use the workflow engine to rebuild state using SDK
	state, err := p.workflowEngine.RebuildState(ctx, definition, events)
	if err != nil {
		p.logger.Error("Failed to rebuild workflow state",
			"workflow_id", definition.Id,
			"error", err)
		return nil, err
	}

	p.logger.Debug("Successfully rebuilt workflow state",
		"workflow_id", definition.Id,
		"current_step", state.CurrentStep,
		"status", state.Status,
		"completed_tasks", len(state.CompletedTasks),
		"pending_tasks", len(state.PendingTasks))

	return state, nil
}

// processNewEvent applies the newest event to the rebuilt state
func (p *WorkflowProcessor) processNewEvent(ctx context.Context, state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	p.logger.Debug("Processing new event",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", state.WorkflowId)

	// Use the workflow engine to process the event
	if err := p.workflowEngine.ProcessEvent(ctx, state, event); err != nil {
		p.logger.Error("Failed to process event",
			"event_id", event.ID,
			"workflow_id", state.WorkflowId,
			"error", err)
		return err
	}

	// Validate state consistency using SDK
	// TODO: Add state validation

	return nil
}

// publishTaskRequestedEvent publishes a TaskRequested event for external execution
func (p *WorkflowProcessor) publishTaskRequestedEvent(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) error {
	// Special-case: wait tasks should be scheduled via timer service, not sent to executor
	if task.TaskType == "wait" {
		executionID := ""
		tenantID := ""
		if state.Context != nil {
			executionID = state.Context.ExecutionId
			tenantID = state.Context.TenantId
		}
		if task.Input == nil {
			return fmt.Errorf("wait task missing input")
		}
		inputMap := task.Input.AsMap()
		durationStr, ok := inputMap["duration"].(string)
		if !ok || durationStr == "" {
			return fmt.Errorf("wait task missing duration")
		}
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid wait duration: %w", err)
		}
		// Delegate scheduling to the WaitScheduler abstraction
		if err := p.waitScheduler.Schedule(ctx, tenantID, state.WorkflowId, executionID, task.Name, duration); err != nil {
			return fmt.Errorf("failed to schedule wait via timer service: %w", err)
		}
		p.logger.Info("Scheduled wait via timer service",
			"workflow_id", state.WorkflowId,
			"task_name", task.Name,
			"duration", duration)
		return nil
	}

	// Default path: publish io.flunq.task.requested for executor
	taskID := fmt.Sprintf("task-%s-%d", task.Name, time.Now().UnixNano())

	// Get execution ID and tenant ID safely
	executionID := ""
	tenantID := ""
	if state.Context != nil {
		executionID = state.Context.ExecutionId
		tenantID = state.Context.TenantId
	}

	p.logger.Debug("Publishing task requested event",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name,
		"execution_id", executionID,
		"tenant_id", tenantID,
		"context_exists", state.Context != nil)

	// Extract task-specific parameters from input
	parameters := make(map[string]interface{})
	inputData := make(map[string]interface{})

	if task.Input != nil {
		inputMap := task.Input.AsMap()

		// For set tasks, extract set data
		if task.TaskType == "set" {
			if setData, exists := inputMap["set_data"]; exists {
				parameters["set_data"] = setData
			}
		}

		// Pass through other input data
		for key, value := range inputMap {
			if key != "set_data" {
				inputData[key] = value
			}
		}
	}

	// Create CloudEvent with JSON data (compatible with Executor Service)
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-requested-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.requested",
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		TaskID:      taskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    task.Name,
			"task_type":    task.TaskType,
			"tenant_id":    tenantID,
			"workflow_id":  state.WorkflowId,
			"execution_id": executionID,
			"input":        inputData,
			"context":      map[string]interface{}{}, // Could include workflow context
			"config": map[string]interface{}{
				"timeout":    300, // 5 minutes
				"retries":    3,
				"parameters": parameters,
			},
		},
	}

	// Publish using the shared event streaming (will auto-route to correct tenant stream)
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish task requested event: %w", err)
	}

	p.logger.Info("Published task requested event",
		"task_id", taskID,
		"task_name", task.Name,
		"workflow_id", state.WorkflowId,
		"event_id", event.ID)

	return nil
}

// publishWorkflowCompletedEvent publishes a workflow completed event
func (p *WorkflowProcessor) publishWorkflowCompletedEvent(ctx context.Context, state *gen.WorkflowState) error {
	// Create workflow completed event data
	eventData := map[string]interface{}{
		"workflow_id":  state.WorkflowId,
		"status":       "completed",
		"output":       state.Output.AsMap(),
		"completed_at": time.Now().Format(time.RFC3339),
	}

	// Get execution ID safely
	executionID := ""
	if state.Context != nil {
		executionID = state.Context.ExecutionId
	}

	// Get tenant ID from state context
	tenantID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
	}

	// Create CloudEvent
	event := &cloudevents.CloudEvent{
		ID:          uuid.New().String(),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.workflow.completed",
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		Time:        time.Now(),
		Data:        eventData,
	}

	// Publish using the shared event streaming (will auto-route to correct tenant stream)
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish workflow completed event: %w", err)
	}

	// Update state status
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
	state.UpdatedAt = timestamppb.Now()

	// Update execution status in database
	if err := p.updateExecutionStatus(ctx, executionID, tenantID, state); err != nil {
		p.logger.Error("Failed to update execution status",
			"execution_id", executionID,
			"workflow_id", state.WorkflowId,
			"error", err)
		// Don't fail the workflow completion, just log the error
	}

	p.logger.Info("Published workflow completed event",
		"workflow_id", state.WorkflowId,
		"event_id", event.ID)

	return nil
}

// updateExecutionStatus updates the execution status when workflow completes
func (p *WorkflowProcessor) updateExecutionStatus(ctx context.Context, executionID, tenantID string, state *gen.WorkflowState) error {
	if executionID == "" {
		return fmt.Errorf("execution ID is empty")
	}

	// If shared database is not configured (e.g., in certain unit tests), skip update
	if p.sharedDatabase == nil {
		p.logger.Warn("Shared database not configured; skipping execution status update",
			"execution_id", executionID,
			"workflow_id", state.GetWorkflowId(),
		)
		return nil
	}

	// Get current execution from database
	execution, err := p.sharedDatabase.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution %s: %w", executionID, err)
	}

	// Update execution status and completion time
	now := time.Now()

	// Backfill missing StartedAt if zero (use state.CreatedAt or now)
	if execution.StartedAt.IsZero() {
		if ts := state.GetCreatedAt(); ts != nil {
			execution.StartedAt = ts.AsTime()
		} else {
			execution.StartedAt = now
		}
	}

	execution.Status = sharedinterfaces.WorkflowStatusCompleted
	execution.CompletedAt = &now
	execution.Duration = now.Sub(execution.StartedAt)

	// Set output if available
	if state.Output != nil {
		execution.Output = state.Output.AsMap()
	}

	// Update execution in database
	if err := p.sharedDatabase.UpdateExecution(ctx, executionID, execution); err != nil {
		return fmt.Errorf("failed to update execution %s: %w", executionID, err)
	}

	p.logger.Info("Updated execution status to completed",
		"execution_id", executionID,
		"workflow_id", state.WorkflowId,
		"duration", execution.Duration)

	return nil
}

// updateWorkflowRecord updates the workflow record in the database
func (p *WorkflowProcessor) updateWorkflowRecord(ctx context.Context, state *gen.WorkflowState) error {
	p.logger.Debug("Updating workflow record",
		"workflow_id", state.WorkflowId,
		"status", state.Status,
		"current_step", state.CurrentStep)

	// Update workflow state in database
	if err := p.database.UpdateWorkflowState(ctx, state.WorkflowId, state); err != nil {
		p.logger.Error("Failed to update workflow state",
			"workflow_id", state.WorkflowId,
			"error", err)
		return err
	}

	p.logger.Debug("Successfully updated workflow record",
		"workflow_id", state.WorkflowId)

	return nil
}

// executeNextSDLStep executes the next step in the Serverless Workflow DSL
func (p *WorkflowProcessor) executeNextSDLStep(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) error {
	p.logger.Debug("Executing next SDL step",
		"workflow_id", state.WorkflowId,
		"current_step", state.CurrentStep,
		"status", state.Status)

	// For DSL 1.0.0 workflows, skip status-based completion check
	// Let GetNextTask() determine completion instead
	dslMap := definition.DslDefinition.AsMap()
	if _, hasDoSection := dslMap["do"]; !hasDoSection {
		// Legacy DSL 0.8 format - use status-based completion
		if p.workflowEngine.IsWorkflowComplete(ctx, state, definition) {
			p.logger.Info("Workflow is complete",
				"workflow_id", state.WorkflowId,
				"status", state.Status)

			// Publish workflow completed event if not already completed
			if state.Status != gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED {
				if err := p.publishWorkflowCompletedEvent(ctx, state); err != nil {
					return fmt.Errorf("failed to publish workflow completed event: %w", err)
				}
			}
			return nil
		}
	}

	// Get next task to execute using SDK (following Java pattern - execute ONE task per event)
	nextTask, err := p.workflowEngine.GetNextTask(ctx, state, definition)
	if err != nil {
		p.logger.Error("Failed to get next task",
			"workflow_id", state.WorkflowId,
			"current_step", state.CurrentStep,
			"error", err)
		return err
	}

	if nextTask == nil {
		// Guard against premature completion for DSL 1.0.0 (task-based)
		dslMap := definition.DslDefinition.AsMap()
		if doSection, ok := dslMap["do"].([]interface{}); ok {
			totalTasks := len(doSection)
			completed := len(state.CompletedTasks)
			pending := len(state.PendingTasks)

			if pending > 0 || completed < totalTasks {
				p.logger.Info("No new task to request yet; waiting for pending/completions",
					"workflow_id", state.WorkflowId,
					"completed_tasks", completed,
					"total_tasks", totalTasks,
					"pending_tasks", pending)
				return nil // Do not mark workflow completed yet
			}
		}

		p.logger.Info("No more tasks to execute - workflow completed",
			"workflow_id", state.WorkflowId,
			"current_step", state.CurrentStep)

		// Publish workflow completed event
		return p.publishWorkflowCompletedEvent(ctx, state)
	}

	p.logger.Info("Requesting task execution",
		"workflow_id", state.WorkflowId,
		"task_name", nextTask.Name,
		"task_type", nextTask.TaskType)

	// Publish task.requested event for Executor Service to handle
	if err := p.publishTaskRequestedEvent(ctx, nextTask, state); err != nil {
		return fmt.Errorf("failed to publish task requested event: %w", err)
	}

	p.logger.Info("Task execution requested",
		"workflow_id", state.WorkflowId,
		"task_name", nextTask.Name,
		"task_type", nextTask.TaskType)

	return nil
}
