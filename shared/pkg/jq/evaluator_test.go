package jq

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEvaluator_EvaluateExpression(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	evaluator := NewEvaluator(logger)

	tests := []struct {
		name       string
		expression string
		data       map[string]interface{}
		expected   interface{}
		expectErr  bool
	}{
		{
			name:       "static string",
			expression: "hello world",
			data:       map[string]interface{}{},
			expected:   "hello world",
			expectErr:  false,
		},
		{
			name:       "now() function",
			expression: "${ now() }",
			data:       map[string]interface{}{},
			expected:   nil, // We'll check this is a valid timestamp
			expectErr:  false,
		},
		{
			name:       "workflow input access",
			expression: "${ $workflow.input.user_id }",
			data: map[string]interface{}{
				"workflow": map[string]interface{}{
					"input": map[string]interface{}{
						"user_id": "test-user-456",
					},
				},
			},
			expected:  "test-user-456",
			expectErr: false,
		},
		{
			name:       "workflow input missing field",
			expression: "${ $workflow.input.missing_field }",
			data: map[string]interface{}{
				"workflow": map[string]interface{}{
					"input": map[string]interface{}{
						"user_id": "test-user-456",
					},
				},
			},
			expected:  nil,
			expectErr: true,
		},
		{
			name:       "empty expression",
			expression: "",
			data:       map[string]interface{}{},
			expected:   nil,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tt.expression, tt.data)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Special handling for now() function
			if tt.expression == "${ now() }" {
				if result == nil {
					t.Errorf("expected timestamp but got nil")
					return
				}
				// Verify it's a valid RFC3339 timestamp
				if _, err := time.Parse(time.RFC3339, result.(string)); err != nil {
					t.Errorf("expected valid RFC3339 timestamp but got: %v", result)
				}
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluator_EvaluateMap(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	evaluator := NewEvaluator(logger)

	input := map[string]interface{}{
		"message":   "Workflow started with DSL 1.0.0",
		"timestamp": "${ now() }",
		"user_id":   "${ $workflow.input.user_id }",
		"status":    "initialized",
	}

	data := map[string]interface{}{
		"workflow": map[string]interface{}{
			"input": map[string]interface{}{
				"user_id": "test-user-456",
			},
		},
	}

	result, err := evaluator.EvaluateMap(input, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check static values
	if result["message"] != "Workflow started with DSL 1.0.0" {
		t.Errorf("expected message to be 'Workflow started with DSL 1.0.0' but got %v", result["message"])
	}

	if result["status"] != "initialized" {
		t.Errorf("expected status to be 'initialized' but got %v", result["status"])
	}

	// Check evaluated values
	if result["user_id"] != "test-user-456" {
		t.Errorf("expected user_id to be 'test-user-456' but got %v", result["user_id"])
	}

	// Check timestamp is valid
	if timestamp, ok := result["timestamp"].(string); ok {
		if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
			t.Errorf("expected valid RFC3339 timestamp but got: %v", timestamp)
		}
	} else {
		t.Errorf("expected timestamp to be a string but got %T", result["timestamp"])
	}
}

func TestEvaluator_BuildWorkflowContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	evaluator := NewEvaluator(logger)

	workflowInput := map[string]interface{}{
		"user_id": "test-user-456",
		"message": "Hello from Multi-Tenant Postman!",
	}

	workflowVariables := map[string]interface{}{
		"step": 1,
		"data": "some data",
	}

	context := evaluator.BuildWorkflowContext(workflowInput, workflowVariables)

	// Check workflow input
	if workflow, ok := context["workflow"].(map[string]interface{}); ok {
		if input, ok := workflow["input"].(map[string]interface{}); ok {
			if input["user_id"] != "test-user-456" {
				t.Errorf("expected user_id to be 'test-user-456' but got %v", input["user_id"])
			}
		} else {
			t.Errorf("expected workflow.input to be a map")
		}

		if variables, ok := workflow["variables"].(map[string]interface{}); ok {
			if variables["step"] != 1 {
				t.Errorf("expected step to be 1 but got %v", variables["step"])
			}
		} else {
			t.Errorf("expected workflow.variables to be a map")
		}
	} else {
		t.Errorf("expected workflow to be a map")
	}

	// Check timestamp
	if now, ok := context["now"].(string); ok {
		if _, err := time.Parse(time.RFC3339, now); err != nil {
			t.Errorf("expected valid RFC3339 timestamp but got: %v", now)
		}
	} else {
		t.Errorf("expected now to be a string but got %T", context["now"])
	}
}
