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
	key := r.getWorkflowKeyWithTenant(workflow.TenantID, workflow.ID)

	// Debug logging to see tenant mode and tenant ID
	r.logger.Info("Creating workflow",
		"workflow_id", workflow.ID,
		"tenant_id", workflow.TenantID,
		"tenant_mode", r.config.TenantMode,
		"key", key)

	// Debug logging to see tenant mode and tenant ID
	r.logger.Info("Creating workflow",
		"workflow_id", workflow.ID,
		"tenant_id", workflow.TenantID,
		"tenant_mode", r.config.TenantMode,
		"key", key)

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
// NOTE: This method only looks for non-tenant keys for backward compatibility
// Use GetWorkflowDefinitionWithTenant when you have tenant context
func (r *RedisDatabase) GetWorkflowDefinition(ctx context.Context, workflowID string) (*interfaces.WorkflowDefinition, error) {
	// Only try non-tenant key for backward compatibility
	key := r.getWorkflowKey(workflowID)

	r.logger.Debug("Looking for workflow definition (non-tenant)", "workflow_id", workflowID, "key", key)

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

	r.logger.Debug("Found workflow (non-tenant)", "workflow_id", workflowID, "key", key)
	return &workflow, nil
}

// GetWorkflowDefinitionWithTenant retrieves a workflow definition using tenant-specific key
func (r *RedisDatabase) GetWorkflowDefinitionWithTenant(ctx context.Context, tenantID, workflowID string) (*interfaces.WorkflowDefinition, error) {
	key := r.getWorkflowKeyWithTenant(tenantID, workflowID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow not found: %s (tenant: %s)", workflowID, tenantID)
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	var workflow interfaces.WorkflowDefinition
	err = json.Unmarshal([]byte(data), &workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	r.logger.Debug("Found workflow with tenant", "workflow_id", workflowID, "tenant_id", tenantID, "key", key)
	return &workflow, nil
}

// UpdateWorkflowDefinition updates a workflow definition
func (r *RedisDatabase) UpdateWorkflowDefinition(ctx context.Context, workflowID string, workflow *interfaces.WorkflowDefinition) error {
	key := r.getWorkflowKeyWithTenant(workflow.TenantID, workflowID)

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
// Searches both tenant-scoped and non-tenant keys for backward compatibility
func (r *RedisDatabase) DeleteWorkflow(ctx context.Context, workflowID string) error {
	var keysToTry []string

	// Try non-tenant key first for backward compatibility
	keysToTry = append(keysToTry, r.getWorkflowKey(workflowID))

	// If tenant mode is enabled, also search tenant-scoped keys
	if r.config.TenantMode {
		// Search for workflow across all tenants using pattern matching
		pattern := fmt.Sprintf("tenant:*:workflow:definition:%s", workflowID)
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			keysToTry = append(keysToTry, iter.Val())
		}

		if err := iter.Err(); err != nil {
			r.logger.Warn("Failed to scan tenant workflow keys for deletion", "pattern", pattern, "error", err)
		}
	}

	// Try to delete from each possible key
	totalDeleted := int64(0)
	for _, key := range keysToTry {
		deleted, err := r.client.Del(ctx, key).Result()
		if err != nil {
			r.logger.Warn("Failed to delete workflow from key", "key", key, "error", err)
			continue
		}
		totalDeleted += deleted
		if deleted > 0 {
			r.logger.Info("Workflow deleted", "workflow_id", workflowID, "key", key)
		}
	}

	if totalDeleted == 0 {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	return nil
}

// DeleteWorkflowWithTenant deletes a workflow definition using tenant-specific key
func (r *RedisDatabase) DeleteWorkflowWithTenant(ctx context.Context, tenantID, workflowID string) error {
	key := r.getWorkflowKeyWithTenant(tenantID, workflowID)

	deleted, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("workflow not found: %s (tenant: %s)", workflowID, tenantID)
	}

	r.logger.Info("Workflow deleted with tenant", "workflow_id", workflowID, "tenant_id", tenantID, "key", key)
	return nil
}

// ListWorkflows lists workflows with optional filters
func (r *RedisDatabase) ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*interfaces.WorkflowDefinition, error) {
	var patterns []string

	if r.config.TenantMode && filters.TenantID != "" {
		// Tenant-specific pattern
		patterns = []string{fmt.Sprintf("tenant:%s:workflow:definition:*", filters.TenantID)}
	} else {
		// Non-tenant pattern for backward compatibility
		patterns = []string{"workflow:definition:*"}

		// If tenant mode is enabled but no tenant filter, scan all tenant patterns
		if r.config.TenantMode {
			patterns = append(patterns, "tenant:*:workflow:definition:*")
		}
	}

	var workflows []*interfaces.WorkflowDefinition

	for _, pattern := range patterns {
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
			return nil, fmt.Errorf("failed to scan workflows with pattern %s: %w", pattern, err)
		}
	}

	return workflows, nil
}

// NOTE: WorkflowState methods have been removed in favor of execution-based storage
// Use CreateExecution, GetExecution, UpdateExecution, ListExecutions instead

// CreateExecution creates an execution
func (r *RedisDatabase) CreateExecution(ctx context.Context, execution *interfaces.WorkflowExecution) error {
	key := r.getExecutionKeyWithTenant(execution.TenantID, execution.ID)

	// Debug logging to see what's happening
	r.logger.Info("Creating execution",
		"execution_id", execution.ID,
		"tenant_id", execution.TenantID,
		"tenant_mode", r.config.TenantMode,
		"key", key)

	// Set started timestamp if not already set
	if execution.StartedAt.IsZero() {
		execution.StartedAt = time.Now()
	}

	data, err := json.Marshal(execution)
	if err != nil {
		return fmt.Errorf("failed to marshal execution: %w", err)
	}

	// Check if execution already exists
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check execution existence: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("execution with ID %s already exists", execution.ID)
	}

	err = r.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store execution: %w", err)
	}

	r.logger.Info("Execution created", "execution_id", execution.ID, "workflow_id", execution.WorkflowID, "tenant_id", execution.TenantID)
	return nil
}

// GetExecution gets an execution
// Note: This method needs tenant context to work properly in tenant mode
// For now, it tries both tenant and non-tenant keys for backward compatibility
func (r *RedisDatabase) GetExecution(ctx context.Context, executionID string) (*interfaces.WorkflowExecution, error) {
	// Try non-tenant key first for backward compatibility
	key := r.getExecutionKey(executionID)
	data, err := r.client.Get(ctx, key).Result()

	// If not found and tenant mode is enabled, try to find it by scanning tenant keys
	if err == redis.Nil && r.config.TenantMode {
		r.logger.Debug("Execution not found with non-tenant key, scanning tenant keys",
			"execution_id", executionID,
			"non_tenant_key", key)

		// Scan for tenant-specific execution keys
		pattern := fmt.Sprintf("tenant:*:execution:%s", executionID)
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			tenantKey := iter.Val()
			data, err = r.client.Get(ctx, tenantKey).Result()
			if err == nil {
				r.logger.Info("Found execution with tenant key",
					"execution_id", executionID,
					"tenant_key", tenantKey)
				break
			}
		}

		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to scan for tenant execution keys: %w", err)
		}

		// If still not found after scanning
		if err == redis.Nil {
			return nil, fmt.Errorf("execution not found: %s", executionID)
		}
	}

	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("execution not found: %s", executionID)
		}
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	var execution interfaces.WorkflowExecution
	err = json.Unmarshal([]byte(data), &execution)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal execution: %w", err)
	}

	return &execution, nil
}

// UpdateExecution updates an execution
func (r *RedisDatabase) UpdateExecution(ctx context.Context, executionID string, execution *interfaces.WorkflowExecution) error {
	// Try non-tenant key first for backward compatibility
	key := r.getExecutionKey(executionID)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check execution existence: %w", err)
	}

	// If not found and tenant mode is enabled, try to find it by scanning tenant keys
	if exists == 0 && r.config.TenantMode {
		r.logger.Debug("Execution not found with non-tenant key, scanning tenant keys for update",
			"execution_id", executionID,
			"non_tenant_key", key)

		// Scan for tenant-specific execution keys
		pattern := fmt.Sprintf("tenant:*:execution:%s", executionID)
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			tenantKey := iter.Val()
			exists, err = r.client.Exists(ctx, tenantKey).Result()
			if err == nil && exists > 0 {
				key = tenantKey // Use the tenant-specific key
				r.logger.Info("Found execution with tenant key for update",
					"execution_id", executionID,
					"tenant_key", tenantKey)
				break
			}
		}

		if err := iter.Err(); err != nil {
			return fmt.Errorf("failed to scan for tenant execution keys: %w", err)
		}

		// Check if we found the execution
		if exists == 0 {
			return fmt.Errorf("execution not found: %s", executionID)
		}
	} else if exists == 0 {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	// Ensure ID consistency
	execution.ID = executionID

	data, err := json.Marshal(execution)
	if err != nil {
		return fmt.Errorf("failed to marshal execution: %w", err)
	}

	err = r.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	r.logger.Info("Execution updated", "execution_id", executionID, "workflow_id", execution.WorkflowID, "tenant_id", execution.TenantID, "key", key)
	return nil
}

// ListExecutions lists executions with optional filters
func (r *RedisDatabase) ListExecutions(ctx context.Context, workflowID string, filters interfaces.ExecutionFilters) ([]*interfaces.WorkflowExecution, error) {
	var patterns []string

	if r.config.TenantMode && filters.TenantID != "" {
		// Tenant-specific pattern
		patterns = []string{fmt.Sprintf("tenant:%s:execution:*", filters.TenantID)}
	} else {
		// Non-tenant pattern for backward compatibility
		patterns = []string{"execution:*"}

		// If tenant mode is enabled but no tenant filter, scan all tenant patterns
		if r.config.TenantMode {
			patterns = append(patterns, "tenant:*:execution:*")
		}
	}

	var executions []*interfaces.WorkflowExecution

	for _, pattern := range patterns {
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			key := iter.Val()

			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				r.logger.Warn("Failed to get execution data", "key", key, "error", err)
				continue
			}

			var execution interfaces.WorkflowExecution
			err = json.Unmarshal([]byte(data), &execution)
			if err != nil {
				r.logger.Warn("Failed to unmarshal execution", "key", key, "error", err)
				continue
			}

			// Apply workflow ID filter first
			if workflowID != "" && execution.WorkflowID != workflowID {
				continue
			}

			// Apply additional filters
			if r.matchesExecutionFilters(&execution, filters) {
				executions = append(executions, &execution)
			}
		}

		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to scan executions with pattern %s: %w", pattern, err)
		}
	}

	return executions, nil
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
	// Count workflows (both tenant and non-tenant)
	workflowCount := int64(0)
	workflowPatterns := []string{"workflow:definition:*"}
	if r.config.TenantMode {
		workflowPatterns = append(workflowPatterns, "tenant:*:workflow:definition:*")
	}

	for _, pattern := range workflowPatterns {
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			workflowCount++
		}
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to count workflows with pattern %s: %w", pattern, err)
		}
	}

	// Count executions (both tenant and non-tenant)
	executionCount := int64(0)
	executionPatterns := []string{"execution:*"}
	if r.config.TenantMode {
		executionPatterns = append(executionPatterns, "tenant:*:execution:*")
	}

	for _, pattern := range executionPatterns {
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			executionCount++
		}
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to count executions with pattern %s: %w", pattern, err)
		}
	}

	return &interfaces.DatabaseStats{
		TotalWorkflows:  workflowCount,
		TotalExecutions: executionCount,
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
	if r.config.TenantMode {
		// In tenant mode, we need the tenant ID from the workflow definition
		// For now, use a placeholder pattern - this will be updated when we have tenant context
		return fmt.Sprintf("workflow:definition:%s", workflowID)
	}
	return fmt.Sprintf("workflow:definition:%s", workflowID)
}

func (r *RedisDatabase) getWorkflowKeyWithTenant(tenantID, workflowID string) string {
	if r.config.TenantMode && tenantID != "" {
		return fmt.Sprintf("tenant:%s:workflow:definition:%s", tenantID, workflowID)
	}
	return fmt.Sprintf("workflow:definition:%s", workflowID)
}

func (r *RedisDatabase) getExecutionKey(executionID string) string {
	// For backward compatibility, use non-tenant key if tenant mode is off
	return fmt.Sprintf("execution:%s", executionID)
}

func (r *RedisDatabase) getExecutionKeyWithTenant(tenantID, executionID string) string {
	if r.config.TenantMode && tenantID != "" {
		return fmt.Sprintf("tenant:%s:execution:%s", tenantID, executionID)
	}
	return fmt.Sprintf("execution:%s", executionID)
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

func (r *RedisDatabase) matchesExecutionFilters(execution *interfaces.WorkflowExecution, filters interfaces.ExecutionFilters) bool {
	// Apply status filter (compare enum values)
	if filters.Status != 0 && execution.Status != filters.Status {
		return false
	}

	// Apply started by filter
	if filters.StartedBy != "" && execution.StartedBy != filters.StartedBy {
		return false
	}

	// Apply tenant filter (if tenant mode is enabled)
	if r.config.TenantMode && filters.TenantID != "" && execution.TenantID != filters.TenantID {
		return false
	}

	// Apply time filters
	if !filters.StartedFrom.IsZero() && execution.StartedAt.Before(filters.StartedFrom) {
		return false
	}

	if !filters.StartedTo.IsZero() && execution.StartedAt.After(filters.StartedTo) {
		return false
	}

	return true
}
