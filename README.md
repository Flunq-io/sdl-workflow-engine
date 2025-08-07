# Flunq.io - Serverless Workflow Engine

A modern, cloud-native workflow engine built on the Serverless Workflow Definition Language (DSL 1.0.0) with pluggable event streaming and storage backends. Designed for high-performance, multi-tenant environments with full CloudEvents compliance.

## 🏗️ Architecture

```
┌─────────────┐    ┌─────────────────┐    ┌─────────────┐
│   API       │───▶│  EventStore     │◀───│   Worker    │
│   Service   │    │  Interface      │    │   Service   │
└─────────────┘    │                 │    └─────────────┘
                   │ ┌─────────────┐ │
┌─────────────┐    │ │Redis Streams│ │    ┌─────────────┐
│  Executor   │───▶│ │ (current)   │ │◀───│ UI Service  │
│  Service    │    │ └─────────────┘ │    └─────────────┘
└─────────────┘    │                 │
                   │ ┌─────────────┐ │
                   │ │   Kafka     │ │
                   │ │  (future)   │ │
                   │ └─────────────┘ │
                   │                 │
                   │ ┌─────────────┐ │
                   │ │ RabbitMQ    │ │
                   │ │  (future)   │ │
                   │ └─────────────┘ │
                   └─────────────────┘
```

## 🚀 Services

### **API Service** (`/api`)
- **Language**: Go
- **Purpose**: HTTP API server, DSL parsing, workflow orchestration
- **Features**: REST/GraphQL endpoints, workflow validation, execution control

### **EventStore Library** (`/worker/pkg/eventstore`)
- **Language**: Go
- **Purpose**: Pluggable event storage and streaming - the nervous system of the workflow engine
- **Features**: Event sourcing, Temporal-level resilience, pluggable backends (Redis/Kafka/RabbitMQ)

### **Worker Service** (`/worker`)
- **Language**: Go  
- **Purpose**: Workflow execution engine
- **Features**: DSL interpretation, state management, task scheduling

### **Executor Service** (`/executor`)
- **Language**: Go
- **Purpose**: External API integration and task execution
- **Features**: HTTP calls, retries, circuit breakers, rate limiting

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

## 🔌 EventStore Interface

Flunq.io uses a unified EventStore interface for maximum flexibility:

### **EventStore Interface** (`worker/pkg/eventstore/interface.go`)
```go
type EventStore interface {
    Publish(ctx context.Context, stream string, event *CloudEvent) error
    Subscribe(ctx context.Context, config SubscriptionConfig) (<-chan *CloudEvent, <-chan error, error)
    ReadHistory(ctx context.Context, stream string, fromID string) ([]*CloudEvent, error)
    CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error
    CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error
    GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error)
    Close() error
}
```

### **Current Implementations**
- **✅ Redis EventStore** (`worker/pkg/eventstore/redis/`) - Redis Streams with consumer groups
- **🚧 Kafka EventStore** (planned) - High-throughput distributed streaming
- **🚧 RabbitMQ EventStore** (planned) - Message queue with advanced routing
- **🚧 PostgreSQL EventStore** (planned) - JSONB-based event storage

### **Database** (`shared/pkg/interfaces/database.go`)
- **Redis** (current) - JSON-serialized workflow state and task data
- **PostgreSQL** (planned) - Relational database for workflow metadata
- **MongoDB** (planned) - Document database for flexible schemas
- **DynamoDB** (planned) - Serverless NoSQL for AWS environments

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
- [x] **Basic workflow execution engine** - Full task execution pipeline
- [x] **EventStore architecture** - Unified interface with pluggable backends (Redis/Kafka/RabbitMQ)
- [x] **Enhanced I/O storage** - Complete workflow and task input/output data storage
- [x] **Generic interfaces** - Pluggable storage and streaming backends
- [x] **Multi-tenant support** - Tenant isolation across all services
- [x] **Web UI with event timeline** - Real-time workflow monitoring with I/O data visualization

## 🎯 Roadmap

- [ ] **Protobuf serialization** - Replace JSON with binary protobuf for performance
- [ ] **PostgreSQL/MongoDB storage** - Alternative database implementations
- [ ] **Kafka/RabbitMQ EventStore** - Alternative EventStore backend implementations
- [ ] **Advanced DSL features** - Parallel, switch, try/catch task types
- [ ] **REST API endpoints** - Complete CRUD operations for workflows
- [ ] **Advanced monitoring** - OpenTelemetry, Prometheus, distributed tracing
- [ ] **Kubernetes operator** - Native Kubernetes deployment and management

## 📄 License

MIT License - see LICENSE file for details.
