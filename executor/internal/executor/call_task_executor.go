package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// CallTaskExecutor handles 'call' type tasks (HTTP/API calls and OpenAPI calls)
type CallTaskExecutor struct {
	logger        *zap.Logger
	httpClient    *http.Client
	openAPIClient OpenAPIClient
}

// NewCallTaskExecutor creates a new CallTaskExecutor
func NewCallTaskExecutor(logger *zap.Logger) TaskExecutor {
	return &CallTaskExecutor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		openAPIClient: NewOpenAPIClient(logger),
	}
}

// GetTaskType returns the task type this executor handles
func (e *CallTaskExecutor) GetTaskType() string {
	return "call"
}

// Execute executes a call task (HTTP or OpenAPI)
func (e *CallTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	startTime := time.Now()

	e.logger.Info("Executing call task",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.String("workflow_id", task.WorkflowID))

	// Extract call parameters from config
	if task.Config == nil || task.Config.Parameters == nil {
		return e.createErrorResult(task, startTime, "missing call configuration")
	}

	// Parse call configuration
	callConfig, err := ParseCallConfig(task.Config.Parameters)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("invalid call configuration: %v", err))
	}

	// Validate configuration
	if err := callConfig.Validate(); err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("call configuration validation failed: %v", err))
	}

	// Route to appropriate handler based on call type
	if callConfig.IsOpenAPICall() {
		return e.executeOpenAPICall(ctx, task, callConfig, startTime)
	} else {
		return e.executeHTTPCall(ctx, task, callConfig, startTime)
	}
}

// executeOpenAPICall executes an OpenAPI call
func (e *CallTaskExecutor) executeOpenAPICall(ctx context.Context, task *TaskRequest, config *CallConfig, startTime time.Time) (*TaskResult, error) {
	e.logger.Info("Executing OpenAPI call",
		zap.String("task_id", task.TaskID),
		zap.String("operation_id", config.OperationID),
		zap.String("document", config.Document.Endpoint))

	// Execute OpenAPI operation
	response, err := e.openAPIClient.ExecuteOperation(ctx, config)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("OpenAPI operation failed: %v", err))
	}

	// Build output based on output format
	var output map[string]interface{}
	switch config.Output {
	case "raw":
		output = map[string]interface{}{
			"status_code": response.StatusCode,
			"headers":     response.Headers,
			"body":        response.Body,
			"request":     response.Request,
		}
	case "response":
		output = map[string]interface{}{
			"status_code": response.StatusCode,
			"headers":     response.Headers,
			"body":        string(response.Body),
			"request":     response.Request,
		}
	case "content", "":
		output = map[string]interface{}{
			"status_code": response.StatusCode,
			"headers":     response.Headers,
			"data":        response.Content,
			"request":     response.Request,
		}
	}

	duration := time.Since(startTime)
	success := response.StatusCode >= 200 && response.StatusCode < 300

	e.logger.Info("OpenAPI call completed",
		zap.String("task_id", task.TaskID),
		zap.String("operation_id", config.OperationID),
		zap.String("method", response.Request.Method),
		zap.String("url", response.Request.URL),
		zap.Int("status_code", response.StatusCode),
		zap.Bool("success", success),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     success,
		Input:       task.Input,
		Output:      output,
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}, nil
}

// executeHTTPCall executes a direct HTTP call (backward compatibility)
func (e *CallTaskExecutor) executeHTTPCall(ctx context.Context, task *TaskRequest, config *CallConfig, startTime time.Time) (*TaskResult, error) {
	e.logger.Info("Executing HTTP call",
		zap.String("task_id", task.TaskID),
		zap.String("method", config.Method),
		zap.String("url", config.URL))

	// Prepare request body
	var requestBody io.Reader
	if config.Body != nil {
		bodyBytes, err := json.Marshal(config.Body)
		if err != nil {
			return e.createErrorResult(task, startTime, fmt.Sprintf("failed to marshal request body: %v", err))
		}
		requestBody = bytes.NewReader(bodyBytes)
	} else if task.Input != nil && len(task.Input) > 0 {
		// Fallback to task input for backward compatibility
		bodyBytes, err := json.Marshal(task.Input)
		if err != nil {
			return e.createErrorResult(task, startTime, fmt.Sprintf("failed to marshal request body: %v", err))
		}
		requestBody = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, requestBody)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("failed to create request: %v", err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "flunq-executor/1.0")

	for key, value := range config.Headers {
		if strValue, ok := value.(string); ok {
			req.Header.Set(key, strValue)
		}
	}

	// Add query parameters
	if len(config.Query) > 0 {
		q := req.URL.Query()
		for key, value := range config.Query {
			q.Add(key, fmt.Sprintf("%v", value))
		}
		req.URL.RawQuery = q.Encode()
	}

	// Execute HTTP request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("HTTP request failed: %v", err))
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("failed to read response: %v", err))
	}

	// Parse response
	var responseData map[string]interface{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &responseData); err != nil {
			// If JSON parsing fails, store as string
			responseData = map[string]interface{}{
				"raw_response": string(responseBody),
			}
		}
	}

	// Add response metadata
	output := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"data":        responseData,
	}

	duration := time.Since(startTime)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	e.logger.Info("HTTP call completed",
		zap.String("task_id", task.TaskID),
		zap.String("method", config.Method),
		zap.String("url", config.URL),
		zap.Int("status_code", resp.StatusCode),
		zap.Bool("success", success),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     success,
		Input:       task.Input,
		Output:      output,
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}, nil
}

func (e *CallTaskExecutor) createErrorResult(task *TaskRequest, startTime time.Time, errorMsg string) (*TaskResult, error) {
	duration := time.Since(startTime)

	// Create structured workflow error
	workflowErr := e.createWorkflowError(errorMsg, task)

	e.logger.Error("Call task failed",
		zap.String("task_id", task.TaskID),
		zap.String("error_type", workflowErr.Type),
		zap.Int("status", workflowErr.Status),
		zap.String("error", workflowErr.Error()),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     false,
		Input:       task.Input,
		Output:      map[string]interface{}{},
		Error:       workflowErr.Error(),
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}, nil
}

// createWorkflowError creates a structured WorkflowError from an error message
func (e *CallTaskExecutor) createWorkflowError(errorMsg string, task *TaskRequest) *WorkflowError {
	// Extract HTTP status from error message if present
	status := e.extractHTTPStatus(errorMsg)

	// Determine error type based on error content and status
	errorType := e.determineErrorType(errorMsg, status)

	// Extract additional context from task configuration
	var endpoint, method string
	if task.Config != nil && task.Config.Parameters != nil {
		if callConfig, err := ParseCallConfig(task.Config.Parameters); err == nil {
			if callConfig.IsOpenAPICall() {
				endpoint = callConfig.Document.Endpoint
				method = "OpenAPI"
			} else {
				endpoint = callConfig.URL
				method = callConfig.Method
			}
		}
	}

	return &WorkflowError{
		Type:     errorType,
		Status:   status,
		Title:    "API Call Failed",
		Detail:   errorMsg,
		Instance: fmt.Sprintf("/tasks/%s", task.TaskID),
		Data: map[string]interface{}{
			"task_id":   task.TaskID,
			"task_type": task.TaskType,
			"endpoint":  endpoint,
			"method":    method,
		},
	}
}

// extractHTTPStatus attempts to extract HTTP status code from error message
func (e *CallTaskExecutor) extractHTTPStatus(errMsg string) int {
	// Look for patterns like "status 503", "HTTP 404", etc.
	patterns := []string{
		`status (\d{3})`,
		`HTTP (\d{3})`,
		`(\d{3}) status`,
		`failed with status (\d{3})`,
	}

	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(errMsg); len(matches) > 1 {
			if status, err := strconv.Atoi(matches[1]); err == nil {
				return status
			}
		}
	}

	return 500 // Default to internal server error
}

// determineErrorType determines the appropriate error type based on error content and status
func (e *CallTaskExecutor) determineErrorType(errMsg string, status int) string {
	errMsgLower := strings.ToLower(errMsg)

	// Check for specific error patterns
	switch {
	case strings.Contains(errMsgLower, "timeout") || strings.Contains(errMsgLower, "deadline"):
		return ErrorTypeTimeout
	case strings.Contains(errMsgLower, "unauthorized") || status == 401:
		return ErrorTypeAuthentication
	case strings.Contains(errMsgLower, "forbidden") || status == 403:
		return ErrorTypeAuthorization
	case strings.Contains(errMsgLower, "validation") || strings.Contains(errMsgLower, "invalid") || status == 400:
		return ErrorTypeValidation
	case strings.Contains(errMsgLower, "configuration") || strings.Contains(errMsgLower, "config"):
		return ErrorTypeConfiguration
	case strings.Contains(errMsgLower, "connection") || strings.Contains(errMsgLower, "network") ||
		strings.Contains(errMsgLower, "http") || status >= 500:
		return ErrorTypeCommunication
	default:
		return ErrorTypeRuntime
	}
}
