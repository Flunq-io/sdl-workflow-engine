package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flunq-io/shared/pkg/interfaces"
	"github.com/go-redis/redis/v8"
)

// RedisDatabase implements the Database interface using Redis as the backend
type RedisDatabase struct {
	client redis.UniversalClient
	logger interfaces.Logger
	config *interfaces.DatabaseConfig
}

// NewRedisDatabase creates a new Redis database implementation
func NewRedisDatabase(client redis.UniversalClient, logger interfaces.Logger, config *interfaces.DatabaseConfig) *RedisDatabase {
	return &RedisDatabase{
		client: client,
		logger: logger,
		config: config,
	}
}

// CreateWorkflow creates a new workflow definition
func (r *RedisDatabase) CreateWorkflow(ctx context.Context, workflow *interfaces.WorkflowDefinition) error {
	key := r.getWorkflowKey(workflow.ID)

	// Set timestamps
	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now

	data, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	// Check if workflow already exists
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check workflow existence: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("workflow with ID %s already exists", workflow.ID)
	}

	err = r.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store workflow: %w", err)
	}

	r.logger.Info("Workflow created", "workflow_id", workflow.ID, "tenant_id", workflow.TenantID)
	return nil
}

// GetWorkflowDefinition retrieves a workflow definition
func (r *RedisDatabase) GetWorkflowDefinition(ctx context.Context, workflowID string) (*interfaces.WorkflowDefinition, error) {
	key := r.getWorkflowKey(workflowID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow not found: %s", workflowID)
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	var workflow interfaces.WorkflowDefinition
	err = json.Unmarshal([]byte(data), &workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	return &workflow, nil
}

// UpdateWorkflowDefinition updates a workflow definition
func (r *RedisDatabase) UpdateWorkflowDefinition(ctx context.Context, workflowID string, workflow *interfaces.WorkflowDefinition) error {
	key := r.getWorkflowKey(workflowID)

	// Check if workflow exists
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check workflow existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Update timestamp
	workflow.UpdatedAt = time.Now()
	workflow.ID = workflowID // Ensure ID consistency

	data, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	err = r.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update workflow: %w", err)
	}

	r.logger.Info("Workflow updated", "workflow_id", workflowID, "tenant_id", workflow.TenantID)
	return nil
}

// DeleteWorkflow deletes a workflow definition
func (r *RedisDatabase) DeleteWorkflow(ctx context.Context, workflowID string) error {
	key := r.getWorkflowKey(workflowID)

	deleted, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	r.logger.Info("Workflow deleted", "workflow_id", workflowID)
	return nil
}

// ListWorkflows lists workflows with optional filters
func (r *RedisDatabase) ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*interfaces.WorkflowDefinition, error) {
	pattern := "workflow:definition:*"

	var workflows []*interfaces.WorkflowDefinition
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()

		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			r.logger.Warn("Failed to get workflow data", "key", key, "error", err)
			continue
		}

		var workflow interfaces.WorkflowDefinition
		err = json.Unmarshal([]byte(data), &workflow)
		if err != nil {
			r.logger.Warn("Failed to unmarshal workflow", "key", key, "error", err)
			continue
		}

		// Apply filters
		if r.matchesFilters(&workflow, filters) {
			workflows = append(workflows, &workflow)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan workflows: %w", err)
	}

	return workflows, nil
}

// CreateWorkflowState creates workflow state (placeholder for future implementation)
func (r *RedisDatabase) CreateWorkflowState(ctx context.Context, workflowID string, state *interfaces.WorkflowState) error {
	// TODO: Implement workflow state storage
	return fmt.Errorf("workflow state storage not implemented yet")
}

// GetWorkflowState gets workflow state (placeholder for future implementation)
func (r *RedisDatabase) GetWorkflowState(ctx context.Context, workflowID string) (*interfaces.WorkflowState, error) {
	// TODO: Implement workflow state retrieval
	return nil, fmt.Errorf("workflow state storage not implemented yet")
}

// UpdateWorkflowState updates workflow state (placeholder for future implementation)
func (r *RedisDatabase) UpdateWorkflowState(ctx context.Context, workflowID string, state *interfaces.WorkflowState) error {
	// TODO: Implement workflow state updates
	return fmt.Errorf("workflow state storage not implemented yet")
}

// DeleteWorkflowState deletes workflow state (placeholder for future implementation)
func (r *RedisDatabase) DeleteWorkflowState(ctx context.Context, workflowID string) error {
	// TODO: Implement workflow state deletion
	return fmt.Errorf("workflow state storage not implemented yet")
}

// CreateExecution creates an execution (placeholder for future implementation)
func (r *RedisDatabase) CreateExecution(ctx context.Context, execution *interfaces.WorkflowExecution) error {
	// TODO: Implement execution storage
	return fmt.Errorf("execution storage not implemented yet")
}

// GetExecution gets an execution (placeholder for future implementation)
func (r *RedisDatabase) GetExecution(ctx context.Context, executionID string) (*interfaces.WorkflowExecution, error) {
	// TODO: Implement execution retrieval
	return nil, fmt.Errorf("execution storage not implemented yet")
}

// UpdateExecution updates an execution (placeholder for future implementation)
func (r *RedisDatabase) UpdateExecution(ctx context.Context, executionID string, execution *interfaces.WorkflowExecution) error {
	// TODO: Implement execution updates
	return fmt.Errorf("execution storage not implemented yet")
}

// ListExecutions lists executions (placeholder for future implementation)
func (r *RedisDatabase) ListExecutions(ctx context.Context, workflowID string, filters interfaces.ExecutionFilters) ([]*interfaces.WorkflowExecution, error) {
	// TODO: Implement execution listing
	return nil, fmt.Errorf("execution storage not implemented yet")
}

// CreateTask creates a task (placeholder for future implementation)
func (r *RedisDatabase) CreateTask(ctx context.Context, task *interfaces.TaskDefinition) error {
	// TODO: Implement task storage
	return fmt.Errorf("task storage not implemented yet")
}

// GetTask gets a task (placeholder for future implementation)
func (r *RedisDatabase) GetTask(ctx context.Context, taskID string) (*interfaces.TaskDefinition, error) {
	// TODO: Implement task retrieval
	return nil, fmt.Errorf("task storage not implemented yet")
}

// UpdateTask updates a task (placeholder for future implementation)
func (r *RedisDatabase) UpdateTask(ctx context.Context, taskID string, task *interfaces.TaskDefinition) error {
	// TODO: Implement task updates
	return fmt.Errorf("task storage not implemented yet")
}

// ListTasks lists tasks (placeholder for future implementation)
func (r *RedisDatabase) ListTasks(ctx context.Context, workflowID string) ([]*interfaces.TaskDefinition, error) {
	// TODO: Implement task listing
	return nil, fmt.Errorf("task storage not implemented yet")
}

// Health checks database health
func (r *RedisDatabase) Health(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// GetStats returns database statistics
func (r *RedisDatabase) GetStats(ctx context.Context) (*interfaces.DatabaseStats, error) {
	// Count workflows
	workflowCount := int64(0)
	iter := r.client.Scan(ctx, 0, "workflow:definition:*", 0).Iterator()
	for iter.Next(ctx) {
		workflowCount++
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	return &interfaces.DatabaseStats{
		TotalWorkflows:  workflowCount,
		TotalExecutions: 0, // TODO: Implement when executions are stored
		TotalTasks:      0, // TODO: Implement when tasks are stored
		DatabaseSize:    0, // TODO: Implement size calculation
		Health:          "healthy",
	}, nil
}

// Close closes the database connection
func (r *RedisDatabase) Close() error {
	return r.client.Close()
}

// Helper methods

func (r *RedisDatabase) getWorkflowKey(workflowID string) string {
	return fmt.Sprintf("workflow:definition:%s", workflowID)
}

func (r *RedisDatabase) matchesFilters(workflow *interfaces.WorkflowDefinition, filters interfaces.WorkflowFilters) bool {
	// Apply tenant filter
	if filters.TenantID != "" && workflow.TenantID != filters.TenantID {
		return false
	}

	// Apply name filter
	if filters.Name != "" && workflow.Name != filters.Name {
		return false
	}

	// Apply version filter
	if filters.Version != "" && workflow.Version != filters.Version {
		return false
	}

	// TODO: Add tag filters when Tags field is added to WorkflowDefinition and WorkflowFilters

	return true
}
