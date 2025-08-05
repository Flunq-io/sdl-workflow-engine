# Flunq.io - Serverless Workflow Engine

A modern, cloud-native workflow engine built on the Serverless Workflow Definition Language (DSL 1.0.0) with pluggable event streaming and storage backends. Designed for high-performance, multi-tenant environments with full CloudEvents compliance.

## 🏗️ Architecture

```
    API Service ──┐
                  │
    Worker ───────┤
                  │     ┌─────────────────────┐
    Executor ─────┼────►│   Event Store       │────► All Services
                  │     │   (Central Hub)     │      (Subscribers)
    UI Service ───┤     │                     │
                  │     │ ┌─────────────────┐ │
    Other ────────┘     │ │ Event Router    │ │
    Services            │ │ WebSocket/gRPC  │ │
                        │ └─────────────────┘ │
                        │ ┌─────────────────┐ │
                        │ │ Event Storage   │ │
                        │ │ (Redis Streams) │ │
                        │ └─────────────────┘ │
                        └─────────────────────┘
```

## 🚀 Services

### **API Service** (`/api`)
- **Language**: Go
- **Purpose**: HTTP API server, DSL parsing, workflow orchestration
- **Features**: REST/GraphQL endpoints, workflow validation, execution control

### **Event Store Service** (`/events`)
- **Language**: Go
- **Purpose**: Centralized event hub - the nervous system of the workflow engine
- **Features**: CloudEvents storage, real-time distribution, subscriber management, event replay

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
- **Database**: PostgreSQL (default), pluggable to MongoDB/DynamoDB
- **Standards**: CloudEvents v1.0, Serverless Workflow DSL 1.0.0
- **Deployment**: Docker, Kubernetes
- **Monitoring**: OpenTelemetry, Prometheus

## 🔌 Generic Interfaces

Flunq.io is built with pluggable backends through generic interfaces:

### **Event Storage** (`shared/pkg/interfaces/event_storage.go`)
- **Redis** (current) - Redis Streams for high-performance event storage
- **PostgreSQL** (planned) - JSONB columns for event data
- **MongoDB** (planned) - Document-based event storage
- **EventStore DB** (planned) - Purpose-built event store

### **Event Streaming** (`shared/pkg/interfaces/event_streaming.go`)
- **Redis Streams** (current) - Built-in Redis streaming
- **Kafka** (planned) - High-throughput distributed streaming
- **RabbitMQ** (planned) - Message queue with routing
- **NATS** (planned) - Lightweight cloud-native messaging

### **Database** (`shared/pkg/interfaces/database.go`)
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
cd events && go run main.go
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
- [x] **Event Store architecture** - Centralized event hub with real-time distribution
- [x] **Generic interfaces** - Pluggable storage and streaming backends
- [x] **Multi-tenant support** - Tenant isolation across all services

## 🎯 Roadmap

- [ ] PostgreSQL/MongoDB storage implementations
- [ ] Kafka/RabbitMQ streaming implementations
- [ ] Advanced DSL features (parallel, switch, try/catch)
- [ ] REST API endpoints
- [ ] Web UI dashboard
- [ ] Advanced monitoring and observability
- [ ] Kubernetes operator

## 📄 License

MIT License - see LICENSE file for details.
