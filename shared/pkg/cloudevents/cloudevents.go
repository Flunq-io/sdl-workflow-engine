package cloudevents

import (
	"encoding/json"
	"time"
)

// CloudEvent represents a CloudEvents v1.0 specification event
// https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/spec.md
// This is the canonical CloudEvents implementation shared across all flunq.io services
type CloudEvent struct {
	// Required attributes (CloudEvents v1.0 spec)
	ID          string `json:"id"`
	Source      string `json:"source"`
	SpecVersion string `json:"specversion"`
	Type        string `json:"type"`

	// Optional attributes (CloudEvents v1.0 spec)
	DataContentType string    `json:"datacontenttype,omitempty"`
	DataSchema      string    `json:"dataschema,omitempty"`
	Subject         string    `json:"subject,omitempty"`
	Time            time.Time `json:"time,omitempty"`

	// Extension attributes (CloudEvents v1.0 spec)
	Extensions map[string]interface{} `json:"-"`

	// Data payload (CloudEvents v1.0 spec)
	Data interface{} `json:"data,omitempty"`

	// Flunq.io specific extensions (following CloudEvents extension pattern)
	TenantID      string `json:"tenantid,omitempty"`
	WorkflowID    string `json:"workflowid,omitempty"`
	ExecutionID   string `json:"executionid,omitempty"`
	TaskID        string `json:"taskid,omitempty"`
	CorrelationID string `json:"correlationid,omitempty"`
}

// EventType constants for flunq.io workflow events
// Following CloudEvents type naming convention: reverse-DNS + action
const (
	// Workflow Lifecycle Events
	WorkflowCreated   = "io.flunq.workflow.created"
	WorkflowStarted   = "io.flunq.workflow.started"
	WorkflowCompleted = "io.flunq.workflow.completed"
	WorkflowFailed    = "io.flunq.workflow.failed"
	WorkflowCancelled = "io.flunq.workflow.cancelled"
	WorkflowPaused    = "io.flunq.workflow.paused"
	WorkflowResumed   = "io.flunq.workflow.resumed"
	WorkflowDeleted   = "io.flunq.workflow.deleted"

	// Execution Events
	ExecutionStarted   = "io.flunq.execution.started"
	ExecutionCompleted = "io.flunq.execution.completed"
	ExecutionFailed    = "io.flunq.execution.failed"
	ExecutionCancelled = "io.flunq.execution.cancelled"
	ExecutionPaused    = "io.flunq.execution.paused"
	ExecutionResumed   = "io.flunq.execution.resumed"

	// Task Events (granular task tracking)
	TaskScheduled = "io.flunq.task.scheduled"
	TaskStarted   = "io.flunq.task.started"
	TaskCompleted = "io.flunq.task.completed"
	TaskFailed    = "io.flunq.task.failed"
	TaskCancelled = "io.flunq.task.cancelled"
	TaskRetried   = "io.flunq.task.retried"
	TaskTimedOut  = "io.flunq.task.timedout"

	// Step Events
	WorkflowStepStarted   = "io.flunq.workflow.step.started"
	WorkflowStepCompleted = "io.flunq.workflow.step.completed"
	WorkflowStepFailed    = "io.flunq.workflow.step.failed"
	WorkflowStepSkipped   = "io.flunq.workflow.step.skipped"

	// System Events
	SystemHealthCheck = "io.flunq.system.healthcheck"
	SystemMetrics     = "io.flunq.system.metrics"
	SystemAlert       = "io.flunq.system.alert"
)

// Source constants for flunq.io services
const (
	SourceAPI      = "io.flunq.api"
	SourceWorker   = "io.flunq.worker"
	SourceExecutor = "io.flunq.executor"
	SourceEvents   = "io.flunq.events"
	SourceUI       = "io.flunq.ui"
	SourceSystem   = "io.flunq.system"
)

// SpecVersion is the CloudEvents specification version we implement
const SpecVersion = "1.0"

// NewCloudEvent creates a new CloudEvent with required fields
func NewCloudEvent(id, source, eventType string) *CloudEvent {
	return &CloudEvent{
		ID:          id,
		Source:      source,
		SpecVersion: SpecVersion,
		Type:        eventType,
		Time:        time.Now(),
		Extensions:  make(map[string]interface{}),
	}
}

// SetData sets the event data payload
func (e *CloudEvent) SetData(data interface{}) {
	e.Data = data
	e.DataContentType = "application/json"
}

// SetWorkflowContext sets workflow-specific extension attributes
func (e *CloudEvent) SetWorkflowContext(workflowID, executionID, correlationID string) {
	e.WorkflowID = workflowID
	e.ExecutionID = executionID
	e.CorrelationID = correlationID
}

// SetTaskContext sets task-specific extension attributes
func (e *CloudEvent) SetTaskContext(taskID string) {
	e.TaskID = taskID
}

// AddExtension adds a custom extension attribute
func (e *CloudEvent) AddExtension(key string, value interface{}) {
	if e.Extensions == nil {
		e.Extensions = make(map[string]interface{})
	}
	e.Extensions[key] = value
}

// GetExtension retrieves a custom extension attribute
func (e *CloudEvent) GetExtension(key string) (interface{}, bool) {
	if e.Extensions == nil {
		return nil, false
	}
	value, exists := e.Extensions[key]
	return value, exists
}

// Validate checks if the CloudEvent meets the v1.0 specification requirements
func (e *CloudEvent) Validate() error {
	if e.ID == "" {
		return &ValidationError{Field: "id", Message: "required field is empty"}
	}
	if e.Source == "" {
		return &ValidationError{Field: "source", Message: "required field is empty"}
	}
	if e.SpecVersion == "" {
		return &ValidationError{Field: "specversion", Message: "required field is empty"}
	}
	if e.Type == "" {
		return &ValidationError{Field: "type", Message: "required field is empty"}
	}
	return nil
}

// ValidationError represents a CloudEvent validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "cloudevents: validation failed for field '" + e.Field + "': " + e.Message
}

// ToJSON serializes the CloudEvent to JSON
func (e *CloudEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes a CloudEvent from JSON
func FromJSON(data []byte) (*CloudEvent, error) {
	var event CloudEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Clone creates a deep copy of the CloudEvent
func (e *CloudEvent) Clone() *CloudEvent {
	clone := &CloudEvent{
		ID:              e.ID,
		Source:          e.Source,
		SpecVersion:     e.SpecVersion,
		Type:            e.Type,
		DataContentType: e.DataContentType,
		DataSchema:      e.DataSchema,
		Subject:         e.Subject,
		Time:            e.Time,
		WorkflowID:      e.WorkflowID,
		ExecutionID:     e.ExecutionID,
		TaskID:          e.TaskID,
		CorrelationID:   e.CorrelationID,
		Data:            e.Data,
	}

	// Deep copy extensions
	if e.Extensions != nil {
		clone.Extensions = make(map[string]interface{})
		for k, v := range e.Extensions {
			clone.Extensions[k] = v
		}
	}

	return clone
}

// IsWorkflowEvent checks if this is a workflow-related event
func (e *CloudEvent) IsWorkflowEvent() bool {
	switch e.Type {
	case WorkflowCreated, WorkflowStarted, WorkflowCompleted, WorkflowFailed,
		WorkflowCancelled, WorkflowPaused, WorkflowResumed, WorkflowDeleted:
		return true
	default:
		return false
	}
}

// IsExecutionEvent checks if this is an execution-related event
func (e *CloudEvent) IsExecutionEvent() bool {
	switch e.Type {
	case ExecutionStarted, ExecutionCompleted, ExecutionFailed,
		ExecutionCancelled, ExecutionPaused, ExecutionResumed:
		return true
	default:
		return false
	}
}

// IsTaskEvent checks if this is a task-related event
func (e *CloudEvent) IsTaskEvent() bool {
	switch e.Type {
	case TaskScheduled, TaskStarted, TaskCompleted, TaskFailed,
		TaskCancelled, TaskRetried, TaskTimedOut:
		return true
	default:
		return false
	}
}

// GetEventCategory returns the category of the event (workflow, execution, task, step, system)
func (e *CloudEvent) GetEventCategory() string {
	if e.IsWorkflowEvent() {
		return "workflow"
	}
	if e.IsExecutionEvent() {
		return "execution"
	}
	if e.IsTaskEvent() {
		return "task"
	}
	switch e.Type {
	case WorkflowStepStarted, WorkflowStepCompleted, WorkflowStepFailed, WorkflowStepSkipped:
		return "step"
	case SystemHealthCheck, SystemMetrics, SystemAlert:
		return "system"
	default:
		return "unknown"
	}
}
