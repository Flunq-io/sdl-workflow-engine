package interfaces

import (
	"context"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventStore defines the unified interface for event storage and publishing
// This combines storage and streaming capabilities for convenience
type EventStore interface {
	// Storage operations
	EventStorage

	// Streaming operations  
	EventStream

	// Publishing operations
	PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error

	// Subscription operations
	Subscribe(ctx context.Context, filters StreamFilters) (StreamSubscription, error)
}

// WorkflowEngine defines the interface for Serverless Workflow execution
type WorkflowEngine interface {
	// ParseDefinition parses a Serverless Workflow DSL definition
	ParseDefinition(ctx context.Context, dslData []byte) (*WorkflowDefinition, error)

	// ValidateDefinition validates a workflow definition
	ValidateDefinition(ctx context.Context, definition *WorkflowDefinition) error

	// InitializeState creates initial workflow state from definition
	InitializeState(ctx context.Context, definition *WorkflowDefinition, input map[string]interface{}) (*WorkflowState, error)

	// RebuildState rebuilds workflow state from event history
	RebuildState(ctx context.Context, definition *WorkflowDefinition, events []*cloudevents.CloudEvent) (*WorkflowState, error)

	// GetNextTask determines the next task to execute based on current state
	GetNextTask(ctx context.Context, state *WorkflowState, definition *WorkflowDefinition) (*PendingTask, error)

	// ProcessEvent processes a single event and updates workflow state
	ProcessEvent(ctx context.Context, state *WorkflowState, event *cloudevents.CloudEvent) error

	// ExecuteTask executes a specific task type (call, run, for, if, switch, try, emit, wait, set)
	ExecuteTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)
}

// TaskExecutor defines the interface for executing different task types
type TaskExecutor interface {
	// ExecuteCallTask executes a call task (REST API, gRPC, etc.)
	ExecuteCallTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteRunTask executes a run task (script, command, etc.)
	ExecuteRunTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteForTask executes a for loop task
	ExecuteForTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteIfTask executes a conditional if task
	ExecuteIfTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteSwitchTask executes a switch task
	ExecuteSwitchTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteTryTask executes a try-catch task
	ExecuteTryTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteEmitTask executes an emit task
	ExecuteEmitTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteWaitTask executes a wait task
	ExecuteWaitTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)

	// ExecuteSetTask executes a set task
	ExecuteSetTask(ctx context.Context, task *PendingTask, state *WorkflowState) (*TaskData, error)
}

// ProtobufSerializer defines the interface for protobuf serialization
type ProtobufSerializer interface {
	// SerializeTaskData serializes task data to protobuf bytes
	SerializeTaskData(data *TaskData) ([]byte, error)

	// DeserializeTaskData deserializes task data from protobuf bytes
	DeserializeTaskData(data []byte) (*TaskData, error)

	// SerializeWorkflowState serializes workflow state to protobuf bytes
	SerializeWorkflowState(state *WorkflowState) ([]byte, error)

	// DeserializeWorkflowState deserializes workflow state from protobuf bytes
	DeserializeWorkflowState(data []byte) (*WorkflowState, error)

	// SerializeWorkflowDefinition serializes workflow definition to protobuf bytes
	SerializeWorkflowDefinition(definition *WorkflowDefinition) ([]byte, error)

	// DeserializeWorkflowDefinition deserializes workflow definition from protobuf bytes
	DeserializeWorkflowDefinition(data []byte) (*WorkflowDefinition, error)
}

// Logger defines the interface for structured logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	With(fields ...interface{}) Logger
}

// Metrics defines the interface for metrics collection
type Metrics interface {
	// IncrementCounter increments a counter metric
	IncrementCounter(name string, tags map[string]string)

	// RecordHistogram records a histogram metric
	RecordHistogram(name string, value float64, tags map[string]string)

	// RecordGauge records a gauge metric
	RecordGauge(name string, value float64, tags map[string]string)

	// StartTimer starts a timer and returns a function to stop it
	StartTimer(name string, tags map[string]string) func()
}

// PendingTask represents a task waiting to be executed
type PendingTask struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Definition  map[string]interface{} `json:"definition"`
	Input       map[string]interface{} `json:"input"`
	CreatedAt   time.Time              `json:"created_at"`
	TenantID    string                 `json:"tenant_id"`
}

// TaskData represents the result of task execution
type TaskData struct {
	TaskID      string                 `json:"task_id"`
	TaskName    string                 `json:"task_name"`
	WorkflowID  string                 `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	Output      map[string]interface{} `json:"output"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error"`
	Duration    time.Duration          `json:"duration"`
	ExecutedAt  time.Time              `json:"executed_at"`
	TenantID    string                 `json:"tenant_id"`
}
