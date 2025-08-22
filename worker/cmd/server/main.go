package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
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
	// Load configuration first
	config := loadConfig()

	// Initialize logger with configurable level
	var logger *zap.Logger
	var err error

	if config.LogLevel == "debug" {
		logger, err = zap.NewDevelopment()
	} else {
		// Create production config with configurable level
		cfg := zap.NewProductionConfig()
		switch config.LogLevel {
		case "error":
			cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		case "warn":
			cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
		case "info":
			cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		default:
			cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel) // Default to WARN to reduce noise
		}
		logger, err = cfg.Build()
	}

	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	zapLogger := &adapters.ZapLogger{Logger: logger}

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
	// Redis Configuration
	RedisURL      string
	RedisHost     string
	RedisPassword string

	// Logging Configuration
	LogLevel string

	// Service Ports
	MetricsPort string
	HealthPort  string

	// Worker Configuration
	WorkerConcurrency    int
	WorkerConsumerGroup  string
	WorkerStreamBatch    int
	WorkerStreamBlock    string
	WorkerReclaimEnabled bool

	// EventStore Configuration
	EventStoreType string

	// Database Configuration
	DBType string

	// Timer Service Configuration
	TimerServiceEnabled   bool
	TimerServicePrecision string

	// Workflow Engine Configuration
	WorkflowEngineType     string
	WorkflowTimeoutSeconds int
	WorkflowMaxRetries     int

	// Event Processing Configuration
	EventBatchSize         int
	EventProcessingTimeout string
	EventRetryAttempts     int
	EventRetryDelayMs      int
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		// Redis Configuration
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Logging Configuration
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Service Ports
		MetricsPort: getEnv("METRICS_PORT", "9090"),
		HealthPort:  getEnv("HEALTH_PORT", "8082"),

		// Worker Configuration
		WorkerConcurrency:    getEnvInt("WORKER_CONCURRENCY", 4),
		WorkerConsumerGroup:  getEnv("WORKER_CONSUMER_GROUP", "worker-service"),
		WorkerStreamBatch:    getEnvInt("WORKER_STREAM_BATCH", 10),
		WorkerStreamBlock:    getEnv("WORKER_STREAM_BLOCK", "1s"),
		WorkerReclaimEnabled: getEnvBool("WORKER_RECLAIM_ENABLED", false),

		// EventStore Configuration
		EventStoreType: getEnv("EVENTSTORE_TYPE", "redis"),

		// Database Configuration
		DBType: getEnv("DB_TYPE", "redis"),

		// Timer Service Configuration
		TimerServiceEnabled:   getEnvBool("TIMER_SERVICE_ENABLED", true),
		TimerServicePrecision: getEnv("TIMER_SERVICE_PRECISION", "1s"),

		// Workflow Engine Configuration
		WorkflowEngineType:     getEnv("WORKFLOW_ENGINE_TYPE", "serverless"),
		WorkflowTimeoutSeconds: getEnvInt("WORKFLOW_TIMEOUT_SECONDS", 3600),
		WorkflowMaxRetries:     getEnvInt("WORKFLOW_MAX_RETRIES", 3),

		// Event Processing Configuration
		EventBatchSize:         getEnvInt("EVENT_BATCH_SIZE", 10),
		EventProcessingTimeout: getEnv("EVENT_PROCESSING_TIMEOUT", "30s"),
		EventRetryAttempts:     getEnvInt("EVENT_RETRY_ATTEMPTS", 3),
		EventRetryDelayMs:      getEnvInt("EVENT_RETRY_DELAY_MS", 200),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as integer with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets an environment variable as boolean with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
