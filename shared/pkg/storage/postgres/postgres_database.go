package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flunq-io/shared/pkg/interfaces"
)

// PostgresDatabase is a placeholder implementation of interfaces.Database.
// It compiles and provides clear errors to guide contributors implementing PostgreSQL.
// Replace these method bodies with real implementations backed by database/sql or pgx.
type PostgresDatabase struct {
	logger interfaces.Logger
	config *interfaces.DatabaseConfig
}

// NewPostgresDatabase creates a new placeholder Postgres database implementation.
func NewPostgresDatabase(logger interfaces.Logger, config *interfaces.DatabaseConfig) *PostgresDatabase {
	if logger == nil {
		panic("PostgresDatabase requires a non-nil logger")
	}
	if config == nil {
		config = &interfaces.DatabaseConfig{Type: "postgres"}
	}
	return &PostgresDatabase{logger: logger, config: config}
}

// --- Helper ---
func notImplemented(method string) error {
	return fmt.Errorf("postgres database: %s not implemented yet; contributions welcome at shared/pkg/storage/postgres", method)
}

// --- Workflow operations ---
func (p *PostgresDatabase) CreateWorkflow(ctx context.Context, workflow *interfaces.WorkflowDefinition) error {
	return notImplemented("CreateWorkflow")
}

func (p *PostgresDatabase) GetWorkflowDefinition(ctx context.Context, workflowID string) (*interfaces.WorkflowDefinition, error) {
	return nil, notImplemented("GetWorkflowDefinition")
}

func (p *PostgresDatabase) GetWorkflowDefinitionWithTenant(ctx context.Context, tenantID, workflowID string) (*interfaces.WorkflowDefinition, error) {
	return nil, notImplemented("GetWorkflowDefinitionWithTenant")
}

func (p *PostgresDatabase) UpdateWorkflowDefinition(ctx context.Context, workflowID string, workflow *interfaces.WorkflowDefinition) error {
	return notImplemented("UpdateWorkflowDefinition")
}

func (p *PostgresDatabase) DeleteWorkflow(ctx context.Context, workflowID string) error {
	return notImplemented("DeleteWorkflow")
}

func (p *PostgresDatabase) DeleteWorkflowWithTenant(ctx context.Context, tenantID, workflowID string) error {
	return notImplemented("DeleteWorkflowWithTenant")
}

func (p *PostgresDatabase) ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*interfaces.WorkflowDefinition, error) {
	return nil, notImplemented("ListWorkflows")
}

// --- Execution operations ---
func (p *PostgresDatabase) CreateExecution(ctx context.Context, execution *interfaces.WorkflowExecution) error {
	return notImplemented("CreateExecution")
}

func (p *PostgresDatabase) GetExecution(ctx context.Context, executionID string) (*interfaces.WorkflowExecution, error) {
	return nil, notImplemented("GetExecution")
}

func (p *PostgresDatabase) UpdateExecution(ctx context.Context, executionID string, execution *interfaces.WorkflowExecution) error {
	return notImplemented("UpdateExecution")
}

func (p *PostgresDatabase) ListExecutions(ctx context.Context, workflowID string, filters interfaces.ExecutionFilters) ([]*interfaces.WorkflowExecution, error) {
	return nil, notImplemented("ListExecutions")
}

// --- Task operations ---
func (p *PostgresDatabase) CreateTask(ctx context.Context, task *interfaces.TaskDefinition) error {
	return notImplemented("CreateTask")
}

func (p *PostgresDatabase) GetTask(ctx context.Context, taskID string) (*interfaces.TaskDefinition, error) {
	return nil, notImplemented("GetTask")
}

func (p *PostgresDatabase) UpdateTask(ctx context.Context, taskID string, task *interfaces.TaskDefinition) error {
	return notImplemented("UpdateTask")
}

func (p *PostgresDatabase) ListTasks(ctx context.Context, workflowID string) ([]*interfaces.TaskDefinition, error) {
	return nil, notImplemented("ListTasks")
}

// --- Health and stats ---
func (p *PostgresDatabase) Health(ctx context.Context) error {
	// If a real connection is added later, perform a ping here.
	return errors.New("postgres database: health check not implemented")
}

func (p *PostgresDatabase) GetStats(ctx context.Context) (*interfaces.DatabaseStats, error) {
	return &interfaces.DatabaseStats{
		TotalWorkflows:  0,
		TotalExecutions: 0,
		TotalTasks:      0,
		DatabaseSize:    0,
		Health:          "unknown",
	}, notImplemented("GetStats")
}

func (p *PostgresDatabase) Close() error {
	// Close underlying connections here once implemented
	return nil
}

// Prevent unused imports warnings for now
var _ = time.Now

