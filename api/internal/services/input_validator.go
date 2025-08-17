package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"go.uber.org/zap"
)

// InputValidator handles JSON Schema validation for workflow inputs
type InputValidator struct {
	logger *zap.Logger
}

// NewInputValidator creates a new input validator
func NewInputValidator(logger *zap.Logger) *InputValidator {
	return &InputValidator{
		logger: logger,
	}
}

// ValidationError represents a validation error with detailed information
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationResult represents the result of input validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidateInput validates input data against a JSON schema
func (v *InputValidator) ValidateInput(inputSchema map[string]interface{}, inputData map[string]interface{}) (*ValidationResult, error) {
	// If no schema is provided, accept any input
	if inputSchema == nil || len(inputSchema) == 0 {
		v.logger.Debug("No input schema provided, accepting any input")
		return &ValidationResult{Valid: true}, nil
	}

	// Convert schema to JSON string
	schemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		v.logger.Error("Failed to marshal input schema", zap.Error(err))
		return nil, fmt.Errorf("invalid input schema: %w", err)
	}

	// Convert input data to JSON string
	inputBytes, err := json.Marshal(inputData)
	if err != nil {
		v.logger.Error("Failed to marshal input data", zap.Error(err))
		return nil, fmt.Errorf("invalid input data: %w", err)
	}

	// Create schema and document loaders
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	documentLoader := gojsonschema.NewBytesLoader(inputBytes)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		v.logger.Error("JSON schema validation failed", zap.Error(err))
		return nil, fmt.Errorf("schema validation error: %w", err)
	}

	// Log detailed validation info
	v.logger.Info("JSON Schema validation result",
		zap.Bool("valid", result.Valid()),
		zap.Int("error_count", len(result.Errors())),
		zap.String("schema", string(schemaBytes)),
		zap.String("input", string(inputBytes)))

	// Build validation result
	validationResult := &ValidationResult{
		Valid: result.Valid(),
	}

	if !result.Valid() {
		validationResult.Errors = make([]ValidationError, 0, len(result.Errors()))

		for _, err := range result.Errors() {
			validationError := ValidationError{
				Field:   err.Field(),
				Message: err.Description(),
			}

			// Try to extract the actual value that failed validation
			if err.Value() != nil {
				validationError.Value = err.Value()
			}

			validationResult.Errors = append(validationResult.Errors, validationError)
		}

		v.logger.Info("Input validation failed",
			zap.Int("error_count", len(validationResult.Errors)),
			zap.Any("errors", validationResult.Errors))
	} else {
		v.logger.Debug("Input validation passed")
	}

	return validationResult, nil
}

// FormatValidationErrors formats validation errors into a user-friendly message
func (v *InputValidator) FormatValidationErrors(errors []ValidationError, schema map[string]interface{}) string {
	if len(errors) == 0 {
		return ""
	}

	var messages []string

	// Add a header message
	messages = append(messages, "Input validation failed:")

	// Add each validation error
	for _, err := range errors {
		var msg string
		if err.Field != "" {
			msg = fmt.Sprintf("  - Field '%s': %s", err.Field, err.Message)
		} else {
			msg = fmt.Sprintf("  - %s", err.Message)
		}

		if err.Value != nil {
			msg += fmt.Sprintf(" (received: %v)", err.Value)
		}

		messages = append(messages, msg)
	}

	// Add schema information if available
	if schema != nil {
		messages = append(messages, "")
		messages = append(messages, "Expected input schema:")

		// Try to extract schema properties for a helpful message
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			for fieldName, fieldSchema := range properties {
				if fieldMap, ok := fieldSchema.(map[string]interface{}); ok {
					fieldType := "any"
					if t, exists := fieldMap["type"]; exists {
						fieldType = fmt.Sprintf("%v", t)
					}

					description := ""
					if desc, exists := fieldMap["description"]; exists {
						description = fmt.Sprintf(" - %v", desc)
					}

					required := ""
					if requiredFields, ok := schema["required"].([]interface{}); ok {
						for _, req := range requiredFields {
							if req == fieldName {
								required = " (required)"
								break
							}
						}
					}

					messages = append(messages, fmt.Sprintf("  - %s: %s%s%s", fieldName, fieldType, required, description))
				}
			}
		}
	}

	return strings.Join(messages, "\n")
}

// ValidateSchema validates that the provided schema is a valid JSON Schema
func (v *InputValidator) ValidateSchema(schema map[string]interface{}) error {
	if schema == nil {
		return nil // No schema is valid (means accept any input)
	}

	// Convert to JSON and back to validate structure
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("invalid schema format: %w", err)
	}

	// Try to create a schema loader to validate the schema itself
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)

	// Test with empty object to see if schema is valid
	testData := map[string]interface{}{}
	testBytes, _ := json.Marshal(testData)
	documentLoader := gojsonschema.NewBytesLoader(testBytes)

	_, err = gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	v.logger.Debug("Input schema validation passed")
	return nil
}
