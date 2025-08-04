package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	// This is the exact JSON data from Redis (from the task.requested event)
	jsonData := `{
		"data": {
			"config": {
				"parameters": {},
				"retries": 3,
				"timeout": 300
			},
			"context": {},
			"execution_id": "exec-12345",
			"input": {},
			"task_id": "task-start_task-1754317741336380000",
			"task_name": "start_task",
			"task_type": "set",
			"workflow_id": "simple-test-workflow"
		},
		"executionid": "exec-12345",
		"id": "task-requested-task-start_task-1754317741336380000",
		"source": "worker-service",
		"specversion": "1.0",
		"taskid": "task-start_task-1754317741336380000",
		"time": "2025-08-04T16:29:01+02:00",
		"type": "io.flunq.task.requested",
		"workflowid": "simple-test-workflow"
	}`

	fmt.Println("=== Testing JSON Parsing ===")
	fmt.Println("Raw JSON:", jsonData)
	fmt.Println()

	// Parse the outer JSON
	var outerData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &outerData); err != nil {
		log.Fatal("Failed to parse outer JSON:", err)
	}

	fmt.Println("=== Outer Data Keys ===")
	for key := range outerData {
		fmt.Printf("- %s\n", key)
	}
	fmt.Println()

	// Extract the inner data
	innerData, ok := outerData["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No 'data' field found or wrong type")
	}

	fmt.Println("=== Inner Data Keys ===")
	for key, value := range innerData {
		fmt.Printf("- %s: %v (type: %T)\n", key, value, value)
	}
	fmt.Println()

	// Test the task parsing logic manually
	fmt.Println("=== Manual Task Parsing ===")

	// This simulates what ParseTaskRequestFromEvent does
	taskID := getStringFromData(innerData, "task_id")
	taskName := getStringFromData(innerData, "task_name")
	taskType := getStringFromData(innerData, "task_type")

	fmt.Printf("TaskID: '%s'\n", taskID)
	fmt.Printf("TaskName: '%s'\n", taskName)
	fmt.Printf("TaskType: '%s'\n", taskType)

	if taskType == "" {
		fmt.Println("\n❌ TASK TYPE IS EMPTY!")
		fmt.Println("Checking task_type field directly:")
		if val, exists := innerData["task_type"]; exists {
			fmt.Printf("task_type exists: %v (type: %T)\n", val, val)
		} else {
			fmt.Println("task_type field does not exist!")
		}
	} else {
		fmt.Println("\n✅ TASK TYPE IS CORRECT!")
	}
}

// getStringFromData extracts a string value from map data
func getStringFromData(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}
