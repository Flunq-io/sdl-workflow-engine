package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/factory"
	"github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/worker/internal/adapters"
	"github.com/flunq-io/worker/internal/engine"
	"github.com/flunq-io/worker/internal/processor"
	"github.com/flunq-io/worker/internal/serializer"
)

// LoggerAdapter adapts zap.Logger to the shared eventstreaming.Logger interface
type LoggerAdapter struct {
	logger *zap.Logger
}

// DatabaseLoggerAdapter adapts zap.Logger to the shared database Logger interface
type DatabaseLoggerAdapter struct {
	logger *zap.Logger
}

// DatabaseLoggerAdapter methods
func (l *DatabaseLoggerAdapter) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, l.convertFields(fields...)...)
}

func (l *DatabaseLoggerAdapter) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, l.convertFields(fields...)...)
}

func (l *DatabaseLoggerAdapter) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, l.convertFields(fields...)...)
}

func (l *DatabaseLoggerAdapter) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, l.convertFields(fields...)...)
}

func (l *DatabaseLoggerAdapter) Fatal(msg string, fields ...interface{}) {
	l.logger.Fatal(msg, l.convertFields(fields...)...)
}

func (l *DatabaseLoggerAdapter) With(fields ...interface{}) interfaces.Logger {
	return &DatabaseLoggerAdapter{
		logger: l.logger.With(l.convertFields(fields...)...),
	}
}

func (l *DatabaseLoggerAdapter) convertFields(fields ...interface{}) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}

func (l *LoggerAdapter) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, l.convertFields(fields...)...)
}

func (l *LoggerAdapter) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, l.convertFields(fields...)...)
}

func (l *LoggerAdapter) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, l.convertFields(fields...)...)
}

func (l *LoggerAdapter) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, l.convertFields(fields...)...)
}

func (l *LoggerAdapter) convertFields(fields ...interface{}) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	zapLogger := &adapters.ZapLogger{Logger: logger}

	// Load configuration
	config := loadConfig()

	zapLogger.Info("Starting Worker service",
		"version", "1.0.0",
		"redis_url", config.RedisURL)

	// Initialize Redis client for database operations
	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		zapLogger.Fatal("Failed to connect to Redis", "error", err)
	}

	// Select Database implementation by config via shared factory
	dbBackend := getEnv("DB_TYPE", "redis")
	sharedDatabase, err := factory.NewDatabase(factory.DatabaseDeps{
		Backend:     dbBackend,
		RedisClient: redisClient,
		Logger:      &DatabaseLoggerAdapter{logger: logger},
		Config: &interfaces.DatabaseConfig{
			Type:       dbBackend,
			Host:       getEnv("REDIS_HOST", "localhost"),
			Port:       6379,
			TenantMode: true,
		},
	})
	if err != nil {
		zapLogger.Fatal("Failed to create Database", "error", err)
	}

	// Select EventStream implementation by config via shared factory
	eventBackend := getEnv("EVENT_STREAM_TYPE", "redis")
	eventStream, err := factory.NewEventStream(factory.EventStreamDeps{
		Backend:     eventBackend,
		RedisClient: redisClient,
		Logger:      &LoggerAdapter{logger: logger},
	})
	if err != nil {
		zapLogger.Fatal("Failed to create EventStream", "error", err)
	}

	// sharedDatabase already selected above via config

	// Create adapter to bridge shared database to worker's interface
	database := adapters.NewSharedDatabaseAdapter(sharedDatabase, zapLogger)

	// Initialize protobuf serializer
	serializer := serializer.NewProtobufSerializer()

	// Initialize metrics (placeholder)
	metrics := &adapters.NoOpMetrics{}

	// eventStream already selected above via config

	// Initialize Serverless Workflow engine with event stream for wait tasks
	workflowEngine := engine.NewServerlessWorkflowEngine(zapLogger, eventStream)

	// Initialize workflow processor with shared event stream and database
	workflowProcessor := processor.NewWorkflowProcessor(
		eventStream,
		database,
		sharedDatabase, // Pass shared database for execution updates
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
	RedisURL      string
	RedisPassword string
	LogLevel      string
	MetricsPort   string
	HealthPort    string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
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
