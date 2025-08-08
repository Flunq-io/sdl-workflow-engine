package adapters

import (
	"context"
	"fmt"

	"github.com/flunq-io/api/internal/models"
	sharedInterfaces "github.com/flunq-io/shared/pkg/interfaces"
)

// ExecutionRepository interface for execution data access (avoiding import cycle)
type ExecutionRepository interface {
	Create(ctx context.Context, execution *models.Execution) error
	GetByID(ctx context.Context, id string) (*models.Execution, error)
	Update(ctx context.Context, execution *models.Execution) error
	List(ctx context.Context, params *models.ExecutionListParams) ([]models.Execution, int, error)
}

// SharedExecutionRepository adapts the shared database interface to the API's execution repository interface
type SharedExecutionRepository struct {
	sharedDB sharedInterfaces.Database
}

// NewSharedExecutionRepository creates a new shared execution repository
func NewSharedExecutionRepository(sharedDB sharedInterfaces.Database) ExecutionRepository {
	return &SharedExecutionRepository{
		sharedDB: sharedDB,
	}
}

// Create creates a new execution
func (r *SharedExecutionRepository) Create(ctx context.Context, execution *models.Execution) error {
	// Use tenant ID from execution model (passed from workflow)
	tenantID := execution.TenantID
	if tenantID == "" {
		// Fallback to context extraction if not provided
		tenantID = r.extractTenantFromContext(ctx)
	}

	// Convert from API model to shared interface model
	sharedExecution := &sharedInterfaces.WorkflowExecution{
		ID:         execution.ID,
		WorkflowID: execution.WorkflowID,
		StartedBy:  "api-service", // TODO: Get from context
		StartedAt:  execution.StartedAt,
		TenantID:   tenantID,
	}

	// Convert status
	switch execution.Status {
	case models.ExecutionStatusPending:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusPending
	case models.ExecutionStatusRunning:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusRunning
	case models.ExecutionStatusCompleted:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusCompleted
	case models.ExecutionStatusFaulted:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusFailed
	case models.ExecutionStatusCancelled:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusCancelled
	default:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusPending
	}

	// Convert input/output
	if execution.Input != nil {
		sharedExecution.Input = execution.Input
	}
	if execution.Output != nil {
		sharedExecution.Output = execution.Output
	}

	// Set completed time if execution is completed
	if execution.CompletedAt != nil {
		sharedExecution.CompletedAt = execution.CompletedAt
	}

	return r.sharedDB.CreateExecution(ctx, sharedExecution)
}

// GetByID retrieves an execution by ID
func (r *SharedExecutionRepository) GetByID(ctx context.Context, id string) (*models.Execution, error) {
	sharedExecution, err := r.sharedDB.GetExecution(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert from shared interface model to API model
	execution := &models.Execution{
		ID:         sharedExecution.ID,
		WorkflowID: sharedExecution.WorkflowID,
		StartedAt:  sharedExecution.StartedAt,
	}

	// Convert status
	switch sharedExecution.Status {
	case sharedInterfaces.WorkflowStatusPending:
		execution.Status = models.ExecutionStatusPending
	case sharedInterfaces.WorkflowStatusRunning:
		execution.Status = models.ExecutionStatusRunning
	case sharedInterfaces.WorkflowStatusCompleted:
		execution.Status = models.ExecutionStatusCompleted
	case sharedInterfaces.WorkflowStatusFailed:
		execution.Status = models.ExecutionStatusFaulted
	case sharedInterfaces.WorkflowStatusCancelled:
		execution.Status = models.ExecutionStatusCancelled
	default:
		execution.Status = models.ExecutionStatusPending
	}

	// Convert input/output
	if sharedExecution.Input != nil {
		execution.Input = sharedExecution.Input
	}
	if sharedExecution.Output != nil {
		execution.Output = sharedExecution.Output
	}

	// Set completed time if available
	if sharedExecution.CompletedAt != nil {
		execution.CompletedAt = sharedExecution.CompletedAt
	}

	return execution, nil
}

// Update updates an execution
func (r *SharedExecutionRepository) Update(ctx context.Context, execution *models.Execution) error {
	// Use tenant ID from execution model
	tenantID := execution.TenantID
	if tenantID == "" {
		// Fallback to context extraction if not provided
		tenantID = r.extractTenantFromContext(ctx)
	}

	// Convert from API model to shared interface model
	sharedExecution := &sharedInterfaces.WorkflowExecution{
		ID:         execution.ID,
		WorkflowID: execution.WorkflowID,
		StartedBy:  "api-service", // TODO: Get from context
		StartedAt:  execution.StartedAt,
		TenantID:   tenantID,
	}

	// Convert status
	switch execution.Status {
	case models.ExecutionStatusPending:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusPending
	case models.ExecutionStatusRunning:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusRunning
	case models.ExecutionStatusCompleted:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusCompleted
	case models.ExecutionStatusFaulted:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusFailed
	case models.ExecutionStatusCancelled:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusCancelled
	default:
		sharedExecution.Status = sharedInterfaces.WorkflowStatusPending
	}

	// Convert input/output
	if execution.Input != nil {
		sharedExecution.Input = execution.Input
	}
	if execution.Output != nil {
		sharedExecution.Output = execution.Output
	}

	// Set completed time if execution is completed
	if execution.CompletedAt != nil {
		sharedExecution.CompletedAt = execution.CompletedAt
	}

	return r.sharedDB.UpdateExecution(ctx, execution.ID, sharedExecution)
}

// List lists executions with optional filtering
func (r *SharedExecutionRepository) List(ctx context.Context, params *models.ExecutionListParams) ([]models.Execution, int, error) {
	// Convert API filters to shared interface filters
	// Use tenant ID from params instead of extracting from context
	sharedFilters := sharedInterfaces.ExecutionFilters{
		TenantID: params.TenantID,
		Limit:    params.Limit,
		Offset:   params.Offset,
	}

	// Convert status filter if provided
	if params.Status != "" {
		switch params.Status {
		case "pending":
			sharedFilters.Status = sharedInterfaces.WorkflowStatusPending
		case "running":
			sharedFilters.Status = sharedInterfaces.WorkflowStatusRunning
		case "completed":
			sharedFilters.Status = sharedInterfaces.WorkflowStatusCompleted
		case "failed":
			sharedFilters.Status = sharedInterfaces.WorkflowStatusFailed
		case "cancelled":
			sharedFilters.Status = sharedInterfaces.WorkflowStatusCancelled
		}
	}

	// Get executions from shared database
	sharedExecutions, err := r.sharedDB.ListExecutions(ctx, params.WorkflowID, sharedFilters)
	if err != nil {
		return nil, 0, err
	}

	// Convert from shared interface models to API models
	executions := make([]models.Execution, len(sharedExecutions))
	for i, sharedExecution := range sharedExecutions {
		execution := models.Execution{
			ID:         sharedExecution.ID,
			WorkflowID: sharedExecution.WorkflowID,
			StartedAt:  sharedExecution.StartedAt,
		}

		// Convert status
		switch sharedExecution.Status {
		case sharedInterfaces.WorkflowStatusPending:
			execution.Status = models.ExecutionStatusPending
		case sharedInterfaces.WorkflowStatusRunning:
			execution.Status = models.ExecutionStatusRunning
		case sharedInterfaces.WorkflowStatusCompleted:
			execution.Status = models.ExecutionStatusCompleted
		case sharedInterfaces.WorkflowStatusFailed:
			execution.Status = models.ExecutionStatusFaulted
		case sharedInterfaces.WorkflowStatusCancelled:
			execution.Status = models.ExecutionStatusCancelled
		default:
			execution.Status = models.ExecutionStatusPending
		}

		// Convert input/output
		if sharedExecution.Input != nil {
			execution.Input = sharedExecution.Input
		}
		if sharedExecution.Output != nil {
			execution.Output = sharedExecution.Output
		}

		// Set completed time if available
		if sharedExecution.CompletedAt != nil {
			execution.CompletedAt = sharedExecution.CompletedAt
		}

		executions[i] = execution
	}

	// For now, return the length as total count
	// TODO: Implement proper pagination in shared database interface
	total := len(executions)

	return executions, total, nil
}

// Delete deletes an execution
func (r *SharedExecutionRepository) Delete(ctx context.Context, id string) error {
	// The shared database interface doesn't have a delete execution method
	// This would need to be added to the interface if needed
	return fmt.Errorf("delete execution not implemented in shared database interface")
}

// extractTenantFromContext extracts tenant ID from context
// For now, returns a default tenant - in production this should extract from JWT/auth headers
func (r *SharedExecutionRepository) extractTenantFromContext(ctx context.Context) string {
	// TODO: Extract tenant from authentication context
	// For now, use a default tenant for testing
	return "default-tenant"
}
