package adapters

import (
	"context"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/worker/internal/interfaces"
)

// EventStoreAdapter adapts the Event Store client to the EventStore interface
type EventStoreAdapter struct {
	client *client.EventClient
	logger interfaces.Logger
}

// NewEventStoreAdapter creates a new Event Store adapter
func NewEventStoreAdapter(client *client.EventClient, logger interfaces.Logger) interfaces.EventStore {
	return &EventStoreAdapter{
		client: client,
		logger: logger,
	}
}

// GetEventHistory retrieves all events for a workflow
func (a *EventStoreAdapter) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	// Use the Event Store client to get event history
	events, err := a.client.GetEventHistory(ctx, workflowID)
	if err != nil {
		a.logger.Error("Failed to get event history from Event Store",
			"workflow_id", workflowID,
			"error", err)
		return nil, err
	}

	a.logger.Debug("Retrieved event history from Event Store",
		"workflow_id", workflowID,
		"event_count", len(events))

	return events, nil
}

// PublishEvent publishes a new event
func (a *EventStoreAdapter) PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Use the Event Store client to publish event
	if err := a.client.PublishEvent(ctx, event); err != nil {
		a.logger.Error("Failed to publish event to Event Store",
			"event_id", event.ID,
			"event_type", event.Type,
			"workflow_id", event.WorkflowID,
			"error", err)
		return err
	}

	a.logger.Debug("Published event to Event Store",
		"event_id", event.ID,
		"event_type", event.Type,
		"workflow_id", event.WorkflowID)

	return nil
}

// Subscribe creates a subscription for events
func (a *EventStoreAdapter) Subscribe(ctx context.Context, eventTypes []string, workflowIDs []string) (interfaces.EventSubscription, error) {
	// Use the Event Store client to create WebSocket subscription
	subscription, err := a.client.SubscribeWebSocket(ctx, eventTypes, workflowIDs, nil)
	if err != nil {
		a.logger.Error("Failed to create subscription to Event Store",
			"event_types", eventTypes,
			"workflow_ids", workflowIDs,
			"error", err)
		return nil, err
	}

	a.logger.Info("Created subscription to Event Store",
		"event_types", eventTypes,
		"workflow_ids", workflowIDs)

	// Wrap the subscription to match our interface
	return &EventSubscriptionAdapter{
		subscription: subscription,
		logger:       a.logger,
	}, nil
}

// EventSubscriptionAdapter adapts the Event Store subscription to our interface
type EventSubscriptionAdapter struct {
	subscription *client.WebSocketSubscription
	logger       interfaces.Logger
}

// Events returns the channel for receiving events
func (s *EventSubscriptionAdapter) Events() <-chan *cloudevents.CloudEvent {
	return s.subscription.Events()
}

// Errors returns the channel for receiving errors
func (s *EventSubscriptionAdapter) Errors() <-chan error {
	return s.subscription.Errors()
}

// Close closes the subscription
func (s *EventSubscriptionAdapter) Close() error {
	s.logger.Debug("Closing event subscription")
	return s.subscription.Close()
}
