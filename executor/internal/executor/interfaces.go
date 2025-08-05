package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/flunq-io/events/pkg/cloudevents"
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
	WorkflowID  string                 `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	Success     bool                   `json:"success"`
	Output      map[string]interface{} `json:"output"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	ExecutedAt  time.Time              `json:"executed_at"`
}

// ParseTaskRequestFromEvent parses a CloudEvent into a TaskRequest
func ParseTaskRequestFromEvent(event *cloudevents.CloudEvent) (*TaskRequest, error) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid event data format, got type: %T", event.Data)
	}

	// Debug: print all keys in the data
	fmt.Printf("DEBUG: Event data keys: ")
	for key := range data {
		fmt.Printf("%s ", key)
	}
	fmt.Printf("\n")
	fmt.Printf("DEBUG: task_type value: '%v' (type: %T)\n", data["task_type"], data["task_type"])

	request := &TaskRequest{
		TaskID:      getStringFromData(data, "task_id"),
		TaskName:    getStringFromData(data, "task_name"),
		TaskType:    getStringFromData(data, "task_type"),
		WorkflowID:  event.WorkflowID,
		ExecutionID: event.ExecutionID,
	}

	// Debug: Check if WorkflowID is empty
	fmt.Printf("DEBUG: Parsed TaskRequest - WorkflowID: '%s', ExecutionID: '%s', TaskName: '%s'\n",
		request.WorkflowID, request.ExecutionID, request.TaskName)

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
