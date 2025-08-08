package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// DocsHandler handles API documentation requests
type DocsHandler struct{}

// NewDocsHandler creates a new docs handler
func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

// SwaggerUI serves the Swagger UI for API documentation
func (h *DocsHandler) SwaggerUI(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Flunq.io API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: '/api/v1/docs/openapi.yaml',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

// OpenAPISpec serves the OpenAPI specification file
func (h *DocsHandler) OpenAPISpec(c *gin.Context) {
	// Serve the OpenAPI spec file from the docs directory
	// The path is relative to the API server's working directory
	specPath := filepath.Join("docs", "openapi.yaml")
	c.File(specPath)
}
