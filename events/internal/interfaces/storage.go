package interfaces

import (
	"context"

	"github.com/flunq-io/events/pkg/cloudevents"
)

// EventStorage defines the interface for event persistence
type EventStorage interface {
	// Store persists an event
	Store(ctx context.Context, event *cloudevents.CloudEvent) error
	
	// GetHistory retrieves all events for a workflow
	GetHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error)
	
	// GetSince retrieves events since a specific version/timestamp
	GetSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error)
	
	// GetStream retrieves events from a stream
	GetStream(ctx context.Context, streamID string, count int64) ([]*cloudevents.CloudEvent, error)
	
	// Delete removes all events for a workflow
	Delete(ctx context.Context, workflowID string) error
	
	// GetStats returns storage statistics
	GetStats(ctx context.Context) (map[string]interface{}, error)
	
	// Close closes the storage connection
	Close() error
}

// EventPublisher defines the interface for event publishing/streaming
type EventPublisher interface {
	// Publish publishes an event to subscribers
	Publish(ctx context.Context, event *cloudevents.CloudEvent) error
	
	// Subscribe creates a subscription for events
	Subscribe(ctx context.Context, subscription *Subscription) (EventSubscription, error)
	
	// Close closes the publisher
	Close() error
}

// EventSubscription represents an active event subscription
type EventSubscription interface {
	// Events returns the channel for receiving events
	Events() <-chan *cloudevents.CloudEvent
	
	// Errors returns the channel for receiving errors
	Errors() <-chan error
	
	// Close closes the subscription
	Close() error
}

// Subscription represents subscription parameters
type Subscription struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	EventTypes  []string          `json:"event_types"`
	WorkflowIDs []string          `json:"workflow_ids"`
	Filters     map[string]string `json:"filters"`
}

// SubscriberManager defines the interface for managing subscribers
type SubscriberManager interface {
	// AddSubscriber adds a new subscriber
	AddSubscriber(subscription *Subscription) (string, error)
	
	// RemoveSubscriber removes a subscriber
	RemoveSubscriber(subscriberID string) error
	
	// GetSubscribers returns all active subscribers
	GetSubscribers() []SubscriberInfo
	
	// NotifySubscribers notifies relevant subscribers of an event
	NotifySubscribers(ctx context.Context, event *cloudevents.CloudEvent) error
	
	// Close closes the subscriber manager
	Close() error
}

// SubscriberInfo represents information about a subscriber
type SubscriberInfo struct {
	ID           string       `json:"id"`
	ServiceName  string       `json:"service_name"`
	Subscription Subscription `json:"subscription"`
	Connected    bool         `json:"connected"`
	EventCount   int64        `json:"event_count"`
	LastSeen     string       `json:"last_seen"`
}
