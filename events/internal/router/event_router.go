package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/subscriber"
	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventRouter handles routing events to appropriate subscribers
type EventRouter struct {
	subscriberManager *subscriber.Manager
	logger            *zap.Logger
	eventQueue        chan *cloudevents.CloudEvent
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	deadLetterQueue   chan *DeadLetterEvent
	retryQueue        chan *RetryEvent
}

// DeadLetterEvent represents an event that failed to be delivered
type DeadLetterEvent struct {
	Event        *cloudevents.CloudEvent
	SubscriberID string
	FailureCount int
	LastError    error
	Timestamp    time.Time
}

// RetryEvent represents an event that should be retried
type RetryEvent struct {
	Event        *cloudevents.CloudEvent
	SubscriberID string
	RetryCount   int
	NextRetry    time.Time
}

// NewEventRouter creates a new event router
func NewEventRouter(subscriberManager *subscriber.Manager, logger *zap.Logger) *EventRouter {
	ctx, cancel := context.WithCancel(context.Background())

	return &EventRouter{
		subscriberManager: subscriberManager,
		logger:            logger,
		eventQueue:        make(chan *cloudevents.CloudEvent, 1000),
		deadLetterQueue:   make(chan *DeadLetterEvent, 100),
		retryQueue:        make(chan *RetryEvent, 500),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start begins the event router background processes
func (r *EventRouter) Start(ctx context.Context) {
	r.logger.Info("Starting event router")

	// Start event processing workers
	for i := 0; i < 5; i++ {
		r.wg.Add(1)
		go r.eventWorker(i)
	}

	// Start retry worker
	r.wg.Add(1)
	go r.retryWorker()

	// Start dead letter queue worker
	r.wg.Add(1)
	go r.deadLetterWorker()

	// Wait for context cancellation
	<-ctx.Done()
	r.Stop()
}

// Stop stops the event router
func (r *EventRouter) Stop() {
	r.logger.Info("Stopping event router")
	r.cancel()

	// Close channels
	close(r.eventQueue)
	close(r.retryQueue)
	close(r.deadLetterQueue)

	// Wait for workers to finish
	r.wg.Wait()
	r.logger.Info("Event router stopped")
}

// RouteEvent routes an event to appropriate subscribers
func (r *EventRouter) RouteEvent(event *cloudevents.CloudEvent) {
	select {
	case r.eventQueue <- event:
		r.logger.Debug("Event queued for routing",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.String("workflow_id", event.WorkflowID))
	default:
		r.logger.Error("Event queue is full, dropping event",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type))
	}
}

// eventWorker processes events from the queue
func (r *EventRouter) eventWorker(workerID int) {
	defer r.wg.Done()

	r.logger.Info("Event worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-r.ctx.Done():
			return
		case event, ok := <-r.eventQueue:
			if !ok {
				return
			}
			r.processEvent(event)
		}
	}
}

// processEvent processes a single event and routes it to subscribers
func (r *EventRouter) processEvent(event *cloudevents.CloudEvent) {
	// Get subscribers interested in this event type
	typeSubscribers := r.subscriberManager.GetSubscribersByEventType(event.Type)

	// Get subscribers interested in this workflow
	workflowSubscribers := r.subscriberManager.GetSubscribersByWorkflow(event.WorkflowID)

	// Combine and deduplicate subscribers
	subscriberMap := make(map[string]*subscriber.Subscriber)

	for _, sub := range typeSubscribers {
		subscriberMap[sub.ID] = sub
	}

	for _, sub := range workflowSubscribers {
		subscriberMap[sub.ID] = sub
	}

	// Send event to each subscriber
	for _, sub := range subscriberMap {
		if r.shouldRouteToSubscriber(event, sub) {
			r.sendEventToSubscriber(event, sub)
		}
	}

	r.logger.Debug("Event processed",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
		zap.Int("subscriber_count", len(subscriberMap)))
}

// shouldRouteToSubscriber determines if an event should be routed to a specific subscriber
func (r *EventRouter) shouldRouteToSubscriber(event *cloudevents.CloudEvent, sub *subscriber.Subscriber) bool {
	// Don't send events back to the source service
	if event.Source == sub.ServiceName {
		return false
	}

	// Apply additional filtering based on subscription
	if sub.Subscription != nil {
		// Check custom filters
		for key, value := range sub.Subscription.Filters {
			if eventValue, exists := event.Extensions[key]; exists {
				if eventValue != value {
					return false
				}
			}
		}
	}

	return true
}

// sendEventToSubscriber sends an event to a specific subscriber with retry logic
func (r *EventRouter) sendEventToSubscriber(event *cloudevents.CloudEvent, sub *subscriber.Subscriber) {
	err := r.subscriberManager.SendEventToSubscriber(sub, event)
	if err != nil {
		r.logger.Warn("Failed to send event to subscriber",
			zap.String("event_id", event.ID),
			zap.String("subscriber_id", sub.ID),
			zap.String("service_name", sub.ServiceName),
			zap.Error(err))

		// Queue for retry
		retryEvent := &RetryEvent{
			Event:        event,
			SubscriberID: sub.ID,
			RetryCount:   1,
			NextRetry:    time.Now().Add(time.Second * 5), // 5 second initial delay
		}

		select {
		case r.retryQueue <- retryEvent:
		default:
			r.logger.Error("Retry queue is full, sending to dead letter queue",
				zap.String("event_id", event.ID),
				zap.String("subscriber_id", sub.ID))
			r.sendToDeadLetterQueue(event, sub.ID, 1, err)
		}
	}
}

// retryWorker handles event retries
func (r *EventRouter) retryWorker() {
	defer r.wg.Done()

	r.logger.Info("Retry worker started")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var pendingRetries []*RetryEvent

	for {
		select {
		case <-r.ctx.Done():
			return
		case retryEvent, ok := <-r.retryQueue:
			if !ok {
				return
			}
			pendingRetries = append(pendingRetries, retryEvent)

		case <-ticker.C:
			now := time.Now()
			var remainingRetries []*RetryEvent

			for _, retryEvent := range pendingRetries {
				if now.After(retryEvent.NextRetry) {
					r.processRetry(retryEvent)
				} else {
					remainingRetries = append(remainingRetries, retryEvent)
				}
			}

			pendingRetries = remainingRetries
		}
	}
}

// processRetry processes a retry event
func (r *EventRouter) processRetry(retryEvent *RetryEvent) {
	// Find the subscriber
	subscribers := r.subscriberManager.GetSubscribers()
	var targetSubscriber *subscriber.Subscriber

	for _, sub := range subscribers {
		if sub.ID == retryEvent.SubscriberID {
			targetSubscriber = sub
			break
		}
	}

	if targetSubscriber == nil {
		r.logger.Warn("Subscriber not found for retry, sending to dead letter queue",
			zap.String("event_id", retryEvent.Event.ID),
			zap.String("subscriber_id", retryEvent.SubscriberID))
		r.sendToDeadLetterQueue(retryEvent.Event, retryEvent.SubscriberID, retryEvent.RetryCount,
			fmt.Errorf("subscriber not found"))
		return
	}

	err := r.subscriberManager.SendEventToSubscriber(targetSubscriber, retryEvent.Event)
	if err != nil {
		// Exponential backoff with max retries
		if retryEvent.RetryCount >= 3 {
			r.logger.Error("Max retries exceeded, sending to dead letter queue",
				zap.String("event_id", retryEvent.Event.ID),
				zap.String("subscriber_id", retryEvent.SubscriberID),
				zap.Int("retry_count", retryEvent.RetryCount))
			r.sendToDeadLetterQueue(retryEvent.Event, retryEvent.SubscriberID, retryEvent.RetryCount, err)
			return
		}

		// Schedule next retry with exponential backoff
		delay := time.Duration(retryEvent.RetryCount*retryEvent.RetryCount) * time.Second * 5
		retryEvent.RetryCount++
		retryEvent.NextRetry = time.Now().Add(delay)

		select {
		case r.retryQueue <- retryEvent:
		default:
			r.logger.Error("Retry queue is full, sending to dead letter queue",
				zap.String("event_id", retryEvent.Event.ID),
				zap.String("subscriber_id", retryEvent.SubscriberID))
			r.sendToDeadLetterQueue(retryEvent.Event, retryEvent.SubscriberID, retryEvent.RetryCount, err)
		}
	} else {
		r.logger.Info("Event retry successful",
			zap.String("event_id", retryEvent.Event.ID),
			zap.String("subscriber_id", retryEvent.SubscriberID),
			zap.Int("retry_count", retryEvent.RetryCount))
	}
}

// sendToDeadLetterQueue sends an event to the dead letter queue
func (r *EventRouter) sendToDeadLetterQueue(event *cloudevents.CloudEvent, subscriberID string, failureCount int, err error) {
	deadLetter := &DeadLetterEvent{
		Event:        event,
		SubscriberID: subscriberID,
		FailureCount: failureCount,
		LastError:    err,
		Timestamp:    time.Now(),
	}

	select {
	case r.deadLetterQueue <- deadLetter:
	default:
		r.logger.Error("Dead letter queue is full, dropping event",
			zap.String("event_id", event.ID),
			zap.String("subscriber_id", subscriberID))
	}
}

// deadLetterWorker handles dead letter events
func (r *EventRouter) deadLetterWorker() {
	defer r.wg.Done()

	r.logger.Info("Dead letter worker started")

	for {
		select {
		case <-r.ctx.Done():
			return
		case deadLetter, ok := <-r.deadLetterQueue:
			if !ok {
				return
			}

			r.logger.Error("Event sent to dead letter queue",
				zap.String("event_id", deadLetter.Event.ID),
				zap.String("subscriber_id", deadLetter.SubscriberID),
				zap.Int("failure_count", deadLetter.FailureCount),
				zap.Error(deadLetter.LastError))

			// TODO: Persist dead letter events for later analysis/replay
		}
	}
}
