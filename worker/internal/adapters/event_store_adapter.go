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
