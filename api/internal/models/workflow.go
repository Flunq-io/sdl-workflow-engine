package models

import (
	"encoding/json"
	"time"
)

// WorkflowState represents the state of a workflow definition
type WorkflowState string

const (
	WorkflowStateActive   WorkflowState = "active"   // The workflow is active and can be executed
	WorkflowStateInactive WorkflowState = "inactive" // The workflow is inactive and cannot be executed
)

// Workflow represents a workflow definition
type Workflow struct {
	ID          string                 `json:"id" db:"id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description" db:"description"`
	TenantID    string                 `json:"tenant_id" db:"tenant_id"`
	Definition  map[string]interface{} `json:"definition" db:"definition"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty" db:"input_schema"`
	State       WorkflowState          `json:"state" db:"state"`
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
	TenantID    string                 `json:"tenant_id" binding:"required"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	Tags        []string               `json:"tags"`
}

// UpdateWorkflowRequest represents the request to update a workflow
type UpdateWorkflowRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Definition  map[string]interface{} `json:"definition,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Total       int  `json:"total"`
	Limit       int  `json:"limit"`
	Offset      int  `json:"offset"`
	Page        int  `json:"page"`
	Size        int  `json:"size"`
	TotalPages  int  `json:"total_pages"`
	HasNext     bool `json:"has_next"`
	HasPrevious bool `json:"has_previous"`
}

// FilterMeta represents applied filters metadata
type FilterMeta struct {
	Applied map[string]interface{} `json:"applied"`
	Count   int                    `json:"count"` // Number of items after filtering but before pagination
}

// WorkflowListResponse represents the response for listing workflows
type WorkflowListResponse struct {
	Items      []Workflow     `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
	Filters    FilterMeta     `json:"filters"`
	Sort       *SortMeta      `json:"sort,omitempty"`
}

// SortMeta represents sorting metadata
type SortMeta struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// NewPaginationMeta creates pagination metadata from parameters and totals
func NewPaginationMeta(total, filteredCount int, params PaginationParams) PaginationMeta {
	// Handle both limit/offset and page/size patterns
	var limit, offset, page, size int

	if params.Page > 0 && params.Size > 0 {
		// Page-based pagination
		page = params.Page
		size = params.Size
		limit = size
		offset = (page - 1) * size
	} else {
		// Offset-based pagination
		limit = params.Limit
		if limit == 0 {
			limit = 20 // default
		}
		offset = params.Offset
		page = (offset / limit) + 1
		size = limit
	}

	totalPages := (filteredCount + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	return PaginationMeta{
		Total:       total,
		Limit:       limit,
		Offset:      offset,
		Page:        page,
		Size:        size,
		TotalPages:  totalPages,
		HasNext:     offset+limit < filteredCount,
		HasPrevious: offset > 0,
	}
}

// NewFilterMeta creates filter metadata from applied filters and count
func NewFilterMeta(appliedFilters map[string]interface{}, count int) FilterMeta {
	if appliedFilters == nil {
		appliedFilters = make(map[string]interface{})
	}
	return FilterMeta{
		Applied: appliedFilters,
		Count:   count,
	}
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
		TenantID:    w.TenantID,
		State:       w.State,
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
