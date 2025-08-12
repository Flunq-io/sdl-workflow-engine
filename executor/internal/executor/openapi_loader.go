package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// OpenAPIDocumentLoader interface for loading OpenAPI documents
type OpenAPIDocumentLoader interface {
	LoadDocument(ctx context.Context, resource *ExternalResource) (*OpenAPIDocument, error)
	CacheDocument(url string, doc *OpenAPIDocument)
	ClearCache()
}

// DefaultOpenAPIDocumentLoader implements OpenAPIDocumentLoader
type DefaultOpenAPIDocumentLoader struct {
	logger      *zap.Logger
	httpClient  *http.Client
	cache       map[string]*cachedDocument
	cacheMutex  sync.RWMutex
	cacheExpiry time.Duration
}

// cachedDocument represents a cached OpenAPI document with expiry
type cachedDocument struct {
	document  *OpenAPIDocument
	loadedAt  time.Time
	expiresAt time.Time
}

// NewOpenAPIDocumentLoader creates a new OpenAPI document loader
func NewOpenAPIDocumentLoader(logger *zap.Logger) OpenAPIDocumentLoader {
	return &DefaultOpenAPIDocumentLoader{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:       make(map[string]*cachedDocument),
		cacheExpiry: 5 * time.Minute, // Cache documents for 5 minutes
	}
}

// LoadDocument loads an OpenAPI document from the specified resource
func (l *DefaultOpenAPIDocumentLoader) LoadDocument(ctx context.Context, resource *ExternalResource) (*OpenAPIDocument, error) {
	if resource == nil || resource.Endpoint == "" {
		return nil, fmt.Errorf("invalid resource: endpoint is required")
	}

	// Check cache first
	if doc := l.getCachedDocument(resource.Endpoint); doc != nil {
		l.logger.Debug("Using cached OpenAPI document", zap.String("endpoint", resource.Endpoint))
		return doc, nil
	}

	l.logger.Info("Loading OpenAPI document", zap.String("endpoint", resource.Endpoint))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", resource.Endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml")
	req.Header.Set("User-Agent", "flunq-executor/1.0")

	// Execute request
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch document: HTTP %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse document
	doc, err := l.parseDocument(body, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	// Validate document
	if err := l.validateDocument(doc); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI document: %w", err)
	}

	// Cache the document
	l.CacheDocument(resource.Endpoint, doc)

	l.logger.Info("Successfully loaded OpenAPI document",
		zap.String("endpoint", resource.Endpoint),
		zap.String("version", doc.OpenAPI),
		zap.Int("paths", len(doc.Paths)))

	return doc, nil
}

// parseDocument parses the document based on content type
func (l *DefaultOpenAPIDocumentLoader) parseDocument(data []byte, contentType string) (*OpenAPIDocument, error) {
	var doc OpenAPIDocument

	// For now, we only support JSON. YAML support can be added later
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON document: %w", err)
	}

	return &doc, nil
}

// validateDocument performs basic validation of the OpenAPI/Swagger document
func (l *DefaultOpenAPIDocumentLoader) validateDocument(doc *OpenAPIDocument) error {
	// Check for OpenAPI 3.x or Swagger 2.0
	if doc.OpenAPI == "" && doc.Swagger == "" {
		return fmt.Errorf("missing version field (expected 'openapi' or 'swagger')")
	}

	// Handle OpenAPI 3.x documents
	if doc.OpenAPI != "" {
		switch doc.OpenAPI {
		case "3.0.0", "3.0.1", "3.0.2", "3.0.3", "3.1.0":
			l.logger.Debug("Detected OpenAPI 3.x document", zap.String("version", doc.OpenAPI))
		default:
			l.logger.Warn("Unsupported OpenAPI version, proceeding anyway", zap.String("version", doc.OpenAPI))
		}
	}

	// Handle Swagger 2.0 documents
	if doc.Swagger != "" {
		if doc.Swagger == "2.0" {
			l.logger.Debug("Detected Swagger 2.0 document", zap.String("version", doc.Swagger))
			// Convert Swagger 2.0 to OpenAPI 3.0 format for internal processing
			l.convertSwagger2ToOpenAPI3(doc)
		} else {
			l.logger.Warn("Unsupported Swagger version, proceeding anyway", zap.String("version", doc.Swagger))
		}
	}

	if doc.Paths == nil || len(doc.Paths) == 0 {
		return fmt.Errorf("document contains no paths")
	}

	return nil
}

// convertSwagger2ToOpenAPI3 converts Swagger 2.0 document to OpenAPI 3.0 format for internal processing
func (l *DefaultOpenAPIDocumentLoader) convertSwagger2ToOpenAPI3(doc *OpenAPIDocument) {
	l.logger.Debug("Converting Swagger 2.0 to OpenAPI 3.0 format")

	// Set OpenAPI version for internal processing
	doc.OpenAPI = "3.0.0"

	// Convert host, basePath, and schemes to servers
	if doc.Host != "" || doc.BasePath != "" || len(doc.Schemes) > 0 {
		var servers []OpenAPIServer

		// Determine schemes (default to https if none specified)
		schemes := doc.Schemes
		if len(schemes) == 0 {
			schemes = []string{"https"}
		}

		// Create server URLs from host, basePath, and schemes
		for _, scheme := range schemes {
			serverURL := scheme + "://"
			if doc.Host != "" {
				serverURL += doc.Host
			} else {
				serverURL += "localhost"
			}
			if doc.BasePath != "" {
				serverURL += doc.BasePath
			}

			servers = append(servers, OpenAPIServer{
				URL: serverURL,
			})
		}

		doc.Servers = servers
		l.logger.Debug("Converted Swagger 2.0 servers",
			zap.String("host", doc.Host),
			zap.String("basePath", doc.BasePath),
			zap.Strings("schemes", doc.Schemes),
			zap.Int("servers_created", len(servers)))
	}

	// Move definitions to components (if needed for future schema validation)
	if len(doc.Definitions) > 0 && doc.Components == nil {
		doc.Components = map[string]interface{}{
			"schemas": doc.Definitions,
		}
	}
}

// getCachedDocument retrieves a document from cache if it exists and hasn't expired
func (l *DefaultOpenAPIDocumentLoader) getCachedDocument(url string) *OpenAPIDocument {
	l.cacheMutex.RLock()
	defer l.cacheMutex.RUnlock()

	cached, exists := l.cache[url]
	if !exists {
		return nil
	}

	// Check if cache entry has expired
	if time.Now().After(cached.expiresAt) {
		// Remove expired entry (will be cleaned up later)
		delete(l.cache, url)
		return nil
	}

	return cached.document
}

// CacheDocument stores a document in the cache
func (l *DefaultOpenAPIDocumentLoader) CacheDocument(url string, doc *OpenAPIDocument) {
	l.cacheMutex.Lock()
	defer l.cacheMutex.Unlock()

	now := time.Now()
	l.cache[url] = &cachedDocument{
		document:  doc,
		loadedAt:  now,
		expiresAt: now.Add(l.cacheExpiry),
	}

	l.logger.Debug("Cached OpenAPI document",
		zap.String("url", url),
		zap.Time("expires_at", now.Add(l.cacheExpiry)))
}

// ClearCache clears all cached documents
func (l *DefaultOpenAPIDocumentLoader) ClearCache() {
	l.cacheMutex.Lock()
	defer l.cacheMutex.Unlock()

	l.cache = make(map[string]*cachedDocument)
	l.logger.Debug("Cleared OpenAPI document cache")
}

// cleanupExpiredCache removes expired entries from cache (called periodically)
func (l *DefaultOpenAPIDocumentLoader) cleanupExpiredCache() {
	l.cacheMutex.Lock()
	defer l.cacheMutex.Unlock()

	now := time.Now()
	for url, cached := range l.cache {
		if now.After(cached.expiresAt) {
			delete(l.cache, url)
			l.logger.Debug("Removed expired cache entry", zap.String("url", url))
		}
	}
}

// StartCacheCleanup starts a background goroutine to clean up expired cache entries
func (l *DefaultOpenAPIDocumentLoader) StartCacheCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute) // Clean up every minute
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.cleanupExpiredCache()
			}
		}
	}()
}
