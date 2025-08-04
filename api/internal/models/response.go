package models

import (
	"time"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error     string                 `json:"error"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status       string            `json:"status"`
	Timestamp    time.Time         `json:"timestamp"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

// EventHistoryResponse represents the response for workflow event history
type EventHistoryResponse struct {
	WorkflowID string       `json:"workflow_id"`
	Events     []CloudEvent `json:"events"`
	Total      int          `json:"total"`
	Since      string       `json:"since,omitempty"`
}

// CloudEvent represents a CloudEvents v1.0 specification event
type CloudEvent struct {
	ID              string                 `json:"id"`
	Source          string                 `json:"source"`
	SpecVersion     string                 `json:"specversion"`
	Type            string                 `json:"type"`
	DataContentType string                 `json:"datacontenttype,omitempty"`
	Subject         string                 `json:"subject,omitempty"`
	Time            time.Time              `json:"time,omitempty"`
	Data            map[string]interface{} `json:"data,omitempty"`
	WorkflowID      string                 `json:"workflowid,omitempty"`
	ExecutionID     string                 `json:"executionid,omitempty"`
}

// PaginationParams represents common pagination parameters
type PaginationParams struct {
	Limit  int `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset int `form:"offset" binding:"omitempty,min=0"`
}

// WorkflowListParams represents parameters for listing workflows
type WorkflowListParams struct {
	PaginationParams
	Status string `form:"status" binding:"omitempty,oneof=created active inactive"`
}

// ExecutionListParams represents parameters for listing executions
type ExecutionListParams struct {
	PaginationParams
	WorkflowID string `form:"workflow_id"`
	Status     string `form:"status" binding:"omitempty,oneof=pending running completed failed cancelled"`
}

// EventHistoryParams represents parameters for getting event history
type EventHistoryParams struct {
	Since string `form:"since"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=1000"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(code, message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   code,
		Message: message,
	}
}

// WithDetails adds details to an error response
func (e *ErrorResponse) WithDetails(details map[string]interface{}) *ErrorResponse {
	e.Details = details
	return e
}

// WithRequestID adds a request ID to an error response
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

// NewHealthResponse creates a new health response
func NewHealthResponse(status string, version string, dependencies map[string]string) *HealthResponse {
	return &HealthResponse{
		Status:       status,
		Timestamp:    time.Now(),
		Version:      version,
		Dependencies: dependencies,
	}
}

// Common error codes
const (
	ErrorCodeValidation        = "VALIDATION_ERROR"
	ErrorCodeNotFound          = "NOT_FOUND"
	ErrorCodeWorkflowNotFound  = "WORKFLOW_NOT_FOUND"
	ErrorCodeExecutionNotFound = "EXECUTION_NOT_FOUND"
	ErrorCodeInternalError     = "INTERNAL_ERROR"
	ErrorCodeDatabaseError     = "DATABASE_ERROR"
	ErrorCodeEventStoreError   = "EVENT_STORE_ERROR"
	ErrorCodeInvalidStatus     = "INVALID_STATUS"
	ErrorCodeExecutionRunning  = "EXECUTION_RUNNING"
	ErrorCodeWorkflowInactive  = "WORKFLOW_INACTIVE"
)

// Common error messages
const (
	MessageValidationFailed  = "The request contains invalid data"
	MessageWorkflowNotFound  = "The specified workflow does not exist"
	MessageExecutionNotFound = "The specified execution does not exist"
	MessageInternalError     = "An unexpected error occurred"
	MessageDatabaseError     = "Database operation failed"
	MessageEventStoreError   = "Event store operation failed"
	MessageInvalidStatus     = "Invalid status transition"
	MessageExecutionRunning  = "Cannot modify running execution"
	MessageWorkflowInactive  = "Workflow is not active"
)
