package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// WorkflowProcessor implements the core event processing pattern for the Worker service
type WorkflowProcessor struct {
	eventStore     interfaces.EventStore
	eventStream    interfaces.EventStream
	database       interfaces.Database
	workflowEngine interfaces.WorkflowEngine
	serializer     interfaces.ProtobufSerializer
	logger         interfaces.Logger
	metrics        interfaces.Metrics

	// Processing state
	subscription interfaces.EventStreamSubscription
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup

	// Workflow processing locks (one per workflow to prevent concurrent processing)
	workflowLocks map[string]*sync.Mutex
	locksMutex    sync.RWMutex
}

// NewWorkflowProcessor creates a new workflow processor
func NewWorkflowProcessor(
	eventStore interfaces.EventStore,
	eventStream interfaces.EventStream,
	database interfaces.Database,
	workflowEngine interfaces.WorkflowEngine,
	serializer interfaces.ProtobufSerializer,
	logger interfaces.Logger,
	metrics interfaces.Metrics,
) interfaces.WorkflowProcessor {
	return &WorkflowProcessor{
		eventStore:     eventStore,
		eventStream:    eventStream,
		database:       database,
		workflowEngine: workflowEngine,
		serializer:     serializer,
		logger:         logger,
		metrics:        metrics,
		stopCh:         make(chan struct{}),
		workflowLocks:  make(map[string]*sync.Mutex),
	}
}

// Start starts the workflow processor
func (p *WorkflowProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting workflow processor")

	// Subscribe to events via EventStream
	filters := interfaces.EventStreamFilters{
		EventTypes: []string{
			"io.flunq.workflow.created",
			"io.flunq.execution.started",
			"io.flunq.task.completed", // Re-added with smart filtering
		},
		WorkflowIDs: []string{}, // Subscribe to all workflows
	}

	subscription, err := p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	p.subscription = subscription
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.processEventsFromStream(ctx)

	p.logger.Info("Workflow processor started successfully")
	return nil
}

// Stop stops the workflow processor
func (p *WorkflowProcessor) Stop(ctx context.Context) error {
	p.logger.Info("Stopping workflow processor")

	p.running = false
	close(p.stopCh)

	// Close subscription
	if p.subscription != nil {
		p.subscription.Close()
	}

	// Wait for processing to complete
	p.wg.Wait()

	p.logger.Info("Workflow processor stopped")
	return nil
}

// processEvents processes incoming workflow events
func (p *WorkflowProcessor) processEvents(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		case event := <-p.subscription.Events():
			if event != nil {
				if err := p.ProcessWorkflowEvent(ctx, event); err != nil {
					p.logger.Error("Failed to process workflow event",
						"event_id", event.ID,
						"event_type", event.Type,
						"workflow_id", event.WorkflowID,
						"error", err)
					p.metrics.IncrementCounter("workflow_event_processing_errors", map[string]string{
						"event_type":  event.Type,
						"workflow_id": event.WorkflowID,
					})
				}
			}
		case err := <-p.subscription.Errors():
			if err != nil {
				p.logger.Error("Event subscription error", "error", err)
				p.metrics.IncrementCounter("workflow_subscription_errors", nil)
			}
		}
	}
}

// processEventsFromStream processes events from the EventStream
func (p *WorkflowProcessor) processEventsFromStream(ctx context.Context) {
	defer p.wg.Done()

	p.logger.Info("Starting event stream processing")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping event processing")
			return
		case <-p.stopCh:
			p.logger.Info("Stop signal received, stopping event processing")
			return
		case event := <-p.subscription.Events():
			if event != nil {
				if err := p.ProcessWorkflowEvent(ctx, event); err != nil {
					p.logger.Error("Failed to process workflow event",
						"event_id", event.ID,
						"event_type", event.Type,
						"workflow_id", event.WorkflowID,
						"error", err)
					p.metrics.IncrementCounter("workflow_event_processing_errors", map[string]string{
						"event_type":  event.Type,
						"workflow_id": event.WorkflowID,
					})
				}
			}
		case err := <-p.subscription.Errors():
			if err != nil {
				p.logger.Error("Event stream subscription error", "error", err)
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

	// DEBUG: Print event data structure
	p.logger.Info("DEBUG: Event data",
		"data_type", fmt.Sprintf("%T", event.Data),
		"data_value", event.Data)

	// Get workflow-specific lock to prevent concurrent processing of same workflow
	workflowLock := p.getWorkflowLock(event.WorkflowID)
	workflowLock.Lock()
	defer workflowLock.Unlock()

	// Handle workflow.created events differently - create the workflow definition
	if event.Type == "io.flunq.workflow.created" {
		return p.handleWorkflowCreated(ctx, event)
	}

	// Skip events published by this worker to prevent infinite loops
	if event.Source == "worker-service" && event.Type == "io.flunq.task.completed" {
		p.logger.Debug("Skipping self-published task.completed event",
			"event_id", event.ID,
			"workflow_id", event.WorkflowID)
		return nil
	}

	// Only process task.completed events from executor-service
	if event.Type == "io.flunq.task.completed" && event.Source != "executor-service" {
		p.logger.Debug("Skipping task.completed event from non-executor source",
			"event_id", event.ID,
			"source", event.Source,
			"workflow_id", event.WorkflowID)
		return nil
	}

	// Step 1: Fetch Complete Event History
	events, err := p.fetchCompleteEventHistory(ctx, event.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to fetch event history: %w", err)
	}

	// Step 2: Get Workflow Definition
	definition, err := p.getWorkflowDefinition(ctx, event.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow definition: %w", err)
	}

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

	events, err := p.eventStore.GetEventHistory(ctx, workflowID)
	if err != nil {
		p.logger.Error("Failed to fetch event history",
			"workflow_id", workflowID,
			"error", err)
		return nil, err
	}

	p.logger.Debug("Fetched event history",
		"workflow_id", workflowID,
		"event_count", len(events))

	return events, nil
}

// handleWorkflowCreated processes workflow.created events by storing the workflow definition
func (p *WorkflowProcessor) handleWorkflowCreated(ctx context.Context, event *cloudevents.CloudEvent) error {
	p.logger.Info("Handling workflow created event",
		"workflow_id", event.WorkflowID)

	// Extract workflow definition from event data
	eventData, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format for workflow.created")
	}

	definitionData, ok := eventData["definition"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid workflow definition in event data")
	}

	// Parse the workflow definition using the workflow engine
	definitionJSON, err := json.Marshal(definitionData)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	definition, err := p.workflowEngine.ParseDefinition(ctx, definitionJSON)
	if err != nil {
		return fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	// Store the workflow definition in the database
	if err := p.database.CreateWorkflow(ctx, definition); err != nil {
		return fmt.Errorf("failed to store workflow definition: %w", err)
	}

	p.logger.Info("Workflow definition stored successfully",
		"workflow_id", definition.Id,
		"name", definition.Name,
		"spec_version", definition.SpecVersion)

	return nil
}

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

	// Send ALL tasks to Executor Service for execution
	return p.publishTaskRequestedEvent(ctx, task, state)
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

	// Get execution ID safely
	executionID := ""
	if state.Context != nil {
		executionID = state.Context.ExecutionId
	}

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
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		TaskID:      taskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    task.Name,
			"task_type":    task.TaskType,
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

	// Publish event
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

	// Create CloudEvent
	event := &cloudevents.CloudEvent{
		ID:          uuid.New().String(),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.workflow.completed",
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		Time:        time.Now(),
		Data:        eventData,
	}

	// Publish event
	if err := p.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish workflow completed event: %w", err)
	}

	// Update state status
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
	state.UpdatedAt = timestamppb.Now()

	p.logger.Info("Published workflow completed event",
		"workflow_id", state.WorkflowId,
		"event_id", event.ID)

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

	// Check if workflow is complete
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
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-scheduled-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.scheduled",
		Time:        time.Now(),
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
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-started-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.started",
		Time:        time.Now(),
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
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-failed-%s", taskID),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.failed",
		Time:        time.Now(),
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
