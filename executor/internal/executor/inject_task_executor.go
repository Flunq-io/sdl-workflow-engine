package executor

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// InjectTaskExecutor handles 'inject' type tasks
type InjectTaskExecutor struct {
	logger *zap.Logger
}

// NewInjectTaskExecutor creates a new InjectTaskExecutor
func NewInjectTaskExecutor(logger *zap.Logger) TaskExecutor {
	return &InjectTaskExecutor{
		logger: logger,
	}
}

// GetTaskType returns the task type this executor handles
func (e *InjectTaskExecutor) GetTaskType() string {
	return "inject"
}

// Execute executes an inject task
func (e *InjectTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()
	
	e.logger.Info("Executing inject task",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.String("workflow_id", task.WorkflowID))

	// Inject task: Inject data/variables into the workflow context
	output := make(map[string]interface{})
	
	// Inject data from config parameters
	if task.Config != nil && task.Config.Parameters != nil {
		for key, value := range task.Config.Parameters {
			output[key] = value
			e.logger.Debug("Injected variable",
				zap.String("key", key),
				zap.Any("value", value))
		}
	}
	
	// Inject data from input
	if task.Input != nil {
		for key, value := range task.Input {
			output[key] = value
		}
	}
	
	// Add metadata about the injection
	output["_injected_at"] = time.Now().Format(time.RFC3339)
	output["_injected_by"] = "task-service"
	output["_task_id"] = task.TaskID
	
	// Simulate some processing time
	time.Sleep(5 * time.Millisecond)
	
	duration := time.Since(startTime)
	
	e.logger.Info("Inject task completed successfully",
		zap.String("task_id", task.TaskID),
		zap.Duration("duration", duration),
		zap.Int("variables_injected", len(output)-3)) // Subtract metadata fields

	return &TaskResult{
		TaskID:     task.TaskID,
		TaskName:   task.TaskName,
		Success:    true,
		Output:     output,
		Duration:   duration,
		ExecutedAt: startTime,
	}, nil
}
