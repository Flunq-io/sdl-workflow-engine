package executor

import (
	"encoding/json"
	"fmt"
)

// ExternalResource represents a reference to an external resource
type ExternalResource struct {
	Endpoint string `json:"endpoint"`
}

// OpenAPIDocument represents a parsed OpenAPI specification (supports both OpenAPI 3.x and Swagger 2.0)
type OpenAPIDocument struct {
	OpenAPI     string                                 `json:"openapi"` // OpenAPI 3.x version
	Swagger     string                                 `json:"swagger"` // Swagger 2.0 version
	Info        map[string]interface{}                 `json:"info"`
	Servers     []OpenAPIServer                        `json:"servers"`  // OpenAPI 3.x
	Host        string                                 `json:"host"`     // Swagger 2.0
	BasePath    string                                 `json:"basePath"` // Swagger 2.0
	Schemes     []string                               `json:"schemes"`  // Swagger 2.0
	Paths       map[string]map[string]OpenAPIOperation `json:"paths"`
	Components  map[string]interface{}                 `json:"components"`  // OpenAPI 3.x
	Definitions map[string]interface{}                 `json:"definitions"` // Swagger 2.0
}

// OpenAPIServer represents an OpenAPI server definition
type OpenAPIServer struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// OpenAPIOperation represents an OpenAPI operation
type OpenAPIOperation struct {
	OperationID string                         `json:"operationId"`
	Summary     string                         `json:"summary,omitempty"`
	Description string                         `json:"description,omitempty"`
	Parameters  []OpenAPIParameter             `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody            `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponseSpec `json:"responses,omitempty"`
	Security    []map[string][]string          `json:"security,omitempty"`
	Tags        []string                       `json:"tags,omitempty"`

	// Internal fields for resolved operation
	Method string `json:"-"` // HTTP method (GET, POST, etc.)
	Path   string `json:"-"` // URL path template
}

// OpenAPIParameter represents an OpenAPI parameter
type OpenAPIParameter struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"` // "query", "header", "path", "cookie"
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
	Style       string                 `json:"style,omitempty"`
	Explode     bool                   `json:"explode,omitempty"`
}

// OpenAPIRequestBody represents an OpenAPI request body
type OpenAPIRequestBody struct {
	Description string                      `json:"description,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
	Required    bool                        `json:"required,omitempty"`
}

// OpenAPIResponseSpec represents an OpenAPI response specification
type OpenAPIResponseSpec struct {
	Description string                      `json:"description"`
	Headers     map[string]OpenAPIHeader    `json:"headers,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

// OpenAPIHeader represents an OpenAPI header
type OpenAPIHeader struct {
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
}

// OpenAPIMediaType represents an OpenAPI media type
type OpenAPIMediaType struct {
	Schema   map[string]interface{} `json:"schema,omitempty"`
	Example  interface{}            `json:"example,omitempty"`
	Examples map[string]interface{} `json:"examples,omitempty"`
}

// Authentication represents authentication configuration
type Authentication struct {
	Type   string                 `json:"type"`   // "basic", "bearer", "apikey", "oauth2"
	Config map[string]interface{} `json:"config"` // Type-specific configuration
}

// CallConfig represents the enhanced call configuration
type CallConfig struct {
	// Existing HTTP call configuration (for backward compatibility)
	URL     string                 `json:"url,omitempty"`
	Method  string                 `json:"method,omitempty"`
	Headers map[string]interface{} `json:"headers,omitempty"`
	Body    interface{}            `json:"body,omitempty"`
	Query   map[string]interface{} `json:"query,omitempty"`

	// New OpenAPI call configuration
	CallType       string                 `json:"call_type,omitempty"`      // "http", "openapi", "grpc", "asyncapi"
	Document       *ExternalResource      `json:"document,omitempty"`       // OpenAPI document reference
	OperationID    string                 `json:"operation_id,omitempty"`   // OpenAPI operation ID
	Parameters     map[string]interface{} `json:"parameters,omitempty"`     // OpenAPI operation parameters
	Authentication *Authentication        `json:"authentication,omitempty"` // Authentication policy
	Output         string                 `json:"output,omitempty"`         // Output format: "content", "response", "raw"
	Redirect       bool                   `json:"redirect,omitempty"`       // Handle redirects (default: false)
}

// ParseCallConfig parses call configuration from task parameters
func ParseCallConfig(parameters map[string]interface{}) (*CallConfig, error) {
	config := &CallConfig{}

	// Convert map to JSON and back to struct for easy parsing
	jsonData, err := json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal call config: %w", err)
	}

	// Set defaults
	if config.CallType == "" {
		// Determine call type based on configuration
		if config.Document != nil && config.OperationID != "" {
			config.CallType = "openapi"
		} else if config.URL != "" {
			config.CallType = "http"
		} else {
			return nil, fmt.Errorf("unable to determine call type from configuration")
		}
	}

	if config.Output == "" {
		config.Output = "content" // Default output format
	}

	if config.Method == "" && config.CallType == "http" {
		config.Method = "GET" // Default HTTP method
	}

	return config, nil
}

// IsOpenAPICall returns true if this is an OpenAPI call
func (c *CallConfig) IsOpenAPICall() bool {
	return c.CallType == "openapi"
}

// IsHTTPCall returns true if this is a direct HTTP call
func (c *CallConfig) IsHTTPCall() bool {
	return c.CallType == "http" || c.CallType == ""
}

// Validate validates the call configuration
func (c *CallConfig) Validate() error {
	switch c.CallType {
	case "openapi":
		if c.Document == nil {
			return fmt.Errorf("document is required for OpenAPI calls")
		}
		if c.Document.Endpoint == "" {
			return fmt.Errorf("document endpoint is required for OpenAPI calls")
		}
		if c.OperationID == "" {
			return fmt.Errorf("operation_id is required for OpenAPI calls")
		}
	case "http", "":
		if c.URL == "" {
			return fmt.Errorf("url is required for HTTP calls")
		}
	default:
		return fmt.Errorf("unsupported call type: %s", c.CallType)
	}

	// Validate output format
	switch c.Output {
	case "content", "response", "raw":
		// Valid output formats
	default:
		return fmt.Errorf("unsupported output format: %s (supported: content, response, raw)", c.Output)
	}

	return nil
}
