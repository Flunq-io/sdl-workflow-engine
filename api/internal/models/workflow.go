package models

import (
	"encoding/json"
	"time"
)

// WorkflowStatus represents the status of a workflow (SDL compliant)
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"   // The workflow has been initiated and is pending execution
	WorkflowStatusRunning   WorkflowStatus = "running"   // The workflow is currently in progress
	WorkflowStatusWaiting   WorkflowStatus = "waiting"   // The workflow execution is temporarily paused, awaiting events or time
	WorkflowStatusSuspended WorkflowStatus = "suspended" // The workflow execution has been manually paused by a user
	WorkflowStatusCancelled WorkflowStatus = "cancelled" // The workflow execution has been terminated before completion
	WorkflowStatusFaulted   WorkflowStatus = "faulted"   // The workflow execution has encountered an error
	WorkflowStatusCompleted WorkflowStatus = "completed" // The workflow ran to completion
)

// Workflow represents a workflow definition
type Workflow struct {
	ID          string                 `json:"id" db:"id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description" db:"description"`
	Definition  map[string]interface{} `json:"definition" db:"definition"`
	Status      WorkflowStatus         `json:"status" db:"status"`
	Tags        []string               `json:"tags" db:"tags"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// WorkflowDetail extends Workflow with additional execution information
type WorkflowDetail struct {
	Workflow
	ExecutionCount int                    `json:"execution_count" db:"execution_count"`
	LastExecution  *time.Time             `json:"last_execution,omitempty" db:"last_execution"`
	Input          map[string]interface{} `json:"input,omitempty"`
	Output         map[string]interface{} `json:"output,omitempty"`
	TaskExecutions []TaskExecution        `json:"task_executions,omitempty"`
}

// TaskExecution represents a single task execution with input/output
type TaskExecution struct {
	Name         string                 `json:"name"`
	TaskType     string                 `json:"task_type"`
	Input        map[string]interface{} `json:"input"`
	Output       map[string]interface{} `json:"output"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	DurationMs   int64                  `json:"duration_ms"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// CreateWorkflowRequest represents the request to create a new workflow
type CreateWorkflowRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Tags        []string               `json:"tags"`
}

// UpdateWorkflowRequest represents the request to update a workflow
type UpdateWorkflowRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Definition  map[string]interface{} `json:"definition,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// WorkflowListResponse represents the response for listing workflows
type WorkflowListResponse struct {
	Items  []Workflow `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// Validate validates the workflow definition
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}

	if w.Definition == nil {
		return &ValidationError{Field: "definition", Message: "definition is required"}
	}

	// Basic validation of workflow definition structure
	if id, ok := w.Definition["id"]; !ok || id == "" {
		return &ValidationError{Field: "definition.id", Message: "workflow definition must have an id"}
	}

	if specVersion, ok := w.Definition["specVersion"]; !ok || specVersion == "" {
		return &ValidationError{Field: "definition.specVersion", Message: "workflow definition must have a specVersion"}
	}

	return nil
}

// ToJSON converts the workflow definition to JSON string
func (w *Workflow) ToJSON() (string, error) {
	data, err := json.Marshal(w.Definition)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into workflow definition
func (w *Workflow) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), &w.Definition)
}

// GetDefinitionField safely gets a field from the workflow definition
func (w *Workflow) GetDefinitionField(field string) (interface{}, bool) {
	if w.Definition == nil {
		return nil, false
	}
	value, exists := w.Definition[field]
	return value, exists
}

// SetDefinitionField safely sets a field in the workflow definition
func (w *Workflow) SetDefinitionField(field string, value interface{}) {
	if w.Definition == nil {
		w.Definition = make(map[string]interface{})
	}
	w.Definition[field] = value
}

// GetWorkflowID returns the workflow ID from the definition
func (w *Workflow) GetWorkflowID() string {
	if id, exists := w.GetDefinitionField("id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}

// GetSpecVersion returns the spec version from the definition
func (w *Workflow) GetSpecVersion() string {
	if version, exists := w.GetDefinitionField("specVersion"); exists {
		if versionStr, ok := version.(string); ok {
			return versionStr
		}
	}
	return ""
}

// GetStartState returns the start state from the definition
func (w *Workflow) GetStartState() string {
	if start, exists := w.GetDefinitionField("start"); exists {
		if startStr, ok := start.(string); ok {
			return startStr
		}
	}
	return ""
}

// GetStates returns the states from the definition
func (w *Workflow) GetStates() []interface{} {
	if states, exists := w.GetDefinitionField("states"); exists {
		if statesSlice, ok := states.([]interface{}); ok {
			return statesSlice
		}
	}
	return nil
}

// GetFunctions returns the functions from the definition
func (w *Workflow) GetFunctions() []interface{} {
	if functions, exists := w.GetDefinitionField("functions"); exists {
		if functionsSlice, ok := functions.([]interface{}); ok {
			return functionsSlice
		}
	}
	return nil
}

// GetEvents returns the events from the definition
func (w *Workflow) GetEvents() []interface{} {
	if events, exists := w.GetDefinitionField("events"); exists {
		if eventsSlice, ok := events.([]interface{}); ok {
			return eventsSlice
		}
	}
	return nil
}

// Clone creates a deep copy of the workflow
func (w *Workflow) Clone() *Workflow {
	clone := &Workflow{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Status:      w.Status,
		Tags:        make([]string, len(w.Tags)),
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}

	// Copy tags
	copy(clone.Tags, w.Tags)

	// Deep copy definition
	if w.Definition != nil {
		definitionJSON, _ := json.Marshal(w.Definition)
		json.Unmarshal(definitionJSON, &clone.Definition)
	}

	return clone
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
