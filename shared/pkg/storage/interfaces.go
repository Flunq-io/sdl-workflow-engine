package storage

import (
	"context"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventStorage defines the generic interface for event storage
// Can be implemented by Redis, PostgreSQL, MongoDB, EventStore DB, etc.
type EventStorage interface {
	// Store persists an event
	Store(ctx context.Context, event *cloudevents.CloudEvent) error

	// GetEventHistory retrieves all events for a workflow (with optional tenant isolation)
	GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error)

	// GetEventHistoryForTenant retrieves all events for a workflow within a specific tenant
	GetEventHistoryForTenant(ctx context.Context, tenantID, workflowID string) ([]*cloudevents.CloudEvent, error)

	// GetExecutionHistory retrieves all events for a specific execution
	GetExecutionHistory(ctx context.Context, tenantID, executionID string) ([]*cloudevents.CloudEvent, error)

	// GetEventsSince retrieves events since a specific version/timestamp
	GetEventsSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error)

	// GetEventsByType retrieves events by type
	GetEventsByType(ctx context.Context, eventType string, limit int) ([]*cloudevents.CloudEvent, error)

	// GetEventsByTimeRange retrieves events within a time range
	GetEventsByTimeRange(ctx context.Context, start, end time.Time) ([]*cloudevents.CloudEvent, error)

	// DeleteEvents deletes events for a workflow
	DeleteEvents(ctx context.Context, workflowID string) error

	// GetStats returns storage statistics
	GetStats(ctx context.Context) (*StorageStats, error)

	// Health checks storage health
	Health(ctx context.Context) error

	// Close closes the storage connection
	Close() error
}

// EventStream defines the generic interface for event streaming
// Can be implemented by Redis Streams, Kafka, RabbitMQ, NATS, etc.
type EventStream interface {
	// Subscribe to events with filters
	Subscribe(ctx context.Context, filters StreamFilters) (StreamSubscription, error)

	// Publish an event to the stream
	Publish(ctx context.Context, event *cloudevents.CloudEvent) error

	// CreateConsumerGroup creates a consumer group
	CreateConsumerGroup(ctx context.Context, groupName string) error

	// DeleteConsumerGroup deletes a consumer group
	DeleteConsumerGroup(ctx context.Context, groupName string) error

	// GetStreamInfo returns stream information
	GetStreamInfo(ctx context.Context) (*StreamInfo, error)

	// Close closes the stream connection
	Close() error
}

// StreamFilters defines filters for event subscription
type StreamFilters struct {
	EventTypes    []string          // Filter by event types
	WorkflowIDs   []string          // Filter by workflow IDs
	Sources       []string          // Filter by event sources
	StartFrom     string            // Start reading from specific position
	ConsumerGroup string            // Consumer group name
	ConsumerName  string            // Consumer name within group
	Extensions    map[string]string // Filter by extension attributes
}

// StreamSubscription represents an active subscription to an event stream
type StreamSubscription interface {
	// Events returns channel for receiving events
	Events() <-chan *cloudevents.CloudEvent

	// Errors returns channel for receiving errors
	Errors() <-chan error

	// Acknowledge acknowledges processing of an event
	Acknowledge(ctx context.Context, eventID string) error

	// Close closes the subscription
	Close() error
}

// WorkflowStorage defines the generic interface for workflow metadata storage
// Can be implemented by Redis, PostgreSQL, MongoDB, etc.
type WorkflowStorage interface {
	// CreateWorkflow creates a new workflow definition
	CreateWorkflow(ctx context.Context, workflow *WorkflowDefinition) error

	// UpdateWorkflow updates a workflow definition
	UpdateWorkflow(ctx context.Context, workflow *WorkflowDefinition) error

	// GetWorkflow retrieves a workflow definition
	GetWorkflow(ctx context.Context, workflowID string) (*WorkflowDefinition, error)

	// DeleteWorkflow deletes a workflow definition
	DeleteWorkflow(ctx context.Context, workflowID string) error

	// ListWorkflows lists workflows with optional filters
	ListWorkflows(ctx context.Context, filters WorkflowFilters) ([]*WorkflowDefinition, error)

	// CreateExecution creates a new workflow execution
	CreateExecution(ctx context.Context, execution *WorkflowExecution) error

	// UpdateExecution updates a workflow execution
	UpdateExecution(ctx context.Context, execution *WorkflowExecution) error

	// GetExecution retrieves a workflow execution
	GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error)

	// DeleteExecution deletes a workflow execution
	DeleteExecution(ctx context.Context, executionID string) error

	// ListExecutions lists executions with optional filters
	ListExecutions(ctx context.Context, filters ExecutionFilters) ([]*WorkflowExecution, error)

	// Health checks storage health
	Health(ctx context.Context) error

	// Close closes the storage connection
	Close() error
}

// WorkflowDefinition represents a workflow definition
type WorkflowDefinition struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	DSL         map[string]interface{} `json:"dsl"`      // Serverless Workflow DSL
	Metadata    map[string]interface{} `json:"metadata"` // Additional metadata
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
	Tags        []string               `json:"tags"`
}

// WorkflowExecution represents a workflow execution instance
type WorkflowExecution struct {
	ID            string                 `json:"id"`
	WorkflowID    string                 `json:"workflow_id"`
	Status        ExecutionStatus        `json:"status"`
	Input         map[string]interface{} `json:"input"`
	Output        map[string]interface{} `json:"output"`
	Variables     map[string]interface{} `json:"variables"`
	CurrentStep   string                 `json:"current_step"`
	CorrelationID string                 `json:"correlation_id"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// ExecutionStatus represents the status of a workflow execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusPaused    ExecutionStatus = "paused"
)

// WorkflowFilters defines filters for workflow queries
type WorkflowFilters struct {
	Name      string            `json:"name,omitempty"`
	Version   string            `json:"version,omitempty"`
	CreatedBy string            `json:"created_by,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Offset    int               `json:"offset,omitempty"`
}

// ExecutionFilters defines filters for execution queries
type ExecutionFilters struct {
	WorkflowID    string            `json:"workflow_id,omitempty"`
	Status        ExecutionStatus   `json:"status,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	StartedAfter  *time.Time        `json:"started_after,omitempty"`
	StartedBefore *time.Time        `json:"started_before,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	Offset        int               `json:"offset,omitempty"`
}

// StorageStats represents storage statistics
type StorageStats struct {
	TotalEvents     int64     `json:"total_events"`
	TotalWorkflows  int64     `json:"total_workflows"`
	TotalExecutions int64     `json:"total_executions"`
	StorageSize     int64     `json:"storage_size_bytes"`
	LastEventTime   time.Time `json:"last_event_time"`
	Health          string    `json:"health"`
}

// StreamInfo represents stream information
type StreamInfo struct {
	StreamName      string            `json:"stream_name"`
	MessageCount    int64             `json:"message_count"`
	ConsumerGroups  []string          `json:"consumer_groups"`
	LastMessageTime time.Time         `json:"last_message_time"`
	Metadata        map[string]string `json:"metadata"`
}

// StorageConfig represents generic storage configuration
type StorageConfig struct {
	Type       string            `json:"type"`        // redis, postgres, mongodb, etc.
	URL        string            `json:"url"`         // Connection URL
	Database   string            `json:"database"`    // Database name
	Username   string            `json:"username"`    // Username
	Password   string            `json:"password"`    // Password
	Options    map[string]string `json:"options"`     // Additional options
	MaxRetries int               `json:"max_retries"` // Max retry attempts
	Timeout    time.Duration     `json:"timeout"`     // Connection timeout
}

// StreamConfig represents generic stream configuration
type StreamConfig struct {
	Type       string            `json:"type"`        // redis, kafka, rabbitmq, nats, etc.
	URL        string            `json:"url"`         // Connection URL
	Topic      string            `json:"topic"`       // Topic/Stream name
	Username   string            `json:"username"`    // Username
	Password   string            `json:"password"`    // Password
	Options    map[string]string `json:"options"`     // Additional options
	MaxRetries int               `json:"max_retries"` // Max retry attempts
	Timeout    time.Duration     `json:"timeout"`     // Connection timeout
}
