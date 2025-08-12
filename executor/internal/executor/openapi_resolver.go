package executor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// OperationResolver interface for resolving OpenAPI operations
type OperationResolver interface {
	ResolveOperation(doc *OpenAPIDocument, operationID string) (*ResolvedOperation, error)
	BuildRequestURL(operation *ResolvedOperation, parameters map[string]interface{}) (string, error)
	ExtractParameters(operation *ResolvedOperation, parameters map[string]interface{}) (*RequestParameters, error)
}

// ResolvedOperation represents a resolved OpenAPI operation with metadata
type ResolvedOperation struct {
	Operation   *OpenAPIOperation
	Method      string
	Path        string
	BaseURL     string
	FullPath    string // BaseURL + Path
}

// RequestParameters represents extracted and categorized parameters
type RequestParameters struct {
	Path   map[string]string
	Query  map[string]string
	Header map[string]string
	Body   interface{}
}

// DefaultOperationResolver implements OperationResolver
type DefaultOperationResolver struct {
	logger *zap.Logger
}

// NewOperationResolver creates a new operation resolver
func NewOperationResolver(logger *zap.Logger) OperationResolver {
	return &DefaultOperationResolver{
		logger: logger,
	}
}

// ResolveOperation finds an operation by operationId and resolves its metadata
func (r *DefaultOperationResolver) ResolveOperation(doc *OpenAPIDocument, operationID string) (*ResolvedOperation, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	if operationID == "" {
		return nil, fmt.Errorf("operationId is required")
	}

	// Search through all paths and methods to find the operation
	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem {
			if operation.OperationID == operationID {
				// Found the operation
				baseURL := r.getBaseURL(doc)
				
				resolved := &ResolvedOperation{
					Operation: &operation,
					Method:    strings.ToUpper(method),
					Path:      path,
					BaseURL:   baseURL,
					FullPath:  baseURL + path,
				}

				r.logger.Debug("Resolved OpenAPI operation",
					zap.String("operation_id", operationID),
					zap.String("method", resolved.Method),
					zap.String("path", resolved.Path),
					zap.String("base_url", resolved.BaseURL))

				return resolved, nil
			}
		}
	}

	return nil, fmt.Errorf("operation with id '%s' not found", operationID)
}

// getBaseURL extracts the base URL from the OpenAPI document
func (r *DefaultOperationResolver) getBaseURL(doc *OpenAPIDocument) string {
	// Use the first server if available
	if len(doc.Servers) > 0 {
		serverURL := doc.Servers[0].URL
		// Remove trailing slash
		return strings.TrimSuffix(serverURL, "/")
	}

	// Fallback: no base URL (relative paths)
	return ""
}

// BuildRequestURL builds the complete request URL with path parameters substituted
func (r *DefaultOperationResolver) BuildRequestURL(operation *ResolvedOperation, parameters map[string]interface{}) (string, error) {
	if operation == nil {
		return "", fmt.Errorf("operation is nil")
	}

	// Start with the full path
	requestURL := operation.FullPath

	// Extract path parameters
	pathParams, err := r.extractPathParameters(operation, parameters)
	if err != nil {
		return "", fmt.Errorf("failed to extract path parameters: %w", err)
	}

	// Substitute path parameters
	for paramName, paramValue := range pathParams {
		placeholder := "{" + paramName + "}"
		requestURL = strings.ReplaceAll(requestURL, placeholder, paramValue)
	}

	// Check if all path parameters were substituted
	if strings.Contains(requestURL, "{") && strings.Contains(requestURL, "}") {
		return "", fmt.Errorf("unresolved path parameters in URL: %s", requestURL)
	}

	// Extract query parameters
	queryParams, err := r.extractQueryParameters(operation, parameters)
	if err != nil {
		return "", fmt.Errorf("failed to extract query parameters: %w", err)
	}

	// Add query parameters
	if len(queryParams) > 0 {
		queryValues := url.Values{}
		for key, value := range queryParams {
			queryValues.Add(key, value)
		}
		
		if strings.Contains(requestURL, "?") {
			requestURL += "&" + queryValues.Encode()
		} else {
			requestURL += "?" + queryValues.Encode()
		}
	}

	return requestURL, nil
}

// ExtractParameters extracts and categorizes all parameters for the request
func (r *DefaultOperationResolver) ExtractParameters(operation *ResolvedOperation, parameters map[string]interface{}) (*RequestParameters, error) {
	if operation == nil {
		return nil, fmt.Errorf("operation is nil")
	}

	result := &RequestParameters{
		Path:   make(map[string]string),
		Query:  make(map[string]string),
		Header: make(map[string]string),
	}

	// Process each parameter defined in the operation
	for _, param := range operation.Operation.Parameters {
		paramValue, exists := parameters[param.Name]
		if !exists {
			if param.Required {
				return nil, fmt.Errorf("required parameter '%s' is missing", param.Name)
			}
			continue
		}

		// Convert parameter value to string
		paramStr := fmt.Sprintf("%v", paramValue)

		// Categorize parameter by location
		switch param.In {
		case "path":
			result.Path[param.Name] = paramStr
		case "query":
			result.Query[param.Name] = paramStr
		case "header":
			result.Header[param.Name] = paramStr
		case "cookie":
			// Cookie parameters are not supported yet
			r.logger.Warn("Cookie parameters are not supported yet", zap.String("param", param.Name))
		default:
			r.logger.Warn("Unknown parameter location", zap.String("param", param.Name), zap.String("location", param.In))
		}
	}

	// Handle request body if present
	if operation.Operation.RequestBody != nil {
		// For now, we'll look for a 'body' parameter in the input
		if bodyValue, exists := parameters["body"]; exists {
			result.Body = bodyValue
		}
	}

	return result, nil
}

// extractPathParameters extracts path parameters from the input parameters
func (r *DefaultOperationResolver) extractPathParameters(operation *ResolvedOperation, parameters map[string]interface{}) (map[string]string, error) {
	pathParams := make(map[string]string)

	for _, param := range operation.Operation.Parameters {
		if param.In == "path" {
			paramValue, exists := parameters[param.Name]
			if !exists {
				if param.Required {
					return nil, fmt.Errorf("required path parameter '%s' is missing", param.Name)
				}
				continue
			}

			pathParams[param.Name] = fmt.Sprintf("%v", paramValue)
		}
	}

	return pathParams, nil
}

// extractQueryParameters extracts query parameters from the input parameters
func (r *DefaultOperationResolver) extractQueryParameters(operation *ResolvedOperation, parameters map[string]interface{}) (map[string]string, error) {
	queryParams := make(map[string]string)

	for _, param := range operation.Operation.Parameters {
		if param.In == "query" {
			paramValue, exists := parameters[param.Name]
			if !exists {
				if param.Required {
					return nil, fmt.Errorf("required query parameter '%s' is missing", param.Name)
				}
				continue
			}

			queryParams[param.Name] = fmt.Sprintf("%v", paramValue)
		}
	}

	return queryParams, nil
}

// ValidatePathParameters validates that all required path parameters are present
func (r *DefaultOperationResolver) ValidatePathParameters(operation *ResolvedOperation, parameters map[string]interface{}) error {
	// Extract path parameter names from the path template
	pathParamRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := pathParamRegex.FindAllStringSubmatch(operation.Path, -1)

	for _, match := range matches {
		paramName := match[1]
		if _, exists := parameters[paramName]; !exists {
			return fmt.Errorf("required path parameter '%s' is missing", paramName)
		}
	}

	return nil
}
