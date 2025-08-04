# API Service

The API service provides a professional REST API with OpenAPI 3.0 specification for managing workflows and executions in the flunq.io platform.

## 🚀 Features

- **OpenAPI 3.0 Specification**: Complete API documentation with examples
- **Swagger UI**: Interactive API documentation at `/docs`
- **RESTful Design**: Standard HTTP methods and status codes
- **Request Validation**: Comprehensive input validation with detailed error messages
- **Event Integration**: Publishes events to the centralized Event Store
- **Health Checks**: Service health monitoring endpoints
- **CORS Support**: Cross-origin resource sharing for web clients

## 📚 API Documentation

- **Swagger UI**: http://localhost:8080/docs
- **OpenAPI Spec**: http://localhost:8080/api/v1/docs/openapi.yaml
- **Health Check**: http://localhost:8080/health

## 🔧 Quick Start

```bash
# Start the API service
go run cmd/server/main.go

# API will be available at http://localhost:8080
```

## 📋 Core Endpoints

### Workflows
- `POST /api/v1/workflows` - Create workflow
- `GET /api/v1/workflows` - List workflows
- `GET /api/v1/workflows/{id}` - Get workflow details
- `PUT /api/v1/workflows/{id}` - Update workflow
- `DELETE /api/v1/workflows/{id}` - Delete workflow
- `POST /api/v1/workflows/{id}/execute` - Execute workflow
- `GET /api/v1/workflows/{id}/events` - Get workflow events

### Executions
- `GET /api/v1/executions` - List executions
- `GET /api/v1/executions/{id}` - Get execution details
- `POST /api/v1/executions/{id}` - Cancel execution

## 🧪 Testing Examples

### Create a Workflow
```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user-onboarding",
    "description": "Complete user onboarding process",
    "definition": {
      "id": "user-onboarding",
      "specVersion": "0.8",
      "name": "User Onboarding Workflow",
      "start": "validate-user-data",
      "states": [
        {
          "name": "validate-user-data",
          "type": "operation",
          "actions": [
            {
              "name": "validate-email",
              "functionRef": {
                "refName": "validate-email"
              }
            }
          ],
          "end": true
        }
      ]
    }
  }'
```

### Execute a Workflow
```bash
curl -X POST http://localhost:8080/api/v1/workflows/{workflow-id}/execute \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "user": {
        "email": "test@example.com",
        "name": "Test User"
      }
    },
    "correlation_id": "test-123"
  }'
```

### List Workflows
```bash
curl http://localhost:8080/api/v1/workflows
```

### Get Execution Details
```bash
curl http://localhost:8080/api/v1/executions/{execution-id}
```

## 🏗️ Architecture

```
┌─────────────────┐
│   HTTP Router   │
│   (Gin/Fiber)   │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  DSL Parser     │
│  (YAML/JSON)    │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Workflow       │
│  Orchestrator   │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Event Client   │
│  (Redis/Kafka)  │
└─────────────────┘
```

## 🚀 Features

- **DSL Parsing**: Serverless Workflow Definition Language support
- **REST API**: CRUD operations for workflows
- **GraphQL**: Advanced querying capabilities
- **Validation**: Schema validation for workflow definitions
- **Authentication**: JWT-based auth with RBAC
- **Rate Limiting**: Request throttling and quotas

## 📁 Structure

```
api/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   ├── middleware/
│   ├── models/
│   ├── parser/
│   └── services/
├── pkg/
│   ├── auth/
│   ├── events/
│   └── validation/
├── configs/
├── docs/
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

## 🔧 Configuration

Environment variables:
- `PORT`: Server port (default: 8080)
- `REDIS_URL`: Redis connection string
- `JWT_SECRET`: JWT signing secret
- `LOG_LEVEL`: Logging level (debug, info, warn, error)

## 🚀 Quick Start

```bash
# Install dependencies
go mod tidy

# Run locally
go run cmd/server/main.go

# Build
go build -o bin/api cmd/server/main.go

# Run with Docker
docker build -t flunq-api .
docker run -p 8080:8080 flunq-api
```

## 📚 API Endpoints

### Workflows
- `POST /api/v1/workflows` - Create workflow
- `GET /api/v1/workflows` - List workflows  
- `GET /api/v1/workflows/{id}` - Get workflow
- `PUT /api/v1/workflows/{id}` - Update workflow
- `DELETE /api/v1/workflows/{id}` - Delete workflow

### Executions
- `POST /api/v1/workflows/{id}/execute` - Start execution
- `GET /api/v1/executions` - List executions
- `GET /api/v1/executions/{id}` - Get execution
- `POST /api/v1/executions/{id}/cancel` - Cancel execution

### Health
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics
