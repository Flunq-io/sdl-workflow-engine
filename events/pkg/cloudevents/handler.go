package cloudevents

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// EventStore interface for storing and retrieving events
type EventStore interface {
	StoreEvent(ctx context.Context, event *CloudEvent) error
	GetEventHistory(ctx context.Context, workflowID string) ([]*CloudEvent, error)
	GetEventsSince(ctx context.Context, workflowID string, since string) ([]*CloudEvent, error)
	GetStreamEvents(ctx context.Context, streamID string, count int64) ([]*CloudEvent, error)
	GetStats(ctx context.Context) (map[string]interface{}, error)
	DeleteWorkflowEvents(ctx context.Context, workflowID string) error
}

// EventRouter interface for routing events to subscribers
type EventRouter interface {
	RouteEvent(event *CloudEvent)
}

// Handler handles CloudEvents operations
type Handler struct {
	store  EventStore
	router EventRouter
	logger *zap.Logger
}

// NewHandler creates a new CloudEvents handler
func NewHandler(store EventStore, router EventRouter, logger *zap.Logger) *Handler {
	return &Handler{
		store:  store,
		router: router,
		logger: logger,
	}
}

// PublishEvent publishes a new CloudEvent
func (h *Handler) PublishEvent(ctx context.Context, event *CloudEvent) error {
	// Validate the event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	// Store the event
	if err := h.store.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store event",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.Error(err))
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Route the event to subscribers
	h.router.RouteEvent(event)

	h.logger.Info("Event published successfully",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
		zap.String("source", event.Source),
		zap.String("workflow_id", event.WorkflowID))

	return nil
}

// GetEventHistory retrieves event history for a workflow
func (h *Handler) GetEventHistory(ctx context.Context, workflowID string) ([]*CloudEvent, error) {
	events, err := h.store.GetEventHistory(ctx, workflowID)
	if err != nil {
		h.logger.Error("Failed to get event history",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get event history: %w", err)
	}

	h.logger.Debug("Retrieved event history",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	return events, nil
}

// GetEventsSince retrieves events since a specific version
func (h *Handler) GetEventsSince(ctx context.Context, workflowID string, since string) ([]*CloudEvent, error) {
	events, err := h.store.GetEventsSince(ctx, workflowID, since)
	if err != nil {
		h.logger.Error("Failed to get events since version",
			zap.String("workflow_id", workflowID),
			zap.String("since", since),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get events since %s: %w", since, err)
	}

	h.logger.Debug("Retrieved events since version",
		zap.String("workflow_id", workflowID),
		zap.String("since", since),
		zap.Int("event_count", len(events)))

	return events, nil
}

// GetStreamEvents retrieves events from a stream
func (h *Handler) GetStreamEvents(ctx context.Context, streamID string, count int64) ([]*CloudEvent, error) {
	events, err := h.store.GetStreamEvents(ctx, streamID, count)
	if err != nil {
		h.logger.Error("Failed to get stream events",
			zap.String("stream_id", streamID),
			zap.Int64("count", count),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get stream events: %w", err)
	}

	h.logger.Debug("Retrieved stream events",
		zap.String("stream_id", streamID),
		zap.Int("event_count", len(events)))

	return events, nil
}

// GetStats retrieves event store statistics
func (h *Handler) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats, err := h.store.GetStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get event store stats", zap.Error(err))
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return stats, nil
}

// ReplayEvents replays events for a workflow
func (h *Handler) ReplayEvents(ctx context.Context, workflowID string) error {
	// Get all events for the workflow
	events, err := h.store.GetEventHistory(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get event history for replay: %w", err)
	}

	// Replay each event
	for _, event := range events {
		// Create a new event with replay marker
		replayEvent := event.Clone()
		replayEvent.SetExtension("replay", true)
		replayEvent.SetExtension("original_event_id", event.ID)

		// Route the replay event
		h.router.RouteEvent(replayEvent)
	}

	h.logger.Info("Events replayed successfully",
		zap.String("workflow_id", workflowID),
		zap.Int("event_count", len(events)))

	return nil
}

// DeleteWorkflowEvents deletes all events for a workflow
func (h *Handler) DeleteWorkflowEvents(ctx context.Context, workflowID string) error {
	err := h.store.DeleteWorkflowEvents(ctx, workflowID)
	if err != nil {
		h.logger.Error("Failed to delete workflow events",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		return fmt.Errorf("failed to delete workflow events: %w", err)
	}

	h.logger.Info("Workflow events deleted",
		zap.String("workflow_id", workflowID))

	return nil
}
