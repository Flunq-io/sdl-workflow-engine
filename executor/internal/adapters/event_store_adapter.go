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

// GetEventHistory retrieves all events for a workflow via HTTP API
func (a *EventStoreAdapter) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	// Use the Event Store client to get event history
	events, err := a.client.GetEventHistory(ctx, workflowID)
	if err != nil {
		a.logger.Error("Failed to get event history from Event Store",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, err
	}

	a.logger.Debug("Retrieved event history from Event Store",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	return events, nil
}
