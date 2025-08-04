package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/cloudevents"
)

// ConnectionType represents the type of connection
type ConnectionType string

const (
	WebSocketConnection ConnectionType = "websocket"
	GRPCConnection     ConnectionType = "grpc"
	HTTPConnection     ConnectionType = "http"
)

// Subscription represents a subscriber's event subscription
type Subscription struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	EventTypes  []string          `json:"event_types"`
	WorkflowIDs []string          `json:"workflow_ids"`
	Filters     map[string]string `json:"filters"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Subscriber represents an active subscriber connection
type Subscriber struct {
	ID           string         `json:"id"`
	ServiceName  string         `json:"service_name"`
	ConnectionType ConnectionType `json:"connection_type"`
	Subscription *Subscription  `json:"subscription"`
	WebSocket    *websocket.Conn `json:"-"`
	GRPCStream   interface{}     `json:"-"` // gRPC stream interface
	LastSeen     time.Time       `json:"last_seen"`
	Connected    bool            `json:"connected"`
	EventCount   int64           `json:"event_count"`
}

// Manager manages all subscriber connections and subscriptions
type Manager struct {
	subscribers   map[string]*Subscriber
	subscriptions map[string]*Subscription
	mutex         sync.RWMutex
	logger        *zap.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewManager creates a new subscriber manager
func NewManager(logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		subscribers:   make(map[string]*Subscriber),
		subscriptions: make(map[string]*Subscription),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the subscriber manager background processes
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupStaleConnections()
		}
	}
}

// Stop stops the subscriber manager
func (m *Manager) Stop() {
	m.cancel()
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Close all WebSocket connections
	for _, subscriber := range m.subscribers {
		if subscriber.WebSocket != nil {
			subscriber.WebSocket.Close()
		}
	}
}

// AddWebSocketSubscriber adds a new WebSocket subscriber
func (m *Manager) AddWebSocketSubscriber(ws *websocket.Conn, serviceName string, subscription *Subscription) *Subscriber {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	subscriber := &Subscriber{
		ID:             uuid.New().String(),
		ServiceName:    serviceName,
		ConnectionType: WebSocketConnection,
		Subscription:   subscription,
		WebSocket:      ws,
		LastSeen:       time.Now(),
		Connected:      true,
		EventCount:     0,
	}

	m.subscribers[subscriber.ID] = subscriber
	if subscription != nil {
		m.subscriptions[subscription.ID] = subscription
	}

	m.logger.Info("WebSocket subscriber added",
		zap.String("subscriber_id", subscriber.ID),
		zap.String("service_name", serviceName))

	return subscriber
}

// AddGRPCSubscriber adds a new gRPC subscriber
func (m *Manager) AddGRPCSubscriber(stream interface{}, serviceName string, subscription *Subscription) *Subscriber {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	subscriber := &Subscriber{
		ID:             uuid.New().String(),
		ServiceName:    serviceName,
		ConnectionType: GRPCConnection,
		Subscription:   subscription,
		GRPCStream:     stream,
		LastSeen:       time.Now(),
		Connected:      true,
		EventCount:     0,
	}

	m.subscribers[subscriber.ID] = subscriber
	if subscription != nil {
		m.subscriptions[subscription.ID] = subscription
	}

	m.logger.Info("gRPC subscriber added",
		zap.String("subscriber_id", subscriber.ID),
		zap.String("service_name", serviceName))

	return subscriber
}

// RemoveSubscriber removes a subscriber
func (m *Manager) RemoveSubscriber(subscriberID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	subscriber, exists := m.subscribers[subscriberID]
	if !exists {
		return
	}

	// Close WebSocket connection if exists
	if subscriber.WebSocket != nil {
		subscriber.WebSocket.Close()
	}

	delete(m.subscribers, subscriberID)

	m.logger.Info("Subscriber removed",
		zap.String("subscriber_id", subscriberID),
		zap.String("service_name", subscriber.ServiceName))
}

// GetSubscribers returns all active subscribers
func (m *Manager) GetSubscribers() []*Subscriber {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	subscribers := make([]*Subscriber, 0, len(m.subscribers))
	for _, subscriber := range m.subscribers {
		subscribers = append(subscribers, subscriber)
	}

	return subscribers
}

// GetSubscribersByEventType returns subscribers interested in a specific event type
func (m *Manager) GetSubscribersByEventType(eventType string) []*Subscriber {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var matchingSubscribers []*Subscriber

	for _, subscriber := range m.subscribers {
		if !subscriber.Connected {
			continue
		}

		if subscriber.Subscription == nil {
			// No subscription filter means interested in all events
			matchingSubscribers = append(matchingSubscribers, subscriber)
			continue
		}

		// Check if subscriber is interested in this event type
		if len(subscriber.Subscription.EventTypes) == 0 {
			// Empty event types means interested in all events
			matchingSubscribers = append(matchingSubscribers, subscriber)
		} else {
			for _, subscribedType := range subscriber.Subscription.EventTypes {
				if subscribedType == eventType || subscribedType == "*" {
					matchingSubscribers = append(matchingSubscribers, subscriber)
					break
				}
			}
		}
	}

	return matchingSubscribers
}

// GetSubscribersByWorkflow returns subscribers interested in a specific workflow
func (m *Manager) GetSubscribersByWorkflow(workflowID string) []*Subscriber {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var matchingSubscribers []*Subscriber

	for _, subscriber := range m.subscribers {
		if !subscriber.Connected {
			continue
		}

		if subscriber.Subscription == nil {
			// No subscription filter means interested in all workflows
			matchingSubscribers = append(matchingSubscribers, subscriber)
			continue
		}

		// Check if subscriber is interested in this workflow
		if len(subscriber.Subscription.WorkflowIDs) == 0 {
			// Empty workflow IDs means interested in all workflows
			matchingSubscribers = append(matchingSubscribers, subscriber)
		} else {
			for _, subscribedWorkflow := range subscriber.Subscription.WorkflowIDs {
				if subscribedWorkflow == workflowID || subscribedWorkflow == "*" {
					matchingSubscribers = append(matchingSubscribers, subscriber)
					break
				}
			}
		}
	}

	return matchingSubscribers
}

// SendEventToSubscriber sends an event to a specific subscriber
func (m *Manager) SendEventToSubscriber(subscriber *Subscriber, event *cloudevents.CloudEvent) error {
	if !subscriber.Connected {
		return fmt.Errorf("subscriber %s is not connected", subscriber.ID)
	}

	switch subscriber.ConnectionType {
	case WebSocketConnection:
		return m.sendWebSocketEvent(subscriber, event)
	case GRPCConnection:
		return m.sendGRPCEvent(subscriber, event)
	default:
		return fmt.Errorf("unsupported connection type: %s", subscriber.ConnectionType)
	}
}

// sendWebSocketEvent sends an event via WebSocket
func (m *Manager) sendWebSocketEvent(subscriber *Subscriber, event *cloudevents.CloudEvent) error {
	if subscriber.WebSocket == nil {
		return fmt.Errorf("WebSocket connection is nil")
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = subscriber.WebSocket.WriteMessage(websocket.TextMessage, eventData)
	if err != nil {
		// Mark subscriber as disconnected
		subscriber.Connected = false
		return fmt.Errorf("failed to send WebSocket message: %w", err)
	}

	// Update subscriber stats
	subscriber.LastSeen = time.Now()
	subscriber.EventCount++

	return nil
}

// sendGRPCEvent sends an event via gRPC stream
func (m *Manager) sendGRPCEvent(subscriber *Subscriber, event *cloudevents.CloudEvent) error {
	// TODO: Implement gRPC event sending
	// This would depend on the specific gRPC stream interface
	m.logger.Debug("gRPC event sending not yet implemented",
		zap.String("subscriber_id", subscriber.ID))
	return nil
}

// cleanupStaleConnections removes disconnected subscribers
func (m *Manager) cleanupStaleConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	staleThreshold := time.Now().Add(-5 * time.Minute)
	var staleSubscribers []string

	for id, subscriber := range m.subscribers {
		if !subscriber.Connected || subscriber.LastSeen.Before(staleThreshold) {
			staleSubscribers = append(staleSubscribers, id)
		}
	}

	for _, id := range staleSubscribers {
		subscriber := m.subscribers[id]
		if subscriber.WebSocket != nil {
			subscriber.WebSocket.Close()
		}
		delete(m.subscribers, id)
		
		m.logger.Info("Removed stale subscriber",
			zap.String("subscriber_id", id),
			zap.String("service_name", subscriber.ServiceName))
	}
}

// CreateSubscription creates a new subscription
func (m *Manager) CreateSubscription(subscription *Subscription) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if subscription.ID == "" {
		subscription.ID = uuid.New().String()
	}
	subscription.CreatedAt = time.Now()

	m.subscriptions[subscription.ID] = subscription

	m.logger.Info("Subscription created",
		zap.String("subscription_id", subscription.ID),
		zap.String("service_name", subscription.ServiceName))
}

// GetSubscriptions returns all subscriptions
func (m *Manager) GetSubscriptions() []*Subscription {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	subscriptions := make([]*Subscription, 0, len(m.subscriptions))
	for _, subscription := range m.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions
}

// DeleteSubscription deletes a subscription
func (m *Manager) DeleteSubscription(subscriptionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.subscriptions, subscriptionID)

	m.logger.Info("Subscription deleted",
		zap.String("subscription_id", subscriptionID))
}
