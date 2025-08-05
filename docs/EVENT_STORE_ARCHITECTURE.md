# Event Store Architecture

The Event Store service is the **nervous system** of the flunq.io workflow engine. It provides **flexible deployment modes** from pure pub/sub to full HTTP/gRPC APIs.

## 🎯 **You Were Right!** - Deployment Modes

The Event Store now supports **3 deployment modes** based on your needs:

### 1. **Pure Pub/Sub Mode** (Recommended for High Performance)
```bash
ENABLE_HTTP=false ENABLE_GRPC=false MODE=pubsub
```
- **No HTTP/gRPC servers** - just Redis connections
- **Direct Redis pub/sub** for maximum performance
- **Minimal overhead** - perfect for high-throughput scenarios
- Services connect directly to Redis using client libraries

### 2. **HTTP Mode** (Recommended for Development/Debugging)
```bash
ENABLE_HTTP=true ENABLE_GRPC=false MODE=http
```
- **HTTP API** for debugging and monitoring
- **WebSocket support** for real-time web dashboards
- **REST endpoints** for manual testing and integration
- **Swagger UI** for API documentation

### 3. **Full Mode** (Maximum Compatibility)
```bash
ENABLE_HTTP=true ENABLE_GRPC=true MODE=full
```
- **All interfaces available**: HTTP, WebSocket, gRPC
- **Maximum compatibility** with different client types
- **Higher resource usage** but most flexible

## 🏗️ **Generic Interface Architecture**

### **Clean Abstractions for Easy Backend Switching**

```go
// Storage Interface - Switch between Redis, PostgreSQL, Kafka
type EventStorage interface {
    Store(ctx context.Context, event *CloudEvent) error
    GetHistory(ctx context.Context, workflowID string) ([]*CloudEvent, error)
    GetSince(ctx context.Context, workflowID string, since string) ([]*CloudEvent, error)
    // ... more methods
}

// Publisher Interface - Switch between Redis Pub/Sub, Kafka, NATS
type EventPublisher interface {
    Publish(ctx context.Context, event *CloudEvent) error
    Subscribe(ctx context.Context, subscription *Subscription) (EventSubscription, error)
    // ... more methods
}
```

### **Current Implementations**
- ✅ **RedisStorage** - Redis Streams for persistence (`events/internal/storage/redis_storage.go`)
- ✅ **RedisPublisher** - Redis Pub/Sub for real-time distribution (`events/internal/publisher/redis_publisher.go`)
- ✅ **Generic Interfaces** - Pluggable backends (`shared/pkg/interfaces/`)

### **Planned Implementations**
- 🚧 **PostgreSQLStorage** - JSONB-based event storage
- 🚧 **KafkaPublisher** - High-throughput streaming
- 🚧 **RabbitMQPublisher** - Message queue with routing
- 🚧 **MongoDBStorage** - Document-based event storage

### **Service Connection Patterns**

#### **Pure Pub/Sub Mode** (High Performance)
```
Services ──► Direct Redis ──► Redis Streams (Storage)
         └─► Redis Pub/Sub ──► Other Services
```

#### **HTTP Mode** (Development/Debugging)
```
Services ──► HTTP API ──► Event Store ──► Redis
Web UI   ──► WebSocket ──► Event Store ──► Redis
```

#### **Hybrid Mode** (Production)
```
High-throughput services ──► Direct Redis
Web clients             ──► HTTP/WebSocket API
Management tools        ──► HTTP API
```

## 🚀 Event Store Service Features

### HTTP API for Event Operations

- `POST /api/v1/events` - write new events
- `GET /api/v1/events/{workflowId}` - get event history
- `GET /api/v1/events/{workflowId}/since/{version}` - incremental updates

### WebSocket/gRPC for Real-time Subscriptions

- Services establish persistent connections
- Event Store pushes events in real-time
- Automatic reconnection and event replay on connection loss
- Subscription filtering and routing

### Event Routing Engine

- Configurable routing rules (which events go to which subscribers)
- Dead letter queues for failed deliveries
- Retry logic for temporary subscriber failures
- Load balancing when multiple instances of same service subscribe

## 🔧 Service Startup Pattern

Each service on startup:

1. Connects to Event Store as subscriber
2. Registers for relevant event types
3. Requests replay of missed events (if restarting)
4. Begins processing new events in real-time

## 🛡️ Fault Tolerance

- Event Store persists all events durably
- Subscribers can reconnect and catch up on missed events
- Event Store tracks last delivered event per subscriber
- Services can request full replay if they lose state

## 📋 CloudEvents Standard

All events follow the CloudEvents v1.0 specification with flunq.io extensions:

```json
{
  "id": "event-123",
  "source": "io.flunq.worker",
  "specversion": "1.0",
  "type": "io.flunq.workflow.started",
  "time": "2024-01-01T12:00:00Z",
  "workflowid": "workflow-456",
  "executionid": "execution-789",
  "data": {
    "workflow_name": "user-onboarding",
    "input": {...}
  }
}
```

## 🔌 Client Integration

Services use the Event Store client library:

```go
// Create client
eventClient := client.NewEventClient("http://localhost:8081", "my-service", logger)

// Publish events
event := cloudevents.NewWorkflowEvent(id, source, eventType, workflowID)
event.SetData(data)
err := eventClient.PublishEvent(ctx, event)

// Subscribe to events
subscription, err := eventClient.SubscribeWebSocket(ctx, eventTypes, workflowIDs, filters)
for event := range subscription.Events() {
    // Process event
}
```

## 📊 Event Types

### Workflow Events
- `io.flunq.workflow.created`
- `io.flunq.workflow.started`
- `io.flunq.workflow.completed`
- `io.flunq.workflow.failed`
- `io.flunq.workflow.cancelled`

### Task Events
- `io.flunq.task.scheduled`
- `io.flunq.task.started`
- `io.flunq.task.completed`
- `io.flunq.task.failed`
- `io.flunq.task.retried`

### State Events
- `io.flunq.state.entered`
- `io.flunq.state.exited`
- `io.flunq.state.error`

## 🗄️ Storage Architecture

### Redis Streams Implementation

- **Workflow Streams**: `events:workflow:{workflowId}` - Events for specific workflows
- **Global Stream**: `events:global` - All events for cross-workflow queries
- **Subscriber Tracking**: Track last delivered event per subscriber
- **Event Ordering**: Guaranteed ordering within workflow streams

### Event Persistence

```
Redis Streams Structure:
├── events:workflow:workflow-123
│   ├── 1640995200000-0: {event_data}
│   ├── 1640995201000-0: {event_data}
│   └── ...
├── events:workflow:workflow-456
│   └── ...
└── events:global
    ├── 1640995200000-0: {event_data + workflow_id}
    ├── 1640995201000-0: {event_data + workflow_id}
    └── ...
```

## 🔄 Event Replay

Services can request event replay for:

- **Recovery**: Rebuild state after service restart
- **Debugging**: Replay events to reproduce issues
- **Migration**: Replay events to new service versions
- **Testing**: Replay production events in test environments

```bash
# Replay all events for a workflow
curl -X POST http://localhost:8081/api/v1/admin/replay/workflow-123
```

## 📈 Monitoring and Observability

### Metrics
- Event throughput (events/second)
- Subscriber connection count
- Event delivery latency
- Failed delivery count
- Dead letter queue size

### Health Checks
- Redis connectivity
- Active subscriber count
- Event processing lag
- Memory usage

### Logging
- All events are logged with structured logging
- Subscriber connection/disconnection events
- Failed event deliveries
- Performance metrics

## 🚀 Getting Started

1. **Start Redis**:
   ```bash
   docker run -d -p 6379:6379 redis:7-alpine
   ```

2. **Configure Environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your Redis connection details
   ```

3. **Start Event Store**:
   ```bash
   cd events
   go run cmd/server/main.go
   ```

4. **Connect Services**:
   ```go
   eventClient := client.NewEventClient("http://localhost:8081", "my-service", logger)
   ```

The Event Store ensures ALL services stay synchronized through events, handles the complexity of distribution, and provides complete auditability and replay capabilities.
