package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// TaskExecutor defines the interface for executing different task types
type TaskExecutor interface {
	// Execute executes a task and returns the result
	Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error)

	// GetTaskType returns the task type this executor handles
	GetTaskType() string
}

// TaskRequest represents a task execution request
type TaskRequest struct {
	TaskID      string                 `json:"task_id"`
	TaskName    string                 `json:"task_name"`
	TaskType    string                 `json:"task_type"`
	WorkflowID  string                 `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	Input       map[string]interface{} `json:"input"`
	Config      *TaskConfig            `json:"config"`
	Context     map[string]interface{} `json:"context"`
}

// TaskConfig contains task-specific configuration
type TaskConfig struct {
	Timeout    time.Duration          `json:"timeout"`
	Retries    int                    `json:"retries"`
	Queue      string                 `json:"queue"`
	Parameters map[string]interface{} `json:"parameters"`
}

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	TaskName    string                 `json:"task_name"`
	TaskType    string                 `json:"task_type"`
	WorkflowID  string                 `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	Success     bool                   `json:"success"`
	Input       map[string]interface{} `json:"input"` // Add input data
	Output      map[string]interface{} `json:"output"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	StartedAt   time.Time              `json:"started_at"` // Add start time
	ExecutedAt  time.Time              `json:"executed_at"`
}

// ParseTaskRequestFromEvent parses a CloudEvent into a TaskRequest
func ParseTaskRequestFromEvent(event *cloudevents.CloudEvent) (*TaskRequest, error) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid event data format, got type: %T", event.Data)
	}

	// (debug) print event data keys only when needed
	// fmt.Printf("Event data keys: %v\n", reflect.ValueOf(data).MapKeys())

	// Extract execution_id from data payload as fallback
	executionID := event.ExecutionID
	if executionID == "" {
		executionID = getStringFromData(data, "execution_id")
	}

	// Extract workflow_id from data payload as fallback
	workflowID := event.WorkflowID
	if workflowID == "" {
		workflowID = getStringFromData(data, "workflow_id")
	}

	taskType := getStringFromData(data, "task_type")

	request := &TaskRequest{
		TaskID:      getStringFromData(data, "task_id"),
		TaskName:    getStringFromData(data, "task_name"),
		TaskType:    taskType,
		WorkflowID:  workflowID,
		ExecutionID: executionID,
	}

	// Debug logging to see what task type is being parsed
	fmt.Printf("🔍 ParseTaskRequestFromEvent DEBUG - TaskType: '%s', TaskName: '%s'\n", taskType, request.TaskName)

	// (debug) Parsed TaskRequest summary
	// fmt.Printf("Parsed TaskRequest - workflow=%s exec=%s task=%s\n", request.WorkflowID, request.ExecutionID, request.TaskName)

	// Parse input if present
	if input, ok := data["input"].(map[string]interface{}); ok {
		request.Input = input
	}

	// Parse context if present
	if context, ok := data["context"].(map[string]interface{}); ok {
		request.Context = context
	}

	// Parse config if present
	if configData, ok := data["config"].(map[string]interface{}); ok {
		request.Config = parseTaskConfig(configData)
	}

	return request, nil
}

func getStringFromData(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func parseTaskConfig(data map[string]interface{}) *TaskConfig {
	config := &TaskConfig{}

	if timeout, ok := data["timeout"].(float64); ok {
		config.Timeout = time.Duration(timeout) * time.Second
	}

	if retries, ok := data["retries"].(float64); ok {
		config.Retries = int(retries)
	}

	if queue, ok := data["queue"].(string); ok {
		config.Queue = queue
	}

	if params, ok := data["parameters"].(map[string]interface{}); ok {
		config.Parameters = params
	}

	return config
}
