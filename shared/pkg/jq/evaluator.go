package jq

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Evaluator handles JQ expression evaluation
type Evaluator struct {
	logger *zap.Logger
}

// NewEvaluator creates a new JQ evaluator
func NewEvaluator(logger *zap.Logger) *Evaluator {
	return &Evaluator{
		logger: logger,
	}
}

// EvaluateExpression evaluates a JQ expression against the provided data context
func (e *Evaluator) EvaluateExpression(expression string, data map[string]interface{}) (interface{}, error) {
	if expression == "" {
		return nil, fmt.Errorf("empty expression")
	}

	e.logger.Debug("Evaluating JQ expression",
		zap.String("expression", expression),
		zap.Any("data", data))

	// Check if this is a JQ expression (starts with ${ and ends with })
	if !strings.HasPrefix(expression, "${") || !strings.HasSuffix(expression, "}") {
		// Not a JQ expression, return as-is
		return expression, nil
	}

	// Extract the expression content
	jqPath := strings.TrimSpace(expression[2 : len(expression)-1])

	// Handle built-in functions
	result, err := e.evaluateBuiltinFunction(jqPath, data)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression '%s': %w", expression, err)
	}

	e.logger.Debug("JQ expression evaluated",
		zap.String("expression", expression),
		zap.Any("result", result))

	return result, nil
}

// evaluateBuiltinFunction handles built-in JQ functions and path expressions
func (e *Evaluator) evaluateBuiltinFunction(jqPath string, data map[string]interface{}) (interface{}, error) {
	// Handle now() function
	if jqPath == "now()" {
		return time.Now().Format(time.RFC3339), nil
	}

	// Handle $workflow.input.* expressions
	if strings.HasPrefix(jqPath, "$workflow.input.") {
		fieldPath := strings.TrimPrefix(jqPath, "$workflow.input.")
		return e.extractFromPath(data, "workflow.input."+fieldPath)
	}

	// Handle $workflow.* expressions
	if strings.HasPrefix(jqPath, "$workflow.") {
		fieldPath := strings.TrimPrefix(jqPath, "$workflow.")
		return e.extractFromPath(data, "workflow."+fieldPath)
	}

	// Handle $.* expressions (root data access)
	if strings.HasPrefix(jqPath, "$.") {
		fieldPath := strings.TrimPrefix(jqPath, "$.")
		return e.extractFromPath(data, fieldPath)
	}

	// Handle .* expressions (standard JQ syntax for current object)
	if strings.HasPrefix(jqPath, ".") {
		fieldPath := strings.TrimPrefix(jqPath, ".")
		return e.extractFromPath(data, fieldPath)
	}

	// Handle simple field access (no prefix)
	if !strings.Contains(jqPath, ".") && !strings.Contains(jqPath, "(") {
		if value, exists := data[jqPath]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("field '%s' not found in data", jqPath)
	}

	return nil, fmt.Errorf("unsupported JQ expression: %s", jqPath)
}

// extractFromPath extracts a value from nested data using dot notation
func (e *Evaluator) extractFromPath(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("path '%s' not found: nil at part '%s'", path, part)
		}

		// Handle array access [index]
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// TODO: Implement array access if needed
			return nil, fmt.Errorf("array access not yet implemented: %s", part)
		}

		// Navigate to the next level
		if value, exists := current[part]; exists {
			if i == len(parts)-1 {
				// Last part, return the value
				return value, nil
			}
			// Continue navigation
			if nextMap, ok := value.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil, fmt.Errorf("path '%s' not found: '%s' is not a map", path, part)
			}
		} else {
			return nil, fmt.Errorf("path '%s' not found: field '%s' does not exist", path, part)
		}
	}

	return nil, fmt.Errorf("path '%s' not found", path)
}

// EvaluateMap evaluates all JQ expressions in a map
func (e *Evaluator) EvaluateMap(input map[string]interface{}, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range input {
		evaluatedValue, err := e.evaluateValue(value, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate key '%s': %w", key, err)
		}
		result[key] = evaluatedValue
	}

	return result, nil
}

// evaluateValue evaluates a single value (string, map, slice, or primitive)
func (e *Evaluator) evaluateValue(value interface{}, data map[string]interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Evaluate string as potential JQ expression
		return e.EvaluateExpression(v, data)
	case map[string]interface{}:
		// Recursively evaluate map
		return e.EvaluateMap(v, data)
	case []interface{}:
		// Evaluate each item in slice
		result := make([]interface{}, len(v))
		for i, item := range v {
			evaluatedItem, err := e.evaluateValue(item, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate array item at index %d: %w", i, err)
			}
			result[i] = evaluatedItem
		}
		return result, nil
	default:
		// Return primitive values as-is
		return value, nil
	}
}

// BuildWorkflowContext builds the workflow context data for JQ evaluation
func (e *Evaluator) BuildWorkflowContext(workflowInput map[string]interface{}, workflowVariables map[string]interface{}) map[string]interface{} {
	context := make(map[string]interface{})

	// Add workflow data
	workflow := make(map[string]interface{})
	if workflowInput != nil {
		workflow["input"] = workflowInput
	}
	if workflowVariables != nil {
		workflow["variables"] = workflowVariables
	}
	context["workflow"] = workflow

	// Add current timestamp
	context["now"] = time.Now().Format(time.RFC3339)

	return context
}
