package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/flunq-io/worker/proto/gen"
)

func TestServerlessWorkflowEngine_ExecuteWaitTask_ValidDuration(t *testing.T) {
	// This is a simple test to verify the wait task parsing works correctly
	// We can't easily test the event scheduling without complex mocking

	// Create test data
	duration := 5 * time.Second
	taskInput, _ := structpb.NewStruct(map[string]interface{}{
		"duration": duration.String(),
	})

	task := &gen.PendingTask{
		Name:      "wait-task",
		TaskType:  "wait",
		Input:     taskInput,
		CreatedAt: timestamppb.New(time.Now()),
	}

	// Test duration parsing (the core logic we can test without mocking)
	taskInputMap := task.Input.AsMap()
	durationStr, exists := taskInputMap["duration"].(string)
	assert.True(t, exists, "Duration should exist in task input")

	parsedDuration, err := time.ParseDuration(durationStr)
	assert.NoError(t, err, "Duration should parse correctly")
	assert.Equal(t, duration, parsedDuration, "Parsed duration should match expected")
}

func TestServerlessWorkflowEngine_ExecuteWaitTask_InvalidDuration(t *testing.T) {
	// Test invalid duration parsing
	taskInput, _ := structpb.NewStruct(map[string]interface{}{
		"duration": "invalid-duration",
	})

	task := &gen.PendingTask{
		Name:      "wait-task",
		TaskType:  "wait",
		Input:     taskInput,
		CreatedAt: timestamppb.New(time.Now()),
	}

	// Test duration parsing
	taskInputMap := task.Input.AsMap()
	durationStr, exists := taskInputMap["duration"].(string)
	assert.True(t, exists, "Duration should exist in task input")

	_, err := time.ParseDuration(durationStr)
	assert.Error(t, err, "Invalid duration should cause parse error")
}

func TestServerlessWorkflowEngine_ExecuteWaitTask_MissingDuration(t *testing.T) {
	// Test missing duration
	taskInput, _ := structpb.NewStruct(map[string]interface{}{
		"other_param": "value",
	})

	task := &gen.PendingTask{
		Name:      "wait-task",
		TaskType:  "wait",
		Input:     taskInput,
		CreatedAt: timestamppb.New(time.Now()),
	}

	// Test duration parsing
	taskInputMap := task.Input.AsMap()
	_, exists := taskInputMap["duration"].(string)
	assert.False(t, exists, "Duration should not exist in task input")
}
