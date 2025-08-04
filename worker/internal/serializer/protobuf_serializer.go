package serializer

import (
	"encoding/json"
	"fmt"

	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// ProtobufSerializer implements the ProtobufSerializer interface
type ProtobufSerializer struct{}

// NewProtobufSerializer creates a new protobuf serializer
func NewProtobufSerializer() interfaces.ProtobufSerializer {
	return &ProtobufSerializer{}
}

// SerializeTaskData serializes task data to JSON bytes (temporary implementation)
func (s *ProtobufSerializer) SerializeTaskData(data *gen.TaskData) ([]byte, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize task data: %w", err)
	}
	return bytes, nil
}

// DeserializeTaskData deserializes task data from JSON bytes (temporary implementation)
func (s *ProtobufSerializer) DeserializeTaskData(data []byte) (*gen.TaskData, error) {
	var taskData gen.TaskData
	if err := json.Unmarshal(data, &taskData); err != nil {
		return nil, fmt.Errorf("failed to deserialize task data: %w", err)
	}
	return &taskData, nil
}

// SerializeWorkflowState serializes workflow state to JSON bytes (temporary implementation)
func (s *ProtobufSerializer) SerializeWorkflowState(state *gen.WorkflowState) ([]byte, error) {
	bytes, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize workflow state: %w", err)
	}
	return bytes, nil
}

// DeserializeWorkflowState deserializes workflow state from JSON bytes (temporary implementation)
func (s *ProtobufSerializer) DeserializeWorkflowState(data []byte) (*gen.WorkflowState, error) {
	var state gen.WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to deserialize workflow state: %w", err)
	}
	return &state, nil
}

// SerializeTaskRequestedEvent serializes task requested event to JSON bytes (temporary implementation)
func (s *ProtobufSerializer) SerializeTaskRequestedEvent(event *gen.TaskRequestedEvent) ([]byte, error) {
	bytes, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize task requested event: %w", err)
	}
	return bytes, nil
}

// DeserializeTaskRequestedEvent deserializes task requested event from JSON bytes (temporary implementation)
func (s *ProtobufSerializer) DeserializeTaskRequestedEvent(data []byte) (*gen.TaskRequestedEvent, error) {
	var event gen.TaskRequestedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to deserialize task requested event: %w", err)
	}
	return &event, nil
}

// SerializeTaskCompletedEvent serializes task completed event to JSON bytes (temporary implementation)
func (s *ProtobufSerializer) SerializeTaskCompletedEvent(event *gen.TaskCompletedEvent) ([]byte, error) {
	bytes, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize task completed event: %w", err)
	}
	return bytes, nil
}

// DeserializeTaskCompletedEvent deserializes task completed event from JSON bytes (temporary implementation)
func (s *ProtobufSerializer) DeserializeTaskCompletedEvent(data []byte) (*gen.TaskCompletedEvent, error) {
	var event gen.TaskCompletedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to deserialize task completed event: %w", err)
	}
	return &event, nil
}
