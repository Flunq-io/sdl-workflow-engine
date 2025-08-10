# EventStore Architecture

The EventStore is the **nervous system** of the flunq.io workflow engine. It provides **high-level resilience** with **pluggable backends** for maximum flexibility.

## 🎯 **New Unified Architecture** - Generic EventStore Interface

The EventStore is now a **library package** that services import directly, providing:

- **🔄 Event Sourcing**: Complete event history with deterministic replay
- **🛡️ High-level Resilience**: Crash recovery, horizontal scaling, time travel debugging
- **🔌 Pluggable Backends**: Easy switching between Redis, Kafka, RabbitMQ
- **⚡ High Performance**: Direct backend access, no HTTP overhead
- **🎯 Single Source of Truth**: Unified event storage and streaming

## 🏗️ **Generic Interface Architecture**

### **EventStore Interface - The Heart of the System**

```go
// EventStore defines the interface for event storage and streaming
// This allows pluggable backends (Redis, Kafka, RabbitMQ, etc.)
type EventStore interface {
    // Publish publishes an event to a stream
    Publish(ctx context.Context, stream string, event *CloudEvent) error

    // Subscribe subscribes to events from multiple streams
    // Returns channels for events and errors
    Subscribe(ctx context.Context, config SubscriptionConfig) (<-chan *CloudEvent, <-chan error, error)

    // ReadHistory reads complete event history for replay/recovery
    ReadHistory(ctx context.Context, stream string, fromID string) ([]*CloudEvent, error)

    // CreateConsumerGroup creates consumer groups for load balancing
    CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error

    // Checkpoint management for crash recovery
    CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error
    GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error)

    // Close closes the event store connection
    Close() error
}
```

### **Current Implementations**

#### **✅ Redis Event Stream** (`shared/pkg/eventstreaming/redis_stream.go`)
```go
// Create Redis Event Stream via factory
stream, err := factory.NewEventStream(factory.EventStreamDeps{
    Backend:     "redis",
    RedisClient: redisClient,
    Logger:      loggerAdapter,
})
```

**Features:**
- **Redis Streams**: High-performance event persistence with ordering guarantees
- **Consumer Groups**: Load balancing and fault tolerance across multiple workers
- **Event Filtering**: By event type, workflow ID, and custom criteria
- **History Replay**: Complete event history from any point in time
- **Checkpointing**: Automatic crash recovery with last-processed message tracking

#### **🚧 Planned Implementations**

- **KafkaEventStore** - High-throughput distributed streaming
- **RabbitMQEventStore** - Message queue with advanced routing
- **PostgreSQLEventStore** - JSONB-based event storage with SQL queries
- **MongoDBEventStore** - Document-based event storage

### **Service Integration Pattern**

```
┌─────────────┐           ┌─────────────┐           ┌─────────────┐
│   API       │───┐   ┌──▶│  Worker     │◀──┐   ┌──▶│  Executor   │
│   Service   │   │   │   │  Service    │   │   │   │  Service    │
└─────────────┘   │   │   └─────────────┘   │   │   └─────────────┘
                  │   │                      │   │
                  ▼   ▼                      ▼   ▼
           ┌────────────────┐        ┌──────────────────┐
           │ Shared Event   │        │ Shared Database  │
           │ Stream (Redis) │        │ (Redis -> pluggable)
           │ -> pluggable   │        │ Postgres/Mongo…  │
           └────────────────┘        └──────────────────┘
```

**Benefits:**
- **Direct Access**: No HTTP overhead, maximum performance
- **Pluggable**: Easy backend switching via configuration
- **Resilient**: Built-in crash recovery and event replay
- **Scalable**: Horizontal scaling with consumer groups

## 🔧 **How to Implement New EventStore Backends**

### **Step 1: Implement the EventStore Interface**

Create a new package under `shared/pkg/eventstreaming/yourbackend/`:

```go
package yourbackend

import (
    "context"
    "github.com/flunq-io/shared/pkg/cloudevents"
    "github.com/flunq-io/shared/pkg/interfaces"
)

type YourEventStore struct {
    // Your backend-specific client/connection
    client YourBackendClient
    logger eventstore.Logger
}

// NewYourEventStore creates a new EventStore implementation
func NewYourEventStore(connectionString string, logger eventstore.Logger) (eventstore.EventStore, error) {
    client, err := YourBackendClient.Connect(connectionString)
    if err != nil {
        return nil, err
    }

    return &YourEventStore{
        client: client,
        logger: logger,
    }, nil
}

// Implement all EventStore interface methods
func (y *YourEventStore) Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error {
    // Your implementation here
}

func (y *YourEventStore) Subscribe(ctx context.Context, config eventstore.SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error) {
    // Your implementation here
}

// ... implement all other interface methods
```

### **Step 2: Key Implementation Requirements**

#### **Event Persistence**
```go
func (y *YourEventStore) Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error {
    // 1. Serialize CloudEvent to your backend's format
    data := y.serializeEvent(event)

    // 2. Store with ordering guarantees (critical for event sourcing)
    messageID, err := y.client.Append(stream, data)
    if err != nil {
        return fmt.Errorf("failed to publish event: %w", err)
    }

    // 3. Log for debugging
    y.logger.Debug("Published event", "stream", stream, "event_id", event.ID, "message_id", messageID)
    return nil
}
```

#### **Event Streaming with Consumer Groups**
```go
func (y *YourEventStore) Subscribe(ctx context.Context, config eventstore.SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error) {
    eventCh := make(chan *cloudevents.CloudEvent, 100)
    errorCh := make(chan error, 10)

    // 1. Create consumer group for load balancing
    err := y.client.CreateConsumerGroup(config.ConsumerGroup, config.Streams)
    if err != nil {
        return nil, nil, err
    }

    // 2. Start subscription loop in goroutine
    go y.subscriptionLoop(ctx, config, eventCh, errorCh)

    return eventCh, errorCh, nil
}
```

#### **Event History for Replay**
```go
func (y *YourEventStore) ReadHistory(ctx context.Context, stream string, fromID string) ([]*cloudevents.CloudEvent, error) {
    // 1. Read all events from stream starting at fromID
    messages, err := y.client.ReadRange(stream, fromID, "end")
    if err != nil {
        return nil, err
    }

    // 2. Convert to CloudEvents
    events := make([]*cloudevents.CloudEvent, 0, len(messages))
    for _, msg := range messages {
        event, err := y.deserializeEvent(msg.Data)
        if err != nil {
            y.logger.Error("Failed to deserialize event", "error", err)
            continue
        }
        events = append(events, event)
    }

    return events, nil
}
```

### **Step 3: Integration with Services**

Update your service's main.go to use the new backend:

```go
// main.go
import (
    yourbackend "github.com/flunq-io/shared/pkg/eventstreaming/yourbackend"
    "github.com/flunq-io/shared/pkg/factory"
)

func main() {
    // Initialize your EventStore implementation
    eventStore, err := yourbackend.NewYourEventStore(config.YourBackendURL, logger)
    if err != nil {
        logger.Fatal("Failed to initialize event store", "error", err)
    }

    // Use it in your services
    workflowProcessor := processor.NewWorkflowProcessor(
        eventStore,  // ← Your implementation
        database,
        workflowEngine,
        serializer,
        logger,
        metrics,
    )
}
```

### **Step 4: Configuration-Driven Backend Selection**

```go
// config.go
type Config struct {
    EventStoreType string // "redis", "kafka", "rabbitmq"
    RedisURL       string
    KafkaBootstrapServers string
    RabbitMQURL    string
}

// factory.go
func NewEventStore(config *Config, logger eventstore.Logger) (eventstore.EventStore, error) {
    switch config.EventStoreType {
    case "redis":
        return redis.NewRedisEventStore(config.RedisURL, logger)
    case "kafka":
        return kafka.NewKafkaEventStore(config.KafkaBootstrapServers, logger)
    case "rabbitmq":
        return rabbitmq.NewRabbitMQEventStore(config.RabbitMQURL, logger)
    default:
        return nil, fmt.Errorf("unsupported event store type: %s", config.EventStoreType)
    }
}
```

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

## 🔌 Service Integration

### **Worker Service Integration**
The Worker service implements sophisticated event processing with the EventStore:

```go
// Initialize EventStore with shared interface
eventStore, err := factory.CreateEventStream(eventStreamType, config, logger)
if err != nil {
    log.Fatal("Failed to initialize event store:", err)
}

// Create WorkflowProcessor with EventStore
processor := processor.NewWorkflowProcessor(
    eventStore,        // Shared event streaming
    database,          // Worker-specific database
    sharedDatabase,    // Shared database for executions
    workflowEngine,    // Serverless Workflow SDK
    serializer,        // Protobuf serializer
    logger,           // Structured logger
    metrics,          // Metrics collector
)
```

#### **Event Processing Pattern**
```go
// Subscribe to events with consumer groups
filters := sharedinterfaces.StreamFilters{
    EventTypes: []string{
        "io.flunq.execution.started",
        "io.flunq.task.completed",
        "io.flunq.timer.fired",
    },
    ConsumerGroup: "worker-service",
    ConsumerName:  "worker-{uuid}",
}

subscription, err := eventStore.Subscribe(ctx, filters)
eventsCh := subscription.Events()

// Process events with resilience
for event := range eventsCh {
    // 1. Filter by execution ID for isolation
    // 2. Rebuild state from event history
    // 3. Process new event
    // 4. Execute next workflow step
    // 5. Acknowledge event
}
```

### **Generic Service Integration**
Services use the EventStore interface directly:

```go
// Initialize EventStore (Redis example)
eventStore, err := redis.NewRedisEventStore("redis://localhost:6379", logger)
if err != nil {
    log.Fatal("Failed to initialize event store:", err)
}

// Publish events
event := &cloudevents.CloudEvent{
    ID:          uuid.New().String(),
    Source:      "io.flunq.worker",
    SpecVersion: "1.0",
    Type:        "io.flunq.task.completed",
    Time:        time.Now(),
    Extensions: map[string]interface{}{
        "workflowid":  workflowID,
        "executionid": executionID,
    },
    Data: taskData,
}
err = eventStore.Publish(ctx, "events:global", event)

// Subscribe to events
config := eventstore.SubscriptionConfig{
    Streams:       []string{"events:global"},
    ConsumerGroup: "worker-service",
    ConsumerName:  "worker-1",
    EventTypes:    []string{"io.flunq.task.completed"},
    FromMessageID: ">",
    BlockTime:     1 * time.Second,
}

eventCh, errorCh, err := eventStore.Subscribe(ctx, config)
for {
    select {
    case event := <-eventCh:
        // Process event
    case err := <-errorCh:
        // Handle error
    }
}
```

## 📊 Event Types

### **Worker Service Event Processing**
The Worker service implements sophisticated event filtering and processing:

#### **Subscribed Events**
- `io.flunq.execution.started` - Triggers workflow execution start
- `io.flunq.task.completed` - Processes task completion (executor-service only)
- `io.flunq.timer.fired` - Resumes workflows after wait tasks

#### **Skipped Events**
- `io.flunq.workflow.created` - Handled by API service, skipped by Worker
- `io.flunq.task.completed` from non-executor sources - Prevents infinite loops

#### **Published Events**
- `io.flunq.task.requested` - Requests task execution from executor service
- `io.flunq.workflow.completed` - Signals workflow completion
- `io.flunq.event.dlq` - Dead letter queue for failed events

### **Complete Event Catalog**

#### **Workflow Events**
- `io.flunq.workflow.created` - Workflow definition created
- `io.flunq.workflow.started` - Workflow execution initiated
- `io.flunq.workflow.completed` - Workflow finished successfully
- `io.flunq.workflow.failed` - Workflow failed with error
- `io.flunq.workflow.cancelled` - Workflow cancelled by user

#### **Execution Events**
- `io.flunq.execution.started` - Execution instance started
- `io.flunq.execution.completed` - Execution instance completed
- `io.flunq.execution.failed` - Execution instance failed

#### **Task Events**
- `io.flunq.task.requested` - Task execution requested
- `io.flunq.task.started` - Task execution started
- `io.flunq.task.completed` - Task execution completed
- `io.flunq.task.failed` - Task execution failed
- `io.flunq.task.retried` - Task execution retried

#### **Timer Events**
- `io.flunq.timer.scheduled` - Timer scheduled for wait task
- `io.flunq.timer.fired` - Timer fired, resume workflow

#### **System Events**
- `io.flunq.event.dlq` - Dead letter queue event
- `io.flunq.state.entered` - State transition entered
- `io.flunq.state.exited` - State transition exited
- `io.flunq.state.error` - State transition error

## 🛡️ **Enterprise-Grade Resilience Features**

### **Event Sourcing Pattern**
- **Complete Event History**: Every workflow event is permanently stored
- **Deterministic Replay**: Rebuild exact workflow state from events
- **Time Travel Debugging**: Replay workflow to any point in time
- **Crash Recovery**: Workers rebuild state from events after restart

### **Horizontal Scaling**
- **Consumer Groups**: Multiple workers process events in parallel
- **Load Balancing**: Events distributed across worker instances
- **Fault Tolerance**: Failed workers don't lose events
- **Stateless Workers**: No local state, everything rebuilt from events

### **Event Ordering and Consistency**
- **Stream Ordering**: Events within a stream are totally ordered
- **Workflow Consistency**: All events for a workflow maintain causal order
- **Idempotent Processing**: Safe to replay events multiple times
- **Exactly-Once Delivery**: Consumer groups ensure no duplicate processing

## 🗄️ Storage Architecture

### Current Redis Streams Implementation

- **Global Stream**: `events:global` - All events with workflow filtering
- **Consumer Groups**: Load balancing across multiple workers
- **Event Acknowledgment**: Automatic tracking of processed messages
- **Checkpointing**: Crash recovery with last-processed message tracking

### Event Persistence Structure

```
Redis Streams:
└── events:global
    ├── 1640995200000-0: {workflow_created_event}
    ├── 1640995201000-0: {task_completed_event}
    ├── 1640995202000-0: {workflow_completed_event}
    └── ...

Consumer Groups:
├── worker-service
│   ├── worker-1: last_processed_id
│   ├── worker-2: last_processed_id
│   └── ...
└── api-service
    └── api-1: last_processed_id
```

## 🔄 Event Replay and Recovery

### **Automatic Crash Recovery**
```go
// Worker automatically recovers on startup
func (p *WorkflowProcessor) Start(ctx context.Context) error {
    // Get last processed message ID from checkpoint
    lastID, err := p.eventStore.GetLastCheckpoint(ctx, "worker-service", "events:global")

    // Subscribe from that point (or beginning if no checkpoint)
    config := eventstore.SubscriptionConfig{
        FromMessageID: lastID, // Resume from last checkpoint
        // ... other config
    }

    eventCh, errorCh, err := p.eventStore.Subscribe(ctx, config)
    // Process events and update checkpoints
}
```

### **Manual Event Replay**
```go
// Replay events for debugging or migration
func ReplayWorkflow(eventStore eventstore.EventStore, workflowID string) error {
    // Read complete event history
    events, err := eventStore.ReadHistory(ctx, "events:global", "0")
    if err != nil {
        return err
    }

    // Filter events for specific workflow
    workflowEvents := filterEventsByWorkflowID(events, workflowID)

    // Replay events to rebuild state
    state := &WorkflowState{}
    for _, event := range workflowEvents {
        state = applyEvent(state, event)
    }

    return nil
}
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

### **1. Start Redis**
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### **2. Initialize EventStore in Your Service**
```go
package main

import (
    "context"
    "log"
    redisstream "github.com/flunq-io/shared/pkg/eventstreaming"
)

func main() {
    // Initialize Redis EventStore
    eventStore, err := redis.NewRedisEventStore("redis://localhost:6379", logger)
    if err != nil {
        log.Fatal("Failed to initialize event store:", err)
    }
    defer eventStore.Close()

    // Use in your services
    processor := NewWorkflowProcessor(eventStore, ...)
    processor.Start(context.Background())
}
```

### **3. Publish Events**
```go
event := &cloudevents.CloudEvent{
    ID:          uuid.New().String(),
    Source:      "io.flunq.worker",
    Type:        "io.flunq.task.completed",
    Time:        time.Now(),
    Extensions:  map[string]interface{}{"workflowid": workflowID},
    Data:        taskData,
}

err := eventStore.Publish(ctx, "events:global", event)
```

### **4. Subscribe to Events**
```go
config := eventstore.SubscriptionConfig{
    Streams:       []string{"events:global"},
    ConsumerGroup: "my-service",
    ConsumerName:  "instance-1",
    EventTypes:    []string{"io.flunq.task.completed"},
    FromMessageID: ">",
}

eventCh, errorCh, err := eventStore.Subscribe(ctx, config)
for event := range eventCh {
    // Process event
}
```

## 🎯 **Key Benefits**

- **🛡️ Enterprise-Grade Resilience**: Complete event history, crash recovery, deterministic replay
- **🔌 Pluggable Backends**: Easy switching between Redis, Kafka, RabbitMQ via configuration
- **⚡ High Performance**: Direct backend access, no HTTP overhead
- **📈 Horizontal Scaling**: Consumer groups for load balancing across multiple workers
- **🔍 Complete Auditability**: Every workflow event permanently stored and queryable
- **🚀 Simple Integration**: Import as library, no separate service to deploy

The EventStore ensures ALL services stay synchronized through events, provides enterprise-grade resilience, and offers complete flexibility for future backend changes.
