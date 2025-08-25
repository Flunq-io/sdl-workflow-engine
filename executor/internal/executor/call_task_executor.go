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
	e.logger.Info("🚀 EXECUTING OPENAPI CALL WITH NEW CODE",
		zap.String("task_id", task.TaskID),
		zap.String("operation_id", config.OperationID),
		zap.String("document", config.Document.Endpoint),
		zap.Any("task_input", task.Input),
		zap.Any("config_parameters", config.Parameters))

	// Evaluate expressions in parameters before executing
	e.logger.Debug("Evaluating parameter expressions",
		zap.String("task_id", task.TaskID),
		zap.Any("original_parameters", config.Parameters))

	evaluatedConfig, err := e.evaluateParameterExpressions(config, task)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("failed to evaluate parameter expressions: %v", err))
	}

	e.logger.Debug("Parameter expressions evaluated",
		zap.String("task_id", task.TaskID),
		zap.Any("evaluated_parameters", evaluatedConfig.Parameters))

	// Build resolved input data for storage in TaskResult
	resolvedInput := e.buildResolvedInput(task, evaluatedConfig)

	// Execute OpenAPI operation
	response, err := e.openAPIClient.ExecuteOperation(ctx, evaluatedConfig)

	// Log the response details for debugging
	if response != nil {
		e.logger.Debug("OpenAPI response received",
			zap.String("task_id", task.TaskID),
			zap.Int("status_code", response.StatusCode),
			zap.Any("headers", response.Headers),
			zap.String("content_preview", fmt.Sprintf("%.200s", response.Body)))
	}

	// Handle the case where we have a response but also an error (HTTP error status)
	if err != nil && response == nil {
		// True error case - no response received
		e.logger.Error("OpenAPI operation failed with no response",
			zap.String("task_id", task.TaskID),
			zap.Error(err))
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
			"content":     response.Content,
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

	// Apply output transformation if configured
	finalOutput := e.applyOutputTransformation(task, output)

	// Log the final output for debugging
	e.logger.Debug("OpenAPI call final output",
		zap.String("task_id", task.TaskID),
		zap.Bool("success", success),
		zap.Any("final_output", finalOutput))

	// Create the task result
	result := &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     success,
		Input:       resolvedInput, // Use resolved input instead of original
		Output:      finalOutput,
		Duration:    duration,
		StartedAt:   startTime,
		ExecutedAt:  time.Now(),
	}

	// If we had an HTTP error, include the error details and return the error for retry handling
	if err != nil {
		result.Error = err.Error()
		e.logger.Warn("OpenAPI call returned HTTP error but response data is available",
			zap.String("task_id", task.TaskID),
			zap.Int("status_code", response.StatusCode),
			zap.Error(err))

		// Return the error so TryTaskExecutor can handle retries
		return result, err
	}

	return result, nil
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

	// Build resolved input for HTTP calls
	resolvedInput := e.buildResolvedInputForHTTP(task, config)

	return &TaskResult{
		TaskID:      task.TaskID,
		TaskName:    task.TaskName,
		TaskType:    task.TaskType,
		WorkflowID:  task.WorkflowID,
		ExecutionID: task.ExecutionID,
		Success:     success,
		Input:       resolvedInput, // Use resolved input instead of original
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
			if callConfig.IsOpenAPICall() && callConfig.Document != nil {
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

// applyOutputTransformation applies output transformation based on task configuration
func (e *CallTaskExecutor) applyOutputTransformation(task *TaskRequest, rawOutput map[string]interface{}) map[string]interface{} {
	// Check if output transformation is configured in task input
	if task.Input == nil {
		e.logger.Info("🔍 NO TASK INPUT - skipping transformation",
			zap.String("task_id", task.TaskID),
			zap.String("task_name", task.TaskName))
		return rawOutput
	}

	outputConfig, exists := task.Input["output_config"]
	if !exists {
		e.logger.Info("🔍 NO OUTPUT_CONFIG - skipping transformation",
			zap.String("task_id", task.TaskID),
			zap.String("task_name", task.TaskName),
			zap.Any("available_input_keys", getMapKeys(task.Input)))
		return rawOutput
	}

	e.logger.Info("🎯 APPLYING OUTPUT TRANSFORMATION",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.Any("output_config", outputConfig),
		zap.String("output_config_type", fmt.Sprintf("%T", outputConfig)))

	// Apply transformation based on configuration type
	transformed, err := e.transformOutput(outputConfig, rawOutput, task.TaskName)
	if err != nil {
		e.logger.Error("❌ TRANSFORMATION FAILED - using raw output",
			zap.String("task_id", task.TaskID),
			zap.String("task_name", task.TaskName),
			zap.Error(err))
		return rawOutput
	}

	e.logger.Info("✅ TRANSFORMATION SUCCESSFUL",
		zap.String("task_id", task.TaskID),
		zap.String("task_name", task.TaskName),
		zap.Any("raw_output_keys", getMapKeys(rawOutput)),
		zap.Any("transformed_output", transformed),
		zap.Any("transformed_output_keys", getMapKeys(transformed)))

	return transformed
}

// transformOutput applies the actual transformation logic
func (e *CallTaskExecutor) transformOutput(outputConfig interface{}, rawOutput map[string]interface{}, taskName string) (map[string]interface{}, error) {
	// Handle SDL "as" syntax: {"as": {"varName": "${ .path }"}}
	if configMap, ok := outputConfig.(map[string]interface{}); ok {
		if asConfig, exists := configMap["as"]; exists {
			if asMap, ok := asConfig.(map[string]interface{}); ok {
				result := make(map[string]interface{})

				for varName, expr := range asMap {
					if exprStr, ok := expr.(string); ok {
						value, err := e.evaluateExpression(exprStr, rawOutput)
						if err != nil {
							e.logger.Warn("Failed to evaluate expression, skipping",
								zap.String("variable", varName),
								zap.String("expression", exprStr),
								zap.Error(err))
							continue
						}
						result[varName] = value
					}
				}

				return result, nil
			}
		}
	}

	// Handle simple string expressions like "${ .content[0].name }" (fallback to task name)
	if expr, ok := outputConfig.(string); ok {
		value, err := e.evaluateExpression(expr, rawOutput)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression '%s': %w", expr, err)
		}

		// Use task name as the variable name when no "as" is provided
		return map[string]interface{}{
			taskName: value,
		}, nil
	}

	return nil, fmt.Errorf("unsupported output configuration type: %T", outputConfig)
}

// evaluateExpression evaluates a single expression like "${ .content[0].name }"
func (e *CallTaskExecutor) evaluateExpression(expr string, rawOutput map[string]interface{}) (interface{}, error) {
	// Remove ${ } wrapper if present
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimSpace(expr[2 : len(expr)-1])
	}

	// Handle field access like ".content", ".content[0]", ".content[0].name"
	if strings.HasPrefix(expr, ".") {
		path := strings.TrimPrefix(expr, ".")
		return e.extractValueFromPath(rawOutput, path)
	}

	return nil, fmt.Errorf("unsupported expression format: %s", expr)
}

// extractValueFromPath extracts a value from a nested map using a path like "content[0].name"
func (e *CallTaskExecutor) extractValueFromPath(data map[string]interface{}, path string) (interface{}, error) {
	parts := e.splitPathWithArrays(path)
	current := interface{}(data)

	for i, part := range parts {
		e.logger.Debug("Processing path part",
			zap.String("part", part),
			zap.Int("index", i),
			zap.String("current_type", fmt.Sprintf("%T", current)))

		// Handle array indexing like "content[0]"
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			fieldName := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]

			// Get the field first
			if currentMap, ok := current.(map[string]interface{}); ok {
				if fieldValue, exists := currentMap[fieldName]; exists {
					// Now handle array indexing
					if arrayValue, ok := fieldValue.([]interface{}); ok {
						index := 0
						if indexStr != "" {
							if idx, err := strconv.Atoi(indexStr); err == nil {
								index = idx
							} else {
								return nil, fmt.Errorf("invalid array index '%s'", indexStr)
							}
						}

						if index < 0 || index >= len(arrayValue) {
							return nil, fmt.Errorf("array index %d out of bounds (length: %d)", index, len(arrayValue))
						}

						current = arrayValue[index]
					} else {
						return nil, fmt.Errorf("field '%s' is not an array", fieldName)
					}
				} else {
					return nil, fmt.Errorf("field '%s' not found", fieldName)
				}
			} else {
				return nil, fmt.Errorf("cannot access field '%s' on non-object", fieldName)
			}
		} else {
			// Handle regular field access
			if currentMap, ok := current.(map[string]interface{}); ok {
				if value, exists := currentMap[part]; exists {
					current = value
				} else {
					return nil, fmt.Errorf("field '%s' not found", part)
				}
			} else {
				return nil, fmt.Errorf("cannot access field '%s' on non-object", part)
			}
		}
	}

	return current, nil
}

// splitPathWithArrays splits a path like "content[0].name" into ["content[0]", "name"]
func (e *CallTaskExecutor) splitPathWithArrays(path string) []string {
	var parts []string
	current := ""
	inBrackets := false

	for _, char := range path {
		switch char {
		case '[':
			inBrackets = true
			current += string(char)
		case ']':
			inBrackets = false
			current += string(char)
		case '.':
			if inBrackets {
				current += string(char)
			} else {
				if current != "" {
					parts = append(parts, current)
					current = ""
				}
			}
		default:
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// evaluateParameterExpressions evaluates expressions in OpenAPI parameters using task input
func (e *CallTaskExecutor) evaluateParameterExpressions(config *CallConfig, task *TaskRequest) (*CallConfig, error) {
	e.logger.Debug("Starting parameter expression evaluation",
		zap.String("task_id", task.TaskID),
		zap.Any("task_input", task.Input),
		zap.Any("config_parameters", config.Parameters))

	// Create a copy of the config to avoid modifying the original
	evaluatedConfig := *config

	// Create evaluation context from workflow context
	evalContext := make(map[string]interface{})

	// Check if workflow context is available
	if task.Context != nil {
		e.logger.Debug("Task context available",
			zap.String("task_id", task.TaskID),
			zap.Any("task_context", task.Context))

		// Use workflow execution input as the primary input context
		if workflowInput, exists := task.Context["workflow_input"]; exists {
			evalContext["input"] = workflowInput
			e.logger.Debug("Using workflow execution input for expression evaluation",
				zap.String("task_id", task.TaskID),
				zap.Any("workflow_input", workflowInput))
		} else {
			// Fallback to task input if no workflow input available
			evalContext["input"] = task.Input
			e.logger.Debug("No workflow input found, using task input",
				zap.String("task_id", task.TaskID),
				zap.Any("task_input", task.Input))
		}

		// Add workflow variables to evaluation context
		if variables, exists := task.Context["variables"]; exists {
			evalContext["variables"] = variables
			e.logger.Debug("Added workflow variables to evaluation context",
				zap.String("task_id", task.TaskID),
				zap.Any("variables", variables))
		}

		// Add previous task outputs to evaluation context
		if taskOutputs, exists := task.Context["task_outputs"]; exists {
			evalContext["task_outputs"] = taskOutputs
			e.logger.Debug("Added task outputs to evaluation context",
				zap.String("task_id", task.TaskID),
				zap.Any("task_outputs", taskOutputs))
		}
	} else {
		// No context, use task input as fallback
		evalContext["input"] = task.Input
		e.logger.Debug("No task context available, using task input",
			zap.String("task_id", task.TaskID),
			zap.Any("task_input", task.Input))
	}

	e.logger.Debug("Created evaluation context",
		zap.String("task_id", task.TaskID),
		zap.Any("eval_context", evalContext))

	// Evaluate expressions in parameters
	if config.Parameters != nil {
		evaluatedParams := make(map[string]interface{})

		for key, value := range config.Parameters {
			if strValue, ok := value.(string); ok {
				// Check if this is an expression
				if strings.HasPrefix(strValue, "${") && strings.HasSuffix(strValue, "}") {
					// Evaluate the expression
					evaluatedValue, err := e.evaluateExpression(strValue, evalContext)
					if err != nil {
						e.logger.Warn("Failed to evaluate parameter expression, using original value",
							zap.String("parameter", key),
							zap.String("expression", strValue),
							zap.Error(err))
						evaluatedParams[key] = value // Use original value if evaluation fails
					} else {
						evaluatedParams[key] = evaluatedValue
						e.logger.Debug("Evaluated parameter expression",
							zap.String("parameter", key),
							zap.String("expression", strValue),
							zap.Any("result", evaluatedValue))
					}
				} else {
					// Not an expression, use as-is
					evaluatedParams[key] = value
				}
			} else {
				// Not a string, use as-is
				evaluatedParams[key] = value
			}
		}

		evaluatedConfig.Parameters = evaluatedParams
	}

	return &evaluatedConfig, nil
}

// getMapKeys returns the keys of a map for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// buildResolvedInput builds the resolved input data for storage in TaskResult
func (e *CallTaskExecutor) buildResolvedInput(task *TaskRequest, evaluatedConfig *CallConfig) map[string]interface{} {
	resolvedInput := make(map[string]interface{})

	// Copy original task input as base
	if task.Input != nil {
		for key, value := range task.Input {
			resolvedInput[key] = value
		}
	}

	// Add resolved call configuration
	resolvedInput["call_type"] = evaluatedConfig.CallType

	if evaluatedConfig.CallType == "openapi" {
		resolvedInput["document"] = map[string]interface{}{
			"endpoint": evaluatedConfig.Document.Endpoint,
		}
		resolvedInput["operation_id"] = evaluatedConfig.OperationID

		// Store resolved parameters (this is the key fix)
		if evaluatedConfig.Parameters != nil {
			resolvedInput["parameters"] = evaluatedConfig.Parameters
		}
	} else if evaluatedConfig.CallType == "http" {
		resolvedInput["method"] = evaluatedConfig.Method
		resolvedInput["url"] = evaluatedConfig.URL

		// Store resolved parameters
		if evaluatedConfig.Parameters != nil {
			resolvedInput["parameters"] = evaluatedConfig.Parameters
		}

		// Store resolved headers
		if evaluatedConfig.Headers != nil {
			resolvedInput["headers"] = evaluatedConfig.Headers
		}

		// Store resolved body
		if evaluatedConfig.Body != nil {
			resolvedInput["body"] = evaluatedConfig.Body
		}
	}

	return resolvedInput
}

// buildResolvedInputForHTTP builds the resolved input data for HTTP calls
func (e *CallTaskExecutor) buildResolvedInputForHTTP(task *TaskRequest, config *CallConfig) map[string]interface{} {
	resolvedInput := make(map[string]interface{})

	// Copy original task input as base
	if task.Input != nil {
		for key, value := range task.Input {
			resolvedInput[key] = value
		}
	}

	// Add resolved HTTP configuration
	resolvedInput["call_type"] = "http"
	resolvedInput["method"] = config.Method
	resolvedInput["url"] = config.URL

	// Store resolved parameters
	if config.Parameters != nil {
		resolvedInput["parameters"] = config.Parameters
	}

	// Store resolved headers
	if config.Headers != nil {
		resolvedInput["headers"] = config.Headers
	}

	// Store resolved body
	if config.Body != nil {
		resolvedInput["body"] = config.Body
	}

	return resolvedInput
}
