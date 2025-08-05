package interfaces

import (
	"context"
	"time"

	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/worker/proto/gen"
)

// EventStore defines the interface for event queries (HTTP API)
type EventStore interface {
	// GetEventHistory retrieves all events for a workflow via HTTP API
	GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error)
}

// EventStream provides generic event streaming capabilities (Redis Streams, Kafka, RabbitMQ, etc.)
type EventStream interface {
	// Subscribe to events with optional filters
	Subscribe(ctx context.Context, filters EventStreamFilters) (EventStreamSubscription, error)
	// Publish an event to the stream
	Publish(ctx context.Context, event *cloudevents.CloudEvent) error
	// Close the stream connection
	Close() error
}

// EventStreamFilters defines filters for event subscription
type EventStreamFilters struct {
	EventTypes  []string // Filter by event types (e.g., "io.flunq.workflow.created")
	WorkflowIDs []string // Filter by workflow IDs (empty = all workflows)
	StartFrom   string   // Start reading from specific position (stream-specific format)
}

// EventStreamSubscription represents an active subscription to an event stream
type EventStreamSubscription interface {
	// Events returns channel for receiving events
	Events() <-chan *cloudevents.CloudEvent
	// Errors returns channel for receiving errors
	Errors() <-chan error
	// Close closes the subscription
	Close() error
}

// Database defines the interface for workflow metadata persistence
type Database interface {
	// CreateWorkflow creates a new workflow record
	CreateWorkflow(ctx context.Context, workflow *gen.WorkflowDefinition) error

	// UpdateWorkflowState updates the workflow state
	UpdateWorkflowState(ctx context.Context, workflowID string, state *gen.WorkflowState) error

	// GetWorkflowState retrieves the current workflow state
	GetWorkflowState(ctx context.Context, workflowID string) (*gen.WorkflowState, error)

	// GetWorkflowDefinition retrieves the workflow definition
	GetWorkflowDefinition(ctx context.Context, workflowID string) (*gen.WorkflowDefinition, error)

	// DeleteWorkflow deletes a workflow and its state
	DeleteWorkflow(ctx context.Context, workflowID string) error

	// ListWorkflows lists all workflows with optional filtering
	ListWorkflows(ctx context.Context, filters map[string]string) ([]*gen.WorkflowState, error)
}

// WorkflowEngine defines the interface for Serverless Workflow execution
type WorkflowEngine interface {
	// ParseDefinition parses a Serverless Workflow DSL definition
	ParseDefinition(ctx context.Context, dslJSON []byte) (*gen.WorkflowDefinition, error)

	// ValidateDefinition validates a workflow definition
	ValidateDefinition(ctx context.Context, definition *gen.WorkflowDefinition) error

	// InitializeState creates initial workflow state from definition
	InitializeState(ctx context.Context, definition *gen.WorkflowDefinition, input map[string]interface{}) (*gen.WorkflowState, error)

	// RebuildState rebuilds workflow state from event history
	RebuildState(ctx context.Context, definition *gen.WorkflowDefinition, events []*cloudevents.CloudEvent) (*gen.WorkflowState, error)

	// GetNextTask determines the next task to execute based on current state
	GetNextTask(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) (*gen.PendingTask, error)

	// ProcessEvent processes a single event and updates workflow state
	ProcessEvent(ctx context.Context, state *gen.WorkflowState, event *cloudevents.CloudEvent) error

	// ExecuteTask executes a specific task type (call, run, for, if, switch, try, emit, wait, set)
	ExecuteTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// IsWorkflowComplete checks if the workflow has completed
	IsWorkflowComplete(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) bool
}

// TaskExecutor defines the interface for executing different task types
type TaskExecutor interface {
	// ExecuteCallTask executes a call task (REST API, gRPC, etc.)
	ExecuteCallTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteRunTask executes a run task (script, command, etc.)
	ExecuteRunTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteForTask executes a for loop task
	ExecuteForTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteIfTask executes a conditional if task
	ExecuteIfTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteSwitchTask executes a switch task
	ExecuteSwitchTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteTryTask executes a try-catch task
	ExecuteTryTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteEmitTask executes an emit event task
	ExecuteEmitTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteWaitTask executes a wait/sleep task
	ExecuteWaitTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)

	// ExecuteSetTask executes a set variables task
	ExecuteSetTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error)
}

// ProtobufSerializer defines the interface for protobuf serialization
type ProtobufSerializer interface {
	// SerializeTaskData serializes task data to protobuf bytes
	SerializeTaskData(data *gen.TaskData) ([]byte, error)

	// DeserializeTaskData deserializes task data from protobuf bytes
	DeserializeTaskData(data []byte) (*gen.TaskData, error)

	// SerializeWorkflowState serializes workflow state to protobuf bytes
	SerializeWorkflowState(state *gen.WorkflowState) ([]byte, error)

	// DeserializeWorkflowState deserializes workflow state from protobuf bytes
	DeserializeWorkflowState(data []byte) (*gen.WorkflowState, error)

	// SerializeTaskRequestedEvent serializes task requested event to protobuf bytes
	SerializeTaskRequestedEvent(event *gen.TaskRequestedEvent) ([]byte, error)

	// DeserializeTaskRequestedEvent deserializes task requested event from protobuf bytes
	DeserializeTaskRequestedEvent(data []byte) (*gen.TaskRequestedEvent, error)

	// SerializeTaskCompletedEvent serializes task completed event to protobuf bytes
	SerializeTaskCompletedEvent(event *gen.TaskCompletedEvent) ([]byte, error)

	// DeserializeTaskCompletedEvent deserializes task completed event from protobuf bytes
	DeserializeTaskCompletedEvent(data []byte) (*gen.TaskCompletedEvent, error)
}

// WorkflowProcessor defines the main interface for processing workflow events
type WorkflowProcessor interface {
	// ProcessWorkflowEvent processes any workflow-related event
	ProcessWorkflowEvent(ctx context.Context, event *cloudevents.CloudEvent) error

	// Start starts the workflow processor
	Start(ctx context.Context) error

	// Stop stops the workflow processor
	Stop(ctx context.Context) error
}

// CircuitBreaker defines the interface for circuit breaker pattern
type CircuitBreaker interface {
	// Execute executes a function with circuit breaker protection
	Execute(fn func() error) error

	// IsOpen returns true if the circuit breaker is open
	IsOpen() bool

	// Reset resets the circuit breaker
	Reset()
}

// RetryPolicy defines the interface for retry policies
type RetryPolicy interface {
	// ShouldRetry determines if an operation should be retried
	ShouldRetry(attempt int, err error) bool

	// GetDelay returns the delay before the next retry attempt
	GetDelay(attempt int) time.Duration

	// GetMaxAttempts returns the maximum number of retry attempts
	GetMaxAttempts() int
}

// Logger defines the interface for structured logging
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, fields ...interface{})

	// Info logs an info message
	Info(msg string, fields ...interface{})

	// Warn logs a warning message
	Warn(msg string, fields ...interface{})

	// Error logs an error message
	Error(msg string, fields ...interface{})

	// Fatal logs a fatal message and exits
	Fatal(msg string, fields ...interface{})

	// With returns a logger with additional fields
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
