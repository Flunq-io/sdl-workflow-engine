package engine

import (
	"testing"

	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

// mockLogger implements the interfaces.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...interface{}) {}
func (m *mockLogger) Info(msg string, fields ...interface{})  {}
func (m *mockLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockLogger) Error(msg string, fields ...interface{}) {}
func (m *mockLogger) Fatal(msg string, fields ...interface{}) {}
func (m *mockLogger) With(fields ...interface{}) interfaces.Logger {
	return m
}

func TestBuildCallTaskConfig(t *testing.T) {
	logger := &mockLogger{}
	engine := &ServerlessWorkflowEngine{
		logger: logger,
	}

	tests := []struct {
		name       string
		taskDefMap map[string]interface{}
		callType   interface{}
		taskName   string
		expected   map[string]interface{}
	}{
		{
			name: "OpenAPI call task",
			taskDefMap: map[string]interface{}{
				"call": "openapi",
				"with": map[string]interface{}{
					"document": map[string]interface{}{
						"endpoint": "https://petstore.swagger.io/v2/swagger.json",
					},
					"operationId": "findPetsByStatus",
					"parameters": map[string]interface{}{
						"status": "available",
						"limit":  10,
					},
				},
			},
			callType: "openapi",
			taskName: "findPet",
			expected: map[string]interface{}{
				"call_type": "openapi",
				"document": map[string]interface{}{
					"endpoint": "https://petstore.swagger.io/v2/swagger.json",
				},
				"operation_id": "findPetsByStatus", // Note: mapped from operationId to operation_id
				"parameters": map[string]interface{}{
					"status": "available",
					"limit":  10,
				},
			},
		},
		{
			name: "HTTP call task",
			taskDefMap: map[string]interface{}{
				"call": "http",
				"with": map[string]interface{}{
					"url":    "https://api.example.com/pets",
					"method": "GET",
					"headers": map[string]interface{}{
						"Authorization": "Bearer token",
					},
				},
			},
			callType: "http",
			taskName: "httpCall",
			expected: map[string]interface{}{
				"call_type": "http",
				"url":       "https://api.example.com/pets",
				"method":    "GET",
				"headers": map[string]interface{}{
					"Authorization": "Bearer token",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.buildCallTaskConfig(tt.taskDefMap, tt.callType, tt.taskName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTaskInputFromDSL_CallTask(t *testing.T) {
	logger := &mockLogger{}
	engine := &ServerlessWorkflowEngine{
		logger: logger,
	}

	// Create a mock workflow definition with OpenAPI call task
	dslData := map[string]interface{}{
		"document": map[string]interface{}{
			"dsl":         "1.0.0",
			"name":        "test-workflow",
			"version":     "1.0.0",
			"description": "Test workflow with OpenAPI call",
		},
		"do": []interface{}{
			map[string]interface{}{
				"findPet": map[string]interface{}{
					"call": "openapi",
					"with": map[string]interface{}{
						"document": map[string]interface{}{
							"endpoint": "https://petstore.swagger.io/v2/swagger.json",
						},
						"operationId": "findPetsByStatus",
						"parameters": map[string]interface{}{
							"status": "available",
						},
					},
				},
			},
		},
	}

	dslStruct, err := structpb.NewStruct(dslData)
	require.NoError(t, err)

	definition := &gen.WorkflowDefinition{
		DslDefinition: dslStruct,
	}

	state := &gen.WorkflowState{
		WorkflowId: "test-workflow-123",
	}

	// Test building task input for the call task
	taskInput := engine.buildTaskInputFromDSL(definition, "findPet", state)

	// Verify basic task information
	assert.Equal(t, "findPet", taskInput["task_name"])
	assert.Equal(t, "test-workflow-123", taskInput["workflow_id"])
	assert.Contains(t, taskInput, "timestamp")

	// Verify call configuration is extracted
	require.Contains(t, taskInput, "call_config")
	callConfig, ok := taskInput["call_config"].(map[string]interface{})
	require.True(t, ok)

	// Verify call configuration content
	assert.Equal(t, "openapi", callConfig["call_type"])
	assert.Equal(t, "findPetsByStatus", callConfig["operation_id"]) // Note: mapped from operationId

	// Verify document configuration
	require.Contains(t, callConfig, "document")
	document, ok := callConfig["document"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https://petstore.swagger.io/v2/swagger.json", document["endpoint"])

	// Verify parameters
	require.Contains(t, callConfig, "parameters")
	parameters, ok := callConfig["parameters"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "available", parameters["status"])
}

func TestGetTaskTypeFromDSL_CallTask(t *testing.T) {
	logger := &mockLogger{}
	engine := &ServerlessWorkflowEngine{
		logger: logger,
	}

	// Create a mock workflow definition with call task
	dslData := map[string]interface{}{
		"do": []interface{}{
			map[string]interface{}{
				"findPet": map[string]interface{}{
					"call": "openapi",
					"with": map[string]interface{}{
						"operationId": "findPetsByStatus",
					},
				},
			},
		},
	}

	dslStruct, err := structpb.NewStruct(dslData)
	require.NoError(t, err)

	definition := &gen.WorkflowDefinition{
		DslDefinition: dslStruct,
	}

	// Test task type identification
	taskType := engine.getTaskTypeFromDSL(definition, "findPet")
	assert.Equal(t, "call", taskType)
}

func TestGetMapKeys(t *testing.T) {
	testMap := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	keys := getMapKeys(testMap)
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
}
