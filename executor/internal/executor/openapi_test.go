package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Mock OpenAPI document for testing
const mockOpenAPIDoc = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Pet Store API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "https://petstore.example.com/v1"
    }
  ],
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "A list of pets"
          }
        }
      }
    },
    "/pets/{petId}": {
      "get": {
        "operationId": "getPet",
        "parameters": [
          {
            "name": "petId",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "A pet"
          }
        }
      }
    }
  }
}`

func TestParseCallConfig(t *testing.T) {
	tests := []struct {
		name        string
		parameters  map[string]interface{}
		expectError bool
		expectType  string
	}{
		{
			name: "OpenAPI call configuration",
			parameters: map[string]interface{}{
				"call_type":    "openapi",
				"document":     map[string]interface{}{"endpoint": "https://example.com/openapi.json"},
				"operation_id": "getPet",
				"parameters":   map[string]interface{}{"petId": "123"},
			},
			expectError: false,
			expectType:  "openapi",
		},
		{
			name: "HTTP call configuration",
			parameters: map[string]interface{}{
				"url":    "https://example.com/api",
				"method": "GET",
			},
			expectError: false,
			expectType:  "http",
		},
		{
			name: "Auto-detect OpenAPI",
			parameters: map[string]interface{}{
				"document":     map[string]interface{}{"endpoint": "https://example.com/openapi.json"},
				"operation_id": "getPet",
			},
			expectError: false,
			expectType:  "openapi",
		},
		{
			name: "Auto-detect HTTP",
			parameters: map[string]interface{}{
				"url": "https://example.com/api",
			},
			expectError: false,
			expectType:  "http",
		},
		{
			name: "Invalid configuration",
			parameters: map[string]interface{}{
				"invalid": "config",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseCallConfig(tt.parameters)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectType, config.CallType)
			assert.NoError(t, config.Validate())
		})
	}
}

func TestOpenAPIDocumentLoader(t *testing.T) {
	// Create a test server that serves the mock OpenAPI document
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAPIDoc))
	}))
	defer server.Close()

	logger := zap.NewNop()
	loader := NewOpenAPIDocumentLoader(logger)

	ctx := context.Background()
	resource := &ExternalResource{
		Endpoint: server.URL,
	}

	// Test loading document
	doc, err := loader.LoadDocument(ctx, resource)
	require.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "3.0.0", doc.OpenAPI)
	assert.Equal(t, "Pet Store API", doc.Info["title"])
	assert.Len(t, doc.Paths, 2)

	// Test caching - second call should use cache
	doc2, err := loader.LoadDocument(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, doc.OpenAPI, doc2.OpenAPI)
}

func TestOperationResolver(t *testing.T) {
	// Parse the mock document
	var doc OpenAPIDocument
	err := json.Unmarshal([]byte(mockOpenAPIDoc), &doc)
	require.NoError(t, err)

	logger := zap.NewNop()
	resolver := NewOperationResolver(logger)

	// Test resolving operation
	operation, err := resolver.ResolveOperation(&doc, "getPet")
	require.NoError(t, err)
	assert.NotNil(t, operation)
	assert.Equal(t, "GET", operation.Method)
	assert.Equal(t, "/pets/{petId}", operation.Path)
	assert.Equal(t, "https://petstore.example.com/v1", operation.BaseURL)

	// Test building request URL
	parameters := map[string]interface{}{
		"petId": "123",
	}

	url, err := resolver.BuildRequestURL(operation, parameters)
	require.NoError(t, err)
	assert.Equal(t, "https://petstore.example.com/v1/pets/123", url)

	// Test extracting parameters
	params, err := resolver.ExtractParameters(operation, parameters)
	require.NoError(t, err)
	assert.Equal(t, "123", params.Path["petId"])
}

func TestCallTaskExecutorOpenAPI(t *testing.T) {
	// Create test servers
	docServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAPIDoc))
	}))
	defer docServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"id":   "123",
			"name": "Fluffy",
			"type": "cat",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	// Update the mock document to use the test API server
	updatedDoc := `{
		"openapi": "3.0.0",
		"info": {"title": "Pet Store API", "version": "1.0.0"},
		"servers": [{"url": "` + apiServer.URL + `"}],
		"paths": {
			"/pets/{petId}": {
				"get": {
					"operationId": "getPet",
					"parameters": [
						{
							"name": "petId",
							"in": "path",
							"required": true,
							"schema": {"type": "string"}
						}
					],
					"responses": {
						"200": {"description": "A pet"}
					}
				}
			}
		}
	}`

	// Update the doc server to serve the updated document
	docServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(updatedDoc))
	})

	logger := zap.NewNop()
	executor := NewCallTaskExecutor(logger)

	// Create task request
	task := &TaskRequest{
		TaskID:      "test-task",
		TaskName:    "getPet",
		TaskType:    "call",
		WorkflowID:  "test-workflow",
		ExecutionID: "test-execution",
		Config: &TaskConfig{
			Parameters: map[string]interface{}{
				"call_type":    "openapi",
				"document":     map[string]interface{}{"endpoint": docServer.URL},
				"operation_id": "getPet",
				"parameters":   map[string]interface{}{"petId": "123"},
				"output":       "content",
			},
		},
	}

	// Execute the task
	ctx := context.Background()
	result, err := executor.Execute(ctx, task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 200, result.Output["status_code"])

	// Check that the response data is properly parsed
	data, ok := result.Output["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "123", data["id"])
	assert.Equal(t, "Fluffy", data["name"])
}

func TestCallTaskExecutorHTTP(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"message": "Hello, World!",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	executor := NewCallTaskExecutor(logger)

	// Create task request for HTTP call
	task := &TaskRequest{
		TaskID:      "test-task",
		TaskName:    "httpCall",
		TaskType:    "call",
		WorkflowID:  "test-workflow",
		ExecutionID: "test-execution",
		Config: &TaskConfig{
			Parameters: map[string]interface{}{
				"url":    server.URL,
				"method": "GET",
			},
		},
	}

	// Execute the task
	ctx := context.Background()
	result, err := executor.Execute(ctx, task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 200, result.Output["status_code"])

	// Check that the response data is properly parsed
	data, ok := result.Output["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", data["message"])
}

func TestSwagger2Support(t *testing.T) {
	// Create a test server that serves a Swagger 2.0 document
	swagger2Doc := `{
		"swagger": "2.0",
		"info": {"title": "Pet Store API", "version": "1.0.0"},
		"host": "petstore.swagger.io",
		"basePath": "/v2",
		"schemes": ["https", "http"],
		"paths": {
			"/pet/findByStatus": {
				"get": {
					"operationId": "findPetsByStatus",
					"parameters": [
						{
							"name": "status",
							"in": "query",
							"required": true,
							"type": "array",
							"items": {"type": "string"}
						}
					],
					"responses": {
						"200": {"description": "successful operation"}
					}
				}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(swagger2Doc))
	}))
	defer server.Close()

	logger := zap.NewNop()
	loader := NewOpenAPIDocumentLoader(logger)

	ctx := context.Background()
	resource := &ExternalResource{
		Endpoint: server.URL,
	}

	// Test loading Swagger 2.0 document
	doc, err := loader.LoadDocument(ctx, resource)
	require.NoError(t, err)
	assert.NotNil(t, doc)

	// Verify conversion to OpenAPI 3.0 format
	assert.Equal(t, "3.0.0", doc.OpenAPI) // Should be converted
	assert.Equal(t, "2.0", doc.Swagger)   // Original field preserved

	// Verify server conversion
	require.Len(t, doc.Servers, 2) // https and http
	assert.Equal(t, "https://petstore.swagger.io/v2", doc.Servers[0].URL)
	assert.Equal(t, "http://petstore.swagger.io/v2", doc.Servers[1].URL)

	// Verify paths are preserved
	assert.Len(t, doc.Paths, 1)
	assert.Contains(t, doc.Paths, "/pet/findByStatus")

	// Test operation resolution with converted document
	resolver := NewOperationResolver(logger)
	operation, err := resolver.ResolveOperation(doc, "findPetsByStatus")
	require.NoError(t, err)
	assert.NotNil(t, operation)
	assert.Equal(t, "GET", operation.Method)
	assert.Equal(t, "/pet/findByStatus", operation.Path)
	assert.Equal(t, "https://petstore.swagger.io/v2", operation.BaseURL)
}
