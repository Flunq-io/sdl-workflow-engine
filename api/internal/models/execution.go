package models

import (
	"encoding/json"
	"time"
)

// ExecutionStatus represents the status of a workflow execution (SDL compliant)
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"   // The execution has been initiated and is pending execution
	ExecutionStatusRunning   ExecutionStatus = "running"   // The execution is currently in progress
	ExecutionStatusWaiting   ExecutionStatus = "waiting"   // The execution is temporarily paused, awaiting events or time
	ExecutionStatusSuspended ExecutionStatus = "suspended" // The execution has been manually paused by a user
	ExecutionStatusCancelled ExecutionStatus = "cancelled" // The execution has been terminated before completion
	ExecutionStatusFaulted   ExecutionStatus = "faulted"   // The execution has encountered an error
	ExecutionStatusCompleted ExecutionStatus = "completed" // The execution ran to completion
)

// Execution represents a workflow execution instance
type Execution struct {
	ID            string                 `json:"id" db:"id"`
	WorkflowID    string                 `json:"workflow_id" db:"workflow_id"`
	TenantID      string                 `json:"tenant_id" db:"tenant_id"`
	Status        ExecutionStatus        `json:"status" db:"status"`
	CorrelationID string                 `json:"correlation_id,omitempty" db:"correlation_id"`
	Input         map[string]interface{} `json:"input,omitempty" db:"input"`
	Output        map[string]interface{} `json:"output,omitempty" db:"output"`
	Error         *ExecutionError        `json:"error,omitempty" db:"error"`
	CurrentState  string                 `json:"current_state,omitempty" db:"current_state"`
	StartedAt     time.Time              `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	DurationMs    *int64                 `json:"duration_ms,omitempty" db:"duration_ms"`
}

// ExecutionError represents an error that occurred during execution
type ExecutionError struct {
	Message string                 `json:"message"`
	Code    string                 `json:"code"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ExecuteWorkflowRequest represents the request to execute a workflow
type ExecuteWorkflowRequest struct {
	Input         map[string]interface{} `json:"input"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	TenantID      string                 // Will be extracted from path parameter
}

// ExecutionResponse represents the response when starting an execution
type ExecutionResponse struct {
	ID            string          `json:"id"`
	WorkflowID    string          `json:"workflow_id"`
	Status        ExecutionStatus `json:"status"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
}

// ExecutionListResponse represents the response for listing executions
type ExecutionListResponse struct {
	Items  []ExecutionResponse `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// CancelExecutionRequest represents the request to cancel an execution
type CancelExecutionRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ToResponse converts an Execution to ExecutionResponse
// Applies a conservative fallback for zero StartedAt only for terminal states
func (e *Execution) ToResponse() ExecutionResponse {
	started := e.StartedAt
	if started.IsZero() {
		// Only fallback in terminal states to avoid masking in-progress bugs
		if (e.Status == ExecutionStatusCompleted || e.Status == ExecutionStatusCancelled || e.Status == ExecutionStatusFaulted) && e.CompletedAt != nil {
			started = *e.CompletedAt
		}
	}
	return ExecutionResponse{
		ID:            e.ID,
		WorkflowID:    e.WorkflowID,
		Status:        e.Status,
		CorrelationID: e.CorrelationID,
		StartedAt:     started,
		CompletedAt:   e.CompletedAt,
	}
}

// SetError sets an error on the execution
func (e *Execution) SetError(code, message string, details map[string]interface{}) {
	e.Error = &ExecutionError{
		Code:    code,
		Message: message,
		Details: details,
	}
	e.Status = ExecutionStatusFaulted
}

// Complete marks the execution as completed
func (e *Execution) Complete(output map[string]interface{}) {
	now := time.Now()
	e.Status = ExecutionStatusCompleted
	e.Output = output
	e.CompletedAt = &now

	// Calculate duration
	duration := now.Sub(e.StartedAt).Milliseconds()
	e.DurationMs = &duration
}

// Cancel marks the execution as cancelled
func (e *Execution) Cancel(reason string) {
	now := time.Now()
	e.Status = ExecutionStatusCancelled
	e.CompletedAt = &now

	// Set cancellation reason in error
	e.SetError("EXECUTION_CANCELLED", "Execution was cancelled", map[string]interface{}{
		"reason": reason,
	})

	// Calculate duration
	duration := now.Sub(e.StartedAt).Milliseconds()
	e.DurationMs = &duration
}

// Fail marks the execution as failed
func (e *Execution) Fail(code, message string, details map[string]interface{}) {
	now := time.Now()
	e.Status = ExecutionStatusFaulted
	e.CompletedAt = &now
	e.SetError(code, message, details)

	// Calculate duration
	duration := now.Sub(e.StartedAt).Milliseconds()
	e.DurationMs = &duration
}

// IsRunning returns true if the execution is currently running
func (e *Execution) IsRunning() bool {
	return e.Status == ExecutionStatusPending || e.Status == ExecutionStatusRunning
}

// IsCompleted returns true if the execution has finished (completed, failed, or cancelled)
func (e *Execution) IsCompleted() bool {
	return e.Status == ExecutionStatusCompleted ||
		e.Status == ExecutionStatusFaulted ||
		e.Status == ExecutionStatusCancelled
}

// GetInputField safely gets a field from the execution input
func (e *Execution) GetInputField(field string) (interface{}, bool) {
	if e.Input == nil {
		return nil, false
	}
	value, exists := e.Input[field]
	return value, exists
}

// SetInputField safely sets a field in the execution input
func (e *Execution) SetInputField(field string, value interface{}) {
	if e.Input == nil {
		e.Input = make(map[string]interface{})
	}
	e.Input[field] = value
}

// GetOutputField safely gets a field from the execution output
func (e *Execution) GetOutputField(field string) (interface{}, bool) {
	if e.Output == nil {
		return nil, false
	}
	value, exists := e.Output[field]
	return value, exists
}

// SetOutputField safely sets a field in the execution output
func (e *Execution) SetOutputField(field string, value interface{}) {
	if e.Output == nil {
		e.Output = make(map[string]interface{})
	}
	e.Output[field] = value
}

// ToJSON converts the execution input to JSON string
func (e *Execution) InputToJSON() (string, error) {
	if e.Input == nil {
		return "{}", nil
	}
	data, err := json.Marshal(e.Input)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON converts the execution output to JSON string
func (e *Execution) OutputToJSON() (string, error) {
	if e.Output == nil {
		return "{}", nil
	}
	data, err := json.Marshal(e.Output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromInputJSON parses JSON string into execution input
func (e *Execution) FromInputJSON(jsonStr string) error {
	if jsonStr == "" {
		e.Input = make(map[string]interface{})
		return nil
	}
	return json.Unmarshal([]byte(jsonStr), &e.Input)
}

// FromOutputJSON parses JSON string into execution output
func (e *Execution) FromOutputJSON(jsonStr string) error {
	if jsonStr == "" {
		e.Output = make(map[string]interface{})
		return nil
	}
	return json.Unmarshal([]byte(jsonStr), &e.Output)
}

// Clone creates a deep copy of the execution
func (e *Execution) Clone() *Execution {
	clone := &Execution{
		ID:            e.ID,
		WorkflowID:    e.WorkflowID,
		Status:        e.Status,
		CorrelationID: e.CorrelationID,
		CurrentState:  e.CurrentState,
		StartedAt:     e.StartedAt,
		DurationMs:    e.DurationMs,
	}

	// Copy completed time
	if e.CompletedAt != nil {
		completedAt := *e.CompletedAt
		clone.CompletedAt = &completedAt
	}

	// Deep copy input
	if e.Input != nil {
		inputJSON, _ := json.Marshal(e.Input)
		json.Unmarshal(inputJSON, &clone.Input)
	}

	// Deep copy output
	if e.Output != nil {
		outputJSON, _ := json.Marshal(e.Output)
		json.Unmarshal(outputJSON, &clone.Output)
	}

	// Copy error
	if e.Error != nil {
		clone.Error = &ExecutionError{
			Message: e.Error.Message,
			Code:    e.Error.Code,
		}
		if e.Error.Details != nil {
			detailsJSON, _ := json.Marshal(e.Error.Details)
			json.Unmarshal(detailsJSON, &clone.Error.Details)
		}
	}

	return clone
}
