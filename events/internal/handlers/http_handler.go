package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/subscriber"
	"github.com/flunq-io/shared/pkg/cloudevents"
)

// HTTPHandler handles HTTP requests for the Event Store
type HTTPHandler struct {
	cloudEventsHandler *cloudevents.Handler
	subscriberManager  *subscriber.Manager
	logger             *zap.Logger
	upgrader           websocket.Upgrader
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(
	cloudEventsHandler *cloudevents.Handler,
	subscriberManager *subscriber.Manager,
	logger *zap.Logger,
) *HTTPHandler {
	return &HTTPHandler{
		cloudEventsHandler: cloudEventsHandler,
		subscriberManager:  subscriberManager,
		logger:             logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for now - should be configured properly in production
				return true
			},
		},
	}
}

// PublishEvent handles POST /api/v1/events
func (h *HTTPHandler) PublishEvent(c *gin.Context) {
	var event cloudevents.CloudEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		h.logger.Error("Failed to bind event JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event format"})
		return
	}

	if err := h.cloudEventsHandler.PublishEvent(c.Request.Context(), &event); err != nil {
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
func (h *HTTPHandler) GetEventHistory(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	events, err := h.cloudEventsHandler.GetEventHistory(c.Request.Context(), workflowID)
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
func (h *HTTPHandler) GetEventsSince(c *gin.Context) {
	workflowID := c.Param("workflowId")
	since := c.Param("version")

	if workflowID == "" || since == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id and version are required"})
		return
	}

	events, err := h.cloudEventsHandler.GetEventsSince(c.Request.Context(), workflowID, since)
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

// GetStreamEvents handles GET /api/v1/events/stream/:streamId
func (h *HTTPHandler) GetStreamEvents(c *gin.Context) {
	streamID := c.Param("streamId")
	if streamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stream_id is required"})
		return
	}

	countStr := c.DefaultQuery("count", "100")
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid count parameter"})
		return
	}

	events, err := h.cloudEventsHandler.GetStreamEvents(c.Request.Context(), streamID, count)
	if err != nil {
		h.logger.Error("Failed to get stream events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stream events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stream_id": streamID,
		"events":    events,
		"count":     len(events),
	})
}

// GetExecutionHistory handles GET /api/v1/executions/:executionId/events
func (h *HTTPHandler) GetExecutionHistory(c *gin.Context) {
	executionID := c.Param("executionId")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_id is required"})
		return
	}

	events, err := h.cloudEventsHandler.GetExecutionHistory(c.Request.Context(), executionID)
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
func (h *HTTPHandler) GetWorkflowExecutions(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	executionIDs, err := h.cloudEventsHandler.GetWorkflowExecutions(c.Request.Context(), workflowID)
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

// CreateSubscription handles POST /api/v1/subscriptions
func (h *HTTPHandler) CreateSubscription(c *gin.Context) {
	var subscription subscriber.Subscription
	if err := c.ShouldBindJSON(&subscription); err != nil {
		h.logger.Error("Failed to bind subscription JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription format"})
		return
	}

	h.subscriberManager.CreateSubscription(&subscription)

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Subscription created successfully",
		"subscription": subscription,
	})
}

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *HTTPHandler) ListSubscriptions(c *gin.Context) {
	subscriptions := h.subscriberManager.GetSubscriptions()

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": subscriptions,
		"count":         len(subscriptions),
	})
}

// DeleteSubscription handles DELETE /api/v1/subscriptions/:id
func (h *HTTPHandler) DeleteSubscription(c *gin.Context) {
	subscriptionID := c.Param("id")
	if subscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription_id is required"})
		return
	}

	h.subscriberManager.DeleteSubscription(subscriptionID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription deleted successfully",
	})
}

// GetStats handles GET /api/v1/admin/stats
func (h *HTTPHandler) GetStats(c *gin.Context) {
	stats, err := h.cloudEventsHandler.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	// Add subscriber stats
	subscribers := h.subscriberManager.GetSubscribers()
	stats["active_subscribers"] = len(subscribers)

	c.JSON(http.StatusOK, stats)
}

// GetSubscribers handles GET /api/v1/admin/subscribers
func (h *HTTPHandler) GetSubscribers(c *gin.Context) {
	subscribers := h.subscriberManager.GetSubscribers()

	c.JSON(http.StatusOK, gin.H{
		"subscribers": subscribers,
		"count":       len(subscribers),
	})
}

// ReplayEvents handles POST /api/v1/admin/replay/:workflowId
func (h *HTTPHandler) ReplayEvents(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	if err := h.cloudEventsHandler.ReplayEvents(c.Request.Context(), workflowID); err != nil {
		h.logger.Error("Failed to replay events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to replay events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Events replayed successfully",
		"workflow_id": workflowID,
	})
}

// WebSocketHandler handles WebSocket connections for real-time event streaming
func (h *HTTPHandler) WebSocketHandler(c *gin.Context) {
	// Get service name and subscription from query parameters
	serviceName := c.Query("service")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service parameter is required"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}
	defer ws.Close()

	// Create subscription from query parameters
	subscription := &subscriber.Subscription{
		ServiceName: serviceName,
		EventTypes:  c.QueryArray("event_types"),
		WorkflowIDs: c.QueryArray("workflow_ids"),
		Filters:     make(map[string]string),
	}

	// Add custom filters from query parameters
	for key, values := range c.Request.URL.Query() {
		if key != "service" && key != "event_types" && key != "workflow_ids" && len(values) > 0 {
			subscription.Filters[key] = values[0]
		}
	}

	// Add subscriber
	subscriber := h.subscriberManager.AddWebSocketSubscriber(ws, serviceName, subscription)
	defer h.subscriberManager.RemoveSubscriber(subscriber.ID)

	h.logger.Info("WebSocket subscriber connected",
		zap.String("subscriber_id", subscriber.ID),
		zap.String("service_name", serviceName))

	// Keep connection alive and handle ping/pong
	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			h.logger.Info("WebSocket connection closed",
				zap.String("subscriber_id", subscriber.ID),
				zap.Error(err))
			break
		}

		// Handle ping messages
		if messageType == websocket.PingMessage {
			if err := ws.WriteMessage(websocket.PongMessage, message); err != nil {
				h.logger.Error("Failed to send pong", zap.Error(err))
				break
			}
		}
	}
}

// Health handles GET /health
func (h *HTTPHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "event-store",
	})
}

// Metrics handles GET /metrics
func (h *HTTPHandler) Metrics(c *gin.Context) {
	// TODO: Implement Prometheus metrics
	c.JSON(http.StatusOK, gin.H{
		"metrics": "not implemented yet",
	})
}
