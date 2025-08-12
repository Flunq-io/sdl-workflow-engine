package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// TryTaskExecutor handles 'try' type tasks with error handling and retry logic
type TryTaskExecutor struct {
	logger        *zap.Logger
	errorHandler  ErrorHandler
	taskExecutors map[string]TaskExecutor
}

// NewTryTaskExecutor creates a new TryTaskExecutor
func NewTryTaskExecutor(logger *zap.Logger, taskExecutors map[string]TaskExecutor) TaskExecutor {
	return &TryTaskExecutor{
		logger:        logger,
		errorHandler:  NewErrorHandler(logger),
		taskExecutors: taskExecutors,
	}
}

// GetTaskType returns the task type this executor handles
func (e *TryTaskExecutor) GetTaskType() string {
	return "try"
}

// Execute executes a try task with error handling and retry logic
func (e *TryTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()

	e.logger.Info("Executing try task",
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

// parseTryConfig parses the try task configuration from the task parameters
func (e *TryTaskExecutor) parseTryConfig(task *TaskRequest) (*TryTaskConfig, error) {
	if task.Config == nil || task.Config.Parameters == nil {
		return nil, fmt.Errorf("missing try configuration")
	}

	// Convert parameters to JSON and back to parse the try configuration
	configBytes, err := json.Marshal(task.Config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal try config: %w", err)
	}

	var tryConfig TryTaskConfig
	if err := json.Unmarshal(configBytes, &tryConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal try config: %w", err)
	}

	// Validate configuration
	if tryConfig.Try == nil || len(tryConfig.Try) == 0 {
		return nil, fmt.Errorf("try block cannot be empty")
	}

	if tryConfig.Catch == nil {
		return nil, fmt.Errorf("catch block is required")
	}

	return &tryConfig, nil
}

// executeTryBlock executes the try block with error handling and retry logic
func (e *TryTaskExecutor) executeTryBlock(ctx context.Context, parentTask *TaskRequest, config *TryTaskConfig, startTime time.Time) (*TaskResult, error) {
	errorCtx := &ErrorContext{
		AttemptCount:  0,
		TotalDuration: 0,
		LastAttempt:   startTime,
		TaskInput:     parentTask.Input,
		WorkflowData:  make(map[string]interface{}), // TODO: Get from workflow context
	}

	var lastResult *TaskResult
	var lastError error

	for {
		errorCtx.AttemptCount++
		errorCtx.LastAttempt = time.Now()

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

		// Handle the error using the catch configuration
		handlingResult, handleErr := e.errorHandler.HandleError(ctx, err, config.Catch, errorCtx)
		if handleErr != nil {
			return e.createErrorResult(parentTask, startTime, fmt.Sprintf("error handling failed: %v", handleErr))
		}

		switch handlingResult.Action {
		case ErrorActionRetry:
			// Wait for retry delay and continue the loop
			if handlingResult.RetryDelay > 0 {
				e.logger.Debug("Waiting before retry",
					zap.Duration("delay", handlingResult.RetryDelay))

				select {
				case <-time.After(handlingResult.RetryDelay):
					// Continue to next iteration
				case <-ctx.Done():
					return e.createErrorResult(parentTask, startTime, "context cancelled during retry delay")
				}
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

// executeTryTasks executes the tasks in the try block
func (e *TryTaskExecutor) executeTryTasks(ctx context.Context, parentTask *TaskRequest, tryTasks map[string]*TaskRequest) (*TaskResult, error) {
	// For simplicity, we'll execute tasks sequentially
	// In a more sophisticated implementation, this could support parallel execution

	var lastResult *TaskResult

	for taskName, taskReq := range tryTasks {
		e.logger.Debug("Executing try task",
			zap.String("parent_task_id", parentTask.TaskID),
			zap.String("try_task_name", taskName))

		// Get the appropriate executor for this task type
		executor, exists := e.taskExecutors[taskReq.TaskType]
		if !exists {
			return nil, fmt.Errorf("no executor found for task type: %s", taskReq.TaskType)
		}

		// Execute the task
		result, err := executor.Execute(ctx, taskReq)
		if err != nil {
			return result, fmt.Errorf("task %s failed: %w", taskName, err)
		}

		if !result.Success {
			return result, fmt.Errorf("task %s failed: %s", taskName, result.Error)
		}

		lastResult = result
	}

	return lastResult, nil
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
