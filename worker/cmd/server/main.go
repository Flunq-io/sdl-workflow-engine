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
	"github.com/flunq-io/worker/internal/adapters"
	"github.com/flunq-io/worker/internal/engine"
	"github.com/flunq-io/worker/internal/processor"
	"github.com/flunq-io/worker/internal/serializer"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	zapLogger := &adapters.ZapLogger{Logger: logger}

	// Load configuration
	config := loadConfig()

	zapLogger.Info("Starting Worker service",
		"version", "1.0.0",
		"event_store_url", config.EventStoreURL,
		"redis_url", config.RedisURL)

	// Initialize Redis client for database operations
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		zapLogger.Fatal("Failed to connect to Redis", "error", err)
	}

	// Initialize Event Store client (for HTTP queries only)
	eventStoreClient := client.NewEventClient(config.EventStoreURL, "worker-service", logger)

	// Initialize database adapter
	database := adapters.NewRedisDatabase(redisClient, zapLogger)

	// Initialize protobuf serializer
	serializer := serializer.NewProtobufSerializer()

	// Initialize metrics (placeholder)
	metrics := &adapters.NoOpMetrics{}

	// Initialize Serverless Workflow engine
	workflowEngine := engine.NewServerlessWorkflowEngine(zapLogger)

	// Initialize Event Store adapter (HTTP queries only)
	eventStore := adapters.NewEventStoreAdapter(eventStoreClient, zapLogger)

	// Initialize Redis Event Stream (for publishing and subscribing)
	eventStream := adapters.NewRedisEventStream(redisClient, zapLogger)

	// Initialize workflow processor
	workflowProcessor := processor.NewWorkflowProcessor(
		eventStore,
		eventStream,
		database,
		workflowEngine,
		serializer,
		zapLogger,
		metrics,
	)

	// Start the workflow processor
	if err := workflowProcessor.Start(ctx); err != nil {
		zapLogger.Fatal("Failed to start workflow processor", "error", err)
	}

	zapLogger.Info("Worker service started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Shutting down Worker service...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop the workflow processor
	if err := workflowProcessor.Stop(shutdownCtx); err != nil {
		zapLogger.Error("Error stopping workflow processor", "error", err)
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		zapLogger.Error("Error closing Redis connection", "error", err)
	}

	zapLogger.Info("Worker service shutdown complete")
}

// Config holds the configuration for the Worker service
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
		MetricsPort:   getEnv("METRICS_PORT", "9090"),
		HealthPort:    getEnv("HEALTH_PORT", "8082"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
