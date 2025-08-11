package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// TimerProcessor processes timer scheduling and firing with enterprise-grade resilience
type TimerProcessor struct {
	eventStream interfaces.EventStream
	redisClient *redis.Client
	logger      *zap.Logger
	config      Config

	subscription interfaces.StreamSubscription
	errorCh      <-chan error
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	// Per-event workers waitgroup (for graceful shutdown when concurrency > 1)
	tasksWG sync.WaitGroup

	// Concurrency control for timer processing
	maxConcurrency int
	sem            chan struct{}
}

// Config holds timer service configuration
type Config struct {
	MaxSleep  time.Duration
	BatchSize int64
	Lookahead time.Duration
}

// TimerPayload represents a timer event payload
type TimerPayload struct {
	TenantID    string `json:"tenant_id"`
	WorkflowID  string `json:"workflow_id"`
	ExecutionID string `json:"execution_id"`
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
	FireAt      string `json:"fire_at"` // RFC3339
	DurationMs  int64  `json:"duration_ms"`
}

const zsetKey = "flunq:timers:all"

// NewTimerProcessor creates a new timer processor with enterprise-grade resilience
func NewTimerProcessor(
	eventStream interfaces.EventStream,
	redisClient *redis.Client,
	logger *zap.Logger,
	config Config,
) *TimerProcessor {
	// Determine concurrency from env (default 2 for timer service)
	maxConc := 2
	if v := os.Getenv("TIMER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConc = n
		}
	}

	return &TimerProcessor{
		eventStream:    eventStream,
		redisClient:    redisClient,
		logger:         logger,
		config:         config,
		stopCh:         make(chan struct{}),
		maxConcurrency: maxConc,
		sem:            make(chan struct{}, maxConc),
	}
}

// Start starts the timer processor with enhanced resilience
func (p *TimerProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting timer processor with enhanced resilience")

	// Health check event stream connectivity
	if info, err := p.eventStream.GetStreamInfo(ctx); err != nil {
		p.logger.Warn("Stream info not available yet", zap.Error(err))
	} else {
		p.logger.Info("Stream info",
			zap.String("stream", info.StreamName),
			zap.Int64("message_count", info.MessageCount),
			zap.Int("consumer_groups", len(info.ConsumerGroups)))
	}

	// Subscribe to timer.scheduled events with enhanced configuration
	filters := interfaces.StreamFilters{
		EventTypes: []string{
			"io.flunq.timer.scheduled",
		},
		WorkflowIDs:   []string{}, // Subscribe to all workflows
		ConsumerGroup: getEnvDefault("TIMER_CONSUMER_GROUP", "timer-service"),
		ConsumerName:  fmt.Sprintf("timer-%s", uuid.New().String()[:8]),
		BatchCount:    getEnvIntDefault("TIMER_STREAM_BATCH", 10),
		BlockTimeout:  getEnvDurationDefault("TIMER_STREAM_BLOCK", time.Second),
	}

	// Create subscription once and keep reference for acks
	subscription, err := p.eventStream.Subscribe(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to subscribe to event stream: %w", err)
	}

	p.subscription = subscription
	p.errorCh = subscription.Errors()
	p.running = true

	// Start event processing goroutine
	p.wg.Add(1)
	go p.eventProcessingLoop(ctx)

	// Start scheduler loop
	p.wg.Add(1)
	go p.schedulerLoop(ctx)

	p.logger.Info("Timer processor started successfully with enhanced resilience")

	// Optionally start background reclaim loop for orphaned pending messages
	if getEnvBoolDefault("TIMER_RECLAIM_ENABLED", false) {
		go p.reclaimOrphanedMessages(ctx, filters.ConsumerGroup, filters.ConsumerName)
	}
	return nil
}

// Stop stops the timer processor with graceful shutdown
func (p *TimerProcessor) Stop(ctx context.Context) error {
	p.logger.Info("Stopping timer processor")

	p.running = false
	close(p.stopCh)

	// Close event stream connection
	if p.eventStream != nil {
		p.eventStream.Close()
	}

	// Wait for processing to complete
	p.wg.Wait()
	// Wait for in-flight tasks to finish or context timeout
	done := make(chan struct{})
	go func() {
		p.tasksWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-ctx.Done():
		p.logger.Warn("Timeout waiting for in-flight tasks to finish")
	}

	p.logger.Info("Timer processor stopped")
	return nil
}

// eventProcessingLoop processes incoming timer events with enhanced resilience
func (p *TimerProcessor) eventProcessingLoop(ctx context.Context) {
	defer p.wg.Done()
	p.logger.Info("Starting timer event processing loop (consumer-group mode)")

	if p.subscription == nil {
		p.logger.Error("No subscription available in processor")
		return
	}
	eventsCh := p.subscription.Events()
	errorsCh := p.subscription.Errors()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping timer event processing")
			return
		case <-p.stopCh:
			p.logger.Info("Stop signal received, stopping timer event processing")
			return
		case event := <-eventsCh:
			if event == nil {
				continue
			}

			// Overall concurrency control
			p.sem <- struct{}{}
			p.tasksWG.Add(1)
			go func(ev *cloudevents.CloudEvent) {
				defer func() {
					<-p.sem
					p.tasksWG.Done()
				}()

				// Process with retry and ack semantics
				var lastErr error
				maxRetries := 3
				for attempt := 1; attempt <= maxRetries; attempt++ {
					if err := p.registerTimer(ctx, ev); err != nil {
						lastErr = err
						p.logger.Error("Failed to register timer, will retry",
							zap.String("event_id", ev.ID),
							zap.String("event_type", ev.Type),
							zap.String("task_id", ev.TaskID),
							zap.Int("attempt", attempt),
							zap.Error(err))
						time.Sleep(time.Duration(attempt) * 200 * time.Millisecond) // simple backoff
						continue
					}

					// Success: acknowledge via subscription if possible
					ackID := ""
					if key, ok := ev.Extensions["ack_key"].(string); ok {
						ackID = key
					} else {
						if stream, ok1 := ev.Extensions["redis_stream"].(string); ok1 {
							if msgID, ok2 := ev.Extensions["redis_msg_id"].(string); ok2 {
								ackID = stream + "|" + msgID
							}
						}
					}
					if ackID == "" {
						p.logger.Error("CRITICAL: No ack key available; cannot acknowledge",
							zap.String("event_id", ev.ID),
							zap.Any("extensions", ev.Extensions))
					} else if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackID); err != nil {
							p.logger.Warn("Ack failed",
								zap.String("event_id", ev.ID),
								zap.String("ack_id", ackID),
								zap.Error(err))
						}
					}
					p.logger.Debug("Processed and acked timer event",
						zap.String("event_id", ev.ID),
						zap.String("task_id", ev.TaskID))
					return
				}

				// After max retries: send to DLQ and log
				p.logger.Error("Timer event failed after max retries; sending to DLQ",
					zap.String("event_id", ev.ID),
					zap.String("task_id", ev.TaskID),
					zap.Error(lastErr))

				// Publish to DLQ using same event with marker
				dlqEvent := ev.Clone()
				dlqEvent.Type = "io.flunq.timer.dlq"
				dlqEvent.AddExtension("reason", fmt.Sprintf("max_retries_exceeded:%v", lastErr))
				_ = p.eventStream.Publish(ctx, dlqEvent)

				// After DLQ publish, acknowledge the original message to prevent it from staying pending
				if ackKey, ok := ev.Extensions["ack_key"].(string); ok {
					if p.subscription != nil {
						if err := p.subscription.Acknowledge(ctx, ackKey); err != nil {
							p.logger.Warn("Ack after DLQ failed",
								zap.String("event_id", ev.ID),
								zap.Error(err))
						} else {
							p.logger.Info("Acked message after DLQ",
								zap.String("event_id", ev.ID))
						}
					}
				}
			}(event)
		case err := <-errorsCh:
			if err != nil {
				p.logger.Error("Event stream error", zap.Error(err))
			}
		}
	}
}

// registerTimer registers a timer in Redis ZSET
func (p *TimerProcessor) registerTimer(ctx context.Context, ev *cloudevents.CloudEvent) error {
	// Accept scheduled from any producer (worker)
	var payload TimerPayload
	b, _ := json.Marshal(ev.Data)
	if err := json.Unmarshal(b, &payload); err != nil {
		p.logger.Error("invalid timer.scheduled payload", zap.Error(err))
		return err
	}
	fireAt, err := time.Parse(time.RFC3339, payload.FireAt)
	if err != nil {
		p.logger.Error("invalid scheduled_for", zap.Error(err))
		return err
	}
	member, _ := json.Marshal(payload)
	score := float64(fireAt.UnixNano()) / 1e9 // seconds with sub-second precision
	if err := p.redisClient.ZAdd(ctx, zsetKey, &redis.Z{Score: score, Member: string(member)}).Err(); err != nil {
		p.logger.Error("failed to ZADD timer", zap.Error(err))
		return err
	}
	p.logger.Info("Registered timer", zap.String("task_id", payload.TaskID), zap.String("at", payload.FireAt))
	return nil
}

// schedulerLoop processes due timers and publishes timer.fired events
func (p *TimerProcessor) schedulerLoop(ctx context.Context) {
	defer p.wg.Done()
	p.logger.Info("Starting timer scheduler loop")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context cancelled, stopping scheduler loop")
			return
		case <-p.stopCh:
			p.logger.Info("Stop signal received, stopping scheduler loop")
			return
		default:
			// Peek next to compute sleep
			next, err := p.redisClient.ZRangeWithScores(ctx, zsetKey, 0, 0).Result()
			if err != nil && err != redis.Nil {
				p.logger.Error("ZRangeWithScores error", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
			sleep := p.config.MaxSleep
			now := time.Now()
			if len(next) > 0 {
				nextAt := time.Unix(0, int64(next[0].Score*1e9))
				d := nextAt.Sub(now)
				if d > 0 {
					if d < p.config.MaxSleep {
						sleep = d
					} else {
						sleep = p.config.MaxSleep
					}
				} else {
					sleep = 0
				}
			}
			if sleep > 0 {
				time.Sleep(sleep)
			}

			// Pop due timers in small batches using ZPOPMIN
			for i := int64(0); i < p.config.BatchSize; i++ {
				z, err := p.redisClient.ZPopMin(ctx, zsetKey).Result()
				if err == redis.Nil {
					break
				}
				if err != nil {
					p.logger.Error("ZPopMin error", zap.Error(err))
					break
				}
				if len(z) == 0 {
					break
				}
				score := z[0].Score
				if score > float64(time.Now().Add(p.config.Lookahead).UnixNano())/1e9 {
					// Not due yet, push back and break
					if err := p.redisClient.ZAdd(ctx, zsetKey, &redis.Z{Score: score, Member: z[0].Member}).Err(); err != nil {
						p.logger.Error("failed to push back timer", zap.Error(err))
					}
					break
				}
				// Process timer
				var payload TimerPayload
				if err := json.Unmarshal([]byte(z[0].Member.(string)), &payload); err != nil {
					p.logger.Error("bad timer member json", zap.Error(err))
					continue
				}
				_ = p.publishTimerFired(ctx, payload)
			}
		}
	}
}

// publishTimerFired publishes a timer.fired event
func (p *TimerProcessor) publishTimerFired(ctx context.Context, payload TimerPayload) error {
	now := time.Now()
	ev := &cloudevents.CloudEvent{
		ID:          "timer-fired-" + payload.TaskID + "-" + now.Format("150405.000"),
		Source:      "timer-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.timer.fired",
		TenantID:    payload.TenantID,
		WorkflowID:  payload.WorkflowID,
		ExecutionID: payload.ExecutionID,
		TaskID:      payload.TaskID,
		Time:        now,
		Data: map[string]interface{}{
			"tenant_id":     payload.TenantID,
			"workflow_id":   payload.WorkflowID,
			"execution_id":  payload.ExecutionID,
			"task_id":       payload.TaskID,
			"task_name":     payload.TaskName,
			"scheduled_for": payload.FireAt,
			"fired_at":      now.Format(time.RFC3339),
		},
	}

	if err := p.eventStream.Publish(ctx, ev); err != nil {
		p.logger.Error("failed to publish timer.fired", zap.Error(err))
		return err
	}

	p.logger.Info("Published timer.fired",
		zap.String("task_id", payload.TaskID),
		zap.String("workflow_id", payload.WorkflowID),
		zap.String("execution_id", payload.ExecutionID))
	return nil
}

// Background goroutine to reclaim orphaned pending messages
func (p *TimerProcessor) reclaimOrphanedMessages(ctx context.Context, consumerGroup, consumerName string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			if p.eventStream != nil {
				if reclaimed, err := p.eventStream.ReclaimPending(ctx, consumerGroup, consumerName, 60*time.Second); err != nil {
					p.logger.Warn("Failed to reclaim pending messages", zap.Error(err))
				} else if reclaimed > 0 {
					p.logger.Info("Reclaimed pending messages", zap.Int("count", reclaimed))
				}
			}
		}
	}
}

// env helpers (scoped to processor package)
func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvIntDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDurationDefault(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getEnvBoolDefault(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if v == "1" || strings.ToLower(v) == "true" || strings.ToLower(v) == "yes" {
			return true
		}
		if v == "0" || strings.ToLower(v) == "false" || strings.ToLower(v) == "no" {
			return false
		}
	}
	return def
}
