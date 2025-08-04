package interfaces

import (
	"context"

	"github.com/flunq-io/events/pkg/cloudevents"
)

// EventStore defines the interface for event storage and publishing
type EventStore interface {
	// PublishEvent publishes a new event
	PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error
}

// EventStream defines the interface for event streaming
type EventStream interface {
	// Subscribe to events with filters
	Subscribe(ctx context.Context, filters EventStreamFilters) (EventStreamSubscription, error)
}

// EventStreamFilters defines filters for event subscription
type EventStreamFilters struct {
	EventTypes  []string // Filter by event types
	WorkflowIDs []string // Filter by workflow IDs
	Sources     []string // Filter by event sources
}

// EventStreamSubscription represents an active event subscription
type EventStreamSubscription interface {
	// Events returns a channel of incoming events
	Events() <-chan *cloudevents.CloudEvent

	// Errors returns a channel of subscription errors
	Errors() <-chan error

	// Close closes the subscription
	Close() error
}
