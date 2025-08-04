package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"

	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// RedisDatabase implements the Database interface using Redis
type RedisDatabase struct {
	client *redis.Client
	logger interfaces.Logger
}

// NewRedisDatabase creates a new Redis database adapter
func NewRedisDatabase(client *redis.Client, logger interfaces.Logger) interfaces.Database {
	return &RedisDatabase{
		client: client,
		logger: logger,
	}
}

// CreateWorkflow creates a new workflow record
func (r *RedisDatabase) CreateWorkflow(ctx context.Context, workflow *gen.WorkflowDefinition) error {
	// Serialize workflow definition to JSON
	data, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	// Store in Redis with key: workflow:definition:{id}
	key := fmt.Sprintf("workflow:definition:%s", workflow.Id)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		r.logger.Error("Failed to store workflow definition",
			"workflow_id", workflow.Id,
			"error", err)
		return fmt.Errorf("failed to store workflow definition: %w", err)
	}

	r.logger.Info("Created workflow definition",
		"workflow_id", workflow.Id,
		"name", workflow.Name)

	return nil
}

// UpdateWorkflowState updates the workflow state
func (r *RedisDatabase) UpdateWorkflowState(ctx context.Context, workflowID string, state *gen.WorkflowState) error {
	// Serialize workflow state to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	// Store in Redis with key: workflow:state:{id}
	key := fmt.Sprintf("workflow:state:%s", workflowID)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		r.logger.Error("Failed to update workflow state",
			"workflow_id", workflowID,
			"error", err)
		return fmt.Errorf("failed to update workflow state: %w", err)
	}

	r.logger.Debug("Updated workflow state",
		"workflow_id", workflowID,
		"status", state.Status,
		"current_step", state.CurrentStep)

	return nil
}

// GetWorkflowState retrieves the current workflow state
func (r *RedisDatabase) GetWorkflowState(ctx context.Context, workflowID string) (*gen.WorkflowState, error) {
	// Get from Redis with key: workflow:state:{id}
	key := fmt.Sprintf("workflow:state:%s", workflowID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow state not found: %s", workflowID)
		}
		r.logger.Error("Failed to get workflow state",
			"workflow_id", workflowID,
			"error", err)
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	// Deserialize from JSON
	var state gen.WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow state: %w", err)
	}

	r.logger.Debug("Retrieved workflow state",
		"workflow_id", workflowID,
		"status", state.Status,
		"current_step", state.CurrentStep)

	return &state, nil
}

// GetWorkflowDefinition retrieves the workflow definition
func (r *RedisDatabase) GetWorkflowDefinition(ctx context.Context, workflowID string) (*gen.WorkflowDefinition, error) {
	// Get from Redis with key: workflow:definition:{id}
	key := fmt.Sprintf("workflow:definition:%s", workflowID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow definition not found: %s", workflowID)
		}
		r.logger.Error("Failed to get workflow definition",
			"workflow_id", workflowID,
			"error", err)
		return nil, fmt.Errorf("failed to get workflow definition: %w", err)
	}

	// Deserialize from JSON
	var definition gen.WorkflowDefinition
	if err := json.Unmarshal(data, &definition); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow definition: %w", err)
	}

	r.logger.Debug("Retrieved workflow definition",
		"workflow_id", workflowID,
		"name", definition.Name)

	return &definition, nil
}

// DeleteWorkflow deletes a workflow and its state
func (r *RedisDatabase) DeleteWorkflow(ctx context.Context, workflowID string) error {
	// Delete both definition and state
	definitionKey := fmt.Sprintf("workflow:definition:%s", workflowID)
	stateKey := fmt.Sprintf("workflow:state:%s", workflowID)

	pipe := r.client.Pipeline()
	pipe.Del(ctx, definitionKey)
	pipe.Del(ctx, stateKey)

	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.Error("Failed to delete workflow",
			"workflow_id", workflowID,
			"error", err)
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	r.logger.Info("Deleted workflow",
		"workflow_id", workflowID)

	return nil
}

// ListWorkflows lists all workflows with optional filtering
func (r *RedisDatabase) ListWorkflows(ctx context.Context, filters map[string]string) ([]*gen.WorkflowState, error) {
	// Get all workflow state keys
	pattern := "workflow:state:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Failed to list workflow keys", "error", err)
		return nil, fmt.Errorf("failed to list workflow keys: %w", err)
	}

	var workflows []*gen.WorkflowState

	// Get all workflow states
	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Bytes()
		if err != nil {
			r.logger.Warn("Failed to get workflow state for key",
				"key", key,
				"error", err)
			continue
		}

		var state gen.WorkflowState
		if err := json.Unmarshal(data, &state); err != nil {
			r.logger.Warn("Failed to unmarshal workflow state for key",
				"key", key,
				"error", err)
			continue
		}

		// Apply filters
		if r.matchesFilters(&state, filters) {
			workflows = append(workflows, &state)
		}
	}

	r.logger.Debug("Listed workflows",
		"count", len(workflows),
		"filters", filters)

	return workflows, nil
}

// matchesFilters checks if a workflow state matches the given filters
func (r *RedisDatabase) matchesFilters(state *gen.WorkflowState, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}

	for key, value := range filters {
		switch key {
		case "status":
			// Simple string comparison for status
			statusStr := fmt.Sprintf("%d", int32(state.Status))
			if statusStr != value {
				return false
			}
		case "workflow_id":
			if state.WorkflowId != value {
				return false
			}
		case "current_step":
			if state.CurrentStep != value {
				return false
			}
		default:
			// Unknown filter, ignore
			continue
		}
	}

	return true
}
