package adapters

import (
	"context"

	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/events/pkg/cloudevents"
	"github.com/flunq-io/executor/internal/interfaces"
)

// EventStoreAdapter adapts the Event Store client to the EventStore interface
type EventStoreAdapter struct {
	client *client.EventClient
	logger *zap.Logger
}

// NewEventStoreAdapter creates a new EventStoreAdapter
func NewEventStoreAdapter(client *client.EventClient, logger *zap.Logger) interfaces.EventStore {
	return &EventStoreAdapter{
		client: client,
		logger: logger,
	}
}

// PublishEvent publishes a new event to the Event Store
func (a *EventStoreAdapter) PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Use the Event Store client to publish the event
	err := a.client.PublishEvent(ctx, event)
	if err != nil {
		a.logger.Error("Failed to publish event to Event Store",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.String("workflow_id", event.WorkflowID),
			zap.Error(err))
		return err
	}

	a.logger.Debug("Successfully published event to Event Store",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
		zap.String("workflow_id", event.WorkflowID))

	return nil
}
