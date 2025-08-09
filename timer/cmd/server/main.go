package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/factory"
	"github.com/flunq-io/shared/pkg/interfaces"
)

type EventLoggerAdapter struct{ logger *zap.Logger }

func (l *EventLoggerAdapter) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, kv(fields...)...)
}
func (l *EventLoggerAdapter) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, kv(fields...)...)
}
func (l *EventLoggerAdapter) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, kv(fields...)...)
}
func (l *EventLoggerAdapter) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, kv(fields...)...)
}

func kv(fields ...interface{}) []zap.Field {
	z := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		if k, ok := fields[i].(string); ok {
			z = append(z, zap.Any(k, fields[i+1]))
		}
	}
	return z
}

type Config struct {
	RedisURL  string
	RedisPass string
	BatchSize int64
	Lookahead time.Duration
	MaxSleep  time.Duration
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustDur(key, def string) time.Duration {
	d, err := time.ParseDuration(getEnv(key, def))
	if err != nil {
		return time.Second
	}
	return d
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := logger

	cfg := Config{
		RedisURL:  getEnv("REDIS_URL", "localhost:6379"),
		RedisPass: getEnv("REDIS_PASSWORD", ""),
		BatchSize: 100,
		Lookahead: mustDur("TIMER_LOOKAHEAD", "100ms"),
		MaxSleep:  mustDur("TIMER_MAX_SLEEP", "1s"),
	}

	ctx := context.Background()
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisURL, Password: cfg.RedisPass})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Shared EventStream
	eventStream, err := factory.NewEventStream(factory.EventStreamDeps{
		Backend:     getEnv("EVENT_STREAM_TYPE", "redis"),
		RedisClient: redisClient,
		Logger:      &EventLoggerAdapter{logger: log},
	})
	if err != nil {
		log.Fatal("Failed to create EventStream", zap.Error(err))
	}

	// Subscribe to timer.scheduled to register timers into ZSET
	sub, err := eventStream.Subscribe(ctx, interfaces.StreamFilters{
		EventTypes:    []string{"io.flunq.timer.scheduled"},
		WorkflowIDs:   []string{},
		ConsumerGroup: "timer-service",
		ConsumerName:  "timer-" + time.Now().Format("150405"),
	})
	if err != nil {
		log.Fatal("Failed to subscribe to timer.scheduled", zap.Error(err))
	}

	// Start consumer loop
	go func() {
		for {
			select {
			case ev := <-sub.Events():
				if ev == nil {
					continue
				}
				_ = registerTimer(ctx, redisClient, log, ev, cfg)
			case err := <-sub.Errors():
				if err != nil {
					log.Error("subscription error", zap.Error(err))
				}
			}
		}
	}()

	// Start scheduler loop
	go schedulerLoop(ctx, redisClient, eventStream, log, cfg)

	log.Info("Timer service started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	_ = sub.Close()
	_ = redisClient.Close()
	log.Info("Timer service stopped")
}

const zsetKey = "flunq:timers:all"

type TimerPayload struct {
	TenantID    string `json:"tenant_id"`
	WorkflowID  string `json:"workflow_id"`
	ExecutionID string `json:"execution_id"`
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
	FireAt      string `json:"fire_at"` // RFC3339
	DurationMs  int64  `json:"duration_ms"`
}

func registerTimer(ctx context.Context, r *redis.Client, log *zap.Logger, ev *cloudevents.CloudEvent, cfg Config) error {
	// Accept scheduled from any producer (worker)
	var p TimerPayload
	b, _ := json.Marshal(ev.Data)
	if err := json.Unmarshal(b, &p); err != nil {
		log.Error("invalid timer.scheduled payload", zap.Error(err))
		return err
	}
	fireAt, err := time.Parse(time.RFC3339, p.FireAt)
	if err != nil {
		log.Error("invalid scheduled_for", zap.Error(err))
		return err
	}
	member, _ := json.Marshal(p)
	score := float64(fireAt.UnixNano()) / 1e9 // seconds with sub-second precision
	if err := r.ZAdd(ctx, zsetKey, &redis.Z{Score: score, Member: string(member)}).Err(); err != nil {
		log.Error("failed to ZADD timer", zap.Error(err))
		return err
	}
	log.Info("Registered timer", zap.String("task_id", p.TaskID), zap.String("at", p.FireAt))
	return nil
}

func schedulerLoop(ctx context.Context, r *redis.Client, es interfaces.EventStream, log *zap.Logger, cfg Config) {
	for {
		// Peek next to compute sleep
		next, err := r.ZRangeWithScores(ctx, zsetKey, 0, 0).Result()
		if err != nil && err != redis.Nil {
			log.Error("ZRangeWithScores error", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		sleep := cfg.MaxSleep
		now := time.Now()
		if len(next) > 0 {
			nextAt := time.Unix(0, int64(next[0].Score*1e9))
			d := nextAt.Sub(now)
			if d > 0 {
				if d < cfg.MaxSleep {
					sleep = d
				} else {
					sleep = cfg.MaxSleep
				}
			} else {
				sleep = 0
			}
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}

		// Pop due timers in small batches using ZPOPMIN
		for i := int64(0); i < cfg.BatchSize; i++ {
			z, err := r.ZPopMin(ctx, zsetKey).Result()
			if err == redis.Nil {
				break
			}
			if err != nil {
				log.Error("ZPopMin error", zap.Error(err))
				break
			}
			if len(z) == 0 {
				break
			}
			score := z[0].Score
			if score > float64(time.Now().Add(cfg.Lookahead).UnixNano())/1e9 {
				// Not due yet, push back and break
				if err := r.ZAdd(ctx, zsetKey, &redis.Z{Score: score, Member: z[0].Member}).Err(); err != nil {
					log.Error("failed to push back timer", zap.Error(err))
				}
				break
			}
			// Process timer
			var p TimerPayload
			if err := json.Unmarshal([]byte(z[0].Member.(string)), &p); err != nil {
				log.Error("bad timer member json", zap.Error(err))
				continue
			}
			_ = publishTimerFired(ctx, es, log, p)
		}
	}
}

func publishTimerFired(ctx context.Context, es interfaces.EventStream, log *zap.Logger, p TimerPayload) error {
	now := time.Now()
	ev := &cloudevents.CloudEvent{
		ID:          "timer-fired-" + p.TaskID + "-" + now.Format("150405.000"),
		Source:      "timer-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.timer.fired",
		TenantID:    p.TenantID,
		WorkflowID:  p.WorkflowID,
		ExecutionID: p.ExecutionID,
		TaskID:      p.TaskID,
		Time:        now,
		Data: map[string]interface{}{
			"tenant_id":     p.TenantID,
			"workflow_id":   p.WorkflowID,
			"execution_id":  p.ExecutionID,
			"task_id":       p.TaskID,
			"task_name":     p.TaskName,
			"scheduled_for": p.FireAt,
			"fired_at":      now.Format(time.RFC3339),
		},
	}
	if err := es.Publish(ctx, ev); err != nil {
		log.Error("failed to publish timer.fired", zap.Error(err))
		return err
	}
	log.Info("Published timer.fired", zap.String("task_id", p.TaskID))
	return nil
}
