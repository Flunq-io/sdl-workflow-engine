package core

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/interfaces"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// EventStore is the core event store service
type EventStore struct {
	storage   interfaces.EventStorage
	publisher interfaces.EventPublisher
	logger    *zap.Logger
}

// NewEventStore creates a new event store
func NewEventStore(
	storage interfaces.EventStorage,
	publisher interfaces.EventPublisher,
	logger *zap.Logger,
) *EventStore {
	return &EventStore{
		storage:   storage,
		publisher: publisher,
		logger:    logger,
	}
}

// PublishEvent publishes and stores an event
func (e *EventStore) PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Validate the event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	// Store the event first (for durability)
	if err := e.storage.Store(ctx, event); err != nil {
		e.logger.Error("Failed to store event",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.Error(err))
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Then publish to subscribers (best effort)
	if err := e.publisher.Publish(ctx, event); err != nil {
		e.logger.Error("Failed to publish event",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.Error(err))
		// Don't fail the operation - event is already stored
	}

	e.logger.Info("Event published successfully",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
		zap.String("source", event.Source),
		zap.String("workflow_id", event.WorkflowID))

	return nil
}

// GetEventHistory retrieves event history for a workflow
func (e *EventStore) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	events, err := e.storage.GetHistory(ctx, workflowID)
	if err != nil {
		e.logger.Error("Failed to get event history",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get event history: %w", err)
	}

	e.logger.Debug("Retrieved event history",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	return events, nil
}

// GetEventsSince retrieves events since a specific version
func (e *EventStore) GetEventsSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error) {
	events, err := e.storage.GetSince(ctx, workflowID, since)
	if err != nil {
		e.logger.Error("Failed to get events since version",
			zap.String("workflow_id", workflowID),
			zap.String("since", since),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get events since %s: %w", since, err)
	}

	e.logger.Debug("Retrieved events since version",
		zap.String("workflow_id", workflowID),
		zap.String("since", since),
		zap.Int("event_count", len(events)))

	return events, nil
}

// Subscribe creates a subscription for events
func (e *EventStore) Subscribe(ctx context.Context, subscription *interfaces.Subscription) (interfaces.EventSubscription, error) {
	sub, err := e.publisher.Subscribe(ctx, subscription)
	if err != nil {
		e.logger.Error("Failed to create subscription",
			zap.String("service_name", subscription.ServiceName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	e.logger.Info("Subscription created",
		zap.String("service_name", subscription.ServiceName),
		zap.Strings("event_types", subscription.EventTypes),
		zap.Strings("workflow_ids", subscription.WorkflowIDs))

	return sub, nil
}

// GetStats retrieves event store statistics
func (e *EventStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats, err := e.storage.GetStats(ctx)
	if err != nil {
		e.logger.Error("Failed to get event store stats", zap.Error(err))
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return stats, nil
}

// ReplayEvents replays events for a workflow
func (e *EventStore) ReplayEvents(ctx context.Context, workflowID string) error {
	// Get all events for the workflow
	events, err := e.storage.GetHistory(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get event history for replay: %w", err)
	}

	// Replay each event
	for _, event := range events {
		// Create a new event with replay marker
		replayEvent := event.Clone()
		replayEvent.SetExtension("replay", true)
		replayEvent.SetExtension("original_event_id", event.ID)

		// Publish the replay event
		if err := e.publisher.Publish(ctx, replayEvent); err != nil {
			e.logger.Error("Failed to publish replay event",
				zap.String("original_event_id", event.ID),
				zap.Error(err))
			// Continue with other events
		}
	}

	e.logger.Info("Events replayed successfully",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	return nil
}

// Close closes the event store
func (e *EventStore) Close() error {
	if err := e.storage.Close(); err != nil {
		e.logger.Error("Failed to close storage", zap.Error(err))
	}
	
	if err := e.publisher.Close(); err != nil {
		e.logger.Error("Failed to close publisher", zap.Error(err))
	}
	
	return nil
}
