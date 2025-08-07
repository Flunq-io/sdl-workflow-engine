# EventStore Implementation Guide

This guide shows you how to implement new EventStore backends for the flunq.io workflow engine.

## 🎯 Overview

The EventStore interface provides **Temporal-level resilience** with **pluggable backends**. You can easily add support for new event streaming systems like Kafka, RabbitMQ, PostgreSQL, or any other backend.

## 🔧 EventStore Interface

All implementations must satisfy this interface:

```go
package eventstore

import (
    "context"
    "github.com/flunq-io/events/pkg/cloudevents"
)

type EventStore interface {
    // Publish publishes an event to a stream
    Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error

    // Subscribe subscribes to events from multiple streams
    // Returns channels for events and errors
    Subscribe(ctx context.Context, config SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error)

    // ReadHistory reads complete event history for replay/recovery
    ReadHistory(ctx context.Context, stream string, fromID string) ([]*cloudevents.CloudEvent, error)

    // CreateConsumerGroup creates consumer groups for load balancing
    CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error

    // Checkpoint management for crash recovery
    CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error
    GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error)

    // Close closes the event store connection
    Close() error
}

type SubscriptionConfig struct {
    Streams       []string // Streams to subscribe to
    ConsumerGroup string   // Consumer group name for load balancing
    ConsumerName  string   // Unique consumer name within group
    EventTypes    []string // Filter by event types (optional)
    WorkflowIDs   []string // Filter by workflow IDs (optional)
    FromMessageID string   // Start reading from this message ID
    BlockTime     time.Duration // How long to block waiting for new messages
    Count         int      // Maximum number of messages to read at once
}

type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

## 🚀 Step-by-Step Implementation

### Step 1: Create Package Structure

Create a new package under `worker/pkg/eventstore/yourbackend/`:

```
worker/pkg/eventstore/yourbackend/
├── yourbackend_store.go      # Main implementation
├── config.go                 # Configuration structs
├── serialization.go          # CloudEvent serialization
└── yourbackend_store_test.go # Unit tests
```

### Step 2: Implement the EventStore Interface

```go
// yourbackend_store.go
package yourbackend

import (
    "context"
    "fmt"
    "time"
    "github.com/flunq-io/events/pkg/cloudevents"
    "github.com/flunq-io/worker/pkg/eventstore"
)

type YourEventStore struct {
    client YourBackendClient
    logger eventstore.Logger
    config *Config
}

func NewYourEventStore(connectionString string, logger eventstore.Logger) (eventstore.EventStore, error) {
    config, err := parseConnectionString(connectionString)
    if err != nil {
        return nil, fmt.Errorf("invalid connection string: %w", err)
    }

    client, err := NewYourBackendClient(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    return &YourEventStore{
        client: client,
        logger: logger,
        config: config,
    }, nil
}

func (y *YourEventStore) Close() error {
    return y.client.Close()
}
```

### Step 3: Implement Event Publishing

```go
func (y *YourEventStore) Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error {
    // 1. Serialize CloudEvent to your backend's format
    data, err := y.serializeEvent(event)
    if err != nil {
        return fmt.Errorf("failed to serialize event: %w", err)
    }

    // 2. Publish to your backend with ordering guarantees
    messageID, err := y.client.Publish(ctx, stream, data)
    if err != nil {
        return fmt.Errorf("failed to publish event: %w", err)
    }

    // 3. Log for debugging
    y.logger.Debug("Published event", 
        "stream", stream, 
        "event_id", event.ID, 
        "message_id", messageID,
        "event_type", event.Type)

    return nil
}
```

### Step 4: Implement Event Subscription

```go
func (y *YourEventStore) Subscribe(ctx context.Context, config eventstore.SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error) {
    eventCh := make(chan *cloudevents.CloudEvent, 100)
    errorCh := make(chan error, 10)

    // 1. Create consumer group for load balancing
    err := y.CreateConsumerGroup(ctx, config.ConsumerGroup, config.Streams)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create consumer group: %w", err)
    }

    // 2. Start subscription loop in goroutine
    go y.subscriptionLoop(ctx, config, eventCh, errorCh)

    return eventCh, errorCh, nil
}

func (y *YourEventStore) subscriptionLoop(ctx context.Context, config eventstore.SubscriptionConfig, eventCh chan<- *cloudevents.CloudEvent, errorCh chan<- error) {
    defer close(eventCh)
    defer close(errorCh)

    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Read messages from your backend
            messages, err := y.client.ReadMessages(ctx, config)
            if err != nil {
                errorCh <- fmt.Errorf("failed to read messages: %w", err)
                time.Sleep(1 * time.Second) // Backoff on error
                continue
            }

            for _, msg := range messages {
                // Deserialize and filter events
                event, err := y.deserializeEvent(msg.Data)
                if err != nil {
                    y.logger.Error("Failed to deserialize event", "error", err)
                    continue
                }

                if y.shouldProcessEvent(event, config) {
                    eventCh <- event
                    
                    // Update checkpoint for crash recovery
                    err = y.CreateCheckpoint(ctx, config.ConsumerGroup, msg.Stream, msg.ID)
                    if err != nil {
                        y.logger.Error("Failed to update checkpoint", "error", err)
                    }
                }
            }
        }
    }
}
```

### Step 5: Implement Event History

```go
func (y *YourEventStore) ReadHistory(ctx context.Context, stream string, fromID string) ([]*cloudevents.CloudEvent, error) {
    // 1. Read all messages from stream starting at fromID
    messages, err := y.client.ReadRange(ctx, stream, fromID, "end")
    if err != nil {
        return nil, fmt.Errorf("failed to read history: %w", err)
    }

    // 2. Convert to CloudEvents
    events := make([]*cloudevents.CloudEvent, 0, len(messages))
    for _, msg := range messages {
        event, err := y.deserializeEvent(msg.Data)
        if err != nil {
            y.logger.Error("Failed to deserialize event in history", "error", err)
            continue
        }
        events = append(events, event)
    }

    y.logger.Debug("Read event history", 
        "stream", stream, 
        "from_id", fromID, 
        "event_count", len(events))

    return events, nil
}
```

### Step 6: Implement Consumer Groups and Checkpointing

```go
func (y *YourEventStore) CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error {
    return y.client.CreateConsumerGroup(ctx, groupName, streams)
}

func (y *YourEventStore) CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error {
    checkpointKey := fmt.Sprintf("checkpoint:%s:%s", groupName, stream)
    return y.client.SetCheckpoint(ctx, checkpointKey, messageID)
}

func (y *YourEventStore) GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error) {
    checkpointKey := fmt.Sprintf("checkpoint:%s:%s", groupName, stream)
    checkpoint, err := y.client.GetCheckpoint(ctx, checkpointKey)
    if err != nil {
        return "0", nil // Start from beginning if no checkpoint
    }
    return checkpoint, nil
}
```

## 🔧 Helper Functions

### Event Serialization

```go
// serialization.go
func (y *YourEventStore) serializeEvent(event *cloudevents.CloudEvent) ([]byte, error) {
    // Convert CloudEvent to your backend's format
    // Most backends can use JSON serialization
    return json.Marshal(event)
}

func (y *YourEventStore) deserializeEvent(data []byte) (*cloudevents.CloudEvent, error) {
    var event cloudevents.CloudEvent
    err := json.Unmarshal(data, &event)
    return &event, err
}
```

### Event Filtering

```go
func (y *YourEventStore) shouldProcessEvent(event *cloudevents.CloudEvent, config eventstore.SubscriptionConfig) bool {
    // Filter by event types
    if len(config.EventTypes) > 0 {
        found := false
        for _, eventType := range config.EventTypes {
            if event.Type == eventType {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }

    // Filter by workflow IDs
    if len(config.WorkflowIDs) > 0 {
        workflowID, ok := event.Extensions["workflowid"].(string)
        if !ok {
            return false
        }
        
        found := false
        for _, wfID := range config.WorkflowIDs {
            if workflowID == wfID {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }

    return true
}
```

## 🧪 Testing Your Implementation

Create comprehensive tests:

```go
// yourbackend_store_test.go
func TestYourEventStore_PublishAndSubscribe(t *testing.T) {
    // Test basic publish/subscribe functionality
}

func TestYourEventStore_EventHistory(t *testing.T) {
    // Test event history replay
}

func TestYourEventStore_ConsumerGroups(t *testing.T) {
    // Test load balancing with consumer groups
}

func TestYourEventStore_Checkpointing(t *testing.T) {
    // Test crash recovery with checkpoints
}
```

## 🔧 Integration

### Factory Pattern

Add your implementation to the factory:

```go
// factory.go
func NewEventStore(config *Config, logger eventstore.Logger) (eventstore.EventStore, error) {
    switch config.EventStoreType {
    case "redis":
        return redis.NewRedisEventStore(config.RedisURL, logger)
    case "yourbackend":
        return yourbackend.NewYourEventStore(config.YourBackendURL, logger)
    default:
        return nil, fmt.Errorf("unsupported event store type: %s", config.EventStoreType)
    }
}
```

### Configuration

```go
type Config struct {
    EventStoreType   string
    RedisURL         string
    YourBackendURL   string
}
```

## 🎯 Key Requirements

1. **Event Ordering**: Maintain order within streams for event sourcing
2. **Consumer Groups**: Support load balancing across multiple workers
3. **Checkpointing**: Enable crash recovery with last-processed message tracking
4. **Event Filtering**: Filter by event type and workflow ID
5. **History Replay**: Read complete event history from any point
6. **Error Handling**: Graceful error handling with retries and backoff

Your implementation will provide **Temporal-level resilience** with the flexibility to use any backend that meets your performance and operational requirements.
