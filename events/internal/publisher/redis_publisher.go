package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/interfaces"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// RedisPublisher implements EventPublisher using Redis Pub/Sub
type RedisPublisher struct {
	client        *redis.Client
	logger        *zap.Logger
	subscriptions map[string]*redisSubscription
	mutex         sync.RWMutex
}

// redisSubscription represents a Redis-based subscription
type redisSubscription struct {
	id           string
	subscription *interfaces.Subscription
	pubsub       *redis.PubSub
	events       chan *cloudevents.CloudEvent
	errors       chan error
	done         chan struct{}
	logger       *zap.Logger
}

// NewRedisPublisher creates a new Redis publisher implementation
func NewRedisPublisher(client *redis.Client, logger *zap.Logger) interfaces.EventPublisher {
	return &RedisPublisher{
		client:        client,
		logger:        logger,
		subscriptions: make(map[string]*redisSubscription),
	}
}

// Publish publishes an event to Redis channels
func (r *RedisPublisher) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
	// Serialize event
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to multiple channels for different subscription patterns
	channels := r.getChannelsForEvent(event)

	for _, channel := range channels {
		err := r.client.Publish(ctx, channel, eventData).Err()
		if err != nil {
			r.logger.Error("Failed to publish event to channel",
				zap.String("channel", channel),
				zap.String("event_id", event.ID),
				zap.Error(err))
			// Continue publishing to other channels
		} else {
			r.logger.Debug("Event published to channel",
				zap.String("channel", channel),
				zap.String("event_id", event.ID))
		}
	}

	return nil
}

// Subscribe creates a subscription for events
func (r *RedisPublisher) Subscribe(ctx context.Context, subscription *interfaces.Subscription) (interfaces.EventSubscription, error) {
	subscriptionID := uuid.New().String()

	// Determine which Redis channels to subscribe to
	channels := r.getChannelsForSubscription(subscription)

	// Create Redis PubSub
	pubsub := r.client.Subscribe(ctx, channels...)

	// Create subscription
	sub := &redisSubscription{
		id:           subscriptionID,
		subscription: subscription,
		pubsub:       pubsub,
		events:       make(chan *cloudevents.CloudEvent, 100),
		errors:       make(chan error, 10),
		done:         make(chan struct{}),
		logger:       r.logger,
	}

	// Store subscription
	r.mutex.Lock()
	r.subscriptions[subscriptionID] = sub
	r.mutex.Unlock()

	// Start listening for messages
	go sub.listen()

	r.logger.Info("Redis subscription created",
		zap.String("subscription_id", subscriptionID),
		zap.String("service_name", subscription.ServiceName),
		zap.Strings("channels", channels))

	return sub, nil
}

// Close closes the publisher
func (r *RedisPublisher) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Close all subscriptions
	for _, sub := range r.subscriptions {
		sub.Close()
	}

	return nil
}

// getChannelsForEvent determines which Redis channels to publish an event to
func (r *RedisPublisher) getChannelsForEvent(event *cloudevents.CloudEvent) []string {
	var channels []string

	// Global channel for all events
	channels = append(channels, "events:all")

	// Event type specific channel
	channels = append(channels, fmt.Sprintf("events:type:%s", event.Type))

	// Workflow specific channel
	if event.WorkflowID != "" {
		channels = append(channels, fmt.Sprintf("events:workflow:%s", event.WorkflowID))
	}

	// Source specific channel
	channels = append(channels, fmt.Sprintf("events:source:%s", event.Source))

	return channels
}

// getChannelsForSubscription determines which Redis channels to subscribe to
func (r *RedisPublisher) getChannelsForSubscription(subscription *interfaces.Subscription) []string {
	var channels []string

	// If no specific filters, subscribe to all events
	if len(subscription.EventTypes) == 0 && len(subscription.WorkflowIDs) == 0 {
		channels = append(channels, "events:all")
		return channels
	}

	// Subscribe to specific event types
	for _, eventType := range subscription.EventTypes {
		if eventType == "*" {
			channels = append(channels, "events:all")
		} else {
			channels = append(channels, fmt.Sprintf("events:type:%s", eventType))
		}
	}

	// Subscribe to specific workflows
	for _, workflowID := range subscription.WorkflowIDs {
		if workflowID == "*" {
			channels = append(channels, "events:all")
		} else {
			channels = append(channels, fmt.Sprintf("events:workflow:%s", workflowID))
		}
	}

	// If no channels determined, subscribe to all
	if len(channels) == 0 {
		channels = append(channels, "events:all")
	}

	return channels
}

// redisSubscription methods

// Events returns the channel for receiving events
func (s *redisSubscription) Events() <-chan *cloudevents.CloudEvent {
	return s.events
}

// Errors returns the channel for receiving errors
func (s *redisSubscription) Errors() <-chan error {
	return s.errors
}

// Close closes the subscription
func (s *redisSubscription) Close() error {
	close(s.done)
	close(s.events)
	close(s.errors)
	return s.pubsub.Close()
}

// listen listens for Redis messages and converts them to CloudEvents
func (s *redisSubscription) listen() {
	defer s.pubsub.Close()

	ch := s.pubsub.Channel()

	for {
		select {
		case <-s.done:
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Parse CloudEvent from message
			var event cloudevents.CloudEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				s.errors <- fmt.Errorf("failed to unmarshal event: %w", err)
				continue
			}

			// Apply additional filtering
			if s.shouldReceiveEvent(&event) {
				select {
				case s.events <- &event:
				case <-s.done:
					return
				default:
					s.logger.Warn("Event channel is full, dropping event",
						zap.String("subscription_id", s.id),
						zap.String("event_id", event.ID))
				}
			}
		}
	}
}

// shouldReceiveEvent applies additional filtering beyond Redis channel subscription
func (s *redisSubscription) shouldReceiveEvent(event *cloudevents.CloudEvent) bool {
	// Don't send events back to the source service
	if event.Source == s.subscription.ServiceName {
		return false
	}

	// Apply custom filters
	for key, value := range s.subscription.Filters {
		if eventValue, exists := event.Extensions[key]; exists {
			if eventValue != value {
				return false
			}
		}
	}

	return true
}
