package eventstore

import (
	"context"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventStore defines the interface for event storage and streaming
// This allows pluggable backends (Redis, Kafka, RabbitMQ, etc.)
type EventStore interface {
	// Publish publishes an event to a stream
	Publish(ctx context.Context, stream string, event *cloudevents.CloudEvent) error

	// Subscribe subscribes to events from multiple streams starting from a specific message ID
	// Returns a channel of events and an error channel
	Subscribe(ctx context.Context, config SubscriptionConfig) (<-chan *cloudevents.CloudEvent, <-chan error, error)

	// ReadHistory reads event history from a stream starting from a specific message ID
	ReadHistory(ctx context.Context, stream string, fromID string) ([]*cloudevents.CloudEvent, error)

	// CreateConsumerGroup creates a consumer group for load balancing
	CreateConsumerGroup(ctx context.Context, groupName string, streams []string) error

	// CreateCheckpoint saves the last processed message ID for a consumer group
	CreateCheckpoint(ctx context.Context, groupName, stream, messageID string) error

	// GetLastCheckpoint gets the last processed message ID for a consumer group
	GetLastCheckpoint(ctx context.Context, groupName, stream string) (string, error)

	// Close closes the event store connection
	Close() error
}

// SubscriptionConfig configures event subscription
type SubscriptionConfig struct {
	// Streams to subscribe to
	Streams []string

	// Consumer group name for load balancing
	ConsumerGroup string

	// Consumer name (unique within group)
	ConsumerName string

	// Event types to filter (empty = all types)
	EventTypes []string

	// Workflow IDs to filter (empty = all workflows)
	WorkflowIDs []string

	// Starting message ID ("0" = from beginning, ">" = only new messages)
	FromMessageID string

	// Block time for reading (0 = non-blocking)
	BlockTime time.Duration

	// Maximum number of messages to read at once
	Count int64
}

// ReadOptions configures history reading
type ReadOptions struct {
	// Starting message ID ("0" = from beginning)
	FromID string

	// Ending message ID ("+" = to end)
	ToID string

	// Maximum number of messages to read
	Count int64
}

// EventHandler handles incoming events
type EventHandler func(event *cloudevents.CloudEvent) error

// Logger interface for event store implementations
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}
