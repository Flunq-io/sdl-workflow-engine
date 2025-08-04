# API Service

The API service handles HTTP requests, parses Serverless Workflow DSL, and orchestrates workflow execution.

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
