package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
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
	workflowRepo  WorkflowRepository
	executionRepo ExecutionRepository
	eventClient   *client.EventClient
	logger        *zap.Logger
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(
	workflowRepo WorkflowRepository,
	executionRepo ExecutionRepository,
	eventClient *client.EventClient,
	logger *zap.Logger,
) *WorkflowService {
	return &WorkflowService{
		workflowRepo:  workflowRepo,
		executionRepo: executionRepo,
		eventClient:   eventClient,
		logger:        logger,
	}
}

// CreateWorkflow creates a new workflow
func (s *WorkflowService) CreateWorkflow(ctx context.Context, req *models.CreateWorkflowRequest) (*models.Workflow, error) {
	// Generate workflow ID
	workflowID := generateWorkflowID()

	// Create workflow model
	workflow := &models.Workflow{
		ID:          workflowID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
		Status:      models.WorkflowStatusPending,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Validate workflow
	if err := workflow.Validate(); err != nil {
		return nil, err
	}

	// Save to repository
	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		s.logger.Error("Failed to create workflow in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Publish workflow created event
	event := cloudevents.NewWorkflowEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.WorkflowCreated,
		workflowID,
	)
	event.SetData(map[string]interface{}{
		"name":        workflow.Name,
		"description": workflow.Description,
		"status":      workflow.Status,
		"created_by":  "api", // TODO: Get from authentication context
	})

	if err := s.eventClient.PublishEvent(ctx, event); err != nil {
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
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrWorkflowNotFound
		}
		s.logger.Error("Failed to get workflow from repository", zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return workflow, nil
}

// UpdateWorkflow updates an existing workflow
func (s *WorkflowService) UpdateWorkflow(ctx context.Context, workflowID string, req *models.UpdateWorkflowRequest) (*models.Workflow, error) {
	// Get existing workflow
	existingWorkflow, err := s.workflowRepo.GetByID(ctx, workflowID)
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
	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		s.logger.Error("Failed to update workflow in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	// Publish workflow updated event
	event := cloudevents.NewWorkflowEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		"io.flunq.workflow.updated",
		workflowID,
	)
	event.SetData(map[string]interface{}{
		"name":        workflow.Name,
		"description": workflow.Description,
		"status":      workflow.Status,
		"updated_by":  "api", // TODO: Get from authentication context
	})

	if err := s.eventClient.PublishEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish workflow updated event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow updated successfully",
		zap.String("workflow_id", workflowID))

	return workflow, nil
}

// DeleteWorkflow deletes a workflow
func (s *WorkflowService) DeleteWorkflow(ctx context.Context, workflowID string) error {
	// Check if workflow exists
	_, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return ErrWorkflowNotFound
		}
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	// TODO: Check if there are running executions and prevent deletion

	// Delete from repository
	if err := s.workflowRepo.Delete(ctx, workflowID); err != nil {
		s.logger.Error("Failed to delete workflow from repository", zap.Error(err))
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	// Publish workflow deleted event
	event := cloudevents.NewWorkflowEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		"io.flunq.workflow.deleted",
		workflowID,
	)
	event.SetData(map[string]interface{}{
		"deleted_by": "api", // TODO: Get from authentication context
	})

	if err := s.eventClient.PublishEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish workflow deleted event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Workflow deleted successfully",
		zap.String("workflow_id", workflowID))

	return nil
}

// ListWorkflows lists workflows with optional filtering
func (s *WorkflowService) ListWorkflows(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	workflows, total, err := s.workflowRepo.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list workflows from repository", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list workflows: %w", err)
	}

	return workflows, total, nil
}

// ExecuteWorkflow starts execution of a workflow
func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, workflowID string, req *models.ExecuteWorkflowRequest) (*models.Execution, error) {
	// Get workflow
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Check if workflow can be executed
	if workflow.Status == models.WorkflowStatusCancelled || workflow.Status == models.WorkflowStatusFaulted {
		return nil, fmt.Errorf("workflow cannot be executed in %s status", workflow.Status)
	}

	// Generate execution ID
	executionID := generateExecutionID()

	// Create execution
	execution := &models.Execution{
		ID:            executionID,
		WorkflowID:    workflowID,
		Status:        models.ExecutionStatusPending,
		CorrelationID: req.CorrelationID,
		Input:         req.Input,
		StartedAt:     time.Now(),
	}

	// Save execution to repository
	if err := s.executionRepo.Create(ctx, execution); err != nil {
		s.logger.Error("Failed to create execution in repository", zap.Error(err))
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	// Publish workflow execution started event
	event := cloudevents.NewExecutionEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.ExecutionStarted,
		workflowID,
		executionID,
	)
	event.SetData(map[string]interface{}{
		"workflow_name":  workflow.Name,
		"correlation_id": execution.CorrelationID,
		"input":          execution.Input,
		"started_by":     "api", // TODO: Get from authentication context
	})

	if err := s.eventClient.PublishEvent(ctx, event); err != nil {
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
	// Events can exist for workflows that might have been deleted or are not in current repository
	var events []*cloudevents.CloudEvent
	var err error

	if params.Since != "" {
		events, err = s.eventClient.GetEventHistory(ctx, workflowID) // TODO: Implement GetEventsSince
	} else {
		events, err = s.eventClient.GetEventHistory(ctx, workflowID)
	}

	if err != nil {
		s.logger.Error("Failed to get workflow events", zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow events: %w", err)
	}

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
