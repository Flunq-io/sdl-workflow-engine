# Tasks Service

The Tasks service handles external API calls, HTTP requests, and integrations with third-party services.

## 🏗️ Architecture

```
┌─────────────────┐
│  HTTP Client    │
│  (Retries/CB)   │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Rate Limiter   │
│  (Token Bucket) │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Auth Manager   │
│  (OAuth/JWT)    │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Task Queue     │
│  (Redis/Memory) │
└─────────────────┘
```

## 🚀 Features

- **HTTP Client**: Configurable HTTP client with timeouts
- **Retry Logic**: Exponential backoff with jitter
- **Circuit Breaker**: Fail-fast for unhealthy services
- **Rate Limiting**: Token bucket and sliding window
- **Authentication**: OAuth2, JWT, API keys
- **Request/Response Transformation**: JSON, XML, form data
- **Monitoring**: Request metrics and tracing

## 📁 Structure

```
tasks/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── client/
│   ├── auth/
│   ├── ratelimit/
│   └── handlers/
├── pkg/
│   ├── http/
│   ├── retry/
│   └── circuit/
├── configs/
├── Dockerfile
├── go.mod
└── README.md
```

## 🔧 Configuration

Environment variables:
- `PORT`: Server port (default: 8083)
- `REDIS_URL`: Redis connection string
- `DEFAULT_TIMEOUT`: Default HTTP timeout (default: 30s)
- `MAX_RETRIES`: Maximum retry attempts (default: 3)
- `RATE_LIMIT`: Requests per second (default: 100)

## 🚀 Quick Start

```bash
# Install dependencies
go mod tidy

# Run locally
go run cmd/server/main.go

# Build
go build -o bin/tasks cmd/server/main.go

# Run with Docker
docker build -t flunq-tasks .
docker run -p 8083:8083 flunq-tasks
```

## 📚 API Endpoints

### Task Execution
- `POST /api/v1/tasks/execute` - Execute HTTP task
- `GET /api/v1/tasks/{id}` - Get task status
- `POST /api/v1/tasks/{id}/cancel` - Cancel task

### Configuration
- `POST /api/v1/configs` - Create configuration
- `GET /api/v1/configs` - List configurations
- `PUT /api/v1/configs/{id}` - Update configuration

## 🔧 Task Types

### HTTP Tasks
```json
{
  "type": "http",
  "method": "POST",
  "url": "https://api.example.com/users",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer ${token}"
  },
  "body": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "timeout": "30s",
  "retries": 3
}
```

### GraphQL Tasks
```json
{
  "type": "graphql",
  "endpoint": "https://api.example.com/graphql",
  "query": "mutation CreateUser($input: UserInput!) { createUser(input: $input) { id name } }",
  "variables": {
    "input": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  }
}
```

### Database Tasks
```json
{
  "type": "database",
  "connection": "postgres://user:pass@localhost/db",
  "query": "INSERT INTO users (name, email) VALUES ($1, $2)",
  "params": ["John Doe", "john@example.com"]
}
```

## 🔄 Retry Policies

- **Fixed Delay**: Wait fixed time between retries
- **Exponential Backoff**: Exponentially increase delay
- **Linear Backoff**: Linearly increase delay
- **Custom**: User-defined retry logic
