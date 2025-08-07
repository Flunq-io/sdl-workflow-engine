package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/core"
	"github.com/flunq-io/events/internal/interfaces"
	"github.com/flunq-io/shared/pkg/cloudevents"
)

// SimpleHTTPHandler handles HTTP requests for the Event Store using the core interfaces
type SimpleHTTPHandler struct {
	eventStore *core.EventStore
	logger     *zap.Logger
}

// NewSimpleHTTPHandler creates a new simple HTTP handler
func NewSimpleHTTPHandler(eventStore *core.EventStore, logger *zap.Logger) *SimpleHTTPHandler {
	return &SimpleHTTPHandler{
		eventStore: eventStore,
		logger:     logger,
	}
}

// PublishEvent handles POST /api/v1/events
func (h *SimpleHTTPHandler) PublishEvent(c *gin.Context) {
	var event cloudevents.CloudEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		h.logger.Error("Failed to bind event JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event format"})
		return
	}

	if err := h.eventStore.PublishEvent(c.Request.Context(), &event); err != nil {
		h.logger.Error("Failed to publish event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish event"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Event published successfully",
		"event_id": event.ID,
	})
}

// GetEventHistory handles GET /api/v1/events/:workflowId
func (h *SimpleHTTPHandler) GetEventHistory(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	events, err := h.eventStore.GetEventHistory(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("Failed to get event history", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get event history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id": workflowID,
		"events":      events,
		"count":       len(events),
	})
}

// GetEventsSince handles GET /api/v1/events/:workflowId/since/:version
func (h *SimpleHTTPHandler) GetEventsSince(c *gin.Context) {
	workflowID := c.Param("workflowId")
	since := c.Param("version")

	if workflowID == "" || since == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id and version are required"})
		return
	}

	events, err := h.eventStore.GetEventsSince(c.Request.Context(), workflowID, since)
	if err != nil {
		h.logger.Error("Failed to get events since version", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id": workflowID,
		"since":       since,
		"events":      events,
		"count":       len(events),
	})
}

// GetExecutionHistory handles GET /api/v1/executions/:executionId/events
func (h *SimpleHTTPHandler) GetExecutionHistory(c *gin.Context) {
	executionID := c.Param("executionId")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_id is required"})
		return
	}

	events, err := h.eventStore.GetExecutionHistory(c.Request.Context(), executionID)
	if err != nil {
		h.logger.Error("Failed to get execution history", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get execution history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"execution_id": executionID,
		"events":       events,
		"count":        len(events),
	})
}

// GetWorkflowExecutions handles GET /api/v1/workflows/:workflowId/executions
func (h *SimpleHTTPHandler) GetWorkflowExecutions(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	executionIDs, err := h.eventStore.GetWorkflowExecutions(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("Failed to get workflow executions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow executions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id":   workflowID,
		"execution_ids": executionIDs,
		"count":         len(executionIDs),
	})
}

// Subscribe handles POST /api/v1/subscribe
func (h *SimpleHTTPHandler) Subscribe(c *gin.Context) {
	var subscription interfaces.Subscription
	if err := c.ShouldBindJSON(&subscription); err != nil {
		h.logger.Error("Failed to bind subscription JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription format"})
		return
	}

	sub, err := h.eventStore.Subscribe(c.Request.Context(), &subscription)
	if err != nil {
		h.logger.Error("Failed to create subscription", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
		return
	}

	// For HTTP subscriptions, we'll return the subscription info
	// In a real implementation, you might want to upgrade to WebSocket here
	c.JSON(http.StatusCreated, gin.H{
		"message":      "Subscription created successfully",
		"subscription": subscription,
		"note":         "Use WebSocket endpoint for real-time events",
	})

	// Close the subscription since we can't maintain it over HTTP
	sub.Close()
}

// GetStats handles GET /api/v1/admin/stats
func (h *SimpleHTTPHandler) GetStats(c *gin.Context) {
	stats, err := h.eventStore.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ReplayEvents handles POST /api/v1/admin/replay/:workflowId
func (h *SimpleHTTPHandler) ReplayEvents(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	if err := h.eventStore.ReplayEvents(c.Request.Context(), workflowID); err != nil {
		h.logger.Error("Failed to replay events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to replay events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Events replayed successfully",
		"workflow_id": workflowID,
	})
}

// Health handles GET /health
func (h *SimpleHTTPHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "event-store",
	})
}

// Metrics handles GET /metrics
func (h *SimpleHTTPHandler) Metrics(c *gin.Context) {
	// TODO: Implement Prometheus metrics
	c.JSON(http.StatusOK, gin.H{
		"metrics": "not implemented yet",
	})
}
