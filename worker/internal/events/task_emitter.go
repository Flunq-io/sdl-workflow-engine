package events

import (
	"context"
	"fmt"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/storage"
	"github.com/flunq-io/worker/internal/interfaces"
)

// TaskEventEmitter handles emitting task-level events for Temporal-like orchestration
type TaskEventEmitter struct {
	eventStore storage.EventStorage
	logger     interfaces.Logger
	workerID   string
}

// NewTaskEventEmitter creates a new task event emitter
func NewTaskEventEmitter(eventStore storage.EventStorage, logger interfaces.Logger, workerID string) *TaskEventEmitter {
	return &TaskEventEmitter{
		eventStore: eventStore,
		logger:     logger,
		workerID:   workerID,
	}
}

// TaskScheduledData represents data for a task scheduled event
type TaskScheduledData struct {
	TenantID       string                 `json:"tenant_id"`
	WorkflowID     string                 `json:"workflow_id"`
	ExecutionID    string                 `json:"execution_id"`
	TaskID         string                 `json:"task_id"`
	TaskName       string                 `json:"task_name"`
	TaskType       string                 `json:"task_type"`
	TaskDefinition map[string]interface{} `json:"task_definition"`
	InputData      map[string]interface{} `json:"input_data"`
	ScheduledBy    string                 `json:"scheduled_by"`
	RetryCount     int                    `json:"retry_count"`
	CorrelationID  string                 `json:"correlation_id"`
}

// TaskStartedData represents data for a task started event
type TaskStartedData struct {
	TenantID      string                 `json:"tenant_id"`
	WorkflowID    string                 `json:"workflow_id"`
	ExecutionID   string                 `json:"execution_id"`
	TaskID        string                 `json:"task_id"`
	TaskName      string                 `json:"task_name"`
	TaskType      string                 `json:"task_type"`
	InputData     map[string]interface{} `json:"input_data"`
	WorkerID      string                 `json:"worker_id"`
	WorkerVersion string                 `json:"worker_version"`
	AttemptNumber int                    `json:"attempt_number"`
	CorrelationID string                 `json:"correlation_id"`
}

// TaskCompletedData represents data for a task completed event
type TaskCompletedData struct {
	TenantID      string                 `json:"tenant_id"`
	WorkflowID    string                 `json:"workflow_id"`
	ExecutionID   string                 `json:"execution_id"`
	TaskID        string                 `json:"task_id"`
	TaskName      string                 `json:"task_name"`
	TaskType      string                 `json:"task_type"`
	InputData     map[string]interface{} `json:"input_data"`
	OutputData    map[string]interface{} `json:"output_data"`
	StartedAt     time.Time              `json:"started_at"`
	WorkerID      string                 `json:"worker_id"`
	DurationMs    int64                  `json:"duration_ms"`
	CorrelationID string                 `json:"correlation_id"`
}

// TaskFailedData represents data for a task failed event
type TaskFailedData struct {
	TenantID      string                 `json:"tenant_id"`
	WorkflowID    string                 `json:"workflow_id"`
	ExecutionID   string                 `json:"execution_id"`
	TaskID        string                 `json:"task_id"`
	TaskName      string                 `json:"task_name"`
	TaskType      string                 `json:"task_type"`
	InputData     map[string]interface{} `json:"input_data"`
	StartedAt     time.Time              `json:"started_at"`
	WorkerID      string                 `json:"worker_id"`
	ErrorType     string                 `json:"error_type"`
	ErrorMessage  string                 `json:"error_message"`
	ErrorStack    string                 `json:"error_stack_trace"`
	AttemptNumber int                    `json:"attempt_number"`
	WillRetry     bool                   `json:"will_retry"`
	NextRetryAt   *time.Time             `json:"next_retry_at,omitempty"`
	CorrelationID string                 `json:"correlation_id"`
}

// EmitTaskScheduled emits a task scheduled event
func (e *TaskEventEmitter) EmitTaskScheduled(ctx context.Context, data TaskScheduledData) error {
	event := &cloudevents.CloudEvent{
		ID:            fmt.Sprintf("task-scheduled-%s-%d", data.TaskID, time.Now().UnixNano()),
		Source:        fmt.Sprintf("tenant://%s/worker/%s", data.TenantID, e.workerID),
		SpecVersion:   "1.0",
		Type:          cloudevents.TaskScheduled,
		Time:          time.Now(),
		TenantID:      data.TenantID,
		WorkflowID:    data.WorkflowID,
		ExecutionID:   data.ExecutionID,
		TaskID:        data.TaskID,
		CorrelationID: data.CorrelationID,
		Data:          data,
	}

	if err := e.eventStore.Store(ctx, event); err != nil {
		e.logger.Error("Failed to emit task scheduled event",
			"task_id", data.TaskID,
			"workflow_id", data.WorkflowID,
			"tenant_id", data.TenantID,
			"error", err)
		return fmt.Errorf("failed to emit task scheduled event: %w", err)
	}

	e.logger.Debug("Task scheduled event emitted",
		"task_id", data.TaskID,
		"task_name", data.TaskName,
		"workflow_id", data.WorkflowID,
		"tenant_id", data.TenantID)

	return nil
}

// EmitTaskStarted emits a task started event
func (e *TaskEventEmitter) EmitTaskStarted(ctx context.Context, data TaskStartedData) error {
	event := &cloudevents.CloudEvent{
		ID:            fmt.Sprintf("task-started-%s-%d", data.TaskID, time.Now().UnixNano()),
		Source:        fmt.Sprintf("tenant://%s/worker/%s", data.TenantID, e.workerID),
		SpecVersion:   "1.0",
		Type:          cloudevents.TaskStarted,
		Time:          time.Now(),
		TenantID:      data.TenantID,
		WorkflowID:    data.WorkflowID,
		ExecutionID:   data.ExecutionID,
		TaskID:        data.TaskID,
		CorrelationID: data.CorrelationID,
		Data:          data,
	}

	if err := e.eventStore.Store(ctx, event); err != nil {
		e.logger.Error("Failed to emit task started event",
			"task_id", data.TaskID,
			"workflow_id", data.WorkflowID,
			"tenant_id", data.TenantID,
			"error", err)
		return fmt.Errorf("failed to emit task started event: %w", err)
	}

	e.logger.Info("Task started",
		"task_id", data.TaskID,
		"task_name", data.TaskName,
		"workflow_id", data.WorkflowID,
		"tenant_id", data.TenantID,
		"worker_id", data.WorkerID)

	return nil
}

// EmitTaskCompleted emits a task completed event
func (e *TaskEventEmitter) EmitTaskCompleted(ctx context.Context, data TaskCompletedData) error {
	event := &cloudevents.CloudEvent{
		ID:            fmt.Sprintf("task-completed-%s-%d", data.TaskID, time.Now().UnixNano()),
		Source:        fmt.Sprintf("tenant://%s/worker/%s", data.TenantID, e.workerID),
		SpecVersion:   "1.0",
		Type:          cloudevents.TaskCompleted,
		Time:          time.Now(),
		TenantID:      data.TenantID,
		WorkflowID:    data.WorkflowID,
		ExecutionID:   data.ExecutionID,
		TaskID:        data.TaskID,
		CorrelationID: data.CorrelationID,
		Data:          data,
	}

	if err := e.eventStore.Store(ctx, event); err != nil {
		e.logger.Error("Failed to emit task completed event",
			"task_id", data.TaskID,
			"workflow_id", data.WorkflowID,
			"tenant_id", data.TenantID,
			"error", err)
		return fmt.Errorf("failed to emit task completed event: %w", err)
	}

	e.logger.Info("Task completed",
		"task_id", data.TaskID,
		"task_name", data.TaskName,
		"workflow_id", data.WorkflowID,
		"tenant_id", data.TenantID,
		"duration_ms", data.DurationMs)

	return nil
}

// EmitTaskFailed emits a task failed event
func (e *TaskEventEmitter) EmitTaskFailed(ctx context.Context, data TaskFailedData) error {
	event := &cloudevents.CloudEvent{
		ID:            fmt.Sprintf("task-failed-%s-%d", data.TaskID, time.Now().UnixNano()),
		Source:        fmt.Sprintf("tenant://%s/worker/%s", data.TenantID, e.workerID),
		SpecVersion:   "1.0",
		Type:          cloudevents.TaskFailed,
		Time:          time.Now(),
		TenantID:      data.TenantID,
		WorkflowID:    data.WorkflowID,
		ExecutionID:   data.ExecutionID,
		TaskID:        data.TaskID,
		CorrelationID: data.CorrelationID,
		Data:          data,
	}

	if err := e.eventStore.Store(ctx, event); err != nil {
		e.logger.Error("Failed to emit task failed event",
			"task_id", data.TaskID,
			"workflow_id", data.WorkflowID,
			"tenant_id", data.TenantID,
			"error", err)
		return fmt.Errorf("failed to emit task failed event: %w", err)
	}

	e.logger.Error("Task failed",
		"task_id", data.TaskID,
		"task_name", data.TaskName,
		"workflow_id", data.WorkflowID,
		"tenant_id", data.TenantID,
		"error_type", data.ErrorType,
		"error_message", data.ErrorMessage,
		"will_retry", data.WillRetry)

	return nil
}
