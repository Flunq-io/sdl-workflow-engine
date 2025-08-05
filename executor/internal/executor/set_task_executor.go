package executor

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// SetTaskExecutor handles 'set' type tasks
type SetTaskExecutor struct {
	logger *zap.Logger
}

// NewSetTaskExecutor creates a new SetTaskExecutor
func NewSetTaskExecutor(logger *zap.Logger) TaskExecutor {
	return &SetTaskExecutor{
		logger: logger,
	}
}

// GetTaskType returns the task type this executor handles
func (e *SetTaskExecutor) GetTaskType() string {
	return "set"
}

// Execute executes a set task
func (e *SetTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()

	e.logger.Info("Executing set task",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.String("workflow_id", task.WorkflowID))

	// Set task: Set variables in the workflow context
	output := make(map[string]interface{})

	// If config has parameters, set them as variables
	if task.Config != nil && task.Config.Parameters != nil {
		for key, value := range task.Config.Parameters {
			output[key] = value
			e.logger.Debug("Set variable",
				zap.String("key", key),
				zap.Any("value", value))
		}
	}

	// If input has data, merge it into output
	if task.Input != nil {
		for key, value := range task.Input {
			output[key] = value
		}
	}

	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)

	duration := time.Since(startTime)

	e.logger.Info("Set task completed successfully",
		zap.String("task_id", task.TaskID),
		zap.Duration("duration", duration),
		zap.Int("variables_set", len(output)))

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
