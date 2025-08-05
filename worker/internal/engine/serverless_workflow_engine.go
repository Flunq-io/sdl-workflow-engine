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

	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// ServerlessWorkflowEngine implements the WorkflowEngine interface using the official Serverless Workflow SDK
type ServerlessWorkflowEngine struct {
	logger interfaces.Logger
}

// NewServerlessWorkflowEngine creates a new Serverless Workflow engine
func NewServerlessWorkflowEngine(logger interfaces.Logger) interfaces.WorkflowEngine {
	return &ServerlessWorkflowEngine{
		logger: logger,
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
		startState = "start" // DSL 1.0.0 doesn't have explicit start state
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

	// Extract execution context
	if eventData, ok := event.Data.(map[string]interface{}); ok {
		context := &gen.ExecutionContext{
			ExecutionId: event.ExecutionID,
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
		taskName, _ := eventData["task_name"].(string)

		// Remove from pending tasks
		for i, pendingTask := range state.PendingTasks {
			if pendingTask.Name == taskName {
				state.PendingTasks = append(state.PendingTasks[:i], state.PendingTasks[i+1:]...)
				break
			}
		}

		// Extract complete TaskData from event
		var taskData *gen.TaskData
		if taskDataMap, exists := eventData["data"].(map[string]interface{}); exists {
			// Reconstruct TaskData from event data
			taskData = &gen.TaskData{
				Input:  e.mapToStruct(taskDataMap["input"]),
				Output: e.mapToStruct(taskDataMap["output"]),
				Metadata: &gen.TaskMetadata{
					CompletedAt: timestamppb.New(event.Time),
					Status:      gen.TaskStatus_TASK_STATUS_COMPLETED,
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
					CompletedAt: timestamppb.New(event.Time),
					Status:      gen.TaskStatus_TASK_STATUS_COMPLETED,
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

// extractOrderedTaskList extracts tasks from DSL in execution order
func (e *ServerlessWorkflowEngine) extractOrderedTaskList(definition *gen.WorkflowDefinition) ([]string, error) {
	if definition.DslDefinition == nil {
		e.logger.Warn("DSL definition is nil, using fallback task list")
		return []string{"initialize", "processData", "waitStep", "finalize"}, nil
	}

	dslMap := definition.DslDefinition.AsMap()
	e.logger.Info("DEBUG: DSL structure", "dsl_keys", getMapKeys(dslMap))

	// Extract tasks from "do" section (DSL 1.0.0 format)
	if doSection, ok := dslMap["do"].([]interface{}); ok {
		e.logger.Info("DEBUG: Found 'do' section", "do_section_length", len(doSection))
		taskList := make([]string, 0, len(doSection))

		for i, item := range doSection {
			e.logger.Info("DEBUG: Processing do item", "index", i, "item_type", fmt.Sprintf("%T", item))
			if taskMap, ok := item.(map[string]interface{}); ok {
				// Each item in "do" is a map with one key (the task name)
				for taskName := range taskMap {
					e.logger.Info("DEBUG: Found task", "task_name", taskName)
					taskList = append(taskList, taskName)
					break // Only process first key in each map
				}
			} else {
				e.logger.Warn("DEBUG: Item is not a map", "item", item)
			}
		}

		if len(taskList) > 0 {
			e.logger.Info("Extracted task list from DSL",
				"task_list", taskList,
				"count", len(taskList))

			return taskList, nil
		}
	} else {
		e.logger.Warn("DEBUG: No 'do' section found in DSL", "available_keys", getMapKeys(dslMap))
	}

	// Fallback for basic workflows
	e.logger.Warn("Could not extract tasks from DSL, using fallback task list")
	return []string{"initialize", "processData", "waitStep", "finalize"}, nil
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
							return "set"
						}
						if _, hasWait := taskDefMap["wait"]; hasWait {
							return "wait"
						}
						if _, hasCall := taskDefMap["call"]; hasCall {
							return "call"
						}
					}
				}
			}
		}
	}

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
								taskInput["set_data"] = setConfig
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
	injectData, exists := taskInput["inject_data"]
	if !exists {
		return nil, fmt.Errorf("no inject_data found in set task input")
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

// executeWaitTask executes a wait/sleep task
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

	// Sleep for the specified duration
	e.logger.Debug("Sleeping for duration", "task_name", task.Name, "duration", duration)
	time.Sleep(duration)

	// Create task result
	taskData := &gen.TaskData{
		Input: task.Input,
		Output: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"slept_duration": structpb.NewStringValue(duration.String()),
			},
		},
		Metadata: &gen.TaskMetadata{
			StartedAt:   timestamppb.New(startTime),
			CompletedAt: timestamppb.Now(),
			DurationMs:  time.Since(startTime).Milliseconds(),
			Status:      gen.TaskStatus_TASK_STATUS_COMPLETED,
			TaskType:    task.TaskType,
			Queue:       task.Queue,
		},
	}

	e.logger.Debug("Executed wait task", "task_name", task.Name, "duration", duration)
	return taskData, nil
}

// IsWorkflowComplete checks if the workflow has completed
func (e *ServerlessWorkflowEngine) IsWorkflowComplete(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) bool {
	// Workflow is complete if status is completed or failed
	if state.Status == gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED ||
		state.Status == gen.WorkflowStatus_WORKFLOW_STATUS_FAILED {
		e.logger.Info("Workflow complete due to status",
			"workflow_id", state.WorkflowId,
			"status", state.Status)
		return true
	}

	// For DSL 1.0.0 format, don't use state-based completion logic
	// Let GetNextTask() handle completion by returning nil when no more tasks
	dslMap := definition.DslDefinition.AsMap()
	if _, hasDoSection := dslMap["do"]; hasDoSection {
		e.logger.Debug("DSL 1.0.0 detected - using task-based completion logic",
			"workflow_id", state.WorkflowId)
		return false // Let GetNextTask() determine completion
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
