package errors

import (
	"errors"
	"fmt"
)

// Common error types for flunq.io services
var (
	// Workflow errors
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowAlreadyExists = errors.New("workflow already exists")
	ErrWorkflowInvalidState  = errors.New("workflow in invalid state")
	ErrWorkflowValidation    = errors.New("workflow validation failed")

	// Task errors
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskExecutionFailed = errors.New("task execution failed")
	ErrTaskTimeout        = errors.New("task execution timeout")
	ErrTaskInvalidType    = errors.New("invalid task type")

	// Event errors
	ErrEventNotFound      = errors.New("event not found")
	ErrEventInvalidFormat = errors.New("invalid event format")
	ErrEventPublishFailed = errors.New("event publish failed")
	ErrEventStoreFailed   = errors.New("event store failed")

	// Storage errors
	ErrStorageConnection = errors.New("storage connection failed")
	ErrStorageTimeout    = errors.New("storage operation timeout")
	ErrStorageNotFound   = errors.New("storage record not found")

	// Streaming errors
	ErrStreamConnection = errors.New("stream connection failed")
	ErrStreamTimeout    = errors.New("stream operation timeout")
	ErrStreamNotFound   = errors.New("stream not found")

	// Authentication/Authorization errors
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidToken = errors.New("invalid token")

	// Validation errors
	ErrInvalidInput    = errors.New("invalid input")
	ErrMissingRequired = errors.New("missing required field")
	ErrInvalidFormat   = errors.New("invalid format")

	// Configuration errors
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrMissingConfig = errors.New("missing configuration")
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Workflow error codes
	CodeWorkflowNotFound      ErrorCode = "WORKFLOW_NOT_FOUND"
	CodeWorkflowAlreadyExists ErrorCode = "WORKFLOW_ALREADY_EXISTS"
	CodeWorkflowInvalidState  ErrorCode = "WORKFLOW_INVALID_STATE"
	CodeWorkflowValidation    ErrorCode = "WORKFLOW_VALIDATION_FAILED"

	// Task error codes
	CodeTaskNotFound       ErrorCode = "TASK_NOT_FOUND"
	CodeTaskExecutionFailed ErrorCode = "TASK_EXECUTION_FAILED"
	CodeTaskTimeout        ErrorCode = "TASK_TIMEOUT"
	CodeTaskInvalidType    ErrorCode = "TASK_INVALID_TYPE"

	// Event error codes
	CodeEventNotFound      ErrorCode = "EVENT_NOT_FOUND"
	CodeEventInvalidFormat ErrorCode = "EVENT_INVALID_FORMAT"
	CodeEventPublishFailed ErrorCode = "EVENT_PUBLISH_FAILED"
	CodeEventStoreFailed   ErrorCode = "EVENT_STORE_FAILED"

	// Storage error codes
	CodeStorageConnection ErrorCode = "STORAGE_CONNECTION_FAILED"
	CodeStorageTimeout    ErrorCode = "STORAGE_TIMEOUT"
	CodeStorageNotFound   ErrorCode = "STORAGE_NOT_FOUND"

	// Streaming error codes
	CodeStreamConnection ErrorCode = "STREAM_CONNECTION_FAILED"
	CodeStreamTimeout    ErrorCode = "STREAM_TIMEOUT"
	CodeStreamNotFound   ErrorCode = "STREAM_NOT_FOUND"

	// Authentication/Authorization error codes
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
	CodeInvalidToken ErrorCode = "INVALID_TOKEN"

	// Validation error codes
	CodeInvalidInput    ErrorCode = "INVALID_INPUT"
	CodeMissingRequired ErrorCode = "MISSING_REQUIRED"
	CodeInvalidFormat   ErrorCode = "INVALID_FORMAT"

	// Configuration error codes
	CodeInvalidConfig ErrorCode = "INVALID_CONFIG"
	CodeMissingConfig ErrorCode = "MISSING_CONFIG"

	// Internal error codes
	CodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// FlunqError represents a structured error with code and context
type FlunqError struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Cause   error                  `json:"-"`
}

// Error implements the error interface
func (e *FlunqError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *FlunqError) Unwrap() error {
	return e.Cause
}

// NewFlunqError creates a new FlunqError
func NewFlunqError(code ErrorCode, message string) *FlunqError {
	return &FlunqError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// NewFlunqErrorWithCause creates a new FlunqError with a cause
func NewFlunqErrorWithCause(code ErrorCode, message string, cause error) *FlunqError {
	return &FlunqError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Cause:   cause,
	}
}

// WithDetail adds a detail to the error
func (e *FlunqError) WithDetail(key string, value interface{}) *FlunqError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// IsFlunqError checks if an error is a FlunqError
func IsFlunqError(err error) bool {
	_, ok := err.(*FlunqError)
	return ok
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if flunqErr, ok := err.(*FlunqError); ok {
		return flunqErr.Code
	}
	return CodeInternalError
}

// Convenience functions for common errors
func WorkflowNotFound(workflowID string) *FlunqError {
	return NewFlunqError(CodeWorkflowNotFound, "workflow not found").
		WithDetail("workflow_id", workflowID)
}

func TaskExecutionFailed(taskID string, cause error) *FlunqError {
	return NewFlunqErrorWithCause(CodeTaskExecutionFailed, "task execution failed", cause).
		WithDetail("task_id", taskID)
}

func EventPublishFailed(eventType string, cause error) *FlunqError {
	return NewFlunqErrorWithCause(CodeEventPublishFailed, "event publish failed", cause).
		WithDetail("event_type", eventType)
}

func StorageConnectionFailed(storageType string, cause error) *FlunqError {
	return NewFlunqErrorWithCause(CodeStorageConnection, "storage connection failed", cause).
		WithDetail("storage_type", storageType)
}

func ValidationFailed(field string, reason string) *FlunqError {
	return NewFlunqError(CodeInvalidInput, "validation failed").
		WithDetail("field", field).
		WithDetail("reason", reason)
}
