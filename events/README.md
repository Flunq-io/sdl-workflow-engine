# Event Store Service

The Event Store service is the centralized nervous system of the flunq.io workflow engine. ALL services connect to it as subscribers/publishers for complete event-driven coordination.

## 🏗️ Architecture - Central Event Hub

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
                        │ ┌─────────────────┐ │
                        │ │ Subscriber Mgmt │ │
                        │ │ (Connections)   │ │
                        │ └─────────────────┘ │
                        └─────────────────────┘
                                  │
                                  ▼
                        ┌─────────────────────┐
                        │  Event Persistence  │
                        │  (Durable Storage)  │
                        └─────────────────────┘
```

## 🚀 Features - Central Event Hub

- **Centralized Event Store**: Single source of truth for ALL events
- **Real-time Distribution**: WebSocket/gRPC connections to all services
- **Subscriber Management**: Active connection tracking and routing
- **Event Filtering**: Route events based on type, workflow ID, service rules
- **Event Replay**: Complete event history and catch-up capabilities
- **CloudEvents Standard**: Standardized event format and metadata
- **Fault Tolerance**: Automatic reconnection and missed event recovery
- **Dead Letter Queue**: Handle failed deliveries with retry logic
- **Event Ordering**: Guaranteed event ordering per workflow
- **Load Balancing**: Multiple service instances with smart routing

## 📁 Structure

```
events/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   ├── store/
│   ├── stream/
│   └── models/
├── pkg/
│   ├── events/
│   ├── backends/
│   └── schema/
├── configs/
├── proto/
├── Dockerfile
├── go.mod
└── README.md
```

## 🔧 Configuration

Environment variables:
- `PORT`: Server port (default: 8081)
- `GRPC_PORT`: gRPC port (default: 9001)
- `BACKEND`: Event backend (redis, kafka, nats)
- `REDIS_URL`: Redis connection string
- `KAFKA_BROKERS`: Kafka broker addresses
- `NATS_URL`: NATS server URL

## 🚀 Quick Start

```bash
# Install dependencies
go mod tidy

# Run locally
go run cmd/server/main.go

# Build
go build -o bin/events cmd/server/main.go

# Run with Docker
docker build -t flunq-events .
docker run -p 8081:8081 -p 9001:9001 flunq-events
```

## 📚 API Endpoints

### Events
- `POST /api/v1/events` - Publish event
- `GET /api/v1/events` - List events
- `GET /api/v1/events/{id}` - Get event
- `GET /api/v1/streams/{stream}/events` - Get stream events

### Streams
- `POST /api/v1/streams` - Create stream
- `GET /api/v1/streams` - List streams
- `GET /api/v1/streams/{id}` - Get stream
- `DELETE /api/v1/streams/{id}` - Delete stream

### Subscriptions
- `POST /api/v1/subscriptions` - Create subscription
- `GET /api/v1/subscriptions` - List subscriptions
- `DELETE /api/v1/subscriptions/{id}` - Delete subscription

## 🔌 Event Types

### Workflow Events
- `workflow.created`
- `workflow.updated`
- `workflow.deleted`
- `workflow.execution.started`
- `workflow.execution.completed`
- `workflow.execution.failed`

### Task Events
- `task.started`
- `task.completed`
- `task.failed`
- `task.retried`
