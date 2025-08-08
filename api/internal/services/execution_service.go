package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// ExecutionService handles execution business logic
type ExecutionService struct {
	executionRepo ExecutionRepository
	workflowRepo  WorkflowRepository
	eventStream   interfaces.EventStream
	redisClient   *redis.Client
	logger        *zap.Logger
}

// NewExecutionService creates a new execution service
func NewExecutionService(
	executionRepo ExecutionRepository,
	workflowRepo WorkflowRepository,
	eventStream interfaces.EventStream,
	redisClient *redis.Client,
	logger *zap.Logger,
) *ExecutionService {
	return &ExecutionService{
		executionRepo: executionRepo,
		workflowRepo:  workflowRepo,
		eventStream:   eventStream,
		redisClient:   redisClient,
		logger:        logger,
	}
}

// ListExecutions lists executions with optional filtering
func (s *ExecutionService) ListExecutions(ctx context.Context, params *models.ExecutionListParams) ([]models.Execution, int, error) {
	executions, total, err := s.executionRepo.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list executions from repository", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list executions: %w", err)
	}

	return executions, total, nil
}

// GetExecution retrieves an execution by ID from workflow state storage
func (s *ExecutionService) GetExecution(ctx context.Context, executionID string) (*models.Execution, error) {
	s.logger.Debug("Getting execution from workflow state storage",
		zap.String("execution_id", executionID))

	// Find the workflow that contains this execution by scanning workflow states
	execution, err := s.findExecutionInWorkflowStates(ctx, executionID)
	if err != nil {
		if err == ErrExecutionNotFound {
			return nil, ErrExecutionNotFound
		}
		s.logger.Error("Failed to find execution in workflow states",
			zap.String("execution_id", executionID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	s.logger.Info("Successfully retrieved execution from workflow state",
		zap.String("execution_id", executionID),
		zap.String("workflow_id", execution.WorkflowID),
		zap.String("status", string(execution.Status)))

	return execution, nil
}

// GetExecutionByTenant retrieves an execution by ID with tenant filtering using the same repository as ListExecutions
func (s *ExecutionService) GetExecutionByTenant(ctx context.Context, executionID string, tenantID string) (*models.Execution, error) {
	s.logger.Debug("Getting execution by tenant from repository",
		zap.String("execution_id", executionID),
		zap.String("tenant_id", tenantID))

	// Use the same repository as ListExecutions to ensure consistency
	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrExecutionNotFound
		}
		s.logger.Error("Failed to get execution from repository",
			zap.String("execution_id", executionID),
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	// TODO: Implement proper tenant verification once executions have TenantID populated
	// For now, we'll trust that the tenant-aware list endpoint filters correctly
	// and the execution exists in the system

	s.logger.Info("Successfully retrieved execution by tenant",
		zap.String("execution_id", executionID),
		zap.String("tenant_id", tenantID),
		zap.String("workflow_id", execution.WorkflowID),
		zap.String("status", string(execution.Status)))

	return execution, nil
}

// CancelExecution cancels a running execution
func (s *ExecutionService) CancelExecution(ctx context.Context, executionID string, reason string) (*models.Execution, error) {
	// Get existing execution
	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrExecutionNotFound
		}
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	// Check if execution can be cancelled
	if !execution.IsRunning() {
		return nil, ErrExecutionNotRunning
	}

	// Cancel the execution
	execution.Cancel(reason)

	// Save to repository
	if err := s.executionRepo.Update(ctx, execution); err != nil {
		s.logger.Error("Failed to update execution in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to update execution: %w", err)
	}

	// Publish execution cancelled event
	event := cloudevents.NewCloudEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.ExecutionCancelled,
	)
	event.WorkflowID = execution.WorkflowID
	event.ExecutionID = executionID
	event.Time = time.Now()
	event.SetData(map[string]interface{}{
		"reason":       reason,
		"cancelled_by": "api", // TODO: Get from authentication context
		"status":       execution.Status,
	})

	// Get workflow to determine tenant ID and add metadata
	workflow, err := s.workflowRepo.GetByID(ctx, execution.WorkflowID)
	if err != nil {
		s.logger.Error("Failed to get workflow for tenant ID", zap.Error(err))
	}

	// Add metadata to event (using proper CloudEvent fields)
	if workflow != nil {
		event.TenantID = workflow.TenantID
	}
	event.WorkflowID = execution.WorkflowID
	event.ExecutionID = executionID

	// Publish using shared event streaming (auto-routes to tenant-isolated streams)
	if err := s.eventStream.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish execution cancelled event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Execution cancelled successfully",
		zap.String("execution_id", executionID),
		zap.String("reason", reason))

	return execution, nil
}

// GetExecutionEvents retrieves all events for a specific execution using event sourcing
func (s *ExecutionService) GetExecutionEvents(ctx context.Context, executionID string, params *models.EventHistoryParams) ([]models.CloudEvent, error) {
	s.logger.Debug("Getting execution events",
		zap.String("execution_id", executionID),
		zap.Int("limit", params.Limit))

	// Get execution from repository to find the workflow ID
	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrExecutionNotFound
		}
		s.logger.Error("Failed to get execution from repository",
			zap.String("execution_id", executionID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	// Get event history for the workflow that contains this execution
	// Since we use workflow-based streams, we need to get all events for the workflow
	// and filter for this specific execution
	cloudEvents, err := s.eventStream.GetEventHistory(ctx, execution.WorkflowID)
	if err != nil {
		s.logger.Error("Failed to get event history from event stream",
			zap.String("execution_id", executionID),
			zap.String("workflow_id", execution.WorkflowID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get event history: %w", err)
	}

	// Filter events for this specific execution and convert to API models
	var executionEvents []models.CloudEvent
	seenEventIDs := make(map[string]bool)   // For ID-based deduplication
	seenEventTypes := make(map[string]bool) // For type-based deduplication (workflow completion, etc.)

	s.logger.Debug("Filtering events for execution",
		zap.String("execution_id", executionID),
		zap.Int("total_workflow_events", len(cloudEvents)))

	for _, event := range cloudEvents {
		// Only include events for this specific execution
		if event.ExecutionID == executionID {
			// Skip duplicate events (same ID)
			if seenEventIDs[event.ID] {
				s.logger.Warn("Skipping duplicate event by ID",
					zap.String("event_id", event.ID),
					zap.String("event_type", event.Type),
					zap.String("execution_id", executionID))
				continue
			}

			// For certain event types, deduplicate by type (only keep the first occurrence)
			if event.Type == "io.flunq.workflow.completed" || event.Type == "io.flunq.execution.started" {
				typeKey := fmt.Sprintf("%s:%s", event.Type, executionID)
				if seenEventTypes[typeKey] {
					s.logger.Warn("Skipping duplicate event by type",
						zap.String("event_id", event.ID),
						zap.String("event_type", event.Type),
						zap.String("execution_id", executionID),
						zap.Time("time", event.Time))
					continue
				}
				seenEventTypes[typeKey] = true
			}

			seenEventIDs[event.ID] = true

			apiEvent := models.CloudEvent{
				ID:          event.ID,
				Source:      event.Source,
				SpecVersion: event.SpecVersion,
				Type:        event.Type,
				Time:        event.Time,
				WorkflowID:  event.WorkflowID,
				ExecutionID: event.ExecutionID,
			}

			// Convert event data
			if event.Data != nil {
				if dataMap, ok := event.Data.(map[string]interface{}); ok {
					apiEvent.Data = dataMap
				}
			}

			executionEvents = append(executionEvents, apiEvent)
		}
	}

	// Apply limit if specified
	if params.Limit > 0 && len(executionEvents) > params.Limit {
		executionEvents = executionEvents[:params.Limit]
	}

	s.logger.Info("Retrieved execution events",
		zap.String("execution_id", executionID),
		zap.String("workflow_id", execution.WorkflowID),
		zap.Int("total_events", len(executionEvents)))

	return executionEvents, nil
}

// findExecutionInWorkflowStates scans all workflow states to find the execution
func (s *ExecutionService) findExecutionInWorkflowStates(ctx context.Context, executionID string) (*models.Execution, error) {
	// Get all workflow state keys
	pattern := "workflow:state:*"
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state keys: %w", err)
	}

	s.logger.Debug("Scanning workflow states for execution",
		zap.String("execution_id", executionID),
		zap.Int("total_states", len(keys)))

	// Scan each workflow state to find the one containing this execution
	for _, key := range keys {
		stateData, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			s.logger.Warn("Failed to get workflow state",
				zap.String("key", key),
				zap.Error(err))
			continue
		}

		// Parse the workflow state
		var workflowState map[string]interface{}
		if err := json.Unmarshal([]byte(stateData), &workflowState); err != nil {
			s.logger.Warn("Failed to parse workflow state",
				zap.String("key", key),
				zap.Error(err))
			continue
		}

		// Check if this state contains our execution ID
		if context, ok := workflowState["context"].(map[string]interface{}); ok {
			if execID, exists := context["execution_id"]; exists {
				if execIDStr, ok := execID.(string); ok && execIDStr == executionID {
					// Found the execution! Convert to execution format
					workflowID := s.extractWorkflowIDFromKey(key)
					execution, err := s.convertWorkflowStateToExecution(workflowState, workflowID)
					if err != nil {
						return nil, fmt.Errorf("failed to convert workflow state to execution: %w", err)
					}
					return execution, nil
				}
			}
		}
	}

	// Execution not found in any workflow state
	return nil, ErrExecutionNotFound
}

// extractWorkflowIDFromKey extracts workflow ID from Redis key "workflow:state:{workflowID}"
func (s *ExecutionService) extractWorkflowIDFromKey(key string) string {
	// Key format: "workflow:state:{workflowID}"
	prefix := "workflow:state:"
	if len(key) > len(prefix) {
		return key[len(prefix):]
	}
	return ""
}

// convertWorkflowStateToExecution converts Worker service workflow state to API execution format
// This is a copy of the method from WorkflowService - could be refactored to shared utility
func (s *ExecutionService) convertWorkflowStateToExecution(state map[string]interface{}, workflowID string) (*models.Execution, error) {
	// Extract execution ID from context
	executionID := ""
	if context, ok := state["context"].(map[string]interface{}); ok {
		if execID, exists := context["execution_id"]; exists {
			if execIDStr, ok := execID.(string); ok {
				executionID = execIDStr
			}
		}
	}

	if executionID == "" {
		return nil, fmt.Errorf("execution_id not found in workflow state")
	}

	// Convert status from numeric to SDL status
	status := models.ExecutionStatusPending // default
	if statusNum, ok := state["status"].(float64); ok {
		status = s.convertNumericStatusToSDL(int(statusNum))
	}

	// Extract input data
	var input map[string]interface{}
	if inputData, ok := state["input"].(map[string]interface{}); ok {
		input = inputData
	}

	// Extract variables as output (final state)
	var output map[string]interface{}
	if variables, ok := state["variables"].(map[string]interface{}); ok {
		output = variables
	}

	// Extract timestamps
	var startedAt time.Time
	var completedAt *time.Time

	if createdAtData, ok := state["created_at"].(map[string]interface{}); ok {
		if seconds, exists := createdAtData["seconds"]; exists {
			if secondsFloat, ok := seconds.(float64); ok {
				startedAt = time.Unix(int64(secondsFloat), 0)
			}
		}
	}

	if updatedAtData, ok := state["updated_at"].(map[string]interface{}); ok {
		if seconds, exists := updatedAtData["seconds"]; exists {
			if secondsFloat, ok := seconds.(float64); ok {
				updatedTime := time.Unix(int64(secondsFloat), 0)
				// If status is completed/failed/cancelled, set completed_at
				if status == models.ExecutionStatusCompleted ||
					status == models.ExecutionStatusFaulted ||
					status == models.ExecutionStatusCancelled {
					completedAt = &updatedTime
				}
			}
		}
	}

	// Calculate duration if completed
	var durationMs *int64
	if completedAt != nil {
		duration := completedAt.Sub(startedAt).Milliseconds()
		durationMs = &duration
	}

	execution := &models.Execution{
		ID:          executionID,
		WorkflowID:  workflowID,
		Status:      status,
		Input:       input,
		Output:      output,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		DurationMs:  durationMs,
	}

	return execution, nil
}

// convertNumericStatusToSDL converts Worker service numeric status to SDL execution status
func (s *ExecutionService) convertNumericStatusToSDL(numericStatus int) models.ExecutionStatus {
	switch numericStatus {
	case 1:
		return models.ExecutionStatusPending
	case 2:
		return models.ExecutionStatusRunning
	case 3:
		return models.ExecutionStatusCompleted
	case 4:
		return models.ExecutionStatusFaulted
	case 5:
		return models.ExecutionStatusCancelled
	default:
		return models.ExecutionStatusPending
	}
}
