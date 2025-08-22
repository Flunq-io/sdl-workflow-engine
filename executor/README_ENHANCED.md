# Executor Service

## ✅ **Status: Production Ready & Tested**

The Executor service is fully production-ready and handles task execution for the flunq.io workflow engine with enterprise-grade resilience. It processes task requests and executes various types of tasks including HTTP calls, data manipulation, and wait operations using sophisticated event processing patterns.

## 🎯 **Overview**

The Executor Service is responsible for executing individual workflow tasks with enterprise-grade error handling and retry capabilities. It subscribes to `task.requested` events and publishes `task.completed` events with proper status tracking and SDL-compliant error handling.

## 🚀 Features

### **Task Execution**
- ✅ **Multiple Task Types**: Supports call, set, wait, inject tasks
- ✅ **OpenAPI Integration**: Full OpenAPI 3.0 specification support with operation resolution
- ✅ **HTTP Integration**: External API calls with proper error handling
- ✅ **Data Processing**: Data transformation and manipulation tasks
- ✅ **Wait Operations**: Non-blocking delay execution

### **Enterprise-Grade Event Processing**
- ✅ **Consumer Groups**: Redis Streams consumer groups for load balancing
- ✅ **Retry Logic**: 3 attempts with exponential backoff (200ms * attempt)
- ✅ **Dead Letter Queue**: Failed events sent to DLQ after max retries
- ✅ **Message Acknowledgment**: Proper ack/nack semantics with Redis Streams
- ✅ **Orphaned Recovery**: Background process reclaims pending messages
- ✅ **Concurrency Control**: Configurable task concurrency with semaphore limiting
- ✅ **Graceful Shutdown**: Waits for in-flight tasks to complete

### **Resilience Features**
- ✅ **Circuit Breaker**: Resilient external API calls
- ✅ **Error Handling**: Comprehensive error logging and metrics
- ✅ **Health Checks**: Stream connectivity validation
- ✅ **Monitoring**: Structured logging with contextual information

## 🏗️ Architecture

The service follows a modular architecture with enhanced resilience:

```
executor/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── executor/        # Task execution logic
│   └── processor/       # Enhanced event processing with resilience
└── README.md
```

## 📊 **Configuration**

Configuration is managed through environment variables. Copy `.env.example` to `.env` and customize:

```bash
# Redis Configuration
REDIS_URL=localhost:6379
REDIS_PASSWORD=

# Logging Configuration
LOG_LEVEL=info

# Service Ports
METRICS_PORT=9091
HEALTH_PORT=8083

# Event Stream Configuration
EVENT_STREAM_TYPE=redis

# Task Execution Configuration
MAX_CONCURRENT_TASKS=10
TASK_TIMEOUT_SECONDS=300
RETRY_MAX_ATTEMPTS=3
RETRY_BASE_DELAY_MS=200

# OpenAPI Configuration
OPENAPI_CACHE_TTL_SECONDS=300
OPENAPI_REQUEST_TIMEOUT_SECONDS=30
```

### **Configuration Options**

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `localhost:6379` | Redis connection URL |
| `REDIS_PASSWORD` | `` | Redis password (if required) |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `METRICS_PORT` | `9091` | Prometheus metrics port |
| `HEALTH_PORT` | `8083` | Health check endpoint port |
| `EVENT_STREAM_TYPE` | `redis` | Event stream backend type |
| `MAX_CONCURRENT_TASKS` | `10` | Maximum concurrent task execution |
| `TASK_TIMEOUT_SECONDS` | `300` | Task execution timeout |
| `RETRY_MAX_ATTEMPTS` | `3` | Maximum retry attempts |
| `RETRY_BASE_DELAY_MS` | `200` | Base retry delay in milliseconds |
| `OPENAPI_CACHE_TTL_SECONDS` | `300` | OpenAPI document cache TTL |
| `OPENAPI_REQUEST_TIMEOUT_SECONDS` | `30` | OpenAPI request timeout |

## 📨 Event Processing

### **Subscribed Events**
- `io.flunq.task.requested` - Task execution requests from Worker service

### **Published Events**
- `io.flunq.task.completed` - Task completion results
- `io.flunq.task.dlq` - Dead letter queue for failed tasks

### **Event Processing Flow**
```
Task Request → Retry Logic → Task Execution → Result Publishing → Message Ack
     ↓              ↓             ↓              ↓               ↓
Consumer Group → Exponential → Task Logic → Event Stream → Redis Streams
                 Backoff                                     Acknowledgment
```

### **Resilience Pattern**
1. **Event Subscription**: Consumer groups with load balancing
2. **Concurrency Control**: Semaphore-based task limiting
3. **Retry Logic**: 3 attempts with exponential backoff
4. **Error Handling**: DLQ for failed events after max retries
5. **Message Acknowledgment**: Proper ack semantics to prevent message loss
6. **Orphaned Recovery**: Background reclaim of pending messages

## 🎯 Task Types

### **Call Tasks**

#### **OpenAPI Calls**
- **OpenAPI 3.0 Support**: Full support for OpenAPI 3.0.x specifications
- **Operation Resolution**: Find operations by `operationId`
- **Parameter Handling**: Automatic path, query, and header parameter processing
- **Authentication**: Bearer, Basic, and API Key authentication support
- **Document Caching**: Automatic caching of OpenAPI documents (5-minute TTL)
- **Response Processing**: Flexible output formats (content, response, raw)

#### **HTTP Calls (Legacy)**
- Execute direct HTTP requests to external APIs
- Support for GET, POST, PUT, DELETE methods
- Configurable timeouts and retries
- Response data extraction and transformation
- Circuit breaker protection

### **Set Tasks**
- Data manipulation and variable assignment
- JSON path operations
- Data transformation functions
- Context variable updates

### **Wait Tasks**
- Delay execution for specified duration
- Support for ISO 8601 duration format (e.g., PT2S for 2 seconds)
- Non-blocking implementation
- Immediate completion (actual wait handled by Timer service)

### **Inject Tasks**
- Data injection into workflow context
- Static data assignment
- Dynamic data generation
- Context enrichment

## 🔧 Configuration

### **Environment Variables**
- `EXECUTOR_CONCURRENCY`: Concurrent task processors (default: 4)
- `EXECUTOR_CONSUMER_GROUP`: Consumer group name (default: "executor-service")
- `EXECUTOR_STREAM_BATCH`: Event batch size (default: 10)
- `EXECUTOR_STREAM_BLOCK`: Block timeout (default: 1s)
- `EXECUTOR_RECLAIM_ENABLED`: Orphaned message reclaim (default: false)
- `REDIS_URL`: Redis connection string
- `EVENT_STREAM_TYPE`: Event stream backend (redis, kafka, etc.)
- `LOG_LEVEL`: Logging level

## 🎯 OpenAPI Call Examples

### **Basic OpenAPI Call**
```json
{
  "task_type": "call",
  "config": {
    "parameters": {
      "call_type": "openapi",
      "document": {
        "endpoint": "https://petstore.swagger.io/v2/swagger.json"
      },
      "operation_id": "findPetsByStatus",
      "parameters": {
        "status": "available"
      }
    }
  }
}
```

### **OpenAPI Call with Authentication**
```json
{
  "task_type": "call",
  "config": {
    "parameters": {
      "call_type": "openapi",
      "document": {
        "endpoint": "https://api.example.com/openapi.json"
      },
      "operation_id": "createUser",
      "parameters": {
        "body": {
          "name": "John Doe",
          "email": "john@example.com"
        }
      },
      "authentication": {
        "type": "bearer",
        "config": {
          "token": "your-jwt-token"
        }
      },
      "output": "content"
    }
  }
}
```

### **HTTP Call (Backward Compatibility)**
```json
{
  "task_type": "call",
  "config": {
    "parameters": {
      "url": "https://api.example.com/users",
      "method": "POST",
      "headers": {
        "Authorization": "Bearer your-token"
      },
      "body": {
        "name": "John Doe"
      }
    }
  }
}
```

## 🚀 Quick Start

```bash
# Build the service
go build -o bin/executor cmd/server/main.go

# Run the service with enhanced resilience
./bin/executor

# Or run directly
go run cmd/server/main.go

# Run with custom configuration
EXECUTOR_CONCURRENCY=8 EXECUTOR_RECLAIM_ENABLED=true go run cmd/server/main.go
```

## 📊 Monitoring & Observability

### **Metrics**
- Task processing counters by type and status
- Processing duration timers
- Error counters by attempt and type
- Dead letter queue counters

### **Logging**
- Structured JSON logging with contextual fields
- Task execution details and timing
- Error tracking with full context
- Performance metrics and resource usage

## 🛡️ Error Handling

### **Retry Strategy**
- **Max Retries**: 3 attempts per task
- **Backoff**: Exponential (200ms * attempt number)
- **Failure Handling**: DLQ after max retries
- **Acknowledgment**: Proper message ack after DLQ

### **Circuit Breaker**
- Protection for external API calls
- Configurable failure thresholds
- Automatic recovery mechanisms
- Graceful degradation

### **Dead Letter Queue**
- Failed tasks sent to `io.flunq.task.dlq` events
- Includes failure reason and original event data
- Enables manual inspection and reprocessing
- Prevents message loss during failures

## 🔄 Deployment

### **Docker**
```bash
# Build Docker image
docker build -t flunq-executor .

# Run with environment variables
docker run -e REDIS_URL=redis://redis:6379 -e EXECUTOR_CONCURRENCY=8 flunq-executor
```

### **Kubernetes**
- Horizontal scaling with multiple replicas
- Consumer groups ensure load balancing
- Health checks for readiness and liveness
- Resource limits and requests configuration

## 📋 To-Do Tasks & Future Enhancements

### **OpenAPI Call Tasks - Phase 2**
- [ ] **YAML OpenAPI Support**: Add YAML document parsing alongside JSON
- [ ] **OAuth2 Authentication**: Implement OAuth2 flow support for advanced authentication
- [ ] **Schema Validation**: Request/response schema validation against OpenAPI specs
- [ ] **Advanced Parameter Handling**: Support for complex parameter serialization (arrays, objects)
- [ ] **Content Type Negotiation**: Support for multiple content types (XML, form-data, etc.)
- [ ] **Persistent Document Caching**: Redis-based caching for OpenAPI documents across service restarts
- [ ] **Document Versioning**: Handle OpenAPI document versioning and updates
- [ ] **Security Schemes**: Support for more complex security schemes (OAuth2 scopes, etc.)

### **Call Task Enhancements**
- [ ] **gRPC Call Support**: Add support for gRPC service calls
- [ ] **AsyncAPI Support**: Support for AsyncAPI specifications and event-driven calls
- [ ] **GraphQL Support**: Add GraphQL query and mutation support
- [ ] **WebSocket Support**: Support for WebSocket connections and messaging
- [ ] **File Upload/Download**: Support for multipart file uploads and binary downloads
- [ ] **Streaming Responses**: Handle streaming HTTP responses
- [ ] **Custom Headers**: Dynamic header generation and templating
- [ ] **Request/Response Transformation**: Built-in data transformation capabilities

### **Performance & Reliability**
- [ ] **Connection Pooling**: HTTP connection pooling for better performance
- [ ] **Request Retries**: Configurable retry policies with exponential backoff
- [ ] **Circuit Breaker**: Enhanced circuit breaker with configurable thresholds
- [ ] **Rate Limiting**: Built-in rate limiting for external API calls
- [ ] **Timeout Configuration**: Per-operation timeout configuration
- [ ] **Metrics Collection**: Detailed metrics for OpenAPI operations (latency, success rate, etc.)
- [ ] **Health Checks**: Health check endpoints for external API dependencies
- [ ] **Load Balancing**: Support for multiple API endpoints with load balancing

### **Developer Experience**
- [ ] **OpenAPI Validation**: Validate workflow DSL against OpenAPI specifications
- [ ] **Auto-completion**: IDE support for OpenAPI operation IDs and parameters
- [ ] **Documentation Generation**: Auto-generate task documentation from OpenAPI specs
- [ ] **Testing Framework**: Built-in testing framework for OpenAPI operations
- [ ] **Mock Server Integration**: Integration with OpenAPI mock servers for testing
- [ ] **Debug Mode**: Enhanced debugging with request/response logging
- [ ] **Configuration Validation**: Pre-flight validation of OpenAPI configurations
- [ ] **Error Mapping**: Map OpenAPI error responses to workflow error handling

### **Security & Compliance**
- [ ] **Certificate Management**: Custom SSL certificate handling
- [ ] **API Key Rotation**: Automatic API key rotation support
- [ ] **Audit Logging**: Detailed audit logs for all external API calls
- [ ] **Data Masking**: Automatic masking of sensitive data in logs
- [ ] **Compliance Checks**: Built-in compliance validation (GDPR, SOX, etc.)
- [ ] **Encryption**: End-to-end encryption for sensitive API calls
- [ ] **Access Control**: Role-based access control for different API operations
- [ ] **Vulnerability Scanning**: Automatic scanning of OpenAPI endpoints

### **Integration & Ecosystem**
- [ ] **Postman Integration**: Import/export Postman collections
- [ ] **Swagger UI Integration**: Built-in Swagger UI for API exploration
- [ ] **API Gateway Integration**: Integration with popular API gateways
- [ ] **Service Mesh Support**: Support for service mesh environments (Istio, Linkerd)
- [ ] **Kubernetes Integration**: Native Kubernetes service discovery
- [ ] **Cloud Provider APIs**: Pre-built integrations for AWS, GCP, Azure APIs
- [ ] **Database Connectors**: Direct database connection support
- [ ] **Message Queue Integration**: Support for RabbitMQ, Kafka, etc.

## 🔗 Related Documentation

- [OpenAPI Call Tasks](docs/OPENAPI_CALLS.md) - Detailed OpenAPI call task documentation
- [Worker Service README](../worker/README.md) - Complete Worker service documentation
- [Event Store Architecture](../docs/EVENT_STORE_ARCHITECTURE.md) - EventStore design and implementation
- [Event Processing Quick Reference](../docs/EVENT_PROCESSING_QUICK_REFERENCE.md) - Quick reference guide
