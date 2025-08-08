package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	sharedInterfaces "github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// SharedDatabaseAdapter adapts the shared database interface to the worker's database interface
type SharedDatabaseAdapter struct {
	sharedDB sharedInterfaces.Database
	logger   interfaces.Logger
}

// NewSharedDatabaseAdapter creates a new adapter
func NewSharedDatabaseAdapter(sharedDB sharedInterfaces.Database, logger interfaces.Logger) interfaces.Database {
	return &SharedDatabaseAdapter{
		sharedDB: sharedDB,
		logger:   logger,
	}
}

// CreateWorkflow creates a new workflow record
func (a *SharedDatabaseAdapter) CreateWorkflow(ctx context.Context, workflow *gen.WorkflowDefinition) error {
	// Convert from worker's protobuf format to shared interface format
	sharedWorkflow := &sharedInterfaces.WorkflowDefinition{
		ID:          workflow.Id,
		Name:        workflow.Name,
		Description: workflow.Description,
		Version:     workflow.Version,
		SpecVersion: workflow.SpecVersion,
		TenantID:    "", // TODO: Extract tenant from context or add to protobuf
	}

	// Convert DSL definition from protobuf to map
	if workflow.DslDefinition != nil {
		definitionBytes, err := json.Marshal(workflow.DslDefinition)
		if err != nil {
			return fmt.Errorf("failed to marshal workflow DSL definition: %w", err)
		}

		var definitionMap map[string]interface{}
		if err := json.Unmarshal(definitionBytes, &definitionMap); err != nil {
			return fmt.Errorf("failed to unmarshal workflow DSL definition: %w", err)
		}
		sharedWorkflow.Definition = definitionMap
	}

	return a.sharedDB.CreateWorkflow(ctx, sharedWorkflow)
}

// UpdateWorkflowState updates the workflow state
// NOTE: This method is being phased out in favor of execution-based storage
func (a *SharedDatabaseAdapter) UpdateWorkflowState(ctx context.Context, workflowID string, state *gen.WorkflowState) error {
	a.logger.Warn("UpdateWorkflowState is deprecated - should use execution-based storage instead",
		"workflow_id", workflowID)

	// Extract execution ID and tenant ID from workflow state context
	executionID := ""
	tenantID := ""
	if state.Context != nil {
		executionID = state.Context.ExecutionId
		tenantID = state.Context.TenantId
	}

	// Fallback to generated ID if not provided (for backward compatibility)
	if executionID == "" {
		executionID = fmt.Sprintf("%s-exec-%d", workflowID, state.CreatedAt.AsTime().Unix())
		a.logger.Warn("No execution ID in context, generating fallback ID",
			"workflow_id", workflowID,
			"generated_id", executionID)
	}

	// For backward compatibility, convert state to execution format
	execution := &sharedInterfaces.WorkflowExecution{
		ID:         executionID,
		WorkflowID: workflowID,
		TenantID:   tenantID,         // Use tenant ID from workflow state
		StartedBy:  "worker-service", // TODO: Get from context
	}

	// Convert status
	switch state.Status {
	case gen.WorkflowStatus_WORKFLOW_STATUS_CREATED:
		execution.Status = sharedInterfaces.WorkflowStatusPending
	case gen.WorkflowStatus_WORKFLOW_STATUS_RUNNING:
		execution.Status = sharedInterfaces.WorkflowStatusRunning
	case gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED:
		execution.Status = sharedInterfaces.WorkflowStatusCompleted
	case gen.WorkflowStatus_WORKFLOW_STATUS_FAILED:
		execution.Status = sharedInterfaces.WorkflowStatusFailed
	case gen.WorkflowStatus_WORKFLOW_STATUS_CANCELLED:
		execution.Status = sharedInterfaces.WorkflowStatusCancelled
	default:
		execution.Status = sharedInterfaces.WorkflowStatusPending
	}

	// Convert timestamps
	if state.CreatedAt != nil {
		execution.StartedAt = state.CreatedAt.AsTime()
	}
	if state.UpdatedAt != nil {
		updatedAt := state.UpdatedAt.AsTime()
		execution.CompletedAt = &updatedAt
	}

	// Convert input/output
	if state.Input != nil {
		inputBytes, err := json.Marshal(state.Input)
		if err != nil {
			return fmt.Errorf("failed to marshal state input: %w", err)
		}
		var inputMap map[string]interface{}
		if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
			return fmt.Errorf("failed to unmarshal state input: %w", err)
		}
		execution.Input = inputMap
	}

	if state.Output != nil {
		outputBytes, err := json.Marshal(state.Output)
		if err != nil {
			return fmt.Errorf("failed to marshal state output: %w", err)
		}
		var outputMap map[string]interface{}
		if err := json.Unmarshal(outputBytes, &outputMap); err != nil {
			return fmt.Errorf("failed to unmarshal state output: %w", err)
		}
		execution.Output = outputMap
	}

	// Try to update existing execution, create if not found
	err := a.sharedDB.UpdateExecution(ctx, execution.ID, execution)
	if err != nil {
		// If execution doesn't exist, create it
		if err := a.sharedDB.CreateExecution(ctx, execution); err != nil {
			return fmt.Errorf("failed to create execution: %w", err)
		}
	}

	return nil
}

// GetWorkflowState retrieves the current workflow state
// NOTE: This method is being phased out in favor of execution-based storage
func (a *SharedDatabaseAdapter) GetWorkflowState(ctx context.Context, workflowID string) (*gen.WorkflowState, error) {
	a.logger.Warn("GetWorkflowState is deprecated - should use execution-based storage instead",
		"workflow_id", workflowID)

	// For backward compatibility, try to find the most recent execution for this workflow
	executions, err := a.sharedDB.ListExecutions(ctx, workflowID, sharedInterfaces.ExecutionFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	if len(executions) == 0 {
		return nil, fmt.Errorf("workflow state not found: %s", workflowID)
	}

	// Get the most recent execution (assuming they're sorted by start time)
	execution := executions[0]
	for _, exec := range executions {
		if exec.StartedAt.After(execution.StartedAt) {
			execution = exec
		}
	}

	// Convert back to worker's state format
	state := &gen.WorkflowState{
		WorkflowId: workflowID,
		// Note: protobuf doesn't have ExecutionId or TenantId fields
	}

	// Convert status
	switch execution.Status {
	case sharedInterfaces.WorkflowStatusPending:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CREATED
	case sharedInterfaces.WorkflowStatusRunning:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_RUNNING
	case sharedInterfaces.WorkflowStatusCompleted:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
	case sharedInterfaces.WorkflowStatusFailed:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_FAILED
	case sharedInterfaces.WorkflowStatusCancelled:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CANCELLED
	default:
		state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CREATED
	}

	// Convert timestamps
	// TODO: Convert timestamps properly using protobuf timestamp

	// Convert input/output
	// TODO: Convert input/output properly using protobuf structs

	return state, nil
}

// GetWorkflowDefinition retrieves the workflow definition
func (a *SharedDatabaseAdapter) GetWorkflowDefinition(ctx context.Context, workflowID string) (*gen.WorkflowDefinition, error) {
	// Try to get tenant ID from context (for now, fallback to scanning method)
	sharedWorkflow, err := a.sharedDB.GetWorkflowDefinition(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	return a.convertToWorkerWorkflowDefinition(sharedWorkflow), nil
}

// GetWorkflowDefinitionWithTenant retrieves the workflow definition with tenant context
func (a *SharedDatabaseAdapter) GetWorkflowDefinitionWithTenant(ctx context.Context, tenantID, workflowID string) (*gen.WorkflowDefinition, error) {
	a.logger.Info("Worker adapter getting workflow definition with tenant",
		"tenant_id", tenantID,
		"workflow_id", workflowID)

	sharedWorkflow, err := a.sharedDB.GetWorkflowDefinitionWithTenant(ctx, tenantID, workflowID)
	if err != nil {
		a.logger.Error("Failed to get workflow definition from shared DB",
			"tenant_id", tenantID,
			"workflow_id", workflowID,
			"error", err)
		return nil, err
	}

	a.logger.Info("Successfully got workflow definition from shared DB",
		"tenant_id", tenantID,
		"workflow_id", workflowID,
		"workflow_name", sharedWorkflow.Name)

	return a.convertToWorkerWorkflowDefinition(sharedWorkflow), nil
}

func (a *SharedDatabaseAdapter) convertToWorkerWorkflowDefinition(sharedWorkflow *sharedInterfaces.WorkflowDefinition) *gen.WorkflowDefinition {

	// Convert from shared interface format to worker's protobuf format
	workflow := &gen.WorkflowDefinition{
		Id:          sharedWorkflow.ID,
		Name:        sharedWorkflow.Name,
		Description: sharedWorkflow.Description,
		Version:     sharedWorkflow.Version,
		SpecVersion: sharedWorkflow.SpecVersion,
		// Note: protobuf doesn't have TenantId field
	}

	// Convert definition from map to protobuf
	if sharedWorkflow.Definition != nil {
		a.logger.Info("Converting DSL definition to protobuf",
			"workflow_id", sharedWorkflow.ID,
			"definition_keys", getMapKeys(sharedWorkflow.Definition))

		dslStruct, err := structpb.NewStruct(sharedWorkflow.Definition)
		if err != nil {
			a.logger.Error("Failed to convert DSL definition to protobuf struct",
				"workflow_id", sharedWorkflow.ID,
				"error", err)
		} else {
			workflow.DslDefinition = dslStruct
			a.logger.Info("Successfully converted DSL definition to protobuf",
				"workflow_id", sharedWorkflow.ID,
				"definition_keys", getMapKeys(sharedWorkflow.Definition))
		}
	} else {
		a.logger.Warn("DSL definition is nil - no conversion performed",
			"workflow_id", sharedWorkflow.ID)
	}

	return workflow
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// DeleteWorkflow deletes a workflow and its state
func (a *SharedDatabaseAdapter) DeleteWorkflow(ctx context.Context, workflowID string) error {
	return a.sharedDB.DeleteWorkflow(ctx, workflowID)
}

// ListWorkflows lists all workflows with optional filtering
func (a *SharedDatabaseAdapter) ListWorkflows(ctx context.Context, filters map[string]string) ([]*gen.WorkflowState, error) {
	// Convert filters to shared interface format
	sharedFilters := sharedInterfaces.WorkflowFilters{}
	if tenantID, ok := filters["tenant_id"]; ok {
		sharedFilters.TenantID = tenantID
	}
	if name, ok := filters["name"]; ok {
		sharedFilters.Name = name
	}

	workflows, err := a.sharedDB.ListWorkflows(ctx, sharedFilters)
	if err != nil {
		return nil, err
	}

	// Convert to worker's state format
	var states []*gen.WorkflowState
	for _, workflow := range workflows {
		// For each workflow, get its most recent execution to create a "state"
		executions, err := a.sharedDB.ListExecutions(ctx, workflow.ID, sharedInterfaces.ExecutionFilters{
			TenantID: workflow.TenantID,
		})
		if err != nil {
			a.logger.Warn("Failed to get executions for workflow", "workflow_id", workflow.ID, "error", err)
			continue
		}

		if len(executions) == 0 {
			// No executions yet, create a placeholder state
			states = append(states, &gen.WorkflowState{
				WorkflowId: workflow.ID,
				Status:     gen.WorkflowStatus_WORKFLOW_STATUS_CREATED,
			})
			continue
		}

		// Get the most recent execution
		execution := executions[0]
		for _, exec := range executions {
			if exec.StartedAt.After(execution.StartedAt) {
				execution = exec
			}
		}

		// Convert to state format
		state := &gen.WorkflowState{
			WorkflowId: workflow.ID,
		}

		// Convert status
		switch execution.Status {
		case sharedInterfaces.WorkflowStatusPending:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CREATED
		case sharedInterfaces.WorkflowStatusRunning:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_RUNNING
		case sharedInterfaces.WorkflowStatusCompleted:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
		case sharedInterfaces.WorkflowStatusFailed:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_FAILED
		case sharedInterfaces.WorkflowStatusCancelled:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CANCELLED
		default:
			state.Status = gen.WorkflowStatus_WORKFLOW_STATUS_CREATED
		}

		states = append(states, state)
	}

	return states, nil
}
