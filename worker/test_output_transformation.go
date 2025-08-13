package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func main() {
	// Create mock task output (simulating the API response)
	taskOutput := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"id":   1,
				"name": "Buddy",
				"category": map[string]interface{}{
					"id":   1,
					"name": "Dog",
				},
			},
			map[string]interface{}{
				"id":   2,
				"name": "Fluffy",
				"category": map[string]interface{}{
					"id":   2,
					"name": "Cat",
				},
			},
		},
		"status_code": 200,
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
	}

	fmt.Println("=== Testing Output Transformation ===")
	fmt.Println("Input task output:")
	outputJSON, _ := json.MarshalIndent(taskOutput, "", "  ")
	fmt.Println(string(outputJSON))

	outputConfig := "${ .content[0].name }"
	fmt.Printf("\nOutput config: %s\n", outputConfig)

	// Test the transformation
	result, err := applySimpleFieldExtraction(outputConfig, taskOutput)
	if err != nil {
		fmt.Printf("❌ Transformation failed: %v\n", err)
		return
	}

	fmt.Println("\nTransformed result:")
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(resultJSON))

	// Check if we got the expected result
	if resultValue, exists := result["result"]; exists {
		if resultValue == "Buddy" {
			fmt.Println("\n✅ SUCCESS: Transformation worked correctly!")
		} else {
			fmt.Printf("\n❌ FAIL: Expected 'Buddy', got '%v'\n", resultValue)
		}
	} else {
		fmt.Println("\n❌ FAIL: No 'result' field in transformed output")
	}
}

// Copy of the transformation logic from the worker
func applySimpleFieldExtraction(outputConfig interface{}, taskOutput map[string]interface{}) (map[string]interface{}, error) {
	// Handle string expressions like "${ .content }"
	if expr, ok := outputConfig.(string); ok {
		// Remove ${ } wrapper if present
		if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
			expr = strings.TrimSpace(expr[2 : len(expr)-1])
		}

		// Handle field access like ".content", ".content[0]", ".content[0].name"
		if strings.HasPrefix(expr, ".") {
			path := strings.TrimPrefix(expr, ".")
			value, err := extractValueFromPath(taskOutput, path)
			if err != nil {
				return nil, fmt.Errorf("failed to extract path '%s': %w", path, err)
			}
			return map[string]interface{}{
				"result": value,
			}, nil
		}
	}

	return nil, fmt.Errorf("unsupported output configuration type: %T", outputConfig)
}

func extractValueFromPath(data map[string]interface{}, path string) (interface{}, error) {
	parts := splitPathWithArrays(path)
	current := interface{}(data)

	for i, part := range parts {
		fmt.Printf("Processing path part: %s (index: %d, current_type: %T)\n", part, i, current)

		// Handle array indexing like "content[0]"
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			fieldName := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]

			// Get the field first
			if currentMap, ok := current.(map[string]interface{}); ok {
				if fieldValue, exists := currentMap[fieldName]; exists {
					// Now handle array indexing
					if arrayValue, ok := fieldValue.([]interface{}); ok {
						index := 0
						if indexStr != "" {
							if idx, err := strconv.Atoi(indexStr); err == nil {
								index = idx
							} else {
								return nil, fmt.Errorf("invalid array index '%s'", indexStr)
							}
						}

						if index < 0 || index >= len(arrayValue) {
							return nil, fmt.Errorf("array index %d out of bounds (length: %d)", index, len(arrayValue))
						}

						current = arrayValue[index]
					} else {
						return nil, fmt.Errorf("field '%s' is not an array", fieldName)
					}
				} else {
					return nil, fmt.Errorf("field '%s' not found", fieldName)
				}
			} else {
				return nil, fmt.Errorf("cannot access field '%s' on non-object", fieldName)
			}
		} else {
			// Handle regular field access
			if currentMap, ok := current.(map[string]interface{}); ok {
				if value, exists := currentMap[part]; exists {
					current = value
				} else {
					return nil, fmt.Errorf("field '%s' not found", part)
				}
			} else {
				return nil, fmt.Errorf("cannot access field '%s' on non-object", part)
			}
		}
	}

	return current, nil
}

func splitPathWithArrays(path string) []string {
	var parts []string
	current := ""
	inBrackets := false

	for _, char := range path {
		switch char {
		case '[':
			inBrackets = true
			current += string(char)
		case ']':
			inBrackets = false
			current += string(char)
		case '.':
			if inBrackets {
				current += string(char)
			} else {
				if current != "" {
					parts = append(parts, current)
					current = ""
				}
			}
		default:
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
