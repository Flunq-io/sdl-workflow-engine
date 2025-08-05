package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/events/pkg/client"
	"github.com/flunq-io/executor/internal/adapters"
	"github.com/flunq-io/executor/internal/executor"
	"github.com/flunq-io/executor/internal/processor"
)

func main() {
	// Initialize logger
	zapLogger, _ := zap.NewDevelopment()
	defer zapLogger.Sync()

	// Load configuration
	config := loadConfig()

	zapLogger.Info("Starting Executor Service",
		zap.String("version", "1.0.0"),
		zap.String("event_store_url", config.EventStoreURL),
		zap.String("redis_url", config.RedisURL))

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		zapLogger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize Event Store client (for HTTP queries only)
	eventStoreClient := client.NewEventClient(config.EventStoreURL, "task-service", zapLogger)

	// Initialize adapters
	eventStore := adapters.NewEventStoreAdapter(eventStoreClient, zapLogger) // HTTP queries only
	eventStream := adapters.NewRedisEventStream(redisClient, zapLogger)      // Publishing and subscribing

	// Initialize task executors
	taskExecutors := map[string]executor.TaskExecutor{
		"set":    executor.NewSetTaskExecutor(zapLogger),
		"call":   executor.NewCallTaskExecutor(zapLogger),
		"wait":   executor.NewWaitTaskExecutor(zapLogger),
		"inject": executor.NewInjectTaskExecutor(zapLogger),
	}

	// Initialize task processor
	taskProcessor := processor.NewTaskProcessor(
		eventStore,
		eventStream,
		taskExecutors,
		zapLogger,
	)

	// Start task processor
	if err := taskProcessor.Start(ctx); err != nil {
		zapLogger.Fatal("Failed to start task processor", zap.Error(err))
	}

	zapLogger.Info("Executor Service started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Shutting down Executor Service...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop the task processor
	if err := taskProcessor.Stop(shutdownCtx); err != nil {
		zapLogger.Error("Error stopping task processor", zap.Error(err))
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		zapLogger.Error("Error closing Redis connection", zap.Error(err))
	}

	zapLogger.Info("Executor Service shutdown complete")
}

// Config holds the configuration for the Executor Service
type Config struct {
	EventStoreURL string
	RedisURL      string
	RedisPassword string
	LogLevel      string
	MetricsPort   string
	HealthPort    string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		EventStoreURL: getEnv("EVENTS_URL", "http://localhost:8081"),
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		MetricsPort:   getEnv("METRICS_PORT", "9091"),
		HealthPort:    getEnv("HEALTH_PORT", "8083"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
