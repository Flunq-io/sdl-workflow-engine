package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// OpenAPIClient handles OpenAPI operation execution
type OpenAPIClient interface {
	ExecuteOperation(ctx context.Context, config *CallConfig) (*OpenAPIResponse, error)
}

// OpenAPIResponse represents the response from an OpenAPI operation
type OpenAPIResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	Content    interface{}         `json:"content,omitempty"`
	Request    *OpenAPIRequestInfo `json:"request"`
}

// OpenAPIRequestInfo contains information about the executed request
type OpenAPIRequestInfo struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        interface{}       `json:"body,omitempty"`
	OperationID string            `json:"operation_id"`
}

// DefaultOpenAPIClient implements OpenAPIClient
type DefaultOpenAPIClient struct {
	logger            *zap.Logger
	httpClient        *http.Client
	documentLoader    OpenAPIDocumentLoader
	operationResolver OperationResolver
}

// NewOpenAPIClient creates a new OpenAPI client
func NewOpenAPIClient(logger *zap.Logger) OpenAPIClient {
	return &DefaultOpenAPIClient{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		documentLoader:    NewOpenAPIDocumentLoader(logger),
		operationResolver: NewOperationResolver(logger),
	}
}

// ExecuteOperation executes an OpenAPI operation based on the provided configuration
func (c *DefaultOpenAPIClient) ExecuteOperation(ctx context.Context, config *CallConfig) (*OpenAPIResponse, error) {
	startTime := time.Now()

	c.logger.Info("Executing OpenAPI operation",
		zap.String("operation_id", config.OperationID),
		zap.String("document", config.Document.Endpoint))

	// Load OpenAPI document
	doc, err := c.documentLoader.LoadDocument(ctx, config.Document)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI document: %w", err)
	}

	// Resolve operation
	operation, err := c.operationResolver.ResolveOperation(doc, config.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve operation: %w", err)
	}

	// Extract parameters
	params, err := c.operationResolver.ExtractParameters(operation, config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to extract parameters: %w", err)
	}

	// Build request URL
	requestURL, err := c.operationResolver.BuildRequestURL(operation, config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to build request URL: %w", err)
	}

	// Build HTTP request
	req, err := c.buildHTTPRequest(ctx, operation, requestURL, params)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Apply authentication if configured
	if config.Authentication != nil {
		if err := c.applyAuthentication(req, config.Authentication); err != nil {
			return nil, fmt.Errorf("failed to apply authentication: %w", err)
		}
	}

	// Execute HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Build response
	response := &OpenAPIResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
		Request: &OpenAPIRequestInfo{
			Method:      operation.Method,
			URL:         requestURL,
			Headers:     c.extractRequestHeaders(req),
			Body:        params.Body,
			OperationID: config.OperationID,
		},
	}

	// Process response content based on output format
	if err := c.processResponseContent(response, config.Output); err != nil {
		c.logger.Warn("Failed to process response content", zap.Error(err))
		// Don't fail the request, just log the warning
	}

	// Check for HTTP errors (unless redirects are allowed)
	if !config.Redirect && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return response, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	if config.Redirect && (resp.StatusCode < 200 || resp.StatusCode >= 400) {
		return response, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	duration := time.Since(startTime)
	c.logger.Info("OpenAPI operation completed",
		zap.String("operation_id", config.OperationID),
		zap.String("method", operation.Method),
		zap.String("url", requestURL),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration))

	return response, nil
}

// buildHTTPRequest builds an HTTP request from the resolved operation and parameters
func (c *DefaultOpenAPIClient) buildHTTPRequest(ctx context.Context, operation *ResolvedOperation, url string, params *RequestParameters) (*http.Request, error) {
	var body io.Reader

	// Handle request body
	if params.Body != nil {
		bodyBytes, err := json.Marshal(params.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, operation.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", "flunq-executor/1.0")

	// Set content type for request body
	if params.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add header parameters
	for name, value := range params.Header {
		req.Header.Set(name, value)
	}

	return req, nil
}

// applyAuthentication applies authentication to the HTTP request
func (c *DefaultOpenAPIClient) applyAuthentication(req *http.Request, auth *Authentication) error {
	switch auth.Type {
	case "bearer":
		if token, ok := auth.Config["token"].(string); ok {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			return fmt.Errorf("bearer token is required for bearer authentication")
		}
	case "basic":
		username, hasUsername := auth.Config["username"].(string)
		password, hasPassword := auth.Config["password"].(string)
		if !hasUsername || !hasPassword {
			return fmt.Errorf("username and password are required for basic authentication")
		}
		req.SetBasicAuth(username, password)
	case "apikey":
		key, hasKey := auth.Config["key"].(string)
		value, hasValue := auth.Config["value"].(string)
		location, _ := auth.Config["in"].(string)

		if !hasKey || !hasValue {
			return fmt.Errorf("key and value are required for API key authentication")
		}

		switch location {
		case "header", "":
			req.Header.Set(key, value)
		case "query":
			q := req.URL.Query()
			q.Add(key, value)
			req.URL.RawQuery = q.Encode()
		default:
			return fmt.Errorf("unsupported API key location: %s", location)
		}
	default:
		return fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}

	return nil
}

// processResponseContent processes the response content based on the output format
func (c *DefaultOpenAPIClient) processResponseContent(response *OpenAPIResponse, outputFormat string) error {
	switch outputFormat {
	case "raw":
		// Raw format: keep body as base64 encoded bytes
		// For now, we'll keep it as bytes in the Body field
		return nil
	case "response":
		// Response format: include full HTTP response details
		// Content is already set to nil, which is correct for this format
		return nil
	case "content", "":
		// Content format: try to parse response body
		if len(response.Body) > 0 {
			var content interface{}
			if err := json.Unmarshal(response.Body, &content); err != nil {
				// If JSON parsing fails, store as string
				response.Content = string(response.Body)
			} else {
				response.Content = content
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

// extractRequestHeaders extracts headers from the HTTP request for logging
func (c *DefaultOpenAPIClient) extractRequestHeaders(req *http.Request) map[string]string {
	headers := make(map[string]string)
	for name, values := range req.Header {
		if len(values) > 0 {
			// Only include the first value for simplicity
			headers[name] = values[0]
		}
	}
	return headers
}
