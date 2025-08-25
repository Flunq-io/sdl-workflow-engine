package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TryTaskExecutor handles 'try' type tasks with error handling and retry logic
type TryTaskExecutor struct {
	logger        *zap.Logger
	errorHandler  ErrorHandler
	taskExecutors map[string]TaskExecutor
	eventStream   sharedinterfaces.EventStream
}

// NewTryTaskExecutor creates a new TryTaskExecutor
func NewTryTaskExecutor(logger *zap.Logger, taskExecutors map[string]TaskExecutor, eventStream sharedinterfaces.EventStream) TaskExecutor {
	return &TryTaskExecutor{
		logger:        logger,
		errorHandler:  NewErrorHandler(logger),
		taskExecutors: taskExecutors,
		eventStream:   eventStream,
	}
}

// GetTaskType returns the task type this executor handles
func (e *TryTaskExecutor) GetTaskType() string {
	return "try"
}

// Execute executes a try task with error handling and retry logic
func (e *TryTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()

	// Add multiple prominent log entries that can't be missed
	e.logger.Error("🚨🚨🚨 TRY TASK EXECUTOR CALLED 🚨🚨🚨")
	e.logger.Error("🚨🚨🚨 TRY TASK EXECUTOR CALLED 🚨🚨🚨")
	e.logger.Error("🚨🚨🚨 TRY TASK EXECUTOR CALLED 🚨🚨🚨")
	e.logger.Info("🎯 EXECUTING TRY TASK - TryTaskExecutor IS BEING CALLED!",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.String("workflow_id", task.WorkflowID))

	// Parse try task configuration
	tryConfig, err := e.parseTryConfig(task)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("invalid try configuration: %v", err))
	}

	// Execute try block with error handling
	result, err := e.executeTryBlock(ctx, task, tryConfig, startTime)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("try block execution failed: %v", err))
	}

	return result, nil
}

// parseTryConfig parses the SDL try task configuration from the task input
func (e *TryTaskExecutor) parseTryConfig(task *TaskRequest) (*TryTaskConfig, error) {
	if task.Input == nil {
		return nil, fmt.Errorf("missing task input")
	}

	// The SDL configuration is in task.Input under "try_catch_config" or "try_config"
	var tryConfigData map[string]interface{}

	if tryConfig, exists := task.Input["try_catch_config"]; exists {
		if configMap, ok := tryConfig.(map[string]interface{}); ok {
			tryConfigData = configMap
		}
	} else if tryConfig, exists := task.Input["try_config"]; exists {
		if configMap, ok := tryConfig.(map[string]interface{}); ok {
			tryConfigData = configMap
		}
	}

	if tryConfigData == nil {
		return nil, fmt.Errorf("try configuration not found in task input")
	}

	// Extract try and catch blocks from SDL format
	tryBlock, hasTry := tryConfigData["try"]
	if !hasTry {
		return nil, fmt.Errorf("try block not found in task configuration")
	}

	catchBlock, hasCatch := tryConfigData["catch"]
	if !hasCatch {
		return nil, fmt.Errorf("catch block is required")
	}

	// Create TryTaskConfig with SDL format
	tryConfig := &TryTaskConfig{
		Try:   tryBlock,   // Store SDL try block directly
		Catch: catchBlock, // Store SDL catch block directly
	}

	e.logger.Debug("Parsed SDL try/catch configuration",
		zap.String("task_id", task.TaskID),
		zap.Bool("has_try", tryBlock != nil),
		zap.Bool("has_catch", catchBlock != nil))

	return tryConfig, nil
}

// executeTryBlock executes the try block with error handling and retry logic
func (e *TryTaskExecutor) executeTryBlock(ctx context.Context, parentTask *TaskRequest, config *TryTaskConfig, startTime time.Time) (*TaskResult, error) {
	// Check if this is a retry and extract existing attempt count
	initialAttemptCount := 0
	if retryContext, exists := parentTask.Input["retry_context"]; exists {
		if retryMap, ok := retryContext.(map[string]interface{}); ok {
			if isRetry, ok := retryMap["is_retry"].(bool); ok && isRetry {
				if attemptCount, ok := retryMap["attempt_count"].(int); ok {
					initialAttemptCount = attemptCount
					e.logger.Info("Continuing retry execution",
						zap.String("task_id", parentTask.TaskID),
						zap.Int("current_attempt", initialAttemptCount))
				}
			}
		}
	}

	errorCtx := &ErrorContext{
		AttemptCount:  initialAttemptCount, // Start from existing attempt count
		TotalDuration: 0,
		LastAttempt:   startTime,
		TaskInput:     parentTask.Input,
		WorkflowData:  make(map[string]interface{}), // TODO: Get from workflow context
	}

	var lastResult *TaskResult
	var lastError error

	// SIMPLE RETRY LIMIT - HARD CODED TO 5 ATTEMPTS
	maxAttempts := 5

	for {
		errorCtx.AttemptCount++
		errorCtx.LastAttempt = time.Now()

		e.logger.Error("🔄 ATTEMPT NUMBER", zap.Int("attempt", errorCtx.AttemptCount), zap.Int("max_attempts", maxAttempts))

		// HARD STOP AFTER 5 ATTEMPTS
		if errorCtx.AttemptCount > maxAttempts {
			e.logger.Error("🚨 MAX ATTEMPTS EXCEEDED - STOPPING", zap.Int("attempts", errorCtx.AttemptCount))
			return e.createErrorResult(parentTask, startTime, fmt.Sprintf("max attempts exceeded: %d", errorCtx.AttemptCount))
		}

		e.logger.Debug("Attempting try block execution",
			zap.String("task_id", parentTask.TaskID),
			zap.Int("attempt", errorCtx.AttemptCount))

		// Execute the try tasks
		result, err := e.executeTryTasks(ctx, parentTask, config.Try)

		if err == nil {
			// Success - return the result
			e.logger.Info("Try block executed successfully",
				zap.String("task_id", parentTask.TaskID),
				zap.Int("attempts", errorCtx.AttemptCount))
			return result, nil
		}

		// Error occurred - handle it
		lastError = err
		lastResult = result

		// Update error context
		errorCtx.TotalDuration = time.Since(startTime)

		// Convert SDL catch block to CatchConfig
		catchConfig, catchErr := e.convertSDLCatchConfig(config.Catch)
		if catchErr != nil {
			return e.createErrorResult(parentTask, startTime, fmt.Sprintf("invalid catch configuration: %v", catchErr))
		}

		// Handle the error using the catch configuration
		handlingResult, handleErr := e.errorHandler.HandleError(ctx, lastError, catchConfig, errorCtx)
		if handleErr != nil {
			return e.createErrorResult(parentTask, startTime, fmt.Sprintf("error handling failed: %v", handleErr))
		}

		switch handlingResult.Action {
		case ErrorActionRetry:
			// Schedule retry via timer service instead of blocking
			if handlingResult.RetryDelay > 0 {
				e.logger.Info("Scheduling retry via timer service",
					zap.String("task_id", parentTask.TaskID),
					zap.Duration("delay", handlingResult.RetryDelay),
					zap.Int("attempt", errorCtx.AttemptCount))

				if err := e.scheduleRetry(ctx, parentTask, handlingResult.RetryDelay, errorCtx.AttemptCount); err != nil {
					e.logger.Error("Failed to schedule retry", zap.Error(err))
					return e.createErrorResult(parentTask, startTime, fmt.Sprintf("failed to schedule retry: %v", err))
				}

				// Return a special "retry scheduled" result that indicates the task is pending retry
				return &TaskResult{
					TaskID:      parentTask.TaskID,
					TaskName:    parentTask.TaskName,
					TaskType:    parentTask.TaskType,
					WorkflowID:  parentTask.WorkflowID,
					ExecutionID: parentTask.ExecutionID,
					Success:     false, // Task is not complete yet
					Input:       parentTask.Input,
					Output: map[string]interface{}{
						"retry_scheduled": true,
						"attempt_count":   errorCtx.AttemptCount,
						"retry_delay":     handlingResult.RetryDelay.String(),
					},
					Error:      fmt.Sprintf("Retry %d scheduled for %v", errorCtx.AttemptCount, handlingResult.RetryDelay),
					Duration:   time.Since(startTime),
					StartedAt:  startTime,
					ExecutedAt: time.Now(),
				}, nil
			}
			continue

		case ErrorActionRecover:
			// Execute recovery tasks
			e.logger.Info("Executing recovery tasks", zap.String("task_id", parentTask.TaskID))
			recoveryResult, recoveryErr := e.executeRecoveryTasks(ctx, parentTask, handlingResult.RecoveryTasks)
			if recoveryErr != nil {
				return e.createErrorResult(parentTask, startTime, fmt.Sprintf("recovery tasks failed: %v", recoveryErr))
			}

			// Merge error data into the result
			if recoveryResult.Output == nil {
				recoveryResult.Output = make(map[string]interface{})
			}
			for key, value := range handlingResult.ErrorData {
				recoveryResult.Output[key] = value
			}

			return recoveryResult, nil

		case ErrorActionIgnore:
			// Error was caught and ignored - return success with error data
			e.logger.Info("Error caught and ignored", zap.String("task_id", parentTask.TaskID))

			output := make(map[string]interface{})
			for key, value := range handlingResult.ErrorData {
				output[key] = value
			}

			return &TaskResult{
				TaskID:      parentTask.TaskID,
				TaskName:    parentTask.TaskName,
				TaskType:    parentTask.TaskType,
				WorkflowID:  parentTask.WorkflowID,
				ExecutionID: parentTask.ExecutionID,
				Success:     true,
				Input:       parentTask.Input,
				Output:      output,
				Duration:    time.Since(startTime),
				StartedAt:   startTime,
				ExecutedAt:  time.Now(),
			}, nil

		case ErrorActionFail:
		default:
			// Error was not caught or retry limits exceeded - fail the task
			e.logger.Error("Try task failed after error handling",
				zap.String("task_id", parentTask.TaskID),
				zap.Error(lastError),
				zap.Int("attempts", errorCtx.AttemptCount))

			if lastResult != nil {
				return lastResult, nil
			}
			return e.createErrorResult(parentTask, startTime, fmt.Sprintf("try task failed: %v", lastError))
		}
	}
}

// executeTryTasks executes the SDL try block directly
func (e *TryTaskExecutor) executeTryTasks(ctx context.Context, parentTask *TaskRequest, tryBlock interface{}) (*TaskResult, error) {
	e.logger.Debug("Executing SDL try block",
		zap.String("parent_task_id", parentTask.TaskID),
		zap.String("task_name", parentTask.TaskName))

	// Parse the SDL try block
	tryMap, ok := tryBlock.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("try block is not a valid map")
	}

	// Handle different types of tasks in the try block
	if callType, hasCall := tryMap["call"]; hasCall {
		return e.executeSDLCall(ctx, parentTask, tryMap, callType)
	}

	// Add support for other SDL task types here (set, wait, etc.)
	return nil, fmt.Errorf("unsupported task type in try block")
}

// executeSDLCall executes an SDL call task within a try block
func (e *TryTaskExecutor) executeSDLCall(ctx context.Context, parentTask *TaskRequest, tryMap map[string]interface{}, callType interface{}) (*TaskResult, error) {
	e.logger.Debug("Executing SDL call within try block",
		zap.String("parent_task_id", parentTask.TaskID),
		zap.Any("call_type", callType))

	// Create a call task request from the SDL format using the original task identity
	callTaskReq := &TaskRequest{
		TaskID:      parentTask.TaskID,   // Keep original task ID
		TaskName:    parentTask.TaskName, // Keep original task name
		TaskType:    "call",
		WorkflowID:  parentTask.WorkflowID,
		ExecutionID: parentTask.ExecutionID,
		Input:       parentTask.Input,
		Context:     parentTask.Context,
		Config: &TaskConfig{
			Parameters: e.buildCallConfigFromSDL(tryMap, callType),
		},
	}

	// Get the call task executor
	callExecutor, exists := e.taskExecutors["call"]
	if !exists {
		return nil, fmt.Errorf("no call executor found")
	}

	// Execute the call task
	result, err := callExecutor.Execute(ctx, callTaskReq)
	if err != nil {
		return result, err // Return error for retry handling
	}

	return result, nil
}

// buildCallConfigFromSDL builds call task configuration from SDL format
func (e *TryTaskExecutor) buildCallConfigFromSDL(tryMap map[string]interface{}, callType interface{}) map[string]interface{} {
	config := make(map[string]interface{})

	// Set the call type (e.g., "openapi")
	config["call_type"] = callType

	// Handle SDL "with" syntax - the actual call configuration is inside "with"
	if withConfig, exists := tryMap["with"]; exists {
		if withMap, ok := withConfig.(map[string]interface{}); ok {
			// Map SDL fields to executor configuration with proper field name conversion
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
		}
	}

	// Handle other SDL properties
	for key, value := range tryMap {
		switch key {
		case "call", "with":
			// Skip these - already handled above
			continue
		case "output":
			// Handle SDL output configuration separately
			// The CallConfig expects a simple string for output format
			config["output"] = "content" // Default output format
			// Store the SDL output config separately for later processing
			config["output_config"] = value
		default:
			// Copy other properties directly (if not already in "with")
			if _, exists := config[key]; !exists {
				config[key] = value
			}
		}
	}

	e.logger.Debug("Built call config from SDL",
		zap.Any("original_try_map", tryMap),
		zap.Any("built_config", config))

	return config
}

// convertSDLCatchConfig converts SDL catch block to CatchConfig
func (e *TryTaskExecutor) convertSDLCatchConfig(sdlCatch interface{}) (*CatchConfig, error) {
	// Convert to JSON and back to parse into CatchConfig
	configBytes, err := json.Marshal(sdlCatch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SDL catch config: %w", err)
	}

	var catchConfig CatchConfig
	if err := json.Unmarshal(configBytes, &catchConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catch config: %w", err)
	}

	return &catchConfig, nil
}

// executeRecoveryTasks executes the recovery tasks in the catch block
func (e *TryTaskExecutor) executeRecoveryTasks(ctx context.Context, parentTask *TaskRequest, recoveryTasks map[string]*TaskRequest) (*TaskResult, error) {
	// Similar to executeTryTasks, but for recovery tasks
	var lastResult *TaskResult

	for taskName, taskReq := range recoveryTasks {
		e.logger.Debug("Executing recovery task",
			zap.String("parent_task_id", parentTask.TaskID),
			zap.String("recovery_task_name", taskName))

		// Get the appropriate executor for this task type
		executor, exists := e.taskExecutors[taskReq.TaskType]
		if !exists {
			return nil, fmt.Errorf("no executor found for task type: %s", taskReq.TaskType)
		}

		// Execute the recovery task
		result, err := executor.Execute(ctx, taskReq)
		if err != nil {
			return result, fmt.Errorf("recovery task %s failed: %w", taskName, err)
		}

		if !result.Success {
			return result, fmt.Errorf("recovery task %s failed: %s", taskName, result.Error)
		}

		lastResult = result
	}

	return lastResult, nil
}

// createErrorResult creates a TaskResult for error cases
func (e *TryTaskExecutor) createErrorResult(task *TaskRequest, startTime time.Time, errorMsg string) (*TaskResult, error) {
	duration := time.Since(startTime)

	e.logger.Error("Try task failed",
		zap.String("task_id", task.TaskID),
		zap.String("error", errorMsg),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     false,
		Input:       task.Input,
		Output:      map[string]interface{}{},
		Error:       errorMsg,
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}, nil
}

// scheduleRetry schedules a retry via the timer service
func (e *TryTaskExecutor) scheduleRetry(ctx context.Context, task *TaskRequest, delay time.Duration, attemptCount int) error {
	retryAt := time.Now().Add(delay)

	// Create a unique retry task ID
	retryTaskID := fmt.Sprintf("%s_retry_%d", task.TaskID, attemptCount)

	// Extract the original task definition to pass along for retry
	var originalTaskDef interface{}
	if task.Config != nil && task.Config.Parameters != nil {
		originalTaskDef = task.Config.Parameters
	}

	// Create timer.scheduled event for the retry
	event := &cloudevents.CloudEvent{
		ID:          uuid.New().String(),
		Source:      "executor-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.timer.scheduled",
		TenantID:    "", // TODO: Extract from task context
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		TaskID:      retryTaskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"tenant_id":                "", // TODO: Extract from task context
			"workflow_id":              task.WorkflowID,
			"execution_id":             task.ExecutionID,
			"task_id":                  retryTaskID,
			"task_name":                task.TaskName + "_retry",
			"duration_ms":              delay.Milliseconds(),
			"fire_at":                  retryAt.Format(time.RFC3339),
			"original_task_id":         task.TaskID,
			"original_task_name":       task.TaskName,
			"attempt_count":            attemptCount,
			"retry_type":               "task_retry",
			"original_task_definition": originalTaskDef,
		},
	}

	// Publish the timer event
	if err := e.eventStream.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish retry timer event: %w", err)
	}

	e.logger.Info("Scheduled retry timer",
		zap.String("original_task_id", task.TaskID),
		zap.String("retry_task_id", retryTaskID),
		zap.Duration("delay", delay),
		zap.Int("attempt", attemptCount),
		zap.String("fire_at", retryAt.Format(time.RFC3339)))

	return nil
}
