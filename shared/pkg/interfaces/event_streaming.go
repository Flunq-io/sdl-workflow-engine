package interfaces

import (
	"context"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventStream defines the generic interface for event streaming
// Can be implemented by Redis Streams, Kafka, RabbitMQ, NATS, etc.
type EventStream interface {
	// Subscribe to events with filters
	Subscribe(ctx context.Context, filters StreamFilters) (StreamSubscription, error)

	// Publish an event to the stream
	Publish(ctx context.Context, event *cloudevents.CloudEvent) error

	// GetEventHistory retrieves all events for a specific workflow
	GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error)

	// CreateConsumerGroup creates a consumer group
	CreateConsumerGroup(ctx context.Context, groupName string) error

	// DeleteConsumerGroup deletes a consumer group
	DeleteConsumerGroup(ctx context.Context, groupName string) error

	// GetStreamInfo returns stream information
	GetStreamInfo(ctx context.Context) (*StreamInfo, error)

	// ReclaimPending reclaims pending messages idle longer than minIdle to the given consumer
	ReclaimPending(ctx context.Context, consumerGroup, consumerName string, minIdle time.Duration) (int, error)

	// Close closes the stream connection
	Close() error
}

// StreamFilters defines filters for event subscription
type StreamFilters struct {
	EventTypes    []string          `json:"event_types"`    // Filter by event types
	WorkflowIDs   []string          `json:"workflow_ids"`   // Filter by workflow IDs
	Sources       []string          `json:"sources"`        // Filter by event sources
	StartFrom     string            `json:"start_from"`     // Start reading from specific position
	ConsumerGroup string            `json:"consumer_group"` // Consumer group name
	ConsumerName  string            `json:"consumer_name"`  // Consumer name within group
	BatchCount    int               `json:"batch_count"`    // Number of messages to request per read (implementation-specific)
	BlockTimeout  time.Duration     `json:"block_timeout"`  // Max time to block waiting for messages
	Extensions    map[string]string `json:"extensions"`     // Filter by extension attributes
}

// StreamSubscription represents an active subscription to an event stream
type StreamSubscription interface {
	// Events returns channel for receiving events
	Events() <-chan *cloudevents.CloudEvent

	// Errors returns channel for receiving errors
	Errors() <-chan error

	// Acknowledge acknowledges processing of an event
	// eventID can be an implementation-specific token; for Redis Streams this may be
	// either the raw message ID or a composite "stream|id" ack key.
	Acknowledge(ctx context.Context, eventID string) error

	// Close closes the subscription
	Close() error
}

// ConsumerGroupInfo represents per-consumer-group stats
type ConsumerGroupInfo struct {
	Name      string `json:"name"`
	Consumers int64  `json:"consumers"`
	Pending   int64  `json:"pending"`
}

// StreamInfo represents stream information
type StreamInfo struct {
	StreamName      string              `json:"stream_name"`
	MessageCount    int64               `json:"message_count"`
	ConsumerGroups  []string            `json:"consumer_groups"`
	Groups          []ConsumerGroupInfo `json:"groups"`
	PendingCount    int64               `json:"pending_count"`
	LastMessageTime time.Time           `json:"last_message_time"`
	Metadata        map[string]string   `json:"metadata"`
}

// StreamConfig represents stream configuration
type StreamConfig struct {
	Type            string            `json:"type"`              // redis, kafka, rabbitmq, nats, etc.
	Brokers         []string          `json:"brokers"`           // Broker addresses
	Topic           string            `json:"topic"`             // Topic/stream name
	ConsumerGroup   string            `json:"consumer_group"`    // Consumer group
	ConsumerName    string            `json:"consumer_name"`     // Consumer name
	Options         map[string]string `json:"options"`           // Additional stream-specific options
	RetryPolicy     *RetryPolicy      `json:"retry_policy"`      // Retry configuration
	DeadLetterTopic string            `json:"dead_letter_topic"` // Dead letter topic for failed messages
}

// RetryPolicy defines retry behavior for stream operations
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
}
