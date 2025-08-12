# Flunq.io - Serverless Workflow Engine

A modern, cloud-native workflow engine built on the Serverless Workflow Definition Language (DSL 1.0.0) with pluggable event streaming and storage backends. Designed for high-performance, multi-tenant environments with full CloudEvents compliance.

## 🏗️ Architecture

```
┌─────────────┐       HTTP/WS        ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
│     UI      │ ───────────────────▶ │    API      │───┐   ┌──▶│   Worker    │◀──┐   ┌──▶│  Executor   │
│   Service   │                      │   Service   │   │   │   │   Service   │   │   │   │  Service    │
└─────────────┘                      └─────────────┘   │   │   └─────────────┘   │   │   └─────────────┘
                                                       │   │         ▲           │   │
                                                       ▼   ▼         │           ▼   ▼
                                             ┌────────────────┐      │   ┌──────────────────┐
                                             │ Shared Event   │◀─────┘   │ Shared Database  │
                                             │ Stream (Redis) │          │ (Redis -> pluggable)
                                             │ -> pluggable   │          │ Postgres/Mongo…  │
                                             └────────────────┘          └──────────────────┘
                                                      ▲
                                                      │
                                              ┌───────────────┐
                                              │ Timer Service │ (ZSET based)
                                              └───────────────┘

Backends are selected via env and factories:
- EVENT_STREAM_TYPE: redis (default), kafka, rabbitmq, nats
- DB_TYPE: redis (default), postgres, mongo, dynamo
```

## 🚀 Services

### **API Service** (`/api`)
- **Language**: Go
- **Purpose**: HTTP API server, DSL parsing, workflow orchestration
- **Features**: REST/GraphQL endpoints, workflow validation, execution control

### **EventStore Library** (`shared/pkg/eventstreaming`)
- **Language**: Go
- **Purpose**: Pluggable event storage and streaming - the nervous system of the workflow engine
- **Features**: Event sourcing, resilient design, pluggable backends (Redis/Kafka/RabbitMQ)

### **Worker Service** (`/worker`)
- **Language**: Go
- **Purpose**: Event-driven workflow execution engine
- **Features**:
  - **Event Processing**: Sophisticated event filtering and processing with consumer groups
  - **State Rebuilding**: Complete workflow state reconstruction from event history
  - **Execution Isolation**: Per-execution event filtering prevents cross-execution contamination
  - **Wait Task Handling**: Event-driven timeouts using timer service integration
  - **Resilience**: Retry logic, dead letter queues, orphaned message recovery
  - **Concurrency**: Configurable worker concurrency with per-workflow locking

### **Executor Service** (`/executor`)
- **Language**: Go
- **Purpose**: External API integration and task execution with enterprise-grade resilience
- **Features**:
  - **Task Execution**: HTTP calls, data manipulation, wait task handling, SDL try/catch blocks
  - **Event Processing**: Consumer groups, retry logic, dead letter queues
  - **Resilience**: Message acknowledgment, orphaned message recovery
  - **Concurrency**: Configurable task concurrency with semaphore control
  - **SDL Error Handling**: Declarative try/catch blocks with sophisticated retry policies
  - **Error Recovery**: Execute specific tasks when errors are caught
  - **Backoff Strategies**: Constant, linear, exponential backoff with jitter support
  - **Error Filtering**: Pattern matching for error types, status codes, and custom properties

### **Timer Service** (`/timer`)
- **Language**: Go
- **Purpose**: Event-driven timer scheduling for wait tasks
- **Features**:
  - **Timer Management**: Redis ZSET-based timer storage and scheduling
  - **Event Processing**: Consumer groups, retry logic, message acknowledgment
  - **Resilience**: Dead letter queues, orphaned message recovery
  - **Precision**: Sub-second timer precision with efficient batch processing
  - **Scalability**: Horizontal scaling with consumer group load balancing

### **UI Service** (`/ui`)
- **Language**: Next.js/TypeScript
- **Purpose**: Web dashboard and workflow designer
- **Features**: Visual workflow builder, monitoring, debugging

## 🔧 Technology Stack

- **Backend**: Go 1.21+
- **Frontend**: Next.js 14, TypeScript, Tailwind CSS
- **Event Streaming**: Redis Streams (default), pluggable to Kafka/RabbitMQ/NATS
- **Event Storage**: Redis (default), pluggable to PostgreSQL/MongoDB/EventStore DB
- **Database**: Redis (current), pluggable to PostgreSQL/MongoDB/DynamoDB
- **Data Serialization**: JSON (current), protobuf definitions for type safety
- **Standards**: CloudEvents v1.0, Serverless Workflow DSL 1.0.0
- **Deployment**: Docker, Kubernetes
- **Monitoring**: OpenTelemetry, Prometheus

## 🔌 Shared interfaces and factories

- Event streaming interface: shared/pkg/interfaces/event_streaming.go
- Database interface: shared/pkg/interfaces/database.go
- Factories: shared/pkg/factory/{eventstream_factory.go,database_factory.go}

Current default implementations
- Event stream: Redis Streams (shared/pkg/eventstreaming/redis_stream.go)
- Database: Redis (shared/pkg/storage/redis_database.go)

Pluggable backends (contributors welcome)
- Event stream: Kafka, RabbitMQ, NATS
- Database: Postgres, MongoDB, DynamoDB

Backends are selected by env via factories:
- EVENT_STREAM_TYPE (redis|kafka|rabbitmq|nats)
- DB_TYPE (redis|postgres|mongo|dynamo)

All implementations follow the same interface contracts, making it easy to switch backends without code changes.

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/your-org/flunq.io
cd flunq.io

# Start all services with Docker Compose
docker-compose up -d

# Or start individual services
cd api && go run main.go
cd worker && go run main.go
cd executor && go run main.go
cd ui && npm run dev
```

## 📋 Development

Each service is independently deployable with its own:
- Dockerfile
- CI/CD pipeline
- Documentation
- Tests

See individual service READMEs for detailed setup instructions.

## ✅ Current Status

- [x] **Core DSL parser and validator** - Supports DSL 1.0.0 (YAML) and 0.8 (JSON)
- [x] **Redis-based event streaming** - CloudEvents compliant with Redis Streams
- [x] **Advanced workflow execution engine** - Event-driven processing with state rebuilding
- [x] **EventStore architecture** - Unified interface with pluggable backends (Redis/Kafka/RabbitMQ)
- [x] **Enhanced I/O storage** - Complete workflow and task input/output data storage
- [x] **Generic interfaces** - Pluggable storage and streaming backends
- [x] **Multi-tenant support** - Tenant isolation across all services
- [x] **Web UI with event timeline** - Real-time workflow monitoring with I/O data visualization
- [x] **Event processing resilience** - Consumer groups, retry logic, dead letter queues across all services
- [x] **Execution isolation** - Per-execution event filtering prevents state contamination
- [x] **Wait task processing** - Event-driven timeouts using enhanced timer service
- [x] **Concurrency control** - Configurable concurrency with per-workflow locking
- [x] **Enhanced executor service** - Resilient task execution with retry logic and acknowledgment
- [x] **Enhanced timer service** - Enterprise-grade timer scheduling with consumer groups
- [x] **Unified resilience patterns** - Consistent error handling across Worker, Executor, and Timer services
- [x] **SDL Try/Catch Error Handling** - Declarative error handling with sophisticated retry policies
- [x] **Task and Workflow Status Tracking** - Proper status updates for failed tasks and workflows
- [x] **Error Recovery System** - Execute specific tasks when errors are caught
- [x] **Advanced Retry Policies** - Multiple backoff strategies with jitter and conditional logic

## 🎯 Roadmap

### **v0.0.1 - Foundation** ✅ **COMPLETE**
- [x] **Core DSL parser and validator** - Supports DSL 1.0.0 (YAML) and 0.8 (JSON)
- [x] **Redis-based event streaming** - CloudEvents compliant with Redis Streams
- [x] **Advanced workflow execution engine** - Event-driven processing with state rebuilding
- [x] **EventStore architecture** - Unified interface with pluggable backends
- [x] **Multi-tenant support** - Tenant isolation across all services
- [x] **Web UI with event timeline** - Real-time workflow monitoring with I/O data visualization
- [x] **Event processing resilience** - Consumer groups, retry logic, dead letter queues
- [x] **SDL Try/Catch Error Handling** - Declarative error handling with sophisticated retry policies
- [x] **Enterprise-grade resilience** - Circuit breakers, graceful shutdown, concurrency control

### **v0.1.0 - Backend Expansion** 🔄 **NEXT**
- [ ] **PostgreSQL storage implementation** - Production-ready relational database backend
- [ ] **Kafka EventStore implementation** - High-throughput event streaming backend
- [ ] **MongoDB storage implementation** - Document database backend
- [ ] **Advanced monitoring** - OpenTelemetry, Prometheus, distributed tracing
- [ ] **Integration test suite** - Comprehensive end-to-end testing
- [ ] **Performance benchmarks** - Load testing and optimization

### **v0.2.0 - Advanced Features** 🚀 **FUTURE**
- [ ] **Advanced DSL features** - Parallel, switch task types
- [ ] **Protobuf serialization** - Binary serialization for performance
- [ ] **Kubernetes operator** - Native Kubernetes deployment and management
- [ ] **Workflow versions** - Versioning of workflow definitions
- [ ] **SAGA Pattern** - Native compensation workflows
- [ ] **DSL Protocol Support** - AsyncAPI, gRPC, OpenAPI protocol implementations

## 📄 License

MIT License - see LICENSE file for details.
