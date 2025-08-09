package processor

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

	// Processing state
	eventCh <-chan *cloudevents.CloudEvent
	errorCh <-chan error
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
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
		stopCh:         make(chan struct{}),
		workflowLocks:  make(map[string]*sync.Mutex),
		maxConcurrency: maxConc,
		sem:            make(chan struct{}, maxConc),
	}
}

// Start starts the workflow processor
func (p *WorkflowProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting workflow processor")

	// Use shared event streaming for tenant-isolated streams
	filters := sharedinterfaces.StreamFilters{
		EventTypes: []string{
			"io.flunq.workflow.created",
			"io.flunq.execution.started",
			"io.flunq.task.completed",
		},
		WorkflowIDs:   []string{}, // Subscribe to all workflows
		ConsumerGroup: fmt.Sprintf("worker-service-%d", time.Now().Unix()),
		ConsumerName:  fmt.Sprintf("worker-%d", time.Now().Unix()),
	}

	subscription, err := p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	// Get channels from subscription
	p.eventCh = subscription.Events()
	p.errorCh = subscription.Errors()
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.eventProcessingLoop(ctx)

	p.logger.Info("Workflow processor started successfully")
	return nil
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

// eventProcessingLoop processes incoming workflow events from the new EventStore
func (p *WorkflowProcessor) eventProcessingLoop(ctx context.Context) {
	defer p.wg.Done()

	p.logger.Info("Starting event processing loop")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping event processing")
			return
		case <-p.stopCh:
			p.logger.Info("Stop signal received, stopping event processing")
			return
		case event := <-p.eventCh:
			p.logger.Debug("Received event from eventCh", "event_present", event != nil)
			if event != nil {
				// Acquire concurrency slot
				p.sem <- struct{}{}
				p.tasksWG.Add(1)
				go func(ev *cloudevents.CloudEvent) {
					defer func() {
						<-p.sem
						p.tasksWG.Done()
					}()
					p.logger.Debug("Processing event", "event_type", ev.Type, "event_id", ev.ID)
					if err := p.ProcessWorkflowEvent(ctx, ev); err != nil {
						// Extract workflow ID from event extensions
						workflowID := ""
						if wfID, ok := ev.Extensions["workflowid"].(string); ok {
							workflowID = wfID
						}
						p.logger.Error("Failed to process workflow event",
							"event_id", ev.ID,
							"event_type", ev.Type,
							"workflow_id", workflowID,
							"error", err)
						p.metrics.IncrementCounter("workflow_event_processing_errors", map[string]string{
							"event_type":  ev.Type,
							"workflow_id": workflowID,
						})
					}
				}(event)
			}
		case err := <-p.errorCh:
			if err != nil {
				p.logger.Error("Event store subscription error", "error", err)
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
		"workflow_id", event.WorkflowID)

	// Event data (debug)
	p.logger.Debug("Event data",
		"data_type", fmt.Sprintf("%T", event.Data),
		"data_value", event.Data)

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
		p.logger.Error("Failed to get workflow definition",
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
	state, err := p.rebuildCompleteWorkflowState(ctx, definition, events)
	if err != nil {
		return fmt.Errorf("failed to rebuild workflow state: %w", err)
	}

	// Step 4: Process New Event
	if err := p.processNewEvent(ctx, state, event); err != nil {
		return fmt.Errorf("failed to process new event: %w", err)
	}

	// Step 5: Execute Next SDL Step
	if err := p.executeNextSDLStep(ctx, state, definition); err != nil {
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

// getWorkflowDefinition gets the workflow definition from database
func (p *WorkflowProcessor) getWorkflowDefinition(ctx context.Context, workflowID string) (*gen.WorkflowDefinition, error) {
	definition, err := p.database.GetWorkflowDefinition(ctx, workflowID)
	if err != nil {
		p.logger.Error("Failed to get workflow definition",
			"workflow_id", workflowID,
			"error", err)
		return nil, err
	}

	return definition, nil
}

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

// executeTask executes a specific task based on its type
func (p *WorkflowProcessor) executeTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) error {
	p.logger.Info("Executing task",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name,
		"task_type", task.TaskType)

	// Handle wait tasks specially - execute locally and set workflow to waiting
	if task.TaskType == "wait" {
		return p.executeWaitTaskLocally(ctx, task, state)
	}

	// Send all other tasks to Executor Service for execution
	return p.publishTaskRequestedEvent(ctx, task, state)
}

// executeWaitTaskLocally executes wait tasks locally and sets workflow to waiting status
func (p *WorkflowProcessor) executeWaitTaskLocally(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) error {
	p.logger.Info("Executing wait task locally - workflow will pause",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name)

	// Execute the wait task using the workflow engine
	taskData, err := p.workflowEngine.ExecuteTask(ctx, task, state)
	if err != nil {
		p.logger.Error("Failed to execute wait task",
			"workflow_id", state.WorkflowId,
			"task_name", task.Name,
			"error", err)
		return fmt.Errorf("failed to execute wait task: %w", err)
	}

	// Set workflow status to waiting
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_WAITING

	// Keep the wait task in pending tasks - do NOT add to completed tasks until wait duration has elapsed
	// The task will remain in PendingTasks until the scheduled timer event completes it

	p.logger.Info("Wait task initiated - workflow is now waiting",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name,
		"workflow_status", "waiting")

	// Publish wait task initiated event
	return p.publishWaitTaskInitiatedEvent(ctx, task, taskData, state)
}

// executeLocalTask is deprecated - all tasks are now sent to executor service
// This function is kept for reference but should not be used
func (p *WorkflowProcessor) executeLocalTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) error {
	p.logger.Warn("executeLocalTask is deprecated - all tasks should go through executor service",
		"task_name", task.Name,
		"task_type", task.TaskType)

	// Redirect to executor service
	return p.executeTask(ctx, task, state)
}

// publishTaskRequestedEvent publishes a TaskRequested event for external execution
func (p *WorkflowProcessor) publishTaskRequestedEvent(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) error {
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

		// For wait tasks, extract duration
		if task.TaskType == "wait" {
			if duration, exists := inputMap["duration"]; exists {
				parameters["duration"] = duration
				p.logger.Debug("Adding duration to task parameters",
					"task_name", task.Name,
					"duration", duration)
			}
		}

		// For set tasks, extract set data
		if task.TaskType == "set" {
			if setData, exists := inputMap["set_data"]; exists {
				parameters["set_data"] = setData
			}
		}

		// Pass through other input data
		for key, value := range inputMap {
			if key != "duration" && key != "set_data" {
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
				"parameters": parameters, // Task-specific parameters including duration
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

// publishTaskCompletedEvent is deprecated - task.completed events are now published by executor service
// This function is kept for reference but should not be used
func (p *WorkflowProcessor) publishTaskCompletedEvent(ctx context.Context, taskID, taskName string, taskData *gen.TaskData, state *gen.WorkflowState, startTime time.Time, duration time.Duration) error {
	p.logger.Warn("publishTaskCompletedEvent is deprecated - task.completed events should come from executor service",
		"task_id", taskID,
		"task_name", taskName,
		"workflow_id", state.WorkflowId)

	// Do not publish - executor service handles this
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

// publishTaskScheduledEvent publishes a TaskScheduled event
func (p *WorkflowProcessor) publishTaskScheduledEvent(ctx context.Context, taskID string, task *gen.PendingTask, state *gen.WorkflowState) error {
	// Get tenant ID from state context
	tenantID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
	}

	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-scheduled-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.scheduled",
		Time:        time.Now(),
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: state.Context.ExecutionId,
		TaskID:      taskID,
		Data: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    task.Name,
			"task_type":    task.TaskType,
			"workflow_id":  state.WorkflowId,
			"execution_id": state.Context.ExecutionId,
			"scheduled_at": time.Now().Format(time.RFC3339),
			"scheduled_by": "worker-service",
		},
	}

	// Publish event using shared event streaming (auto-routes to tenant-isolated streams)
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to store task scheduled event: %w", err)
	}

	p.logger.Info("Task scheduled",
		"task_id", taskID,
		"task_name", task.Name,
		"workflow_id", state.WorkflowId)

	return nil
}

// publishTaskStartedEvent publishes a TaskStarted event
func (p *WorkflowProcessor) publishTaskStartedEvent(ctx context.Context, taskID string, task *gen.PendingTask, state *gen.WorkflowState) error {
	// Get tenant ID from state context
	tenantID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
	}

	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-started-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.started",
		Time:        time.Now(),
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: state.Context.ExecutionId,
		TaskID:      taskID,
		Data: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    task.Name,
			"task_type":    task.TaskType,
			"workflow_id":  state.WorkflowId,
			"execution_id": state.Context.ExecutionId,
			"started_at":   time.Now().Format(time.RFC3339),
			"worker_id":    "worker-service",
		},
	}

	// Publish event using shared event streaming (auto-routes to tenant-isolated streams)
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to store task started event: %w", err)
	}

	p.logger.Info("Task started",
		"task_id", taskID,
		"task_name", task.Name,
		"workflow_id", state.WorkflowId)

	return nil
}

// publishTaskFailedEvent publishes a TaskFailed event
func (p *WorkflowProcessor) publishTaskFailedEvent(ctx context.Context, taskID string, task *gen.PendingTask, state *gen.WorkflowState, taskErr error, startTime time.Time) error {
	// Get tenant ID from state context
	tenantID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
	}

	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-failed-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.failed",
		Time:        time.Now(),
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: state.Context.ExecutionId,
		TaskID:      taskID,
		Data: map[string]interface{}{
			"task_id":       taskID,
			"task_name":     task.Name,
			"task_type":     task.TaskType,
			"workflow_id":   state.WorkflowId,
			"execution_id":  state.Context.ExecutionId,
			"started_at":    startTime.Format(time.RFC3339),
			"failed_at":     time.Now().Format(time.RFC3339),
			"error_message": taskErr.Error(),
			"worker_id":     "worker-service",
		},
	}

	// Publish event using shared event streaming (auto-routes to tenant-isolated streams)
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to store task failed event: %w", err)
	}

	p.logger.Error("Task failed",
		"task_id", taskID,
		"task_name", task.Name,
		"workflow_id", state.WorkflowId,
		"error", taskErr)

	return nil
}

// publishWaitTaskInitiatedEvent publishes an event indicating that a wait task has been initiated
func (p *WorkflowProcessor) publishWaitTaskInitiatedEvent(ctx context.Context, task *gen.PendingTask, taskData *gen.TaskData, state *gen.WorkflowState) error {
	// Get tenant ID from state context
	tenantID := ""
	executionID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
		executionID = state.Context.ExecutionId
	}

	// Create task ID
	taskID := fmt.Sprintf("task-%s", task.Name)

	// Create CloudEvent for wait task initiated
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("wait-initiated-%s-%d", task.Name, time.Now().Unix()),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.completed", // Use completed type since wait is "completed" (initiated)
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		TaskID:      taskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_name": task.Name,
			"task_type": "wait",
			"data": map[string]interface{}{
				"input":  taskData.Input.AsMap(),
				"output": taskData.Output.AsMap(),
				"metadata": map[string]interface{}{
					"started_at":   taskData.Metadata.StartedAt.AsTime().Format(time.RFC3339),
					"completed_at": nil, // Will be set when wait actually completes
					"duration_ms":  taskData.Metadata.DurationMs,
					"task_type":    taskData.Metadata.TaskType,
					"status":       "waiting", // Special status for wait tasks
				},
			},
		},
	}

	// Publish the event
	if err := p.eventStream.Publish(ctx, event); err != nil {
		p.logger.Error("Failed to publish wait task initiated event",
			"workflow_id", state.WorkflowId,
			"task_name", task.Name,
			"event_id", event.ID,
			"error", err)
		return fmt.Errorf("failed to publish wait task initiated event: %w", err)
	}

	p.logger.Info("Published wait task initiated event",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name,
		"event_id", event.ID,
		"event_type", event.Type)

	return nil
}
