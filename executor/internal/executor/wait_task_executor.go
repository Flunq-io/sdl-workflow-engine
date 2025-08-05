package executor

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// WaitTaskExecutor handles 'wait' type tasks
type WaitTaskExecutor struct {
	logger *zap.Logger
}

// NewWaitTaskExecutor creates a new WaitTaskExecutor
func NewWaitTaskExecutor(logger *zap.Logger) TaskExecutor {
	return &WaitTaskExecutor{
		logger: logger,
	}
}

// GetTaskType returns the task type this executor handles
func (e *WaitTaskExecutor) GetTaskType() string {
	return "wait"
}

// Execute executes a wait task
func (e *WaitTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()

	e.logger.Info("Executing wait task",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.String("workflow_id", task.WorkflowID))

	// Default wait duration
	waitDuration := 1 * time.Second

	// Extract wait parameters from config
	if task.Config != nil && task.Config.Parameters != nil {
		if duration, ok := task.Config.Parameters["duration"].(string); ok {
			if parsed, err := time.ParseDuration(duration); err == nil {
				waitDuration = parsed
			} else {
				e.logger.Warn("Invalid duration format, using default",
					zap.String("duration", duration),
					zap.Duration("default", waitDuration))
			}
		} else if seconds, ok := task.Config.Parameters["seconds"].(float64); ok {
			waitDuration = time.Duration(seconds) * time.Second
		}
	}

	e.logger.Info("Starting wait",
		zap.String("task_id", task.TaskID),
		zap.Duration("duration", waitDuration))

	// Perform the wait with context cancellation support
	select {
	case <-time.After(waitDuration):
		// Wait completed normally
	case <-ctx.Done():
		// Context was cancelled
		return e.createErrorResult(task, startTime, "wait cancelled due to context cancellation")
	}

	duration := time.Since(startTime)

	output := map[string]interface{}{
		"waited_duration": waitDuration.String(),
		"actual_duration": duration.String(),
		"completed_at":    time.Now().Format(time.RFC3339),
	}

	e.logger.Info("Wait task completed successfully",
		zap.String("task_id", task.TaskID),
		zap.Duration("waited", waitDuration),
		zap.Duration("total_duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     true,
		Input:       task.Input,
		Output:      output,
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}, nil
}

func (e *WaitTaskExecutor) createErrorResult(task *TaskRequest, startTime time.Time, errorMsg string) (*TaskResult, error) {
	duration := time.Since(startTime)

	e.logger.Error("Wait task failed",
		zap.String("task_id", task.TaskID),
		zap.String("error", errorMsg),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     false,
		Output:      map[string]interface{}{},
		Error:       errorMsg,
		Duration:    duration,
		ExecutedAt:  startTime,
	}, nil
}
