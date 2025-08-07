package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventClient provides a client interface to the Event Store service
type EventClient struct {
	baseURL     string
	httpClient  *http.Client
	logger      *zap.Logger
	serviceName string
}

// NewEventClient creates a new Event Store client
func NewEventClient(baseURL, serviceName string, logger *zap.Logger) *EventClient {
	return &EventClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger,
		serviceName: serviceName,
	}
}

// PublishEvent publishes an event to the Event Store
func (c *EventClient) PublishEvent(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Set source if not already set
	if event.Source == "" {
		event.Source = c.serviceName
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/events", bytes.NewBuffer(eventData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to publish event, status: %d", resp.StatusCode)
	}

	c.logger.Debug("Event published successfully",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type))

	return nil
}

// GetEventHistory retrieves event history for a workflow
func (c *EventClient) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	url := fmt.Sprintf("%s/api/v1/events/%s", c.baseURL, workflowID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get event history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get event history, status: %d", resp.StatusCode)
	}

	var response struct {
		Events []*cloudevents.CloudEvent `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Events, nil
}

// SubscribeWebSocket creates a WebSocket subscription to receive real-time events
func (c *EventClient) SubscribeWebSocket(ctx context.Context, eventTypes, workflowIDs []string, filters map[string]string) (*WebSocketSubscription, error) {
	// Build WebSocket URL with query parameters
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Convert to WebSocket scheme
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}

	u.Path = "/ws"

	// Add query parameters
	query := u.Query()
	query.Set("service", c.serviceName)

	for _, eventType := range eventTypes {
		query.Add("event_types", eventType)
	}

	for _, workflowID := range workflowIDs {
		query.Add("workflow_ids", workflowID)
	}

	for key, value := range filters {
		query.Set(key, value)
	}

	u.RawQuery = query.Encode()

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	subscription := &WebSocketSubscription{
		conn:   conn,
		logger: c.logger,
		events: make(chan *cloudevents.CloudEvent, 100),
		errors: make(chan error, 10),
		done:   make(chan struct{}),
	}

	// Start reading messages
	go subscription.readMessages()

	c.logger.Info("WebSocket subscription established",
		zap.String("service", c.serviceName),
		zap.Strings("event_types", eventTypes),
		zap.Strings("workflow_ids", workflowIDs))

	return subscription, nil
}

// WebSocketSubscription represents an active WebSocket subscription
type WebSocketSubscription struct {
	conn   *websocket.Conn
	logger *zap.Logger
	events chan *cloudevents.CloudEvent
	errors chan error
	done   chan struct{}
}

// Events returns the channel for receiving events
func (s *WebSocketSubscription) Events() <-chan *cloudevents.CloudEvent {
	return s.events
}

// Errors returns the channel for receiving errors
func (s *WebSocketSubscription) Errors() <-chan error {
	return s.errors
}

// Done returns the channel that's closed when the subscription ends
func (s *WebSocketSubscription) Done() <-chan struct{} {
	return s.done
}

// Close closes the WebSocket subscription
func (s *WebSocketSubscription) Close() error {
	close(s.done)
	return s.conn.Close()
}

// readMessages reads messages from the WebSocket connection
func (s *WebSocketSubscription) readMessages() {
	defer close(s.events)
	defer close(s.errors)

	for {
		select {
		case <-s.done:
			return
		default:
			messageType, message, err := s.conn.ReadMessage()
			if err != nil {
				s.errors <- fmt.Errorf("failed to read WebSocket message: %w", err)
				return
			}

			if messageType == websocket.TextMessage {
				var event cloudevents.CloudEvent
				if err := json.Unmarshal(message, &event); err != nil {
					s.errors <- fmt.Errorf("failed to unmarshal event: %w", err)
					continue
				}

				select {
				case s.events <- &event:
				case <-s.done:
					return
				default:
					s.logger.Warn("Event channel is full, dropping event",
						zap.String("event_id", event.ID))
				}
			}
		}
	}
}

// Helper functions for creating common events

// NewWorkflowStartedEvent creates a workflow started event
func NewWorkflowStartedEvent(workflowID string, data interface{}) *cloudevents.CloudEvent {
	event := cloudevents.NewWorkflowEvent("", "", cloudevents.WorkflowStarted, workflowID)
	event.SetData(data)
	return event
}

// NewWorkflowCompletedEvent creates a workflow completed event
func NewWorkflowCompletedEvent(workflowID string, data interface{}) *cloudevents.CloudEvent {
	event := cloudevents.NewWorkflowEvent("", "", cloudevents.WorkflowCompleted, workflowID)
	event.SetData(data)
	return event
}

// NewTaskStartedEvent creates a task started event
func NewTaskStartedEvent(workflowID, executionID, taskID string, data interface{}) *cloudevents.CloudEvent {
	event := cloudevents.NewTaskEvent("", "", cloudevents.TaskStarted, workflowID, executionID, taskID)
	event.SetData(data)
	return event
}

// NewTaskCompletedEvent creates a task completed event
func NewTaskCompletedEvent(workflowID, executionID, taskID string, data interface{}) *cloudevents.CloudEvent {
	event := cloudevents.NewTaskEvent("", "", cloudevents.TaskCompleted, workflowID, executionID, taskID)
	event.SetData(data)
	return event
}
