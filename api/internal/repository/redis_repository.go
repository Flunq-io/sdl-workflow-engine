package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
)

// RedisWorkflowRepository implements WorkflowRepository using Redis
type RedisWorkflowRepository struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisWorkflowRepository creates a new Redis-based workflow repository
func NewRedisWorkflowRepository(client *redis.Client, logger *zap.Logger) *RedisWorkflowRepository {
	return &RedisWorkflowRepository{
		client: client,
		logger: logger,
	}
}

// WorkflowData represents the structure stored in Redis
type WorkflowData struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	TenantID      string                 `json:"tenant_id"`
	Version       string                 `json:"version"`
	SpecVersion   string                 `json:"spec_version"`
	DSLDefinition map[string]interface{} `json:"dsl_definition"`
	StartState    string                 `json:"start_state"`
}

// Create creates a new workflow in Redis
func (r *RedisWorkflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	workflowData := WorkflowData{
		ID:            workflow.ID,
		Name:          workflow.Name,
		TenantID:      workflow.TenantID,
		Version:       "1.0.0",
		SpecVersion:   "1.0.0",
		DSLDefinition: workflow.Definition,
		StartState:    "start",
	}

	data, err := json.Marshal(workflowData)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	key := fmt.Sprintf("workflow:definition:%s", workflow.ID)
	err = r.client.Set(ctx, key, string(data), 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store workflow in Redis: %w", err)
	}

	r.logger.Info("Created workflow in Redis", zap.String("id", workflow.ID))
	return nil
}

// GetByID retrieves a workflow by ID from Redis
func (r *RedisWorkflowRepository) GetByID(ctx context.Context, id string) (*models.WorkflowDetail, error) {
	key := fmt.Sprintf("workflow:definition:%s", id)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, fmt.Errorf("failed to get workflow from Redis: %w", err)
	}

	var workflowData WorkflowData
	if err := json.Unmarshal([]byte(data), &workflowData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow data: %w", err)
	}

	// Get workflow state to extract timestamps and status
	stateKey := fmt.Sprintf("workflow:state:%s", id)
	stateData, err := r.client.Get(ctx, stateKey).Result()

	var createdAt, updatedAt time.Time
	var status models.WorkflowStatus = models.WorkflowStatusPending

	if err == nil {
		// Parse workflow state to get timestamps and status
		var state map[string]interface{}
		if json.Unmarshal([]byte(stateData), &state) == nil {
			// Extract created_at timestamp
			if createdAtData, ok := state["created_at"].(map[string]interface{}); ok {
				if seconds, ok := createdAtData["seconds"].(float64); ok {
					createdAt = time.Unix(int64(seconds), 0)
				}
			}

			// Extract updated_at timestamp
			if updatedAtData, ok := state["updated_at"].(map[string]interface{}); ok {
				if seconds, ok := updatedAtData["seconds"].(float64); ok {
					nanos := int64(0)
					if nanosData, ok := updatedAtData["nanos"].(float64); ok {
						nanos = int64(nanosData)
					}
					updatedAt = time.Unix(int64(seconds), nanos)
				}
			}

			// Extract status
			if statusData, ok := state["status"].(float64); ok {
				switch int(statusData) {
				case 1:
					status = models.WorkflowStatusPending
				case 2:
					status = models.WorkflowStatusRunning
				case 3:
					status = models.WorkflowStatusCompleted
				case 4:
					status = models.WorkflowStatusFaulted
				case 5:
					status = models.WorkflowStatusCancelled
				default:
					status = models.WorkflowStatusPending
				}
			}
		}
	}

	// Fallback to current time if timestamps not found
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	// Convert to models.Workflow
	workflow := &models.Workflow{
		ID:          workflowData.ID,
		Name:        workflowData.Name,
		Description: fmt.Sprintf("Workflow %s (v%s)", workflowData.Name, workflowData.Version),
		Definition:  workflowData.DSLDefinition,
		Status:      status,
		Tags:        []string{},
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Status is already set from the state parsing above

	detail := &models.WorkflowDetail{
		Workflow:       *workflow,
		ExecutionCount: 0, // TODO: Count executions
		LastExecution:  nil,
	}

	return detail, nil
}

// List retrieves workflows with pagination
func (r *RedisWorkflowRepository) List(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	limit := params.Limit
	offset := params.Offset

	// Set defaults
	if limit <= 0 {
		limit = 20
	}
	// Get all workflow definition keys
	pattern := "workflow:definition:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get workflow keys: %w", err)
	}

	total := len(keys)
	if total == 0 {
		return []models.Workflow{}, 0, nil
	}

	// Sort keys for consistent pagination
	sort.Strings(keys)

	// Apply pagination
	start := offset
	end := offset + limit
	if start >= total {
		return []models.Workflow{}, total, nil
	}
	if end > total {
		end = total
	}

	paginatedKeys := keys[start:end]
	workflows := make([]models.Workflow, 0, len(paginatedKeys))

	// Get workflow data for each key
	for _, key := range paginatedKeys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			r.logger.Warn("Failed to get workflow data", zap.String("key", key), zap.Error(err))
			continue
		}

		var workflowData WorkflowData
		if err := json.Unmarshal([]byte(data), &workflowData); err != nil {
			r.logger.Warn("Failed to unmarshal workflow data", zap.String("key", key), zap.Error(err))
			continue
		}

		// Extract workflow ID from key
		workflowID := strings.TrimPrefix(key, "workflow:definition:")

		// Get workflow state to extract timestamps and status
		stateKey := fmt.Sprintf("workflow:state:%s", workflowID)
		stateData, stateErr := r.client.Get(ctx, stateKey).Result()

		var createdAt, updatedAt time.Time
		var status models.WorkflowStatus = models.WorkflowStatusPending

		if stateErr == nil {
			// Parse workflow state to get timestamps and status
			var state map[string]interface{}
			if json.Unmarshal([]byte(stateData), &state) == nil {
				// Extract created_at timestamp
				if createdAtData, ok := state["created_at"].(map[string]interface{}); ok {
					if seconds, ok := createdAtData["seconds"].(float64); ok {
						createdAt = time.Unix(int64(seconds), 0)
					}
				}

				// Extract updated_at timestamp
				if updatedAtData, ok := state["updated_at"].(map[string]interface{}); ok {
					if seconds, ok := updatedAtData["seconds"].(float64); ok {
						nanos := int64(0)
						if nanosData, ok := updatedAtData["nanos"].(float64); ok {
							nanos = int64(nanosData)
						}
						updatedAt = time.Unix(int64(seconds), nanos)
					}
				}

				// Extract status
				if statusNum, ok := state["status"].(float64); ok {
					status = r.mapProtobufStatusToSDL(int(statusNum))
				}
			}
		}

		// Fallback to current time if timestamps not found
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}

		workflow := &models.Workflow{
			ID:          workflowData.ID,
			Name:        workflowData.Name,
			Description: fmt.Sprintf("Workflow %s (v%s)", workflowData.Name, workflowData.Version),
			Definition:  workflowData.DSLDefinition,
			Status:      status,
			Tags:        []string{},
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}

		workflows = append(workflows, *workflow)
	}

	return workflows, total, nil
}

// Update updates an existing workflow
func (r *RedisWorkflowRepository) Update(ctx context.Context, workflow *models.Workflow) error {
	// Check if workflow exists
	key := fmt.Sprintf("workflow:definition:%s", workflow.ID)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check workflow existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("workflow not found")
	}

	// Update the workflow
	return r.Create(ctx, workflow)
}

// Delete deletes a workflow
func (r *RedisWorkflowRepository) Delete(ctx context.Context, id string) error {
	keys := []string{
		fmt.Sprintf("workflow:definition:%s", id),
		fmt.Sprintf("workflow:state:%s", id),
		fmt.Sprintf("events:workflow:%s", id),
	}

	for _, key := range keys {
		err := r.client.Del(ctx, key).Err()
		if err != nil {
			r.logger.Warn("Failed to delete key", zap.String("key", key), zap.Error(err))
		}
	}

	r.logger.Info("Deleted workflow from Redis", zap.String("id", id))
	return nil
}

// mapProtobufStatusToSDL maps protobuf enum values to SDL-compliant status strings
func (r *RedisWorkflowRepository) mapProtobufStatusToSDL(status int) models.WorkflowStatus {
	switch status {
	case 0: // WORKFLOW_STATUS_UNSPECIFIED
		return models.WorkflowStatusPending
	case 1: // WORKFLOW_STATUS_CREATED
		return models.WorkflowStatusPending
	case 2: // WORKFLOW_STATUS_RUNNING
		return models.WorkflowStatusRunning
	case 3: // WORKFLOW_STATUS_COMPLETED
		return models.WorkflowStatusCompleted
	case 4: // WORKFLOW_STATUS_FAILED
		return models.WorkflowStatusFaulted
	case 5: // WORKFLOW_STATUS_CANCELLED
		return models.WorkflowStatusCancelled
	case 6: // WORKFLOW_STATUS_SUSPENDED
		return models.WorkflowStatusSuspended
	case 7: // WORKFLOW_STATUS_WAITING
		return models.WorkflowStatusWaiting
	default:
		return models.WorkflowStatusPending
	}
}
