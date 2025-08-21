package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestParseDocument_YAML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewOpenAPIDocumentLoader(logger).(*DefaultOpenAPIDocumentLoader)

	// Test YAML OpenAPI document
	yamlContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /pets:
    get:
      operationId: listPets
      summary: List all pets
      responses:
        '200':
          description: A list of pets
`

	doc, err := loader.parseDocument([]byte(yamlContent), "application/yaml")
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "3.0.0", doc.OpenAPI)
	assert.NotNil(t, doc.Info)
	assert.NotNil(t, doc.Paths)
	assert.Contains(t, doc.Paths, "/pets")
}

func TestParseDocument_JSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewOpenAPIDocumentLoader(logger).(*DefaultOpenAPIDocumentLoader)

	// Test JSON OpenAPI document
	jsonContent := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/pets": {
				"get": {
					"operationId": "listPets",
					"summary": "List all pets",
					"responses": {
						"200": {
							"description": "A list of pets"
						}
					}
				}
			}
		}
	}`

	doc, err := loader.parseDocument([]byte(jsonContent), "application/json")
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "3.0.0", doc.OpenAPI)
	assert.NotNil(t, doc.Info)
	assert.NotNil(t, doc.Paths)
	assert.Contains(t, doc.Paths, "/pets")
}

func TestIsYAMLContent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewOpenAPIDocumentLoader(logger).(*DefaultOpenAPIDocumentLoader)

	tests := []struct {
		name        string
		content     string
		contentType string
		expected    bool
	}{
		{
			name:        "YAML content type",
			content:     "openapi: 3.0.0",
			contentType: "application/yaml",
			expected:    true,
		},
		{
			name:        "JSON content type",
			content:     `{"openapi": "3.0.0"}`,
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "YAML content pattern",
			content:     "openapi: 3.0.0\ninfo:",
			contentType: "",
			expected:    true,
		},
		{
			name:        "JSON content pattern",
			content:     `{"openapi": "3.0.0"}`,
			contentType: "",
			expected:    false,
		},
		{
			name:        "YAML with dashes",
			content:     "---\nopenapi: 3.0.0",
			contentType: "",
			expected:    true,
		},
		{
			name:        "YAML with comments and whitespace",
			content:     "# OpenAPI specification\n\nopenapi: 3.0.0\ninfo:\n  title: Test API",
			contentType: "",
			expected:    true,
		},
		{
			name:        "YAML with key-value pattern",
			content:     "openapi: 3.0.0\ninfo:\n  title: My API\npaths:",
			contentType: "",
			expected:    true,
		},
		{
			name:        "JSON array",
			content:     `[{"openapi": "3.0.0"}]`,
			contentType: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.isYAMLContent([]byte(tt.content), tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
