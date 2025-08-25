package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/shared/pkg/jq"
	workerInterfaces "github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
	"go.uber.org/zap"
)

// ServerlessWorkflowEngine implements the WorkflowEngine interface using the official Serverless Workflow SDK
type ServerlessWorkflowEngine struct {
	logger      workerInterfaces.Logger
	jqEvaluator *jq.Evaluator
	eventStream interfaces.EventStream
}

// NewServerlessWorkflowEngine creates a new Serverless Workflow engine
func NewServerlessWorkflowEngine(logger workerInterfaces.Logger, eventStream interfaces.EventStream) workerInterfaces.WorkflowEngine {
	// Create a zap logger for JQ evaluator
	zapLogger, _ := zap.NewDevelopment()

	return &ServerlessWorkflowEngine{
		logger:      logger,
		jqEvaluator: jq.NewEvaluator(zapLogger),
		eventStream: eventStream,
	}
}

// ParseDefinition parses a Serverless Workflow DSL definition
// Supports both DSL 1.0.0 (YAML) and DSL 0.8 (JSON) formats
func (e *ServerlessWorkflowEngine) ParseDefinition(ctx context.Context, dslData []byte) (*gen.WorkflowDefinition, error) {
	// Try to parse as YAML first (DSL 1.0.0), then JSON (DSL 0.8)
	var dslMap map[string]interface{}

	// Try YAML first
	if err := yaml.Unmarshal(dslData, &dslMap); err != nil {
		// Try JSON
		if err := json.Unmarshal(dslData, &dslMap); err != nil {
			e.logger.Error("Failed to parse workflow DSL as YAML or JSON", "error", err)
			return nil, fmt.Errorf("failed to parse workflow DSL: %w", err)
		}
	}

	// Extract basic information from DSL
	var workflowID, name, description, version, specVersion, startState string

	// Handle DSL 1.0.0 format
	if document, ok := dslMap["document"].(map[string]interface{}); ok {
		workflowID = getStringValue(document, "name")
		name = getStringValue(document, "name")
		description = getStringValue(document, "description")
		version = getStringValue(document, "version")
		specVersion = getStringValue(document, "dsl")

		// For DSL 1.0.0, extract the first task name from the "do" section as start state
		if doSection, ok := dslMap["do"].([]interface{}); ok && len(doSection) > 0 {
			if firstTask, ok := doSection[0].(map[string]interface{}); ok {
				for taskName := range firstTask {
					startState = taskName
					break // Use the first (and should be only) key as start state
				}
			}
		}
		if startState == "" {
			startState = "start" // Fallback if no tasks found
		}
	} else {
		// Handle DSL 0.8 format
		workflowID = getStringValue(dslMap, "id")
		name = getStringValue(dslMap, "name")
		description = getStringValue(dslMap, "description")
		version = getStringValue(dslMap, "version")
		specVersion = getStringValue(dslMap, "specVersion")
		startState = getStringValue(dslMap, "start")
	}

	// Create workflow definition
	definition := &gen.WorkflowDefinition{
		Id:          workflowID,
		Name:        name,
		Description: description,
		Version:     version,
		SpecVersion: specVersion,
		StartState:  startState,
	}

	// Store the complete DSL definition
	definition.DslDefinition, _ = structpb.NewStruct(dslMap)

	e.logger.Info("Successfully parsed workflow definition",
		"workflow_id", definition.Id,
		"name", definition.Name,
		"version", definition.Version,
		"spec_version", definition.SpecVersion)

	return definition, nil
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// ValidateDefinition validates a workflow definition
// For now, this is a basic validation - can be enhanced later
func (e *ServerlessWorkflowEngine) ValidateDefinition(ctx context.Context, definition *gen.WorkflowDefinition) error {
	// Basic validation
	if definition.Id == "" {
		return fmt.Errorf("workflow ID is required")
	}
	if definition.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if definition.DslDefinition == nil {
		return fmt.Errorf("workflow DSL definition is required")
	}

	e.logger.Debug("Workflow validation passed", "workflow_id", definition.Id)
	return nil
}

// InitializeState creates initial workflow state from definition
func (e *ServerlessWorkflowEngine) InitializeState(ctx context.Context, definition *gen.WorkflowDefinition, input map[string]interface{}) (*gen.WorkflowState, error) {
	now := timestamppb.Now()

	// Convert input to protobuf struct
	inputStruct, err := structpb.NewStruct(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input to protobuf struct: %w", err)
	}

	// Initialize variables with input data
	variables := make(map[string]interface{})
	for k, v := range input {
		variables[k] = v
	}
	variablesStruct, _ := structpb.NewStruct(variables)

	state := &gen.WorkflowState{
		WorkflowId:     definition.Id,
		CurrentStep:    definition.StartState,
		Status:         gen.WorkflowStatus_WORKFLOW_STATUS_CREATED,
		Variables:      variablesStruct,
		Input:          inputStruct,
		CompletedTasks: []*gen.CompletedTask{},
		PendingTasks:   []*gen.PendingTask{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	e.logger.Info("Initialized workflow state",
		"workflow_id", definition.Id,
		"start_state", definition.StartState)

	return state, nil
}

// RebuildState rebuilds workflow state from event history using the SDK
func (e *ServerlessWorkflowEngine) RebuildState(ctx context.Context, definition *gen.WorkflowDefinition, events []*cloudevents.CloudEvent) (*gen.WorkflowState, error) {
	e.logger.Info("Rebuilding workflow state from events",
		"workflow_id", definition.Id,
		"event_count", len(events))

	// Start with empty state
	state := &gen.WorkflowState{
		WorkflowId:     definition.Id,
		Status:         gen.WorkflowStatus_WORKFLOW_STATUS_CREATED,
		CompletedTasks: []*gen.CompletedTask{},
		PendingTasks:   []*gen.PendingTask{},
	}

	// Process each event in chronological order
	for i, event := range events {
		if err := e.ProcessEvent(ctx, state, event); err != nil {
			e.logger.Error("Failed to process event during state rebuild",
				"event_index", i,
				"event_id", event.ID,
				"event_type", event.Type,
				"error", err)
			return nil, fmt.Errorf("failed to process event %s: %w", event.ID, err)
		}
	}

	e.logger.Info("Successfully rebuilt workflow state",
		"workflow_id", definition.Id,
		"final_status", state.Status,
		"current_step", state.CurrentStep,
		"completed_tasks", len(state.CompletedTasks))

	return state, nil
}

// ProcessEvent processes a single event and updates workflow state
func (e *ServerlessWorkflowEngine) ProcessEvent(ctx context.Context, state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	e.logger.Debug("Processing event",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", state.WorkflowId)

	switch event.Type {
	case "io.flunq.workflow.created":
		return e.processWorkflowCreatedEvent(state, event)
	case "io.flunq.execution.started":
		return e.processExecutionStartedEvent(state, event)
	case "io.flunq.task.requested":
		return e.processTaskRequestedEvent(state, event)
	case "io.flunq.task.completed":
		return e.processTaskCompletedEvent(state, event)
	case "io.flunq.task.failed":
		return e.processTaskFailedEvent(state, event)
	case "io.flunq.timer.fired":
		// Check if this is a retry timer or a wait task timer
		return e.processTimerFiredEvent(state, event)
	case "io.flunq.workflow.step.completed":
		return e.processStepCompletedEvent(state, event)
	case "io.flunq.workflow.completed":
		return e.processWorkflowCompletedEvent(state, event)
	case "io.flunq.workflow.failed":
		return e.processWorkflowFailedEvent(state, event)
	default:
		e.logger.Warn("Unknown event type", "event_type", event.Type, "event_id", event.ID)
		return nil // Don't fail on unknown events
	}
}

// processWorkflowCreatedEvent processes workflow creation events
func (e *ServerlessWorkflowEngine) processWorkflowCreatedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CREATED
	state.CreatedAt = timestamppb.New(event.Time)
	state.UpdatedAt = timestamppb.New(event.Time)

	// Extract workflow data from event
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		if name, exists := eventData["name"]; exists {
			if nameStr, ok := name.(string); ok {
				// Store workflow name in variables
				if state.Variables == nil {
					state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
				}
				state.Variables.Fields["workflow_name"] = structpb.NewStringValue(nameStr)
			}
		}
	}

	e.logger.Debug("Processed workflow created event", "workflow_id", state.WorkflowId)
	return nil
}

// processExecutionStartedEvent processes execution started events
func (e *ServerlessWorkflowEngine) processExecutionStartedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_RUNNING

	// Ensure workflow instance ID stays consistent with event stream routing
	// Use the instance-level identifier from the event, not the definition ID
	if event.WorkflowID != "" {
		state.WorkflowId = event.WorkflowID
	}

	// Extract execution context
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		context := &gen.ExecutionContext{
			ExecutionId: event.ExecutionID,
			TenantId:    event.TenantID, // Set tenant ID from CloudEvent
		}

		if correlationId, exists := eventData["correlation_id"]; exists {
			if corrId, ok := correlationId.(string); ok {
				context.CorrelationId = corrId
			}
		}

		if trigger, exists := eventData["started_by"]; exists {
			if triggerStr, ok := trigger.(string); ok {
				context.Trigger = triggerStr
			}
		}

		state.Context = context

		e.logger.Debug("Set execution context during state rebuild",
			"workflow_id", state.WorkflowId,
			"execution_id", context.ExecutionId,
			"tenant_id", context.TenantId,
			"event_id", event.ID)

		// Extract input data
		if input, exists := eventData["input"]; exists {
			if inputMap, ok := input.(map[string]interface{}); ok {
				inputStruct, _ := structpb.NewStruct(inputMap)
				state.Input = inputStruct

				// Initialize variables with input
				if state.Variables == nil {
					state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
				}
				for k, v := range inputMap {
					val, _ := structpb.NewValue(v)
					state.Variables.Fields[k] = val
				}
			}
		}
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Debug("Processed execution started event", "workflow_id", state.WorkflowId, "execution_id", event.ExecutionID)
	return nil
}

// processTaskRequestedEvent processes task requested events
func (e *ServerlessWorkflowEngine) processTaskRequestedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		taskName, _ := eventData["task_name"].(string)
		taskType, _ := eventData["task_type"].(string)
		queue, _ := eventData["queue"].(string)
		activityName, _ := eventData["activity_name"].(string)

		// Create pending task
		pendingTask := &gen.PendingTask{
			Name:         taskName,
			TaskType:     taskType,
			Queue:        queue,
			ActivityName: activityName,
			CreatedAt:    timestamppb.New(event.Time),
		}

		// Extract input data
		if input, exists := eventData["input"]; exists {
			if inputMap, ok := input.(map[string]interface{}); ok {
				pendingTask.Input, _ = structpb.NewStruct(inputMap)
			}
		}

		state.PendingTasks = append(state.PendingTasks, pendingTask)
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Debug("Processed task requested event", "workflow_id", state.WorkflowId, "event_id", event.ID)
	return nil
}

// processTaskCompletedEvent processes task completed events
func (e *ServerlessWorkflowEngine) processTaskCompletedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		// Extract task name - try multiple possible locations
		var taskName string

		// Method 1: Check if task_name is directly in eventData
		if name, exists := eventData["task_name"].(string); exists && name != "" {
			taskName = name
		}

		// Method 2: Check nested data structure
		if taskName == "" {
			if dataSection, exists := eventData["data"].(map[string]interface{}); exists {
				if name, exists := dataSection["task_name"].(string); exists && name != "" {
					taskName = name
				}
			}
		}

		// Method 3: Check if it's in a nested event structure
		if taskName == "" {
			if eventSection, exists := eventData["event"].(map[string]interface{}); exists {
				if name, exists := eventSection["task_name"].(string); exists && name != "" {
					taskName = name
				}
			}
		}

		// Extract task success status - CRITICAL for error handling
		taskSuccess := true // Default to true for backward compatibility
		if success, exists := eventData["success"].(bool); exists {
			taskSuccess = success
		}

		// Extract error message if task failed
		var errorMessage string
		if !taskSuccess {
			if errMsg, exists := eventData["error"].(string); exists {
				errorMessage = errMsg
			}
		}

		e.logger.Debug("Processing task completed event",
			"event_id", event.ID,
			"extracted_task_name", taskName,
			"task_success", taskSuccess,
			"error_message", errorMessage,
			"current_completed_tasks", len(state.CompletedTasks),
			"event_data_keys", getMapKeys(eventData))

		// Debug the full event data structure to understand the nesting
		debugEventData(e.logger, eventData)

		// If task failed, transition workflow to failed state
		if !taskSuccess {
			e.logger.Error("Task failed - transitioning workflow to failed state",
				"workflow_id", state.WorkflowId,
				"task_name", taskName,
				"error", errorMessage)

			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_FAILED

			// Store error information in workflow variables
			if state.Variables == nil {
				state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
			}

			errorVal, _ := structpb.NewValue(errorMessage)
			taskNameVal, _ := structpb.NewValue(taskName)
			state.Variables.Fields["error_message"] = errorVal
			state.Variables.Fields["failed_task"] = taskNameVal

			state.UpdatedAt = timestamppb.New(event.Time)
			return nil // Don't process further - workflow has failed
		}

		// Remove from pending tasks
		for i, pendingTask := range state.PendingTasks {
			if pendingTask.Name == taskName {
				state.PendingTasks = append(state.PendingTasks[:i], state.PendingTasks[i+1:]...)
				break
			}
		}

		// Extract complete TaskData from event
		var taskData *gen.TaskData

		// Determine task status based on success
		taskStatus := gen.TaskStatus_TASK_STATUS_COMPLETED
		if !taskSuccess {
			taskStatus = gen.TaskStatus_TASK_STATUS_FAILED
		}

		if taskDataMap, exists := eventData["data"].(map[string]interface{}); exists {
			// Reconstruct TaskData from event data
			taskData = &gen.TaskData{
				Input:  e.mapToStruct(taskDataMap["input"]),
				Output: e.mapToStruct(taskDataMap["output"]),
				Metadata: &gen.TaskMetadata{
					CompletedAt:  timestamppb.New(event.Time),
					Status:       taskStatus,
					ErrorMessage: errorMessage, // Store error message in metadata
				},
			}

			// Extract additional metadata if available
			if metadataMap, metaExists := taskDataMap["metadata"].(map[string]interface{}); metaExists {
				if startedAtStr, ok := metadataMap["started_at"].(string); ok {
					if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
						taskData.Metadata.StartedAt = timestamppb.New(startedAt)
					}
				}
				if durationMs, ok := metadataMap["duration_ms"].(float64); ok {
					taskData.Metadata.DurationMs = int64(durationMs)
				}
				if taskType, ok := metadataMap["task_type"].(string); ok {
					taskData.Metadata.TaskType = taskType
				}
			}
		} else {
			// Fallback to minimal TaskData if event doesn't contain full data
			taskData = &gen.TaskData{
				Metadata: &gen.TaskMetadata{
					CompletedAt:  timestamppb.New(event.Time),
					Status:       taskStatus,
					ErrorMessage: errorMessage, // Store error message in metadata
				},
			}
		}

		// Create completed task with full data
		completedTask := &gen.CompletedTask{
			Name: taskName,
			Data: taskData,
		}

		// Update workflow variables with task output (if available)
		if taskData.Output != nil && taskData.Output.Fields != nil {
			if state.Variables == nil {
				state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
			}

			// Add task output to workflow variables
			for k, v := range taskData.Output.Fields {
				state.Variables.Fields[k] = v
			}
		}

		// Check if this task is already completed to avoid duplicates
		taskAlreadyCompleted := false
		for _, existingTask := range state.CompletedTasks {
			if existingTask.Name == taskName {
				taskAlreadyCompleted = true
				break
			}
		}

		// Only add if not already completed
		if !taskAlreadyCompleted {
			state.CompletedTasks = append(state.CompletedTasks, completedTask)
			e.logger.Debug("Added completed task",
				"task_name", taskName,
				"total_completed_tasks", len(state.CompletedTasks))
		} else {
			e.logger.Debug("Task already completed, skipping duplicate",
				"task_name", taskName,
				"total_completed_tasks", len(state.CompletedTasks))
		}

		// Update execution output with current workflow variables
		// This ensures task outputs are visible immediately after task completion
		if state.Variables != nil {
			state.Output = state.Variables
			e.logger.Debug("Updated execution output with workflow variables",
				"task_name", taskName,
				"output_keys", getMapKeys(state.Variables.AsMap()))
		}
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Debug("Processed task completed event", "workflow_id", state.WorkflowId, "event_id", event.ID)
	return nil
}

// processStepCompletedEvent processes workflow step completed events
func (e *ServerlessWorkflowEngine) processStepCompletedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		if stepName, exists := eventData["step_name"]; exists {
			if stepStr, ok := stepName.(string); ok {
				state.CurrentStep = stepStr
			}
		}

		// Update variables with step results
		if result, exists := eventData["result"]; exists {
			if state.Variables == nil {
				state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
			}
			val, _ := structpb.NewValue(result)
			state.Variables.Fields["last_step_result"] = val
		}
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Debug("Processed step completed event", "workflow_id", state.WorkflowId, "current_step", state.CurrentStep)
	return nil
}

// processWorkflowCompletedEvent processes workflow completion events
func (e *ServerlessWorkflowEngine) processWorkflowCompletedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED

	// Extract output data
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		if output, exists := eventData["output"]; exists {
			if outputMap, ok := output.(map[string]interface{}); ok {
				state.Output, _ = structpb.NewStruct(outputMap)
			}
		}
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Info("Processed workflow completed event", "workflow_id", state.WorkflowId)
	return nil
}

// processWorkflowFailedEvent processes workflow failure events
func (e *ServerlessWorkflowEngine) processWorkflowFailedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_FAILED

	// Store error information in variables
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		if state.Variables == nil {
			state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
		}

		if errorMsg, exists := eventData["error"]; exists {
			val, _ := structpb.NewValue(errorMsg)
			state.Variables.Fields["error_message"] = val
		}
		if errorCode, exists := eventData["error_code"]; exists {
			val, _ := structpb.NewValue(errorCode)
			state.Variables.Fields["error_code"] = val
		}
	}

	state.UpdatedAt = timestamppb.New(event.Time)
	e.logger.Error("Processed workflow failed event", "workflow_id", state.WorkflowId)
	return nil
}

// GetNextTask determines the next task to execute based on current state using simple sequential execution
func (e *ServerlessWorkflowEngine) GetNextTask(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) (*gen.PendingTask, error) {
	// Extract ordered task list from DSL
	taskList, err := e.extractOrderedTaskList(definition)
	if err != nil {
		e.logger.Error("Failed to extract task list from DSL", "error", err)
		return nil, fmt.Errorf("failed to extract task list: %w", err)
	}

	if state.CurrentStep == "" {
		state.CurrentStep = definition.StartState
	}

	// Simple sequential execution: completed count = next task index
	completedCount := len(state.CompletedTasks)

	e.logger.Info("Checking workflow progression",
		"workflow_id", state.WorkflowId,
		"completed_tasks", completedCount,
		"total_tasks", len(taskList),
		"task_list", taskList,
		"pending_tasks", len(state.PendingTasks))

	// Check if all tasks are completed
	if completedCount >= len(taskList) {
		e.logger.Info("Workflow completed - all tasks done",
			"workflow_id", state.WorkflowId,
			"completed_tasks", completedCount,
			"total_tasks", len(taskList),
			"task_list", taskList)
		return nil, nil // No more tasks - workflow complete
	}

	// Get the next task name
	nextTaskName := taskList[completedCount]

	e.logger.Debug("Next task determined",
		"workflow_id", state.WorkflowId,
		"next_task", nextTaskName,
		"task_position", fmt.Sprintf("%d/%d", completedCount+1, len(taskList)))

	// Check if this task is already pending
	for _, pendingTask := range state.PendingTasks {
		if pendingTask.Name == nextTaskName {
			e.logger.Debug("Task already pending, not creating duplicate",
				"task_name", nextTaskName)
			return nil, nil // Task already pending
		}
	}

	// Check if this task is already completed
	for _, completedTask := range state.CompletedTasks {
		if completedTask.Name == nextTaskName {
			e.logger.Debug("Task already completed, not creating duplicate",
				"task_name", nextTaskName)
			return nil, nil // Task already completed
		}
	}

	// Determine task type from DSL
	taskType := e.getTaskTypeFromDSL(definition, nextTaskName)

	// Create next task
	task := &gen.PendingTask{
		Name:      nextTaskName,
		TaskType:  taskType,
		Queue:     "local-queue",
		CreatedAt: timestamppb.Now(),
	}

	// Build task input from DSL
	taskInput := e.buildTaskInputFromDSL(definition, nextTaskName, state)
	task.Input, _ = structpb.NewStruct(taskInput)

	e.logger.Debug("Generated next task",
		"task_name", task.Name,
		"task_type", task.TaskType,
		"task_position", fmt.Sprintf("%d/%d", completedCount+1, len(taskList)))

	return task, nil
}

// extractOrderedTaskList extracts tasks using the official Serverless Workflow SDK
func (e *ServerlessWorkflowEngine) extractOrderedTaskList(definition *gen.WorkflowDefinition) ([]string, error) {
	if definition.DslDefinition == nil {
		e.logger.Warn("DSL definition is nil, using fallback task list")
		return []string{"initialize", "processData", "waitStep", "finalize"}, nil
	}

	// Use manual parsing for now - SDK v3 has compatibility issues with our SDL format
	e.logger.Info("Using manual DSL parsing for task extraction")
	return e.extractOrderedTaskListManual(definition)
}

// extractOrderedTaskListManual is the fallback manual parsing method
func (e *ServerlessWorkflowEngine) extractOrderedTaskListManual(definition *gen.WorkflowDefinition) ([]string, error) {
	dslMap := definition.DslDefinition.AsMap()
	e.logger.Info("Manual DSL parsing fallback", "dsl_keys", getMapKeys(dslMap))

	// Try to extract tasks from "do" section
	if doSection, ok := dslMap["do"].([]interface{}); ok {
		e.logger.Info("Found 'do' section in manual parsing", "do_section_length", len(doSection))
		taskList := make([]string, 0, len(doSection))

		for _, item := range doSection {
			if taskMap, ok := item.(map[string]interface{}); ok {
				for taskName := range taskMap {
					e.logger.Info("Found task in manual parsing", "task_name", taskName)
					taskList = append(taskList, taskName)
					break
				}
			}
		}

		if len(taskList) > 0 {
			return taskList, nil
		}
	}

	// Final fallback
	e.logger.Warn("Manual parsing failed, using hardcoded fallback")
	return []string{"initialize", "processData", "waitStep", "finalize"}, nil
}

// hasTaskProperties checks if a map contains task-like properties
func hasTaskProperties(taskDef map[string]interface{}) bool {
	// Check for common task properties
	taskProperties := []string{"try", "call", "set", "wait", "inject", "switch", "foreach", "parallel"}

	for _, prop := range taskProperties {
		if _, exists := taskDef[prop]; exists {
			return true
		}
	}

	return false
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// debugEventData logs the full structure of event data for debugging
func debugEventData(logger workerInterfaces.Logger, eventData map[string]interface{}) {
	logger.Debug("Event data structure")
	for key, value := range eventData {
		if nestedMap, ok := value.(map[string]interface{}); ok {
			logger.Debug("Nested map", "key", key, "nested_keys", getMapKeys(nestedMap))
			if key == "data" {
				for nestedKey, nestedValue := range nestedMap {
					logger.Debug("Data field", "nested_key", nestedKey, "value_type", fmt.Sprintf("%T", nestedValue))
					if nestedKey == "task_name" {
						logger.Debug("Found task_name in data", "task_name", nestedValue)
					}
				}
			}
		} else {
			logger.Debug("Direct field", "key", key, "value_type", fmt.Sprintf("%T", value), "value", value)
		}
	}
}

// extractTaskNamesFromDSL extracts task names from the DSL definition
func (e *ServerlessWorkflowEngine) extractTaskNamesFromDSL(definition *gen.WorkflowDefinition) ([]string, error) {
	if definition.DslDefinition == nil {
		return nil, fmt.Errorf("DSL definition is nil")
	}

	dslMap := definition.DslDefinition.AsMap()

	// Extract tasks from "do" section (DSL 1.0.0 format)
	if doSection, ok := dslMap["do"].([]interface{}); ok {
		taskNames := make([]string, 0, len(doSection))

		for _, item := range doSection {
			if taskMap, ok := item.(map[string]interface{}); ok {
				// Each item in "do" is a map with one key (the task name)
				for taskName := range taskMap {
					taskNames = append(taskNames, taskName)
					break // Only take the first (and should be only) key
				}
			}
		}

		e.logger.Debug("Extracted task names from DSL",
			"task_names", taskNames,
			"count", len(taskNames))

		return taskNames, nil
	}

	// Fallback for other DSL formats or if "do" section not found
	return []string{"initialize", "processData", "waitStep", "finalize"}, nil
}

// getTaskTypeFromDSL determines the task type from DSL definition
func (e *ServerlessWorkflowEngine) getTaskTypeFromDSL(definition *gen.WorkflowDefinition, taskName string) string {
	if definition.DslDefinition == nil {
		e.logger.Debug("No DSL definition found, using default task type", "task_name", taskName, "task_type", "set")
		return "set" // Default fallback
	}

	dslMap := definition.DslDefinition.AsMap()

	// Look for the task in "do" section
	if doSection, ok := dslMap["do"].([]interface{}); ok {
		for _, item := range doSection {
			if taskMap, ok := item.(map[string]interface{}); ok {
				if taskDef, exists := taskMap[taskName]; exists {
					if taskDefMap, ok := taskDef.(map[string]interface{}); ok {
						// Check what type of task this is
						if _, hasSet := taskDefMap["set"]; hasSet {
							e.logger.Info("Identified task type from DSL", "task_name", taskName, "task_type", "set")
							return "set"
						}
						if _, hasWait := taskDefMap["wait"]; hasWait {
							e.logger.Info("Identified task type from DSL", "task_name", taskName, "task_type", "wait")
							return "wait"
						}
						if _, hasCall := taskDefMap["call"]; hasCall {
							e.logger.Info("Identified task type from DSL", "task_name", taskName, "task_type", "call")
							return "call"
						}

						// Check for try/catch pattern - extract the inner task type
						if tryBlock, hasTry := taskDefMap["try"]; hasTry {
							e.logger.Info("Found try block in task definition", "task_name", taskName, "try_block", tryBlock)

							if tryMap, ok := tryBlock.(map[string]interface{}); ok {
								e.logger.Info("Try block is valid map", "task_name", taskName, "try_keys", getMapKeys(tryMap))

								// Any task with try/catch should be handled by TryTaskExecutor
								if _, hasCallInTry := tryMap["call"]; hasCallInTry {
									e.logger.Info("Identified task type from DSL (try/catch pattern with call)", "task_name", taskName, "task_type", "try")
									return "try" // Let TryTaskExecutor handle the try/catch logic
								}
								// Add support for other task types inside try blocks
								if _, hasSetInTry := tryMap["set"]; hasSetInTry {
									e.logger.Info("Identified task type from DSL (try/catch pattern with set)", "task_name", taskName, "task_type", "try")
									return "try"
								}
								if _, hasWaitInTry := tryMap["wait"]; hasWaitInTry {
									e.logger.Info("Identified task type from DSL (try/catch pattern with wait)", "task_name", taskName, "task_type", "try")
									return "try"
								}

								e.logger.Warn("Try block found but no recognized inner task type", "task_name", taskName, "try_keys", getMapKeys(tryMap))
							} else {
								e.logger.Error("Try block is not a valid map", "task_name", taskName, "try_block_type", fmt.Sprintf("%T", tryBlock))
							}
						}
						e.logger.Debug("Task definition found but no recognized task type", "task_name", taskName, "available_keys", getMapKeys(taskDefMap))
					}
				}
			}
		}
	}

	e.logger.Debug("Task not found in DSL, using default task type", "task_name", taskName, "task_type", "set")
	return "set" // Default fallback
}

// buildTaskInputFromDSL builds task input from DSL definition
func (e *ServerlessWorkflowEngine) buildTaskInputFromDSL(definition *gen.WorkflowDefinition, taskName string, state *gen.WorkflowState) map[string]interface{} {
	taskInput := make(map[string]interface{})

	// Add basic task information
	taskInput["task_name"] = taskName
	taskInput["workflow_id"] = state.WorkflowId
	taskInput["timestamp"] = time.Now().Format(time.RFC3339)

	// Try to extract task-specific configuration from DSL
	if definition.DslDefinition != nil {
		dslMap := definition.DslDefinition.AsMap()

		if doSection, ok := dslMap["do"].([]interface{}); ok {
			for _, item := range doSection {
				if taskMap, ok := item.(map[string]interface{}); ok {
					if taskDef, exists := taskMap[taskName]; exists {
						taskInput["task_definition"] = taskDef

						// Extract task-specific parameters
						if taskDefMap, ok := taskDef.(map[string]interface{}); ok {
							// Handle wait task duration
							if waitConfig, hasWait := taskDefMap["wait"]; hasWait {
								if waitMap, ok := waitConfig.(map[string]interface{}); ok {
									if duration, hasDuration := waitMap["duration"]; hasDuration {
										if durationStr, ok := duration.(string); ok {
											// Convert ISO 8601 duration (PT2S) to Go duration (2s)
											goDuration := e.convertISO8601ToGoDuration(durationStr)
											taskInput["duration"] = goDuration

											e.logger.Debug("Extracted wait duration",
												"task_name", taskName,
												"iso8601_duration", durationStr,
												"go_duration", goDuration)
										}
									}
								}
							}

							// Handle set task parameters
							if setConfig, hasSet := taskDefMap["set"]; hasSet {
								// Evaluate JQ expressions in set task data
								evaluatedSetConfig, err := e.evaluateSetTaskData(setConfig, state)
								if err != nil {
									e.logger.Error("Failed to evaluate JQ expressions in set task",
										"task_name", taskName,
										"error", err.Error())
									// Fallback to original config if evaluation fails
									taskInput["set_data"] = setConfig
								} else {
									taskInput["set_data"] = evaluatedSetConfig
								}
							}

							// Handle call task parameters
							if callType, hasCall := taskDefMap["call"]; hasCall {
								e.logger.Debug("Processing call task configuration",
									"task_name", taskName,
									"call_type", callType)

								// Extract call configuration
								callConfig := e.buildCallTaskConfig(taskDefMap, callType, taskName)
								taskInput["call_config"] = callConfig

								e.logger.Debug("Built call task configuration",
									"task_name", taskName,
									"config", callConfig)
							} else if tryBlock, hasTry := taskDefMap["try"]; hasTry {
								// Handle try/catch pattern - extract inner task configuration
								e.logger.Debug("Processing try/catch block",
									"task_name", taskName,
									"has_try", hasTry)

								// Store the try/catch metadata for error handling
								taskInput["try_catch_config"] = map[string]interface{}{
									"try":   tryBlock,
									"catch": taskDefMap["catch"], // May be nil if no catch block
								}

								// Extract catch block from task definition
								catchBlock, hasCatch := taskDefMap["catch"]

								// For try tasks, pass the complete try/catch configuration to TryTaskExecutor
								taskInput["try_config"] = map[string]interface{}{
									"try":   tryBlock,
									"catch": catchBlock,
								}

								e.logger.Info("Built try/catch configuration for TryTaskExecutor",
									"task_name", taskName,
									"has_try", tryBlock != nil,
									"has_catch", hasCatch)

								e.logger.Debug("Built try/catch task configuration",
									"task_name", taskName,
									"has_try_catch_config", taskInput["try_catch_config"] != nil,
									"has_config", taskInput["config"] != nil)
							}
						}
						break
					}
				}
			}
		}
	}

	return taskInput
}

// buildCallTaskConfig builds call task configuration from DSL
func (e *ServerlessWorkflowEngine) buildCallTaskConfig(taskDefMap map[string]interface{}, callType interface{}, taskName string) map[string]interface{} {
	config := make(map[string]interface{})

	// Determine call type and extract configuration
	if callTypeStr, ok := callType.(string); ok {
		config["call_type"] = callTypeStr

		e.logger.Debug("Processing call type", "task_name", taskName, "call_type", callTypeStr)

		switch callTypeStr {
		case "openapi":
			// Extract OpenAPI configuration from "with" section
			if withConfig, hasWith := taskDefMap["with"]; hasWith {
				if withMap, ok := withConfig.(map[string]interface{}); ok {
					// Map DSL fields to executor configuration
					for key, value := range withMap {
						switch key {
						case "operationId":
							// Map camelCase to snake_case
							config["operation_id"] = value
						default:
							// Copy other fields as-is
							config[key] = value
						}
					}

					e.logger.Debug("Extracted OpenAPI configuration",
						"task_name", taskName,
						"config_keys", getMapKeys(withMap),
						"mapped_operation_id", config["operation_id"])
				}
			}

		case "http":
			// Extract HTTP configuration from "with" section
			if withConfig, hasWith := taskDefMap["with"]; hasWith {
				if withMap, ok := withConfig.(map[string]interface{}); ok {
					// Copy all "with" parameters to the config
					for key, value := range withMap {
						config[key] = value
					}

					e.logger.Debug("Extracted HTTP configuration",
						"task_name", taskName,
						"config_keys", getMapKeys(withMap))
				}
			}

		default:
			e.logger.Warn("Unknown call type, treating as HTTP",
				"task_name", taskName,
				"call_type", callTypeStr)

			// Fallback: treat as HTTP and extract "with" configuration
			if withConfig, hasWith := taskDefMap["with"]; hasWith {
				if withMap, ok := withConfig.(map[string]interface{}); ok {
					for key, value := range withMap {
						config[key] = value
					}
				}
			}
		}
	} else {
		e.logger.Warn("Call type is not a string, using default HTTP configuration",
			"task_name", taskName,
			"call_type", fmt.Sprintf("%T", callType))

		// Fallback: assume HTTP call and extract basic configuration
		config["call_type"] = "http"
		if withConfig, hasWith := taskDefMap["with"]; hasWith {
			if withMap, ok := withConfig.(map[string]interface{}); ok {
				for key, value := range withMap {
					config[key] = value
				}
			}
		}
	}

	return config
}

// buildCallTaskConfigFromTryBlock builds call task configuration from SDL try block
func (e *ServerlessWorkflowEngine) buildCallTaskConfigFromTryBlock(tryMap map[string]interface{}, callType interface{}, taskName string) map[string]interface{} {
	config := make(map[string]interface{})

	// Set the call type
	if callTypeStr, ok := callType.(string); ok {
		config["call_type"] = callTypeStr

		e.logger.Info("Processing call type in try block",
			"task_name", taskName,
			"call_type", callTypeStr,
			"try_map_keys", getMapKeys(tryMap))

		switch callTypeStr {
		case "openapi":
			// Extract OpenAPI configuration from "with" section inside try block
			if withConfig, hasWith := tryMap["with"]; hasWith {
				if withMap, ok := withConfig.(map[string]interface{}); ok {
					// Map DSL fields to executor configuration
					for key, value := range withMap {
						switch key {
						case "operationId":
							// Map camelCase to snake_case
							config["operation_id"] = value
						case "document":
							// Copy document configuration directly
							config["document"] = value
						case "parameters":
							// Copy parameters directly
							config["parameters"] = value
						default:
							// Copy other fields as-is
							config[key] = value
						}
					}

					e.logger.Debug("Extracted OpenAPI configuration from try block",
						"task_name", taskName,
						"config_keys", getMapKeys(config),
						"with_keys", getMapKeys(withMap))
				}
			}

			// Handle output configuration if present in try block
			if outputConfig, hasOutput := tryMap["output"]; hasOutput {
				config["output_config"] = outputConfig
				e.logger.Debug("Added output configuration from try block",
					"task_name", taskName,
					"output_config", outputConfig)
			}

		default:
			e.logger.Warn("Unsupported call type in try block", "task_name", taskName, "call_type", callTypeStr)
		}
	}

	e.logger.Info("Final call configuration built from try block",
		"task_name", taskName,
		"config", config)

	return config
}

// convertISO8601ToGoDuration converts ISO 8601 duration (PT2S) to Go duration (2s)
func (e *ServerlessWorkflowEngine) convertISO8601ToGoDuration(iso8601Duration string) string {
	// Simple conversion for common cases
	// PT2S -> 2s, PT30S -> 30s, PT1M -> 1m, PT1H -> 1h

	if len(iso8601Duration) < 3 || !strings.HasPrefix(iso8601Duration, "PT") {
		return "1s" // Default fallback
	}

	// Remove "PT" prefix
	duration := iso8601Duration[2:]

	// Convert common patterns
	if strings.HasSuffix(duration, "S") {
		// PT2S -> 2s
		return strings.ToLower(duration)
	} else if strings.HasSuffix(duration, "M") {
		// PT1M -> 1m
		return strings.ToLower(duration)
	} else if strings.HasSuffix(duration, "H") {
		// PT1H -> 1h
		return strings.ToLower(duration)
	}

	// Fallback
	return "1s"
}

// TODO: Implement full Serverless Workflow SDK integration
// For now, these are placeholder implementations

// ExecuteTask executes a specific task type
func (e *ServerlessWorkflowEngine) ExecuteTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error) {
	e.logger.Info("Executing task", "task_name", task.Name, "task_type", task.TaskType)

	switch task.TaskType {
	case "set":
		return e.executeSetTask(ctx, task, state)
	case "wait":
		return e.executeWaitTask(ctx, task, state)
	case "call":
		// For call tasks, we need to delegate to external executor
		return nil, fmt.Errorf("call tasks must be executed by external executor service")
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
}

// executeSetTask executes a set variables task
func (e *ServerlessWorkflowEngine) executeSetTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error) {
	startTime := time.Now()

	// Extract data to inject
	taskInput := task.Input.AsMap()
	injectData, exists := taskInput["set_data"]
	if !exists {
		return nil, fmt.Errorf("no set_data found in set task input")
	}

	// Update workflow variables
	if state.Variables == nil {
		state.Variables, _ = structpb.NewStruct(make(map[string]interface{}))
	}

	var injectMapLen int
	if injectMap, ok := injectData.(map[string]interface{}); ok {
		injectMapLen = len(injectMap)
		for k, v := range injectMap {
			val, _ := structpb.NewValue(v)
			state.Variables.Fields[k] = val
		}
	}

	// Create task result
	taskData := &gen.TaskData{
		Input:  task.Input,
		Output: task.Input, // For set tasks, output is same as input
		Metadata: &gen.TaskMetadata{
			StartedAt:   timestamppb.New(startTime),
			CompletedAt: timestamppb.Now(),
			DurationMs:  time.Since(startTime).Milliseconds(),
			Status:      gen.TaskStatus_TASK_STATUS_COMPLETED,
			TaskType:    task.TaskType,
			Queue:       task.Queue,
		},
	}

	e.logger.Debug("Executed set task", "task_name", task.Name, "variables_updated", injectMapLen)
	return taskData, nil
}

// executeWaitTask executes a wait/sleep task using event-driven scheduling
func (e *ServerlessWorkflowEngine) executeWaitTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error) {
	startTime := time.Now()

	// Extract duration
	taskInput := task.Input.AsMap()
	durationStr, exists := taskInput["duration"].(string)
	if !exists {
		return nil, fmt.Errorf("no duration found in wait task input")
	}

	// Parse duration
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format: %w", err)
	}

	e.logger.Info("Scheduling wait task using event stream - workflow will pause",
		"task_name", task.Name,
		"duration", duration,
		"workflow_id", state.WorkflowId,
		"resume_at", time.Now().Add(duration).Format(time.RFC3339))

	// Schedule wait completion event using native event streaming
	if err := e.scheduleWaitCompletionEvent(ctx, task, state, duration); err != nil {
		return nil, fmt.Errorf("failed to schedule wait completion event: %w", err)
	}

	// Create task result indicating the wait has been scheduled (not completed yet)
	taskData := &gen.TaskData{
		Input: task.Input,
		Output: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"wait_duration": structpb.NewStringValue(duration.String()),
				"scheduled_at":  structpb.NewStringValue(time.Now().Format(time.RFC3339)),
				"resume_at":     structpb.NewStringValue(time.Now().Add(duration).Format(time.RFC3339)),
				"status":        structpb.NewStringValue("waiting"),
			},
		},
		Metadata: &gen.TaskMetadata{
			StartedAt:   timestamppb.New(startTime),
			CompletedAt: nil,                                // Not completed yet - will be completed when timer fires
			DurationMs:  0,                                  // Duration will be calculated when wait completes
			Status:      gen.TaskStatus_TASK_STATUS_PENDING, // Set to pending status (waiting for timer)
			TaskType:    task.TaskType,
			Queue:       task.Queue,
		},
	}

	e.logger.Info("Wait task scheduled successfully - workflow paused",
		"task_name", task.Name,
		"duration", duration,
		"workflow_id", state.WorkflowId)

	return taskData, nil
}

// IsWorkflowComplete checks if the workflow has completed
func (e *ServerlessWorkflowEngine) IsWorkflowComplete(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) bool {
	// For DSL 1.0.0 format, only consider failed status - ignore completed status
	// Let GetNextTask() handle completion by returning nil when no more tasks
	dslMap := definition.DslDefinition.AsMap()
	if _, hasDoSection := dslMap["do"]; hasDoSection {
		// Only fail on failed status, ignore completed status for DSL 1.0.0
		if state.Status == gen.WorkflowStatus_WORKFLOW_STATUS_FAILED {
			e.logger.Info("Workflow failed",
				"workflow_id", state.WorkflowId,
				"status", state.Status)
			return true
		}
		e.logger.Debug("DSL 1.0.0 detected - using task-based completion logic",
			"workflow_id", state.WorkflowId)
		return false // Let GetNextTask() determine completion
	}

	// For legacy DSL 0.8 format, use status-based completion logic
	if state.Status == gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED ||
		state.Status == gen.WorkflowStatus_WORKFLOW_STATUS_FAILED {
		e.logger.Info("Workflow complete due to status",
			"workflow_id", state.WorkflowId,
			"status", state.Status)
		return true
	}

	// Legacy DSL 0.8 format - check if current state is an end state
	if states, exists := dslMap["states"]; exists {
		if statesArray, ok := states.([]interface{}); ok {
			for _, stateInterface := range statesArray {
				if stateMap, ok := stateInterface.(map[string]interface{}); ok {
					if name, exists := stateMap["name"]; exists && name == state.CurrentStep {
						if end, exists := stateMap["end"]; exists {
							if endBool, ok := end.(bool); ok && endBool {
								e.logger.Info("Workflow complete due to end state",
									"workflow_id", state.WorkflowId,
									"current_step", state.CurrentStep)
								return true
							}
						}
					}
				}
			}
		}
	}

	e.logger.Debug("Workflow not complete",
		"workflow_id", state.WorkflowId,
		"status", state.Status,
		"current_step", state.CurrentStep)
	return false
}

// mapToStruct converts a map[string]interface{} to *structpb.Struct
func (e *ServerlessWorkflowEngine) mapToStruct(data interface{}) *structpb.Struct {
	if data == nil {
		return nil
	}

	if dataMap, ok := data.(map[string]interface{}); ok {
		if struct_, err := structpb.NewStruct(dataMap); err == nil {
			return struct_
		}
	}

	// Fallback: try to convert any type to struct
	if struct_, err := structpb.NewValue(data); err == nil {
		if structValue := struct_.GetStructValue(); structValue != nil {
			return structValue
		}
	}

	return nil
}

// evaluateSetTaskData evaluates JQ expressions in set task configuration
func (e *ServerlessWorkflowEngine) evaluateSetTaskData(setConfig interface{}, state *gen.WorkflowState) (interface{}, error) {
	// Build workflow context for JQ evaluation
	workflowContext := e.buildWorkflowContext(state)

	// Convert setConfig to map[string]interface{} for evaluation
	setConfigMap, ok := setConfig.(map[string]interface{})
	if !ok {
		return setConfig, fmt.Errorf("set config is not a map")
	}

	// Evaluate JQ expressions in the set config
	evaluatedConfig, err := e.jqEvaluator.EvaluateMap(setConfigMap, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate JQ expressions: %w", err)
	}

	e.logger.Debug("Evaluated set task data",
		"original", setConfigMap,
		"evaluated", evaluatedConfig)

	return evaluatedConfig, nil
}

// buildWorkflowContext builds the workflow context for JQ expression evaluation
func (e *ServerlessWorkflowEngine) buildWorkflowContext(state *gen.WorkflowState) map[string]interface{} {
	context := make(map[string]interface{})

	// Add workflow variables directly to root level for easy access
	if state.Variables != nil {
		for key, value := range state.Variables.AsMap() {
			context[key] = value
		}
	}

	// Add workflow input directly to root level
	if state.Input != nil {
		for key, value := range state.Input.AsMap() {
			// Don't overwrite variables with input
			if _, exists := context[key]; !exists {
				context[key] = value
			}
		}
	}

	// Add special functions
	context["now"] = func() string {
		return time.Now().Format(time.RFC3339)
	}

	// Also keep the nested structure for backward compatibility
	workflow := make(map[string]interface{})
	if state.Input != nil {
		workflow["input"] = state.Input.AsMap()
	}
	if state.Variables != nil {
		workflow["variables"] = state.Variables.AsMap()
	}
	context["workflow"] = workflow

	return context
}

// scheduleWaitCompletionEvent schedules a wait completion event using the native event stream
func (e *ServerlessWorkflowEngine) scheduleWaitCompletionEvent(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState, duration time.Duration) error {
	// Get tenant and execution IDs from state context
	tenantID := ""
	executionID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
		executionID = state.Context.ExecutionId
	}

	// Instead of local sleep, publish a timer.scheduled event and let timer-service fire it
	resumeAt := time.Now().Add(duration)
	scheduled := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("timer-scheduled-%s-%d", task.Name, time.Now().Unix()),
		Source:      "workflow-engine",
		SpecVersion: "1.0",
		Type:        "io.flunq.timer.scheduled",
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		TaskID:      task.Name,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"tenant_id":    tenantID,
			"workflow_id":  state.WorkflowId,
			"execution_id": executionID,
			"task_id":      task.Name,
			"task_name":    task.Name,
			"duration_ms":  duration.Milliseconds(),
			"fire_at":      resumeAt.Format(time.RFC3339),
		},
	}
	if err := e.eventStream.Publish(ctx, scheduled); err != nil {
		return fmt.Errorf("failed to publish timer.scheduled: %w", err)
	}

	e.logger.Info("Scheduled wait completion event",
		"workflow_id", state.WorkflowId,
		"task_name", task.Name,
		"duration", duration,
		"completion_time", time.Now().Add(duration).Format(time.RFC3339))

	return nil
}

// buildTryTaskConfig passes through SDL try/catch format directly
func (e *ServerlessWorkflowEngine) buildTryTaskConfig(taskName string, taskDefMap map[string]interface{}, tryBlock interface{}) map[string]interface{} {
	// Pass through the SDL format directly - TryTaskExecutor should handle SDL natively
	tryConfig := make(map[string]interface{})

	// Add the try block as-is
	tryConfig["try"] = tryBlock

	// Add catch configuration if present
	if catchConfig, hasCatch := taskDefMap["catch"]; hasCatch {
		tryConfig["catch"] = catchConfig
	}

	e.logger.Debug("Built SDL try/catch configuration",
		"task_name", taskName,
		"has_try", tryBlock != nil,
		"has_catch", tryConfig["catch"] != nil)

	return tryConfig
}

// processTaskFailedEvent processes task failed events and handles try/catch/retry logic
func (e *ServerlessWorkflowEngine) processTaskFailedEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	e.logger.Info("Processing task failed event",
		"workflow_id", state.WorkflowId,
		"event_id", event.ID,
		"task_id", event.TaskID)

	// Extract task information from the event
	eventData, ok := event.Data.(map[string]interface{})
	if !ok {
		e.logger.Error("Task failed event has invalid data format", "event_id", event.ID)
		return fmt.Errorf("invalid task failed event data")
	}

	// Extract task name and error details
	taskName, ok := eventData["task_name"].(string)
	if !ok {
		e.logger.Error("Task failed event missing task_name", "event_id", event.ID)
		return fmt.Errorf("task failed event missing task_name")
	}

	errorMessage := ""
	if err, exists := eventData["error"]; exists {
		errorMessage = fmt.Sprintf("%v", err)
	}

	statusCode := 0
	if code, exists := eventData["status_code"]; exists {
		if codeFloat, ok := code.(float64); ok {
			statusCode = int(codeFloat)
		}
	}

	e.logger.Info("Task failed details",
		"task_name", taskName,
		"error", errorMessage,
		"status_code", statusCode)

	// Check if this task has try/catch configuration
	tryCatchConfig := e.extractTryCatchConfigFromState(state, taskName)
	if tryCatchConfig == nil {
		e.logger.Info("No try/catch configuration found, treating as final failure", "task_name", taskName)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	e.logger.Info("Found try/catch configuration, processing error handling", "task_name", taskName)
	return e.handleTaskFailureWithTryCatch(state, event, taskName, tryCatchConfig, statusCode, errorMessage)
}

// extractTryCatchConfigFromState extracts try/catch configuration from the workflow state
func (e *ServerlessWorkflowEngine) extractTryCatchConfigFromState(state *gen.WorkflowState, taskName string) map[string]interface{} {
	// Look for try/catch configuration in pending tasks
	for _, pendingTask := range state.PendingTasks {
		if pendingTask.Name == taskName && pendingTask.Input != nil {
			inputMap := pendingTask.Input.AsMap()
			if tryCatchConfig, exists := inputMap["try_catch_config"]; exists {
				if configMap, ok := tryCatchConfig.(map[string]interface{}); ok {
					e.logger.Info("Found try/catch config in pending task", "task_name", taskName)
					return configMap
				}
			}
		}
	}

	// Look for try/catch configuration in completed tasks (for retry scenarios)
	for _, completedTask := range state.CompletedTasks {
		if completedTask.Name == taskName {
			// CompletedTask doesn't have Input field, so we'll need to get it from the original task definition
			e.logger.Debug("Found completed task, but cannot access input directly", "task_name", taskName)
			// We'll rely on the pending task configuration for now
		}
	}

	e.logger.Debug("No try/catch configuration found for task", "task_name", taskName)
	return nil
}

// handleTaskFailureWithoutTryCatch handles task failures when no try/catch configuration is present
func (e *ServerlessWorkflowEngine) handleTaskFailureWithoutTryCatch(state *gen.WorkflowState, event *cloudevents.CloudEvent, taskName, errorMessage string) error {
	e.logger.Error("Task failed without try/catch configuration - workflow will fail",
		"task_name", taskName,
		"error", errorMessage,
		"workflow_id", state.WorkflowId)

	// Mark workflow as failed
	state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_FAILED
	state.UpdatedAt = timestamppb.New(event.Time)

	// Publish workflow failed event
	return e.publishWorkflowFailedEvent(state, taskName, errorMessage)
}

// handleTaskFailureWithTryCatch handles task failures when try/catch configuration is present
func (e *ServerlessWorkflowEngine) handleTaskFailureWithTryCatch(state *gen.WorkflowState, event *cloudevents.CloudEvent, taskName string, tryCatchConfig map[string]interface{}, statusCode int, errorMessage string) error {
	e.logger.Info("Processing task failure with try/catch configuration",
		"task_name", taskName,
		"status_code", statusCode,
		"error", errorMessage)

	// Extract catch configuration
	catchConfig, hasCatch := tryCatchConfig["catch"].(map[string]interface{})
	if !hasCatch {
		e.logger.Warn("Try/catch config found but no catch section", "task_name", taskName)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	// Check if this error matches the catch criteria
	if !e.errorMatchesCatchCriteria(catchConfig, statusCode, errorMessage) {
		e.logger.Info("Error does not match catch criteria, treating as final failure",
			"task_name", taskName,
			"status_code", statusCode)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	// Extract retry configuration
	retryConfig, hasRetry := catchConfig["retry"].(map[string]interface{})
	if !hasRetry {
		e.logger.Info("Error matches catch criteria but no retry configuration", "task_name", taskName)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	// Check retry limits and schedule retry
	return e.scheduleTaskRetry(state, event, taskName, retryConfig, errorMessage)
}

// publishWorkflowFailedEvent publishes a workflow failed event
func (e *ServerlessWorkflowEngine) publishWorkflowFailedEvent(state *gen.WorkflowState, taskName, errorMessage string) error {
	failedEvent := &cloudevents.CloudEvent{
		Type:       "io.flunq.workflow.failed",
		Source:     "worker-service",
		WorkflowID: state.WorkflowId,
		TaskID:     "",
		Time:       time.Now(),
		Data: map[string]interface{}{
			"workflow_id":  state.WorkflowId,
			"failed_task":  taskName,
			"error":        errorMessage,
			"final_status": "failed",
		},
	}

	e.logger.Error("Publishing workflow failed event",
		"workflow_id", state.WorkflowId,
		"failed_task", taskName,
		"error", errorMessage)

	return e.eventStream.Publish(context.Background(), failedEvent)
}

// errorMatchesCatchCriteria checks if the error matches the catch criteria
func (e *ServerlessWorkflowEngine) errorMatchesCatchCriteria(catchConfig map[string]interface{}, statusCode int, errorMessage string) bool {
	// Extract errors configuration
	errorsConfig, hasErrors := catchConfig["errors"].(map[string]interface{})
	if !hasErrors {
		e.logger.Debug("No errors configuration in catch block, matching all errors")
		return true // If no specific error criteria, catch all errors
	}

	// Check status code criteria
	if withConfig, hasWith := errorsConfig["with"].(map[string]interface{}); hasWith {
		if expectedStatus, hasStatus := withConfig["status"]; hasStatus {
			if expectedStatusFloat, ok := expectedStatus.(float64); ok {
				expectedStatusInt := int(expectedStatusFloat)
				if statusCode == expectedStatusInt {
					e.logger.Info("Error matches catch criteria by status code",
						"expected_status", expectedStatusInt,
						"actual_status", statusCode)
					return true
				}
			}
		}
	}

	e.logger.Debug("Error does not match catch criteria",
		"status_code", statusCode,
		"error", errorMessage)
	return false
}

// scheduleTaskRetry schedules a task retry using the timer service
func (e *ServerlessWorkflowEngine) scheduleTaskRetry(state *gen.WorkflowState, event *cloudevents.CloudEvent, taskName string, retryConfig map[string]interface{}, errorMessage string) error {
	e.logger.Info("Scheduling task retry",
		"task_name", taskName,
		"retry_config", retryConfig)

	// Extract retry limit
	limitConfig, hasLimit := retryConfig["limit"].(map[string]interface{})
	if !hasLimit {
		e.logger.Warn("No retry limit configuration", "task_name", taskName)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	attemptConfig, hasAttempt := limitConfig["attempt"].(map[string]interface{})
	if !hasAttempt {
		e.logger.Warn("No attempt configuration in retry limit", "task_name", taskName)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	maxAttempts := 3 // Default
	if count, hasCount := attemptConfig["count"]; hasCount {
		if countFloat, ok := count.(float64); ok {
			maxAttempts = int(countFloat)
		}
	}

	// Get current attempt count (we'll implement this tracking)
	currentAttempt := e.getCurrentAttemptCount(state, taskName)

	if currentAttempt >= maxAttempts {
		e.logger.Info("Maximum retry attempts reached",
			"task_name", taskName,
			"current_attempt", currentAttempt,
			"max_attempts", maxAttempts)
		return e.handleTaskFailureWithoutTryCatch(state, event, taskName, errorMessage)
	}

	// Extract delay configuration
	delayConfig, hasDelay := retryConfig["delay"].(map[string]interface{})
	delaySeconds := 3 // Default delay
	if hasDelay {
		if seconds, hasSeconds := delayConfig["seconds"]; hasSeconds {
			if secondsFloat, ok := seconds.(float64); ok {
				delaySeconds = int(secondsFloat)
			}
		}
	}

	// Schedule retry timer
	return e.scheduleRetryTimer(state, taskName, currentAttempt+1, delaySeconds, errorMessage)
}

// getCurrentAttemptCount gets the current retry attempt count for a task
func (e *ServerlessWorkflowEngine) getCurrentAttemptCount(state *gen.WorkflowState, taskName string) int {
	// For now, we'll use a simple approach - count failed attempts in completed tasks
	// In a production system, you'd want to track this more systematically
	attemptCount := 0

	for _, completedTask := range state.CompletedTasks {
		if completedTask.Name == taskName {
			// For now, we'll assume this is a retry attempt
			// In a production system, you'd track retry attempts more systematically
			attemptCount++
		}
	}

	e.logger.Debug("Current attempt count for task",
		"task_name", taskName,
		"attempt_count", attemptCount)

	return attemptCount
}

// scheduleRetryTimer schedules a retry timer for a failed task
func (e *ServerlessWorkflowEngine) scheduleRetryTimer(state *gen.WorkflowState, taskName string, attemptNumber, delaySeconds int, errorMessage string) error {
	timerID := fmt.Sprintf("retry-%s-%d-%d", taskName, attemptNumber, time.Now().Unix())

	timerEvent := &cloudevents.CloudEvent{
		Type:       "io.flunq.timer.requested",
		Source:     "worker-service",
		WorkflowID: state.WorkflowId,
		TaskID:     timerID,
		Time:       time.Now(),
		Data: map[string]interface{}{
			"timer_id":           timerID,
			"workflow_id":        state.WorkflowId,
			"delay_seconds":      delaySeconds,
			"retry_type":         "task_retry",
			"original_task_name": taskName,
			"attempt_count":      attemptNumber,
			"original_error":     errorMessage,
		},
	}

	e.logger.Info("Scheduling retry timer",
		"task_name", taskName,
		"attempt_number", attemptNumber,
		"delay_seconds", delaySeconds,
		"timer_id", timerID)

	return e.eventStream.Publish(context.Background(), timerEvent)
}

// processTimerFiredEvent processes timer.fired events, distinguishing between wait tasks and retry timers
func (e *ServerlessWorkflowEngine) processTimerFiredEvent(state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	e.logger.Debug("Processing timer fired event",
		"workflow_id", state.WorkflowId,
		"event_id", event.ID,
		"task_id", event.TaskID)

	// Extract event data to determine timer type
	eventData, ok := event.Data.(map[string]interface{})
	if !ok {
		e.logger.Warn("Timer fired event has invalid data format", "event_id", event.ID)
		// Fallback to treating as wait task completion
		return e.processTaskCompletedEvent(state, event)
	}

	// Check if this is a retry timer
	if retryType, exists := eventData["retry_type"]; exists && retryType == "task_retry" {
		return e.processRetryTimerFired(state, event, eventData)
	}

	// Default: treat as wait task completion
	return e.processTaskCompletedEvent(state, event)
}

// processRetryTimerFired handles retry timer events by re-executing the original task
func (e *ServerlessWorkflowEngine) processRetryTimerFired(state *gen.WorkflowState, event *cloudevents.CloudEvent, eventData map[string]interface{}) error {
	e.logger.Info("Processing retry timer fired",
		"workflow_id", state.WorkflowId,
		"event_id", event.ID,
		"task_id", event.TaskID)

	// Extract retry information
	originalTaskID, ok := eventData["original_task_id"].(string)
	if !ok {
		return fmt.Errorf("retry timer missing original_task_id")
	}

	attemptCount, ok := eventData["attempt_count"].(float64)
	if !ok {
		return fmt.Errorf("retry timer missing attempt_count")
	}

	e.logger.Info("Re-executing task after retry delay",
		"workflow_id", state.WorkflowId,
		"original_task_id", originalTaskID,
		"attempt_count", int(attemptCount))

	// Extract task name from original task ID (remove timestamp suffix)
	taskName := e.extractTaskNameFromTaskID(originalTaskID)
	if taskName == "" {
		return fmt.Errorf("could not extract task name from task ID: %s", originalTaskID)
	}

	// We need the workflow definition to rebuild the task
	// For now, we'll create a simplified retry task.requested event
	// In a full implementation, we'd need to store the original task definition

	// Create a new task.requested event for the retry
	retryTaskID := fmt.Sprintf("task-%s-%d", taskName, time.Now().UnixNano())

	// Build basic task input with retry context
	taskInput := map[string]interface{}{
		"task_name":   taskName,
		"workflow_id": state.WorkflowId,
		"timestamp":   time.Now().Format(time.RFC3339),
		"retry_context": map[string]interface{}{
			"is_retry":         true,
			"attempt_count":    int(attemptCount),
			"original_task_id": originalTaskID,
		},
	}

	// Add workflow input to task context for parameter evaluation
	if state.Input != nil {
		taskInput["workflow_input"] = state.Input.AsMap()
	}

	// IMPORTANT: We need to reconstruct the original task definition for retries
	// For now, we'll extract it from the timer event data if available
	if originalTaskDef, exists := eventData["original_task_definition"]; exists {
		taskInput["task_definition"] = originalTaskDef
	}

	// Determine task type (simplified - in full implementation would come from DSL)
	taskType := "try" // Since we're retrying a failed try task

	// Publish task.requested event for the retry
	return e.publishRetryTaskRequestedEvent(state, retryTaskID, taskName, taskType, taskInput)
}

// extractTaskNameFromTaskID extracts the task name from a task ID
// Task IDs are typically in format: "task-{taskName}-{timestamp}"
func (e *ServerlessWorkflowEngine) extractTaskNameFromTaskID(taskID string) string {
	// Remove "task-" prefix if present
	if strings.HasPrefix(taskID, "task-") {
		taskID = taskID[5:]
	}

	// Find the last dash and remove timestamp suffix
	lastDash := strings.LastIndex(taskID, "-")
	if lastDash > 0 {
		return taskID[:lastDash]
	}

	return taskID
}

// publishRetryTaskRequestedEvent publishes a task.requested event for a retry
func (e *ServerlessWorkflowEngine) publishRetryTaskRequestedEvent(state *gen.WorkflowState, taskID, taskName, taskType string, taskInput map[string]interface{}) error {
	// Extract tenant and execution IDs
	tenantID := ""
	executionID := ""
	if state.Context != nil {
		tenantID = state.Context.TenantId
		executionID = state.Context.ExecutionId
	}

	// Create task.requested event
	event := &cloudevents.CloudEvent{
		ID:          fmt.Sprintf("task-requested-%s", taskID),
		Source:      "workflow-engine",
		SpecVersion: "1.0",
		Type:        "io.flunq.task.requested",
		TenantID:    tenantID,
		WorkflowID:  state.WorkflowId,
		ExecutionID: executionID,
		TaskID:      taskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    taskName,
			"task_type":    taskType,
			"workflow_id":  state.WorkflowId,
			"execution_id": executionID,
			"input":        taskInput,
			"context": map[string]interface{}{
				"workflow_input": taskInput["workflow_input"],
				"retry_context":  taskInput["retry_context"],
			},
		},
	}

	// Publish the event
	if err := e.eventStream.Publish(context.Background(), event); err != nil {
		return fmt.Errorf("failed to publish retry task.requested event: %w", err)
	}

	e.logger.Info("Published retry task.requested event",
		"task_id", taskID,
		"task_name", taskName,
		"task_type", taskType,
		"workflow_id", state.WorkflowId,
		"execution_id", executionID)

	return nil
}
