# Events Service

The Events service handles event streaming, storage, and distribution across the flunq.io platform.

## 🏗️ Architecture

```
┌─────────────────┐
│   Event Router  │
│   (gRPC/HTTP)   │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Event Store    │
│  (Redis/Kafka)  │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Stream Manager │
│  (Pub/Sub)      │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Event Sourcing │
│  (Snapshots)    │
└─────────────────┘
```

## 🚀 Features

- **Event Streaming**: Real-time event distribution
- **Event Sourcing**: Complete audit trail of all events
- **Pluggable Backends**: Redis Streams, Apache Kafka, NATS
- **Event Replay**: Replay events from any point in time
- **Dead Letter Queue**: Handle failed event processing
- **Schema Registry**: Event schema validation and evolution

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
