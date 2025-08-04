package cloudevents

import (
	"encoding/json"
	"time"
)

// CloudEvent represents a CloudEvents v1.0 specification event
// https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/spec.md
type CloudEvent struct {
	// Required attributes
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	SpecVersion string    `json:"specversion"`
	Type        string    `json:"type"`
	
	// Optional attributes
	DataContentType string                 `json:"datacontenttype,omitempty"`
	DataSchema      string                 `json:"dataschema,omitempty"`
	Subject         string                 `json:"subject,omitempty"`
	Time            time.Time              `json:"time,omitempty"`
	
	// Extension attributes
	Extensions map[string]interface{} `json:"-"`
	
	// Data payload
	Data interface{} `json:"data,omitempty"`
	
	// Flunq.io specific extensions
	WorkflowID     string `json:"workflowid,omitempty"`
	ExecutionID    string `json:"executionid,omitempty"`
	TaskID         string `json:"taskid,omitempty"`
	CorrelationID  string `json:"correlationid,omitempty"`
}

// EventType constants for common workflow events
const (
	// Workflow Events
	WorkflowCreated   = "io.flunq.workflow.created"
	WorkflowStarted   = "io.flunq.workflow.started"
	WorkflowCompleted = "io.flunq.workflow.completed"
	WorkflowFailed    = "io.flunq.workflow.failed"
	WorkflowCancelled = "io.flunq.workflow.cancelled"
	WorkflowPaused    = "io.flunq.workflow.paused"
	WorkflowResumed   = "io.flunq.workflow.resumed"
	
	// Execution Events
	ExecutionStarted   = "io.flunq.execution.started"
	ExecutionCompleted = "io.flunq.execution.completed"
	ExecutionFailed    = "io.flunq.execution.failed"
	ExecutionCancelled = "io.flunq.execution.cancelled"
	
	// Task Events
	TaskScheduled = "io.flunq.task.scheduled"
	TaskStarted   = "io.flunq.task.started"
	TaskCompleted = "io.flunq.task.completed"
	TaskFailed    = "io.flunq.task.failed"
	TaskRetried   = "io.flunq.task.retried"
	TaskSkipped   = "io.flunq.task.skipped"
	
	// State Events
	StateEntered = "io.flunq.state.entered"
	StateExited  = "io.flunq.state.exited"
	StateError   = "io.flunq.state.error"
	
	// System Events
	ServiceStarted = "io.flunq.service.started"
	ServiceStopped = "io.flunq.service.stopped"
	ServiceHealth  = "io.flunq.service.health"
)

// Source constants for different services
const (
	SourceAPI      = "io.flunq.api"
	SourceWorker   = "io.flunq.worker"
	SourceExecutor = "io.flunq.executor"
	SourceEvents   = "io.flunq.events"
	SourceUI       = "io.flunq.ui"
)

// NewCloudEvent creates a new CloudEvent with required fields
func NewCloudEvent(id, source, eventType string) *CloudEvent {
	return &CloudEvent{
		ID:          id,
		Source:      source,
		SpecVersion: "1.0",
		Type:        eventType,
		Time:        time.Now(),
		Extensions:  make(map[string]interface{}),
	}
}

// NewWorkflowEvent creates a new workflow-related CloudEvent
func NewWorkflowEvent(id, source, eventType, workflowID string) *CloudEvent {
	event := NewCloudEvent(id, source, eventType)
	event.WorkflowID = workflowID
	event.Subject = "workflow/" + workflowID
	return event
}

// NewExecutionEvent creates a new execution-related CloudEvent
func NewExecutionEvent(id, source, eventType, workflowID, executionID string) *CloudEvent {
	event := NewWorkflowEvent(id, source, eventType, workflowID)
	event.ExecutionID = executionID
	event.Subject = "execution/" + executionID
	return event
}

// NewTaskEvent creates a new task-related CloudEvent
func NewTaskEvent(id, source, eventType, workflowID, executionID, taskID string) *CloudEvent {
	event := NewExecutionEvent(id, source, eventType, workflowID, executionID)
	event.TaskID = taskID
	event.Subject = "task/" + taskID
	return event
}

// SetData sets the event data payload
func (e *CloudEvent) SetData(data interface{}) {
	e.Data = data
	e.DataContentType = "application/json"
}

// SetDataWithContentType sets the event data with specific content type
func (e *CloudEvent) SetDataWithContentType(data interface{}, contentType string) {
	e.Data = data
	e.DataContentType = contentType
}

// SetExtension sets an extension attribute
func (e *CloudEvent) SetExtension(key string, value interface{}) {
	if e.Extensions == nil {
		e.Extensions = make(map[string]interface{})
	}
	e.Extensions[key] = value
}

// GetExtension gets an extension attribute
func (e *CloudEvent) GetExtension(key string) (interface{}, bool) {
	if e.Extensions == nil {
		return nil, false
	}
	value, exists := e.Extensions[key]
	return value, exists
}

// Validate validates the CloudEvent according to the specification
func (e *CloudEvent) Validate() error {
	if e.ID == "" {
		return &ValidationError{Field: "id", Message: "id is required"}
	}
	if e.Source == "" {
		return &ValidationError{Field: "source", Message: "source is required"}
	}
	if e.SpecVersion == "" {
		return &ValidationError{Field: "specversion", Message: "specversion is required"}
	}
	if e.Type == "" {
		return &ValidationError{Field: "type", Message: "type is required"}
	}
	return nil
}

// ValidationError represents a CloudEvent validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// MarshalJSON implements custom JSON marshaling to include extensions
func (e *CloudEvent) MarshalJSON() ([]byte, error) {
	// Create a map with all the standard fields
	eventMap := map[string]interface{}{
		"id":          e.ID,
		"source":      e.Source,
		"specversion": e.SpecVersion,
		"type":        e.Type,
	}
	
	// Add optional fields if they exist
	if e.DataContentType != "" {
		eventMap["datacontenttype"] = e.DataContentType
	}
	if e.DataSchema != "" {
		eventMap["dataschema"] = e.DataSchema
	}
	if e.Subject != "" {
		eventMap["subject"] = e.Subject
	}
	if !e.Time.IsZero() {
		eventMap["time"] = e.Time.Format(time.RFC3339)
	}
	if e.Data != nil {
		eventMap["data"] = e.Data
	}
	
	// Add Flunq.io specific fields
	if e.WorkflowID != "" {
		eventMap["workflowid"] = e.WorkflowID
	}
	if e.ExecutionID != "" {
		eventMap["executionid"] = e.ExecutionID
	}
	if e.TaskID != "" {
		eventMap["taskid"] = e.TaskID
	}
	if e.CorrelationID != "" {
		eventMap["correlationid"] = e.CorrelationID
	}
	
	// Add extension attributes
	for key, value := range e.Extensions {
		eventMap[key] = value
	}
	
	return json.Marshal(eventMap)
}

// UnmarshalJSON implements custom JSON unmarshaling to handle extensions
func (e *CloudEvent) UnmarshalJSON(data []byte) error {
	var eventMap map[string]interface{}
	if err := json.Unmarshal(data, &eventMap); err != nil {
		return err
	}
	
	// Extract standard fields
	if id, ok := eventMap["id"].(string); ok {
		e.ID = id
	}
	if source, ok := eventMap["source"].(string); ok {
		e.Source = source
	}
	if specVersion, ok := eventMap["specversion"].(string); ok {
		e.SpecVersion = specVersion
	}
	if eventType, ok := eventMap["type"].(string); ok {
		e.Type = eventType
	}
	
	// Extract optional fields
	if dataContentType, ok := eventMap["datacontenttype"].(string); ok {
		e.DataContentType = dataContentType
	}
	if dataSchema, ok := eventMap["dataschema"].(string); ok {
		e.DataSchema = dataSchema
	}
	if subject, ok := eventMap["subject"].(string); ok {
		e.Subject = subject
	}
	if timeStr, ok := eventMap["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			e.Time = t
		}
	}
	if data, ok := eventMap["data"]; ok {
		e.Data = data
	}
	
	// Extract Flunq.io specific fields
	if workflowID, ok := eventMap["workflowid"].(string); ok {
		e.WorkflowID = workflowID
	}
	if executionID, ok := eventMap["executionid"].(string); ok {
		e.ExecutionID = executionID
	}
	if taskID, ok := eventMap["taskid"].(string); ok {
		e.TaskID = taskID
	}
	if correlationID, ok := eventMap["correlationid"].(string); ok {
		e.CorrelationID = correlationID
	}
	
	// Extract extension attributes
	e.Extensions = make(map[string]interface{})
	standardFields := map[string]bool{
		"id": true, "source": true, "specversion": true, "type": true,
		"datacontenttype": true, "dataschema": true, "subject": true, "time": true, "data": true,
		"workflowid": true, "executionid": true, "taskid": true, "correlationid": true,
	}
	
	for key, value := range eventMap {
		if !standardFields[key] {
			e.Extensions[key] = value
		}
	}
	
	return nil
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
		Extensions:      make(map[string]interface{}),
	}
	
	// Deep copy extensions
	for key, value := range e.Extensions {
		clone.Extensions[key] = value
	}
	
	return clone
}
