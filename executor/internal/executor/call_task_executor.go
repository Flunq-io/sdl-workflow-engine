package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// CallTaskExecutor handles 'call' type tasks (HTTP/API calls)
type CallTaskExecutor struct {
	logger     *zap.Logger
	httpClient *http.Client
}

// NewCallTaskExecutor creates a new CallTaskExecutor
func NewCallTaskExecutor(logger *zap.Logger) TaskExecutor {
	return &CallTaskExecutor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTaskType returns the task type this executor handles
func (e *CallTaskExecutor) GetTaskType() string {
	return "call"
}

// Execute executes a call task
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

	url, ok := task.Config.Parameters["url"].(string)
	if !ok {
		return e.createErrorResult(task, startTime, "missing or invalid 'url' parameter")
	}

	method := "GET"
	if m, ok := task.Config.Parameters["method"].(string); ok {
		method = m
	}

	// Prepare request body
	var requestBody io.Reader
	if task.Input != nil && len(task.Input) > 0 {
		bodyBytes, err := json.Marshal(task.Input)
		if err != nil {
			return e.createErrorResult(task, startTime, fmt.Sprintf("failed to marshal request body: %v", err))
		}
		requestBody = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return e.createErrorResult(task, startTime, fmt.Sprintf("failed to create request: %v", err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if headers, ok := task.Config.Parameters["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
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

	e.logger.Info("Call task completed",
		zap.String("task_id", task.TaskID),
		zap.String("method", method),
		zap.String("url", url),
		zap.Int("status_code", resp.StatusCode),
		zap.Bool("success", success),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:     task.TaskID,
		TaskName:   task.TaskName,
		Success:    success,
		Output:     output,
		Duration:   duration,
		ExecutedAt: startTime,
	}, nil
}

func (e *CallTaskExecutor) createErrorResult(task *TaskRequest, startTime time.Time, errorMsg string) (*TaskResult, error) {
	duration := time.Since(startTime)
	
	e.logger.Error("Call task failed",
		zap.String("task_id", task.TaskID),
		zap.String("error", errorMsg),
		zap.Duration("duration", duration))

	return &TaskResult{
		TaskID:     task.TaskID,
		TaskName:   task.TaskName,
		Success:    false,
		Output:     map[string]interface{}{},
		Error:      errorMsg,
		Duration:   duration,
		ExecutedAt: startTime,
	}, nil
}
