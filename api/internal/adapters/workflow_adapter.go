package adapters

import (
	"context"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// WorkflowAdapter adapts between API models and shared interfaces
type WorkflowAdapter struct {
	database interfaces.Database
}

// NewWorkflowAdapter creates a new workflow adapter
func NewWorkflowAdapter(database interfaces.Database) *WorkflowAdapter {
	return &WorkflowAdapter{
		database: database,
	}
}

// Create creates a workflow using the shared database interface
func (a *WorkflowAdapter) Create(ctx context.Context, workflow *models.Workflow) error {
	sharedWorkflow := a.toSharedWorkflowDefinition(workflow)
	return a.database.CreateWorkflow(ctx, sharedWorkflow)
}

// GetByID retrieves a workflow by ID and converts to API model
func (a *WorkflowAdapter) GetByID(ctx context.Context, id string) (*models.WorkflowDetail, error) {
	// For backward compatibility, use the non-tenant method
	sharedWorkflow, err := a.database.GetWorkflowDefinition(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.toAPIWorkflowDetail(sharedWorkflow), nil
}

// GetByIDWithTenant retrieves a workflow by ID with tenant context and converts to API model
func (a *WorkflowAdapter) GetByIDWithTenant(ctx context.Context, tenantID, id string) (*models.WorkflowDetail, error) {
	sharedWorkflow, err := a.database.GetWorkflowDefinitionWithTenant(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	return a.toAPIWorkflowDetail(sharedWorkflow), nil
}

// Update updates a workflow using the shared database interface
func (a *WorkflowAdapter) Update(ctx context.Context, workflow *models.Workflow) error {
	sharedWorkflow := a.toSharedWorkflowDefinition(workflow)
	return a.database.UpdateWorkflowDefinition(ctx, workflow.ID, sharedWorkflow)
}

// Delete deletes a workflow using the shared database interface
func (a *WorkflowAdapter) Delete(ctx context.Context, id string) error {
	return a.database.DeleteWorkflow(ctx, id)
}

// List lists workflows with filters and converts to API models
func (a *WorkflowAdapter) List(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	filters := a.toSharedWorkflowFilters(params)

	sharedWorkflows, err := a.database.ListWorkflows(ctx, filters)
	if err != nil {
		return nil, 0, err
	}

	apiWorkflows := make([]models.Workflow, len(sharedWorkflows))
	for i, sharedWorkflow := range sharedWorkflows {
		apiWorkflows[i] = *a.toAPIWorkflow(sharedWorkflow)
	}

	return apiWorkflows, len(apiWorkflows), nil
}

// Conversion methods

// toSharedWorkflowDefinition converts API Workflow to shared WorkflowDefinition
func (a *WorkflowAdapter) toSharedWorkflowDefinition(workflow *models.Workflow) *interfaces.WorkflowDefinition {
	return &interfaces.WorkflowDefinition{
		ID:          workflow.ID,
		Name:        workflow.Name,
		Description: workflow.Description,
		Version:     "1.0.0", // Default version if not specified
		SpecVersion: "1.0.0", // Default spec version
		Definition:  workflow.Definition,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
		CreatedBy:   "api", // TODO: Get from authentication context
		TenantID:    workflow.TenantID,
	}
}

// toAPIWorkflow converts shared WorkflowDefinition to API Workflow
func (a *WorkflowAdapter) toAPIWorkflow(sharedWorkflow *interfaces.WorkflowDefinition) *models.Workflow {
	return &models.Workflow{
		ID:          sharedWorkflow.ID,
		Name:        sharedWorkflow.Name,
		Description: sharedWorkflow.Description,
		TenantID:    sharedWorkflow.TenantID,
		Definition:  sharedWorkflow.Definition,
		State:       models.WorkflowStateActive, // Default state
		Tags:        []string{},                 // TODO: Add tags support to shared interface
		CreatedAt:   sharedWorkflow.CreatedAt,
		UpdatedAt:   sharedWorkflow.UpdatedAt,
	}
}

// toAPIWorkflowDetail converts shared WorkflowDefinition to API WorkflowDetail
func (a *WorkflowAdapter) toAPIWorkflowDetail(sharedWorkflow *interfaces.WorkflowDefinition) *models.WorkflowDetail {
	workflow := a.toAPIWorkflow(sharedWorkflow)

	return &models.WorkflowDetail{
		Workflow:       *workflow,
		ExecutionCount: 0,   // TODO: Implement execution counting
		LastExecution:  nil, // TODO: Implement last execution tracking
		Input:          nil, // TODO: Implement input tracking
		Output:         nil, // TODO: Implement output tracking
		TaskExecutions: nil, // TODO: Implement task execution tracking
	}
}

// toSharedWorkflowFilters converts API WorkflowListParams to shared WorkflowFilters
func (a *WorkflowAdapter) toSharedWorkflowFilters(params *models.WorkflowListParams) interfaces.WorkflowFilters {
	filters := interfaces.WorkflowFilters{
		Limit:  params.Limit,
		Offset: params.Offset,
	}

	// TODO: Add tenant and name filters when available in WorkflowListParams
	// The current WorkflowListParams only has Status field

	// Add status filter if specified
	if params.Status != "" {
		// Note: The shared interface doesn't have status filters yet
		// This would need to be added to the shared WorkflowFilters
	}

	return filters
}

// Health checks the database health
func (a *WorkflowAdapter) Health(ctx context.Context) error {
	return a.database.Health(ctx)
}

// GetStats returns database statistics
func (a *WorkflowAdapter) GetStats(ctx context.Context) (*interfaces.DatabaseStats, error) {
	return a.database.GetStats(ctx)
}
