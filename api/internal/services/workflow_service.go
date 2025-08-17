package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/adapters"
	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// Common service errors
var (
	ErrWorkflowNotFound    = errors.New("workflow not found")
	ErrExecutionNotFound   = errors.New("execution not found")
	ErrExecutionNotRunning = errors.New("execution is not running")
)

// WorkflowRepository interface for workflow data access
type WorkflowRepository interface {
	Create(ctx context.Context, workflow *models.Workflow) error
	GetByID(ctx context.Context, id string) (*models.WorkflowDetail, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error)
}

// ExecutionRepository interface for execution data access
type ExecutionRepository interface {
	Create(ctx context.Context, execution *models.Execution) error
	GetByID(ctx context.Context, id string) (*models.Execution, error)
	Update(ctx context.Context, execution *models.Execution) error
	List(ctx context.Context, params *models.ExecutionListParams) ([]models.Execution, int, error)
}

// WorkflowService handles workflow business logic
type WorkflowService struct {
	workflowAdapter *adapters.WorkflowAdapter
	executionRepo   ExecutionRepository
	eventStream     interfaces.EventStream
	redisClient     *redis.Client
	inputValidator  *InputValidator
	logger          *zap.Logger
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(
	database interfaces.Database,
	executionRepo ExecutionRepository,
	eventStream interfaces.EventStream,
	redisClient *redis.Client,
	logger *zap.Logger,
) *WorkflowService {
	return &WorkflowService{
		workflowAdapter: adapters.NewWorkflowAdapter(database),
		executionRepo:   executionRepo,
		eventStream:     eventStream,
		redisClient:     redisClient,
		inputValidator:  NewInputValidator(logger),
		logger:          logger,
	}
}

// CreateWorkflow creates a new workflow
func (s *WorkflowService) CreateWorkflow(ctx context.Context, req *models.CreateWorkflowRequest) (*models.Workflow, error) {
	// Validate input schema if provided
	if req.InputSchema != nil {
		if err := s.inputValidator.ValidateSchema(req.InputSchema); err != nil {
			s.logger.Error("Invalid input schema provided", zap.Error(err))
			return nil, &models.ValidationError{
				Field:   "inputSchema",
				Message: fmt.Sprintf("Invalid JSON schema: %v", err),
			}
		}
		s.logger.Debug("Input schema validation passed", zap.String("workflow_name", req.Name))
	}

	// Generate workflow ID
	workflowID := generateWorkflowID()

	// Create workflow model
	workflow := &models.Workflow{
		ID:          workflowID,
		Name:        req.Name,
		Description: req.Description,
		TenantID:    req.TenantID,
		Definition:  req.Definition,
		InputSchema: req.InputSchema,
		State:       models.WorkflowStateActive,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Validate workflow
	if err := workflow.Validate(); err != nil {
		return nil, err
	}

	// Save to repository
	if err := s.workflowAdapter.Create(ctx, workflow); err != nil {
		s.logger.Error("Failed to create workflow in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Publish workflow created event
	event := cloudevents.NewCloudEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.WorkflowCreated,
	)
	event.WorkflowID = workflowID
	event.Time = time.Now()
	event.TenantID = workflow.TenantID

	// Extract workflow definition ID for stream routing
	workflowDefinitionID := "default"
	if workflow.Definition != nil {
		if defID, exists := workflow.Definition["id"]; exists {
			if defIDStr, ok := defID.(string); ok && defIDStr != "" {
				workflowDefinitionID = defIDStr
			}
		} else if defName, exists := workflow.Definition["name"]; exists {
			if defNameStr, ok := defName.(string); ok && defNameStr != "" {
				workflowDefinitionID = defNameStr
			}
		}
	}

	event.SetData(map[string]interface{}{
		"name":                   workflow.Name,
		"description":            workflow.Description,
		"state":                  workflow.State,
		"tenant_id":              workflow.TenantID,
		"workflow_definition_id": workflowDefinitionID, // Just the definition ID for stream routing
		"definition":             workflow.Definition,  // Include the workflow definition
		"created_by":             "api",                // TODO: Get from authentication context
	})

	// Add tenant and workflow metadata to event (using proper CloudEvent fields)
	event.TenantID = workflow.TenantID
	event.WorkflowID = workflowID

	// Publish using shared event streaming (auto-routes to tenant-isolated streams)
	if err := s.eventStream.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish workflow created event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow created successfully",
		zap.String("workflow_id", workflowID),
		zap.String("name", workflow.Name))

	return workflow, nil
}

// GetWorkflow retrieves a workflow by ID
func (s *WorkflowService) GetWorkflow(ctx context.Context, workflowID string) (*models.WorkflowDetail, error) {
	workflow, err := s.workflowAdapter.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow from repository", zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return workflow, nil
}

// GetWorkflowByTenant retrieves a workflow by ID with explicit tenant scoping
func (s *WorkflowService) GetWorkflowByTenant(ctx context.Context, tenantID string, workflowID string) (*models.WorkflowDetail, error) {
	workflow, err := s.workflowAdapter.GetByIDWithTenant(ctx, tenantID, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow from repository (tenant)", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return workflow, nil
}

// UpdateWorkflow updates an existing workflow
func (s *WorkflowService) UpdateWorkflow(ctx context.Context, workflowID string, req *models.UpdateWorkflowRequest) (*models.Workflow, error) {
	// Get existing workflow
	existingWorkflow, err := s.workflowAdapter.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Convert to base workflow for updating
	workflow := &existingWorkflow.Workflow

	// Update fields if provided
	if req.Name != nil {
		workflow.Name = *req.Name
	}
	if req.Description != nil {
		workflow.Description = *req.Description
	}
	if req.Definition != nil {
		workflow.Definition = req.Definition
	}
	if req.Tags != nil {
		workflow.Tags = req.Tags
	}
	workflow.UpdatedAt = time.Now()

	// Validate updated workflow
	if err := workflow.Validate(); err != nil {
		return nil, err
	}

	// Save to repository
	if err := s.workflowAdapter.Update(ctx, workflow); err != nil {
		s.logger.Error("Failed to update workflow in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	// Publish workflow updated event
	event := cloudevents.NewCloudEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		"io.flunq.workflow.updated",
	)
	event.WorkflowID = workflowID
	event.Time = time.Now()
	event.SetData(map[string]interface{}{
		"name":        workflow.Name,
		"description": workflow.Description,
		"state":       workflow.State,
		"updated_by":  "api", // TODO: Get from authentication context
	})

	// Add tenant and workflow metadata to event (using proper CloudEvent fields)
	event.TenantID = workflow.TenantID
	event.WorkflowID = workflowID

	// Publish using shared event streaming (auto-routes to tenant-isolated streams)
	if err := s.eventStream.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish workflow updated event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow updated successfully",
		zap.String("workflow_id", workflowID))

	return workflow, nil
}

// DeleteWorkflow deletes a workflow
func (s *WorkflowService) DeleteWorkflow(ctx context.Context, workflowID string) error {
	// Check if workflow exists and get tenant ID for event publishing
	workflow, err := s.workflowAdapter.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return ErrWorkflowNotFound
		}
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	// TODO: Check if there are running executions and prevent deletion

	// Delete from repository
	if err := s.workflowAdapter.Delete(ctx, workflowID); err != nil {
		s.logger.Error("Failed to delete workflow from repository", zap.Error(err))
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	// Publish workflow deleted event
	event := cloudevents.NewCloudEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		"io.flunq.workflow.deleted",
	)
	event.WorkflowID = workflowID
	event.Time = time.Now()
	event.SetData(map[string]interface{}{
		"deleted_by": "api", // TODO: Get from authentication context
	})

	// Add tenant and workflow metadata to event (using proper CloudEvent fields)
	event.TenantID = workflow.TenantID
	event.WorkflowID = workflowID

	// Publish using shared event streaming (auto-routes to tenant-isolated streams)
	if err := s.eventStream.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish workflow deleted event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow deleted successfully",
		zap.String("workflow_id", workflowID))

	return nil
}

// ListWorkflows lists workflows with optional filtering
func (s *WorkflowService) ListWorkflows(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	workflows, total, err := s.workflowAdapter.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list workflows from repository", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list workflows: %w", err)
	}

	return workflows, total, nil
}

// ListWorkflowsWithFilters lists workflows with enhanced filtering and returns filter metadata
func (s *WorkflowService) ListWorkflowsWithFilters(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, int, map[string]interface{}, error) {
	// Get all workflows first (without pagination) to apply filters to entire dataset
	allParams := *params
	allParams.Limit = 0 // Get all records
	allParams.Offset = 0
	allParams.Page = 0
	allParams.Size = 0

	allWorkflows, total, err := s.workflowAdapter.List(ctx, &allParams)
	if err != nil {
		s.logger.Error("Failed to list all workflows from repository", zap.Error(err))
		return nil, 0, 0, nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	// Apply filters to the entire dataset
	filteredWorkflows, appliedFilters := s.applyWorkflowFilters(allWorkflows, params)
	filteredCount := len(filteredWorkflows)

	// Apply sorting
	s.sortWorkflows(filteredWorkflows, params.SortBy, params.SortOrder)

	// Apply pagination to filtered results
	paginatedWorkflows := s.paginateWorkflows(filteredWorkflows, params.PaginationParams)

	return paginatedWorkflows, total, filteredCount, appliedFilters, nil
}

// ExecuteWorkflow starts execution of a workflow
func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, workflowID string, req *models.ExecuteWorkflowRequest) (*models.Execution, error) {
	s.logger.Info("Starting workflow execution",
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", req.TenantID))

	// Get workflow using tenant-aware lookup
	workflow, err := s.workflowAdapter.GetByIDWithTenant(ctx, req.TenantID, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			s.logger.Error("Workflow not found",
				zap.String("workflow_id", workflowID),
				zap.String("tenant_id", req.TenantID))
			return nil, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow",
			zap.String("workflow_id", workflowID),
			zap.String("tenant_id", req.TenantID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	s.logger.Info("Found workflow for execution",
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", req.TenantID),
		zap.String("workflow_name", workflow.Name))

	// Check if workflow can be executed
	if workflow.State == models.WorkflowStateInactive {
		return nil, fmt.Errorf("workflow cannot be executed in %s state", workflow.State)
	}

	// Validate input against workflow input schema if defined
	s.logger.Info("Checking input schema for validation",
		zap.String("workflow_id", workflowID),
		zap.Bool("has_input_schema", workflow.InputSchema != nil),
		zap.Int("schema_fields", len(workflow.InputSchema)),
		zap.String("execution_input_keys", fmt.Sprintf("%v", getMapKeys(req.Input))))

	if workflow.InputSchema != nil && len(workflow.InputSchema) > 0 {
		s.logger.Info("Validating execution input against workflow schema",
			zap.String("workflow_id", workflowID),
			zap.String("execution_input_keys", fmt.Sprintf("%v", getMapKeys(req.Input))))

		validationResult, err := s.inputValidator.ValidateInput(workflow.InputSchema, req.Input)
		if err != nil {
			s.logger.Error("Input validation error",
				zap.String("workflow_id", workflowID),
				zap.Error(err))
			return nil, fmt.Errorf("input validation failed: %w", err)
		}

		if !validationResult.Valid {
			errorMessage := s.inputValidator.FormatValidationErrors(validationResult.Errors, workflow.InputSchema)
			s.logger.Info("Input validation failed",
				zap.String("workflow_id", workflowID),
				zap.Int("error_count", len(validationResult.Errors)),
				zap.String("formatted_errors", errorMessage))

			return nil, &models.ValidationError{
				Field:   "input",
				Message: errorMessage,
			}
		}

		s.logger.Info("Input validation passed",
			zap.String("workflow_id", workflowID))
	} else {
		s.logger.Info("No input schema defined, accepting any input",
			zap.String("workflow_id", workflowID))
	}

	// Generate execution ID
	executionID := generateExecutionID()

	// Create execution with tenant context from request
	execution := &models.Execution{
		ID:            executionID,
		WorkflowID:    workflowID,
		Status:        models.ExecutionStatusPending,
		CorrelationID: req.CorrelationID,
		Input:         req.Input,
		StartedAt:     time.Now(),
		TenantID:      req.TenantID, // Use tenant ID from request
	}

	s.logger.Info("Creating execution",
		zap.String("execution_id", executionID),
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", execution.TenantID))

	// Save execution to repository
	if err := s.executionRepo.Create(ctx, execution); err != nil {
		s.logger.Error("Failed to create execution in repository",
			zap.String("execution_id", executionID),
			zap.String("workflow_id", workflowID),
			zap.String("tenant_id", execution.TenantID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	s.logger.Info("Execution created successfully",
		zap.String("execution_id", executionID),
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", execution.TenantID))

	// Publish workflow execution started event
	event := cloudevents.NewCloudEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.ExecutionStarted,
	)
	event.WorkflowID = workflowID
	event.ExecutionID = executionID
	event.Time = time.Now()
	event.TenantID = req.TenantID
	// Extract workflow definition ID for stream routing
	workflowDefinitionID := "default"
	if workflow.Definition != nil {
		if defID, exists := workflow.Definition["id"]; exists {
			if defIDStr, ok := defID.(string); ok && defIDStr != "" {
				workflowDefinitionID = defIDStr
			}
		} else if defName, exists := workflow.Definition["name"]; exists {
			if defNameStr, ok := defName.(string); ok && defNameStr != "" {
				workflowDefinitionID = defNameStr
			}
		}
	}

	event.SetData(map[string]interface{}{
		"workflow_name":          workflow.Name,
		"workflow_definition_id": workflowDefinitionID, // Just the definition ID for stream routing
		"correlation_id":         execution.CorrelationID,
		"input":                  execution.Input,
		"tenant_id":              req.TenantID,
		"started_by":             "api", // TODO: Get from authentication context
	})

	// Add tenant, workflow, and execution metadata to event (using proper CloudEvent fields)
	event.TenantID = req.TenantID
	event.WorkflowID = workflow.ID
	event.ExecutionID = executionID

	// Publish using shared event streaming (auto-routes to tenant-isolated streams)
	if err := s.eventStream.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish execution started event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow execution started",
		zap.String("workflow_id", workflowID),
		zap.String("execution_id", executionID))

	return execution, nil
}

// GetWorkflowEvents retrieves event history for a workflow
func (s *WorkflowService) GetWorkflowEvents(ctx context.Context, workflowID string, params *models.EventHistoryParams) ([]models.CloudEvent, error) {
	// Get events from Event Store directly - don't check if workflow exists
	// TODO: Implement event history reading with shared event streaming
	// For now, return empty list since shared event streaming doesn't have ReadHistory method
	// This can be implemented later by querying Redis streams directly or adding ReadHistory to shared interface
	var events []*cloudevents.CloudEvent

	s.logger.Warn("GetWorkflowEvents not fully implemented with shared event streaming",
		zap.String("workflow_id", workflowID))

	// Convert to API models
	apiEvents := make([]models.CloudEvent, len(events))
	for i, event := range events {
		apiEvents[i] = models.CloudEvent{
			ID:              event.ID,
			Source:          event.Source,
			SpecVersion:     event.SpecVersion,
			Type:            event.Type,
			DataContentType: event.DataContentType,
			Subject:         event.Subject,
			Time:            event.Time,
			Data:            event.Data.(map[string]interface{}),
			WorkflowID:      event.WorkflowID,
			ExecutionID:     event.ExecutionID,
		}
	}

	// Apply limit if specified
	if params.Limit > 0 && len(apiEvents) > params.Limit {
		apiEvents = apiEvents[:params.Limit]
	}

	return apiEvents, nil
}

// GetWorkflowExecutions retrieves all executions for a specific workflow
// NOTE: This now uses the execution repository (backed by memory for now) instead of reading a single workflow state key.
// Next step is to switch to the shared Database interface for tenant-aware, persistent storage (e.g., Postgres).
func (s *WorkflowService) GetWorkflowExecutions(ctx context.Context, workflowID string, params *models.ExecutionListParams) ([]models.Execution, int, error) {
	s.logger.Debug("Getting workflow executions",
		zap.String("workflow_id", workflowID))

	// Ensure workflow exists (and get tenant for future filtering)
	wf, err := s.workflowAdapter.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, 0, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow from adapter",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Use the execution repository for now (returns many executions)
	if params == nil {
		params = &models.ExecutionListParams{}
	}
	params.WorkflowID = workflowID

	executions, total, err := s.executionRepo.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list executions from repository",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list executions: %w", err)
	}

	// Sort by start date (newest first); if StartedAt is zero, fallback to CompletedAt
	sort.Slice(executions, func(i, j int) bool {
		si := executions[i].StartedAt
		sj := executions[j].StartedAt
		if si.IsZero() && executions[i].CompletedAt != nil {
			si = *executions[i].CompletedAt
		}
		if sj.IsZero() && executions[j].CompletedAt != nil {
			sj = *executions[j].CompletedAt
		}
		return si.After(sj)
	})

	s.logger.Info("Retrieved workflow executions",
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", wf.Workflow.TenantID),
		zap.Int("total_executions", total))

	return executions, total, nil
}

// GetWorkflowExecutionsByTenant ensures the workflow exists for the given tenant before listing executions
func (s *WorkflowService) GetWorkflowExecutionsByTenant(ctx context.Context, tenantID string, workflowID string, params *models.ExecutionListParams) ([]models.Execution, int, error) {
	s.logger.Debug("Getting workflow executions (tenant)",
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", tenantID))

	// Ensure workflow exists scoped to tenant
	wf, err := s.workflowAdapter.GetByIDWithTenant(ctx, tenantID, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, 0, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow from adapter (tenant)",
			zap.String("workflow_id", workflowID),
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Use the execution repository (not yet tenant-scoped)
	if params == nil {
		params = &models.ExecutionListParams{}
	}
	params.WorkflowID = workflowID

	executions, total, err := s.executionRepo.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list executions from repository",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list executions: %w", err)
	}

	// Sort by start date (newest first); if StartedAt is zero, fallback to CompletedAt
	sort.Slice(executions, func(i, j int) bool {
		si := executions[i].StartedAt
		sj := executions[j].StartedAt
		if si.IsZero() && executions[i].CompletedAt != nil {
			si = *executions[i].CompletedAt
		}
		if sj.IsZero() && executions[j].CompletedAt != nil {
			sj = *executions[j].CompletedAt
		}
		return si.After(sj)
	})

	s.logger.Info("Retrieved workflow executions (tenant)",
		zap.String("workflow_id", workflowID),
		zap.String("tenant_id", wf.Workflow.TenantID),
		zap.Int("total_executions", total))

	return executions, total, nil
}

// getExecutionsFromWorkflowState reads workflow state from Redis and converts to execution format
func (s *WorkflowService) getExecutionsFromWorkflowState(ctx context.Context, workflowID string) ([]models.Execution, error) {
	// The Worker service stores workflow state with key: workflow:state:{workflowID}
	stateKey := fmt.Sprintf("workflow:state:%s", workflowID)

	s.logger.Debug("Reading workflow state from Redis",
		zap.String("workflow_id", workflowID),
		zap.String("state_key", stateKey))

	// Get workflow state from Redis
	stateData, err := s.redisClient.Get(ctx, stateKey).Result()
	if err != nil {
		if err == redis.Nil {
			// No state found - workflow hasn't been executed yet
			s.logger.Debug("No workflow state found",
				zap.String("workflow_id", workflowID))
			return []models.Execution{}, nil
		}
		return nil, fmt.Errorf("failed to get workflow state from Redis: %w", err)
	}

	// Parse the workflow state JSON
	var workflowState map[string]interface{}
	if err := json.Unmarshal([]byte(stateData), &workflowState); err != nil {
		return nil, fmt.Errorf("failed to parse workflow state JSON: %w", err)
	}

	// Convert workflow state to execution format
	execution, err := s.convertWorkflowStateToExecution(workflowState, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow state to execution: %w", err)
	}

	s.logger.Info("Successfully converted workflow state to execution",
		zap.String("workflow_id", workflowID),
		zap.String("execution_id", execution.ID),
		zap.String("status", string(execution.Status)))

	return []models.Execution{*execution}, nil
}

// convertWorkflowStateToExecution converts Worker service workflow state to API execution format
func (s *WorkflowService) convertWorkflowStateToExecution(state map[string]interface{}, workflowID string) (*models.Execution, error) {
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
func (s *WorkflowService) convertNumericStatusToSDL(numericStatus int) models.ExecutionStatus {
	// Based on the Worker service protobuf definitions:
	// 0 = WORKFLOW_STATUS_UNSPECIFIED
	// 1 = WORKFLOW_STATUS_CREATED
	// 2 = WORKFLOW_STATUS_RUNNING
	// 3 = WORKFLOW_STATUS_COMPLETED
	// 4 = WORKFLOW_STATUS_FAILED
	// 5 = WORKFLOW_STATUS_CANCELLED

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

// generateWorkflowID generates a unique workflow ID
func generateWorkflowID() string {
	return "wf_" + uuid.New().String()[:16]
}

// generateExecutionID generates a unique execution ID
func generateExecutionID() string {
	return "ex_" + uuid.New().String()[:16]
}

// isNotFoundError checks if an error represents a "not found" condition
func isNotFoundError(err error) bool {
	// This would be implemented based on your repository layer
	// For now, we'll use a simple string check
	return err != nil && (err.Error() == "not found" || err.Error() == "record not found")
}

// applyWorkflowFilters applies filters to workflows and returns filtered results with applied filter metadata
func (s *WorkflowService) applyWorkflowFilters(workflows []models.Workflow, params *models.WorkflowListParams) ([]models.Workflow, map[string]interface{}) {
	filtered := make([]models.Workflow, 0, len(workflows))
	appliedFilters := make(map[string]interface{})

	for _, workflow := range workflows {
		include := true

		// Status filter
		if params.Status != "" {
			if string(workflow.State) != params.Status {
				include = false
			}
			appliedFilters["status"] = params.Status
		}

		// Name filter (partial match, case-insensitive)
		if params.Name != "" {
			if !containsIgnoreCase(workflow.Name, params.Name) {
				include = false
			}
			appliedFilters["name"] = params.Name
		}

		// Description filter (partial match, case-insensitive)
		if params.Description != "" {
			if !containsIgnoreCase(workflow.Description, params.Description) {
				include = false
			}
			appliedFilters["description"] = params.Description
		}

		// Tags filter (comma-separated, any match)
		if params.Tags != "" {
			tagFilters := parseCommaSeparated(params.Tags)
			if !hasAnyTag(workflow.Tags, tagFilters) {
				include = false
			}
			appliedFilters["tags"] = tagFilters
		}

		// Search filter (searches name, description, and tags)
		if params.Search != "" {
			if !searchWorkflow(workflow, params.Search) {
				include = false
			}
			appliedFilters["search"] = params.Search
		}

		// Tenant filter
		if params.TenantID != "" {
			if workflow.TenantID != params.TenantID {
				include = false
			}
			appliedFilters["tenant_id"] = params.TenantID
		}

		// Date range filters
		if params.CreatedAt != "" {
			if !isInDateRange(workflow.CreatedAt, params.CreatedAt) {
				include = false
			}
			appliedFilters["created_at"] = params.CreatedAt
		}

		if params.UpdatedAt != "" {
			if !isInDateRange(workflow.UpdatedAt, params.UpdatedAt) {
				include = false
			}
			appliedFilters["updated_at"] = params.UpdatedAt
		}

		if include {
			filtered = append(filtered, workflow)
		}
	}

	return filtered, appliedFilters
}

// sortWorkflows sorts workflows by the specified field and order
func (s *WorkflowService) sortWorkflows(workflows []models.Workflow, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(workflows, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = workflows[i].Name < workflows[j].Name
		case "description":
			less = workflows[i].Description < workflows[j].Description
		case "state":
			less = workflows[i].State < workflows[j].State
		case "tenant_id":
			less = workflows[i].TenantID < workflows[j].TenantID
		case "updated_at":
			less = workflows[i].UpdatedAt.Before(workflows[j].UpdatedAt)
		case "created_at":
		default:
			less = workflows[i].CreatedAt.Before(workflows[j].CreatedAt)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// paginateWorkflows applies pagination to workflows
func (s *WorkflowService) paginateWorkflows(workflows []models.Workflow, params models.PaginationParams) []models.Workflow {
	var limit, offset int

	if params.Page > 0 && params.Size > 0 {
		// Page-based pagination
		limit = params.Size
		offset = (params.Page - 1) * params.Size
	} else {
		// Offset-based pagination
		limit = params.Limit
		if limit == 0 {
			limit = 20 // default
		}
		offset = params.Offset
	}

	if offset >= len(workflows) {
		return []models.Workflow{}
	}

	end := offset + limit
	if end > len(workflows) {
		end = len(workflows)
	}

	return workflows[offset:end]
}

// searchWorkflow performs a global search across workflow fields
func searchWorkflow(workflow models.Workflow, searchTerm string) bool {
	// Search in name
	if containsIgnoreCase(workflow.Name, searchTerm) {
		return true
	}

	// Search in description
	if containsIgnoreCase(workflow.Description, searchTerm) {
		return true
	}

	// Search in tags
	for _, tag := range workflow.Tags {
		if containsIgnoreCase(tag, searchTerm) {
			return true
		}
	}

	// Search in tenant ID
	if containsIgnoreCase(workflow.TenantID, searchTerm) {
		return true
	}

	return false
}

// getMapKeys returns the keys of a map for debugging purposes
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
