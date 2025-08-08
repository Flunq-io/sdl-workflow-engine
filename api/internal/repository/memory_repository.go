package repository

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/flunq-io/api/internal/models"
)

// MemoryWorkflowRepository is an in-memory implementation of WorkflowRepository
type MemoryWorkflowRepository struct {
	workflows map[string]*models.Workflow
	mutex     sync.RWMutex
}

// NewMemoryWorkflowRepository creates a new in-memory workflow repository
func NewMemoryWorkflowRepository() *MemoryWorkflowRepository {
	return &MemoryWorkflowRepository{
		workflows: make(map[string]*models.Workflow),
	}
}

// Create creates a new workflow
func (r *MemoryWorkflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.workflows[workflow.ID]; exists {
		return errors.New("workflow already exists")
	}

	r.workflows[workflow.ID] = workflow.Clone()
	return nil
}

// GetByID retrieves a workflow by ID
func (r *MemoryWorkflowRepository) GetByID(ctx context.Context, id string) (*models.WorkflowDetail, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	workflow, exists := r.workflows[id]
	if !exists {
		return nil, errors.New("not found")
	}

	// Convert to WorkflowDetail (for now, just copy the workflow data)
	detail := &models.WorkflowDetail{
		Workflow:       *workflow.Clone(),
		ExecutionCount: 0,   // TODO: Count executions
		LastExecution:  nil, // TODO: Get last execution time
	}

	return detail, nil
}

// Update updates an existing workflow
func (r *MemoryWorkflowRepository) Update(ctx context.Context, workflow *models.Workflow) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.workflows[workflow.ID]; !exists {
		return errors.New("not found")
	}

	r.workflows[workflow.ID] = workflow.Clone()
	return nil
}

// Delete deletes a workflow
func (r *MemoryWorkflowRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.workflows[id]; !exists {
		return errors.New("not found")
	}

	delete(r.workflows, id)
	return nil
}

// List lists workflows with optional filtering
func (r *MemoryWorkflowRepository) List(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var workflows []models.Workflow
	for _, workflow := range r.workflows {
		// Apply status filter if specified
		if params.Status != "" && string(workflow.Status) != params.Status {
			continue
		}

		workflows = append(workflows, *workflow.Clone())
	}

	// Sort by created date (newest first)
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].CreatedAt.After(workflows[j].CreatedAt)
	})

	total := len(workflows)

	// Apply pagination
	start := params.Offset
	if start > len(workflows) {
		start = len(workflows)
	}

	end := start + params.Limit
	if end > len(workflows) {
		end = len(workflows)
	}

	if start >= len(workflows) {
		return []models.Workflow{}, total, nil
	}

	return workflows[start:end], total, nil
}

// MemoryExecutionRepository is an in-memory implementation of ExecutionRepository
type MemoryExecutionRepository struct {
	executions map[string]*models.Execution
	mutex      sync.RWMutex
}

// NewMemoryExecutionRepository creates a new in-memory execution repository
func NewMemoryExecutionRepository() *MemoryExecutionRepository {
	return &MemoryExecutionRepository{
		executions: make(map[string]*models.Execution),
	}
}

// Create creates a new execution
func (r *MemoryExecutionRepository) Create(ctx context.Context, execution *models.Execution) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.executions[execution.ID]; exists {
		return errors.New("execution already exists")
	}

	r.executions[execution.ID] = execution.Clone()
	return nil
}

// GetByID retrieves an execution by ID
func (r *MemoryExecutionRepository) GetByID(ctx context.Context, id string) (*models.Execution, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	execution, exists := r.executions[id]
	if !exists {
		return nil, errors.New("not found")
	}

	return execution.Clone(), nil
}

// Update updates an existing execution
func (r *MemoryExecutionRepository) Update(ctx context.Context, execution *models.Execution) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.executions[execution.ID]; !exists {
		return errors.New("not found")
	}

	r.executions[execution.ID] = execution.Clone()
	return nil
}

// List lists executions with optional filtering
func (r *MemoryExecutionRepository) List(ctx context.Context, params *models.ExecutionListParams) ([]models.Execution, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var executions []models.Execution
	for _, execution := range r.executions {
		// Apply tenant ID filter if specified
		if params.TenantID != "" && execution.TenantID != params.TenantID {
			continue
		}

		// Apply workflow ID filter if specified
		if params.WorkflowID != "" && execution.WorkflowID != params.WorkflowID {
			continue
		}

		// Apply status filter if specified
		if params.Status != "" && string(execution.Status) != params.Status {
			continue
		}

		executions = append(executions, *execution.Clone())
	}

	// Sort by started date (newest first)
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].StartedAt.After(executions[j].StartedAt)
	})

	total := len(executions)

	// Apply pagination
	start := params.Offset
	if start > len(executions) {
		start = len(executions)
	}

	end := start + params.Limit
	if end > len(executions) {
		end = len(executions)
	}

	if start >= len(executions) {
		return []models.Execution{}, total, nil
	}

	return executions[start:end], total, nil
}
