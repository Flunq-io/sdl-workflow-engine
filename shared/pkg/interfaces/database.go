package interfaces

import (
	"context"
	"time"
)

// Database defines the generic interface for workflow metadata persistence
// Can be implemented by PostgreSQL, MongoDB, DynamoDB, etc.
type Database interface {
	// Workflow operations
	CreateWorkflow(ctx context.Context, workflow *WorkflowDefinition) error
	GetWorkflowDefinition(ctx context.Context, workflowID string) (*WorkflowDefinition, error)
	UpdateWorkflowDefinition(ctx context.Context, workflowID string, workflow *WorkflowDefinition) error
	DeleteWorkflow(ctx context.Context, workflowID string) error
	ListWorkflows(ctx context.Context, filters WorkflowFilters) ([]*WorkflowDefinition, error)

	// Workflow state operations
	CreateWorkflowState(ctx context.Context, workflowID string, state *WorkflowState) error
	GetWorkflowState(ctx context.Context, workflowID string) (*WorkflowState, error)
	UpdateWorkflowState(ctx context.Context, workflowID string, state *WorkflowState) error
	DeleteWorkflowState(ctx context.Context, workflowID string) error

	// Execution operations
	CreateExecution(ctx context.Context, execution *WorkflowExecution) error
	GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error)
	UpdateExecution(ctx context.Context, executionID string, execution *WorkflowExecution) error
	ListExecutions(ctx context.Context, workflowID string, filters ExecutionFilters) ([]*WorkflowExecution, error)

	// Task operations
	CreateTask(ctx context.Context, task *TaskDefinition) error
	GetTask(ctx context.Context, taskID string) (*TaskDefinition, error)
	UpdateTask(ctx context.Context, taskID string, task *TaskDefinition) error
	ListTasks(ctx context.Context, workflowID string) ([]*TaskDefinition, error)

	// Health and stats
	Health(ctx context.Context) error
	GetStats(ctx context.Context) (*DatabaseStats, error)
	Close() error
}

// WorkflowDefinition represents a workflow definition
type WorkflowDefinition struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	SpecVersion string                 `json:"spec_version"`
	Definition  map[string]interface{} `json:"definition"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
	TenantID    string                 `json:"tenant_id"`
}

// WorkflowState represents the current state of a workflow execution
type WorkflowState struct {
	WorkflowID     string                 `json:"workflow_id"`
	ExecutionID    string                 `json:"execution_id"`
	Status         WorkflowStatus         `json:"status"`
	CurrentStep    string                 `json:"current_step"`
	Variables      map[string]interface{} `json:"variables"`
	CompletedTasks []string               `json:"completed_tasks"`
	PendingTasks   []string               `json:"pending_tasks"`
	ErrorMessage   string                 `json:"error_message"`
	StartedAt      time.Time              `json:"started_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	TenantID       string                 `json:"tenant_id"`
}

// WorkflowExecution represents a workflow execution instance
type WorkflowExecution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Status      WorkflowStatus         `json:"status"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
	StartedBy   string                 `json:"started_by"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	TenantID    string                 `json:"tenant_id"`
}

// TaskDefinition represents a task definition
type TaskDefinition struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Definition  map[string]interface{} `json:"definition"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WorkflowStatus represents workflow execution status
type WorkflowStatus int

const (
	WorkflowStatusPending WorkflowStatus = iota
	WorkflowStatusRunning
	WorkflowStatusCompleted
	WorkflowStatusFailed
	WorkflowStatusCancelled
)

// WorkflowFilters defines filters for workflow queries
type WorkflowFilters struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	CreatedBy   string    `json:"created_by"`
	CreatedFrom time.Time `json:"created_from"`
	CreatedTo   time.Time `json:"created_to"`
	TenantID    string    `json:"tenant_id"`
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
}

// ExecutionFilters defines filters for execution queries
type ExecutionFilters struct {
	Status      WorkflowStatus `json:"status"`
	StartedBy   string         `json:"started_by"`
	StartedFrom time.Time      `json:"started_from"`
	StartedTo   time.Time      `json:"started_to"`
	TenantID    string         `json:"tenant_id"`
	Limit       int            `json:"limit"`
	Offset      int            `json:"offset"`
}

// DatabaseStats represents database statistics
type DatabaseStats struct {
	TotalWorkflows  int64  `json:"total_workflows"`
	TotalExecutions int64  `json:"total_executions"`
	TotalTasks      int64  `json:"total_tasks"`
	DatabaseSize    int64  `json:"database_size_bytes"`
	Health          string `json:"health"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type         string            `json:"type"`          // postgresql, mongodb, dynamodb, etc.
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Database     string            `json:"database"`
	Username     string            `json:"username"`
	Password     string            `json:"password"`
	SSLMode      string            `json:"ssl_mode"`
	MaxConns     int               `json:"max_conns"`
	MaxIdleConns int               `json:"max_idle_conns"`
	Options      map[string]string `json:"options"`       // Additional database-specific options
	TenantMode   bool              `json:"tenant_mode"`   // Enable multi-tenant isolation
}
