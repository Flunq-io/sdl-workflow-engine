package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// ExecutionService handles execution business logic
type ExecutionService struct {
	executionRepo ExecutionRepository
	eventClient   *client.EventClient
	logger        *zap.Logger
}

// NewExecutionService creates a new execution service
func NewExecutionService(
	executionRepo ExecutionRepository,
	eventClient *client.EventClient,
	logger *zap.Logger,
) *ExecutionService {
	return &ExecutionService{
		executionRepo: executionRepo,
		eventClient:   eventClient,
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

// GetExecution retrieves an execution by ID
func (s *ExecutionService) GetExecution(ctx context.Context, executionID string) (*models.Execution, error) {
	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrExecutionNotFound
		}
		s.logger.Error("Failed to get execution from repository", zap.Error(err))
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

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
	event := cloudevents.NewExecutionEvent(
		uuid.New().String(),
		cloudevents.SourceAPI,
		cloudevents.ExecutionCancelled,
		execution.WorkflowID,
		executionID,
	)
	event.SetData(map[string]interface{}{
		"reason":       reason,
		"cancelled_by": "api", // TODO: Get from authentication context
		"status":       execution.Status,
	})

	if err := s.eventClient.PublishEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish execution cancelled event", zap.Error(err))
		// Don't fail the operation, just log the error
	}

	s.logger.Info("Execution cancelled successfully",
		zap.String("execution_id", executionID),
		zap.String("reason", reason))

	return execution, nil
}
