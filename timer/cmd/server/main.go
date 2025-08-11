package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/factory"
	"github.com/flunq-io/timer/internal/processor"
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

	cfg := processor.Config{
		BatchSize: 100,
		Lookahead: mustDur("TIMER_LOOKAHEAD", "100ms"),
		MaxSleep:  mustDur("TIMER_MAX_SLEEP", "1s"),
	}

	ctx := context.Background()
	redisURL := getEnv("REDIS_URL", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisClient := redis.NewClient(&redis.Options{Addr: redisURL, Password: redisPassword})
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

	// Initialize enhanced timer processor
	timerProcessor := processor.NewTimerProcessor(
		eventStream,
		redisClient,
		log,
		cfg,
	)

	// Start timer processor
	if err := timerProcessor.Start(ctx); err != nil {
		log.Fatal("Failed to start timer processor", zap.Error(err))
	}

	log.Info("Timer service started with enhanced resilience")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Timer Service...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop the timer processor
	if err := timerProcessor.Stop(shutdownCtx); err != nil {
		log.Error("Error stopping timer processor", zap.Error(err))
	}

	log.Info("Timer service stopped")
}
