package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/flunq-io/executor/internal/processor"
	"github.com/flunq-io/shared/pkg/factory"
)

// LoggerAdapter adapts zap.Logger to the shared eventstreaming.Logger interface
type LoggerAdapter struct {
	logger *zap.Logger
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
	fmt.Println("🔥🔥🔥 STARTING EXECUTOR WITH PARAMETER EVALUATION FIX 🔥🔥🔥")

	// Load configuration first
	config := loadConfig()

	// Initialize logger with proper level
	var zapLogger *zap.Logger
	var err error

	if config.LogLevel == "debug" {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapConfig := zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(parseLogLevel(config.LogLevel))
		zapLogger, err = zapConfig.Build()
	}

	if err != nil {
		panic(err)
	}
	defer zapLogger.Sync()

	zapLogger.Info("Starting Executor Service",
		zap.String("version", "1.0.0"),
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

	// Create logger adapter and select EventStream via factory
	loggerAdapter := &LoggerAdapter{logger: zapLogger}
	eventBackend := getEnv("EVENT_STREAM_TYPE", "redis")
	eventStream, err := factory.NewEventStream(factory.EventStreamDeps{
		Backend:     eventBackend,
		RedisClient: redisClient,
		Logger:      loggerAdapter,
	})
	if err != nil {
		zapLogger.Fatal("Failed to create EventStream", zap.Error(err))
	}

	// Initialize task processor with new SDL-compliant executor registry
	// The registry automatically includes all executors including the new try task executor
	taskProcessor := processor.NewTaskProcessor(
		eventStream,
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
		LogLevel:      getEnv("LOG_LEVEL", "warn"),
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

// parseLogLevel converts string log level to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
